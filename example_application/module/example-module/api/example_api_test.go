package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	adaptorerrorhandler "github.com/lamxy/fiberhouse/adaptor/errorhandler"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/lamxy/fiberhouse/component/database/dbmongo"
	fiberhouseconstant "github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/example_application"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/responsevo"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	appexceptions "github.com/lamxy/fiberhouse/example_application/providers/exceptions"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type contextKey string

const exampleAPITestCodecKey = "example-api-test-codec"

type exampleTestApplication struct{ fiberhouse.IApplication }

func (*exampleTestApplication) GetFastTrafficCodecKey() string { return exampleAPITestCodecKey }

type exampleTestStarter struct {
	fiberhouse.ApplicationStarter
	application fiberhouse.IApplication
}

func (s *exampleTestStarter) GetApplication() fiberhouse.IApplication { return s.application }

type exampleRecoverManager struct {
	fiberhouse.IProviderManager
	recovery fiberhouse.IRecover
}

func (m *exampleRecoverManager) LoadProvider(...fiberhouse.ProviderLoadFunc) (any, error) {
	return m.recovery, nil
}

type fakeExampleUseCase struct {
	operation string
	ctx       context.Context
	id        string
	createReq requestvo.CreateExampleReqVo
	listReq   requestvo.ListExamplesReqVo
	updateReq requestvo.UpdateExampleReqVo
	err       error
}

func TestExampleHandler_FiberMapsStableDomainErrorsThroughErrorHandler(t *testing.T) {
	ctx := newExampleHandlerTestContext(t)
	errorHandler := fiberhouse.NewErrorHandler(ctx)
	errorHandler.SetRecoverManager(&exampleRecoverManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx),
		recovery:         fiberhouse.NewFiberRecovery(ctx),
	})
	const validID = "0123456789abcdef01234567"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   int
		wantMsg    string
		wantNil    bool
		method     string
		path       string
		body       string
	}{
		{name: "invalid input after whitespace normalization", err: service.ErrInvalidInput, wantStatus: http.StatusBadRequest, wantCode: http.StatusBadRequest, wantMsg: service.ErrInvalidInput.Error(), wantNil: true, method: http.MethodPost, path: "/examples", body: `{"name":"  "}`},
		{name: "invalid id", err: repository.ErrInvalidID, wantStatus: http.StatusBadRequest, wantCode: http.StatusBadRequest, wantMsg: repository.ErrInvalidID.Error(), wantNil: true},
		{name: "not found", err: repository.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: http.StatusNotFound, wantMsg: repository.ErrNotFound.Error(), wantNil: true},
		{name: "conflict", err: repository.ErrConflict, wantStatus: http.StatusConflict, wantCode: http.StatusConflict, wantMsg: repository.ErrConflict.Error(), wantNil: true},
		{name: "unchanged", err: repository.ErrUnchanged, wantStatus: http.StatusConflict, wantCode: http.StatusConflict, wantMsg: repository.ErrUnchanged.Error(), wantNil: true},
		{name: "unknown", err: errors.New("private cause"), wantStatus: http.StatusInternalServerError, wantCode: fiberhouseconstant.UnknownErrCode, wantMsg: fiberhouseconstant.UnknownErrMsg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{
				ErrorHandler: adaptorerrorhandler.FiberErrorHandler(errorHandler.ErrorHandler),
			})
			registerExampleRoutes(app, NewExampleHandler(ctx, &fakeExampleUseCase{err: tt.err}))
			method, path := tt.method, tt.path
			if method == "" {
				method, path = http.MethodGet, "/examples/"+validID
			}
			request := httptest.NewRequest(method, path, strings.NewReader(tt.body))
			if tt.body != "" {
				request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var envelope map[string]interface{}
			if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tt.wantStatus ||
				envelope["code"] != float64(tt.wantCode) ||
				envelope["msg"] != tt.wantMsg ||
				(envelope["data"] == nil) != tt.wantNil {
				t.Fatalf("status/envelope = %d %#v, want %d code/msg/data=%d/%q/nil",
					response.StatusCode, envelope, tt.wantStatus, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func newExampleHandlerTestContext(t *testing.T) fiberhouse.IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	logger := zerolog.Nop()
	ctx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&fiberhouse.BootConfig{})
	ctx.RegisterStarterApp(&exampleTestStarter{application: &exampleTestApplication{}})
	fiberhouse.NewRespInfoPManager(ctx)
	ctx.GetContainer().Unregister(exampleAPITestCodecKey)
	if !ctx.GetContainer().Register(exampleAPITestCodecKey, func() (interface{}, error) {
		return jsoncodec.StdJsonDefault(), nil
	}) {
		t.Fatalf("register test codec")
	}
	exceptionKey := fiberhouseconstant.RegisterKeyPrefix + "exceptions"
	ctx.GetContainer().Unregister(exceptionKey)
	if !ctx.GetContainer().Register(exceptionKey, func() (interface{}, error) {
		return appexceptions.GetGlobalExceptions(), nil
	}) {
		t.Fatalf("register test exceptions")
	}
	t.Cleanup(func() {
		ctx.GetContainer().Unregister(exceptionKey)
		ctx.GetContainer().Unregister(exampleAPITestCodecKey)
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

func TestRegisterExampleRoutes_FiberCRUDContract(t *testing.T) {
	app := fiber.New()
	registerExampleRoutes(app, NewExampleHandler(nil, &fakeExampleUseCase{}))

	var got []string
	for _, route := range app.GetRoutes() {
		if route.Method != http.MethodHead && (route.Path == "/examples" || route.Path == "/examples/:id") {
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

func TestRegisterRouteHandlers_FiberKeepsDemonstrationsOutsideCRUDRoutes(t *testing.T) {
	app := fiber.New()
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
	RegisterRouteHandlers(ctx, app)

	registered := make(map[string]bool)
	for _, route := range app.GetRoutes() {
		if route.Method == http.MethodHead {
			continue
		}
		registered[route.Method+" "+route.Path] = true
		if strings.HasPrefix(route.Path, "/examples/health") || strings.HasPrefix(route.Path, "/examples/common") {
			t.Fatalf("demonstration route mounted under CRUD path: %s %s", route.Method, route.Path)
		}
	}
	for _, route := range []string{
		"GET /health/livez",
		"GET /common/test/get-instance",
		"GET /common/test/get-must-instance",
		"GET /common/test/get-must-instance-failed",
	} {
		if !registered[route] {
			t.Fatalf("production route %q not registered", route)
		}
	}
}

func TestExampleHandler_FiberBindsRequestsPropagatesContextAndSetsStatus(t *testing.T) {
	caller := context.WithValue(context.Background(), contextKey("caller"), "fiber")
	fake := &fakeExampleUseCase{}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.SetUserContext(caller)
		return c.Next()
	})
	registerExampleRoutes(app, NewExampleHandler(newExampleHandlerTestContext(t), fake))
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
			body:       `{"name":"beta","tags":["fiber"]}`,
			wantStatus: http.StatusOK,
			wantBody:   true,
			assertCall: func(t *testing.T) {
				if fake.operation != "update" || fake.id != validID {
					t.Fatalf("Update call = %q id %q", fake.operation, fake.id)
				}
				if fake.updateReq.Name == nil || *fake.updateReq.Name != "beta" ||
					fake.updateReq.Tags == nil || !reflect.DeepEqual(*fake.updateReq.Tags, []string{"fiber"}) {
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
				req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			}
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantBody {
				var envelope map[string]interface{}
				if err := json.Unmarshal(body, &envelope); err != nil {
					t.Fatalf("decode response envelope: %v; body = %s", err, body)
				}
				if envelope["code"] != float64(0) || envelope["msg"] != "ok" {
					t.Fatalf("response envelope = %#v", envelope)
				}
				if _, ok := envelope["data"]; !ok {
					t.Fatalf("response envelope missing data: %#v", envelope)
				}
			} else if len(body) != 0 {
				t.Fatalf("204 response body = %q, want empty", body)
			}
			if fake.ctx != caller || fake.ctx.Value(contextKey("caller")) != "fiber" {
				t.Fatalf("service context = %#v, want caller context", fake.ctx)
			}
			tt.assertCall(t)
		})
	}
}

func TestExampleHandler_FiberForwardsUseCaseErrors(t *testing.T) {
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
			app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
				forwarded = err
				return c.SendStatus(http.StatusInternalServerError)
			}})
			registerExampleRoutes(app, NewExampleHandler(newExampleHandlerTestContext(t), &fakeExampleUseCase{err: wantErr}))
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			}
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if !errors.Is(forwarded, wantErr) {
				t.Fatalf("forwarded error = %v, want %v", forwarded, wantErr)
			}
		})
	}
}

func TestExampleHandler_FiberRejectsInvalidInputBeforeUseCase(t *testing.T) {
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
			app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
				forwarded = err
				return c.SendStatus(http.StatusUnprocessableEntity)
			}})
			registerExampleRoutes(app, NewExampleHandler(newExampleHandlerTestContext(t), fake))
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			}
			if _, err := app.Test(req, -1); err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if forwarded == nil {
				t.Fatal("validation error was not forwarded")
			}
			if fake.operation != "" {
				t.Fatalf("use case called for invalid input: %s", fake.operation)
			}
		})
	}
}

func TestExampleHandler_FiberForwardsBindingErrorsBeforeUseCase(t *testing.T) {
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
			app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
				forwarded = err
				return c.SendStatus(http.StatusBadRequest)
			}})
			registerExampleRoutes(app, NewExampleHandler(newExampleHandlerTestContext(t), fake))
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			}
			if _, err := app.Test(req, -1); err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if forwarded == nil {
				t.Fatal("binding error was not forwarded")
			}
			if fake.operation != "" {
				t.Fatalf("use case called after binding error: %s", fake.operation)
			}
		})
	}
}
