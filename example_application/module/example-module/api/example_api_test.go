package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/responsevo"
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

func TestExampleHandler_FiberBindsRequestsPropagatesContextAndSetsStatus(t *testing.T) {
	caller := context.WithValue(context.Background(), contextKey("caller"), "fiber")
	fake := &fakeExampleUseCase{}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.SetUserContext(caller)
		return c.Next()
	})
	registerExampleRoutes(app, NewExampleHandler(nil, fake))

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		assertCall func(*testing.T)
	}{
		{
			name: "create", method: http.MethodPost, path: "/examples",
			body:       `{"name":"alpha","description":"first","status":"active","tags":["go"]}`,
			wantStatus: http.StatusCreated,
			assertCall: func(t *testing.T) {
				want := requestvo.CreateExampleReqVo{Name: "alpha", Description: "first", Status: "active", Tags: []string{"go"}}
				if fake.operation != "create" || !reflect.DeepEqual(fake.createReq, want) {
					t.Fatalf("Create call = %q %#v, want %#v", fake.operation, fake.createReq, want)
				}
			},
		},
		{
			name: "get", method: http.MethodGet, path: "/examples/example-42",
			wantStatus: http.StatusOK,
			assertCall: func(t *testing.T) {
				if fake.operation != "get" || fake.id != "example-42" {
					t.Fatalf("Get call = %q id %q", fake.operation, fake.id)
				}
			},
		},
		{
			name: "list", method: http.MethodGet, path: "/examples?page=2&page_size=7&status=archived",
			wantStatus: http.StatusOK,
			assertCall: func(t *testing.T) {
				want := requestvo.ListExamplesReqVo{Page: 2, PageSize: 7, Status: "archived"}
				if fake.operation != "list" || fake.listReq != want {
					t.Fatalf("List call = %q %#v, want %#v", fake.operation, fake.listReq, want)
				}
			},
		},
		{
			name: "update", method: http.MethodPut, path: "/examples/example-42",
			body:       `{"name":"beta","tags":["fiber"]}`,
			wantStatus: http.StatusOK,
			assertCall: func(t *testing.T) {
				if fake.operation != "update" || fake.id != "example-42" {
					t.Fatalf("Update call = %q id %q", fake.operation, fake.id)
				}
				if fake.updateReq.Name == nil || *fake.updateReq.Name != "beta" ||
					fake.updateReq.Tags == nil || !reflect.DeepEqual(*fake.updateReq.Tags, []string{"fiber"}) {
					t.Fatalf("Update request = %#v", fake.updateReq)
				}
			},
		},
		{
			name: "delete", method: http.MethodDelete, path: "/examples/example-42",
			wantStatus: http.StatusNoContent,
			assertCall: func(t *testing.T) {
				if fake.operation != "delete" || fake.id != "example-42" {
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
			_, _ = io.Copy(io.Discard, resp.Body)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
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
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/examples", body: `{"name":"alpha"}`},
		{name: "get", method: http.MethodGet, path: "/examples/id"},
		{name: "list", method: http.MethodGet, path: "/examples"},
		{name: "update", method: http.MethodPut, path: "/examples/id", body: `{"name":"beta"}`},
		{name: "delete", method: http.MethodDelete, path: "/examples/id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var forwarded error
			app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
				forwarded = err
				return c.SendStatus(http.StatusInternalServerError)
			}})
			registerExampleRoutes(app, NewExampleHandler(nil, &fakeExampleUseCase{err: wantErr}))
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
