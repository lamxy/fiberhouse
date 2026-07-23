package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/database/dbmongo"
	fiberhouseconstant "github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/example_application"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/responsevo"
	appexceptions "github.com/lamxy/fiberhouse/example_application/providers/exceptions"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type contextKey string

type fakeExampleUseCase struct {
	operation string
	ctx       context.Context
	id        string
	createReq requestvo.CreateExampleReqVo
	listReq   requestvo.ListExamplesReqVo
	updateReq requestvo.UpdateExampleReqVo
	err       error
}

func newExampleHandlerTestContext(t *testing.T) fiberhouse.IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	logger := zerolog.Nop()
	ctx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	exceptionKey := fiberhouseconstant.RegisterKeyPrefix + "exceptions"
	ctx.GetContainer().Unregister(exceptionKey)
	if !ctx.GetContainer().Register(exceptionKey, func() (interface{}, error) {
		return appexceptions.GetGlobalExceptions(), nil
	}) {
		t.Fatalf("register test exceptions")
	}
	t.Cleanup(func() {
		ctx.GetContainer().Unregister(exceptionKey)
	})
	return ctx
}

func (f *fakeExampleUseCase) Create(ctx context.Context, req requestvo.CreateExampleReqVo) (*responsevo.ExampleRespVo, error) {
	f.operation, f.ctx, f.createReq = "create", ctx, req
	return &responsevo.ExampleRespVo{ID: "created"}, f.err
}

func (f *fakeExampleUseCase) Get(ctx context.Context, id string) (*responsevo.ExampleRespVo, error) {
	f.operation, f.ctx, f.id = "get", ctx, id
	return &responsevo.ExampleRespVo{ID: id}, f.err
}

func (f *fakeExampleUseCase) List(ctx context.Context, req requestvo.ListExamplesReqVo) (*responsevo.ExampleListRespVo, error) {
	f.operation, f.ctx, f.listReq = "list", ctx, req
	return &responsevo.ExampleListRespVo{}, f.err
}

func (f *fakeExampleUseCase) Update(ctx context.Context, id string, req requestvo.UpdateExampleReqVo) (*responsevo.ExampleRespVo, error) {
	f.operation, f.ctx, f.id, f.updateReq = "update", ctx, id, req
	return &responsevo.ExampleRespVo{ID: id}, f.err
}

func (f *fakeExampleUseCase) Delete(ctx context.Context, id string) error {
	f.operation, f.ctx, f.id = "delete", ctx, id
	return f.err
}

func TestRegisterExampleRoutes_GinCRUDContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerExampleRoutes(router, NewExampleHandler(nil, &fakeExampleUseCase{}))

	var got []string
	for _, route := range router.Routes() {
		if route.Path == "/examples" || route.Path == "/examples/:id" {
			got = append(got, route.Method+" "+route.Path)
		}
	}
	sort.Strings(got)
	want := []string{
		"DELETE /examples/:id",
		"GET /examples",
		"GET /examples/:id",
		"POST /examples",
		"PUT /examples/:id",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registered CRUD routes = %v, want %v", got, want)
	}
}

func TestRegisterRouteHandlers_GinKeepsDemonstrationsOutsideCRUDRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	ctx := newExampleHandlerTestContext(t)
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("create inert Mongo client: %v", err)
	}
	ctx.GetContainer().Unregister(example_application.KEY_MONGODB)
	if !ctx.GetContainer().Register(example_application.KEY_MONGODB, func() (interface{}, error) {
		return &dbmongo.MongoDb{Client: client, Ctx: ctx}, nil
	}) {
		t.Fatal("register test Mongo dependency")
	}
	t.Cleanup(func() {
		ctx.GetContainer().Unregister(example_application.KEY_MONGODB)
		_ = client.Disconnect(context.Background())
	})
	RegisterRouteHandlers(ctx, router)

	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
		if strings.HasPrefix(route.Path, "/examples/gin/common") {
			t.Fatalf("demonstration route mounted under CRUD path: %s %s", route.Method, route.Path)
		}
	}
	for _, route := range []string{
		"GET /gin/common/test/get-instance",
		"GET /gin/common/test/get-must-instance",
		"GET /gin/common/test/get-must-instance-failed",
	} {
		if !registered[route] {
			t.Fatalf("production route %q not registered", route)
		}
	}
}

func TestExampleHandler_GinBindsRequestsPropagatesContextAndSetsStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	caller := context.WithValue(context.Background(), contextKey("caller"), "gin")
	fake := &fakeExampleUseCase{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(caller)
		c.Next()
	})
	registerExampleRoutes(router, NewExampleHandler(newExampleHandlerTestContext(t), fake))
	const validID = "0123456789abcdef01234567"

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   bool
		assertCall func(*testing.T)
	}{
		{
			name: "create", method: http.MethodPost, path: "/examples",
			body:       `{"name":"alpha","description":"first","status":"active","tags":["go"]}`,
			wantStatus: http.StatusCreated,
			wantBody:   true,
			assertCall: func(t *testing.T) {
				want := requestvo.CreateExampleReqVo{Name: "alpha", Description: "first", Status: "active", Tags: []string{"go"}}
				if fake.operation != "create" || !reflect.DeepEqual(fake.createReq, want) {
					t.Fatalf("Create call = %q %#v, want %#v", fake.operation, fake.createReq, want)
				}
			},
		},
		{
			name: "get", method: http.MethodGet, path: "/examples/" + validID,
			wantStatus: http.StatusOK,
			wantBody:   true,
			assertCall: func(t *testing.T) {
				if fake.operation != "get" || fake.id != validID {
					t.Fatalf("Get call = %q id %q", fake.operation, fake.id)
				}
			},
		},
		{
			name: "list", method: http.MethodGet, path: "/examples?page=2&page_size=7&status=archived",
			wantStatus: http.StatusOK,
			wantBody:   true,
			assertCall: func(t *testing.T) {
				want := requestvo.ListExamplesReqVo{Page: 2, PageSize: 7, Status: "archived"}
				if fake.operation != "list" || fake.listReq != want {
					t.Fatalf("List call = %q %#v, want %#v", fake.operation, fake.listReq, want)
				}
			},
		},
		{
			name: "update", method: http.MethodPut, path: "/examples/" + validID,
			body:       `{"name":"beta","tags":["gin"]}`,
			wantStatus: http.StatusOK,
			wantBody:   true,
			assertCall: func(t *testing.T) {
				if fake.operation != "update" || fake.id != validID {
					t.Fatalf("Update call = %q id %q", fake.operation, fake.id)
				}
				if fake.updateReq.Name == nil || *fake.updateReq.Name != "beta" ||
					fake.updateReq.Tags == nil || !reflect.DeepEqual(*fake.updateReq.Tags, []string{"gin"}) {
					t.Fatalf("Update request = %#v", fake.updateReq)
				}
			},
		},
		{
			name: "delete", method: http.MethodDelete, path: "/examples/" + validID,
			wantStatus: http.StatusNoContent,
			assertCall: func(t *testing.T) {
				if fake.operation != "delete" || fake.id != validID {
					t.Fatalf("Delete call = %q id %q", fake.operation, fake.id)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake.operation, fake.ctx, fake.id = "", nil, ""
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if tt.wantBody {
				var envelope map[string]interface{}
				if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
					t.Fatalf("decode response envelope: %v; body = %s", err, recorder.Body.String())
				}
				if envelope["code"] != float64(0) || envelope["msg"] != "ok" {
					t.Fatalf("response envelope = %#v", envelope)
				}
				if _, ok := envelope["data"]; !ok {
					t.Fatalf("response envelope missing data: %#v", envelope)
				}
			} else if recorder.Body.Len() != 0 {
				t.Fatalf("204 response body = %q, want empty", recorder.Body.String())
			}
			if fake.ctx != caller || fake.ctx.Value(contextKey("caller")) != "gin" {
				t.Fatalf("service context = %#v, want caller context", fake.ctx)
			}
			tt.assertCall(t)
		})
	}
}

func TestExampleHandler_GinForwardsUseCaseErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	wantErr := errors.New("use case failed")
	const validID = "0123456789abcdef01234567"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/examples", body: `{"name":"alpha"}`},
		{name: "get", method: http.MethodGet, path: "/examples/" + validID},
		{name: "list", method: http.MethodGet, path: "/examples"},
		{name: "update", method: http.MethodPut, path: "/examples/" + validID, body: `{"name":"beta"}`},
		{name: "delete", method: http.MethodDelete, path: "/examples/" + validID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var forwarded error
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Next()
				if last := c.Errors.Last(); last != nil {
					forwarded = last.Err
					c.Status(http.StatusInternalServerError)
				}
			})
			registerExampleRoutes(router, NewExampleHandler(newExampleHandlerTestContext(t), &fakeExampleUseCase{err: wantErr}))
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if !errors.Is(forwarded, wantErr) {
				t.Fatalf("forwarded error = %v, want %v", forwarded, wantErr)
			}
		})
	}
}

func TestExampleHandler_GinRejectsInvalidInputBeforeUseCase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const validID = "0123456789abcdef01234567"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/examples", body: `{"name":""}`},
		{name: "get path id", method: http.MethodGet, path: "/examples/bad-id"},
		{name: "list", method: http.MethodGet, path: "/examples?page_size=1000"},
		{name: "update body", method: http.MethodPut, path: "/examples/" + validID, body: `{"status":"unsupported"}`},
		{name: "update path id", method: http.MethodPut, path: "/examples/bad-id", body: `{"name":"beta"}`},
		{name: "delete path id", method: http.MethodDelete, path: "/examples/bad-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeExampleUseCase{}
			var forwarded error
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Next()
				if last := c.Errors.Last(); last != nil {
					forwarded = last.Err
					c.Status(http.StatusUnprocessableEntity)
				}
			})
			registerExampleRoutes(router, NewExampleHandler(newExampleHandlerTestContext(t), fake))
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if forwarded == nil {
				t.Fatal("validation error was not forwarded")
			}
			if fake.operation != "" {
				t.Fatalf("use case called for invalid input: %s", fake.operation)
			}
		})
	}
}

func TestExampleHandler_GinForwardsBindingErrorsBeforeUseCase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create body", method: http.MethodPost, path: "/examples", body: `{"name":`},
		{name: "update body", method: http.MethodPut, path: "/examples/0123456789abcdef01234567", body: `{"name":`},
		{name: "list query", method: http.MethodGet, path: "/examples?page=not-an-int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeExampleUseCase{}
			var forwarded error
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Next()
				if last := c.Errors.Last(); last != nil {
					forwarded = last.Err
					c.Status(http.StatusBadRequest)
				}
			})
			registerExampleRoutes(router, NewExampleHandler(newExampleHandlerTestContext(t), fake))
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if forwarded == nil {
				t.Fatal("binding error was not forwarded")
			}
			if fake.operation != "" {
				t.Fatalf("use case called after binding error: %s", fake.operation)
			}
		})
	}
}
