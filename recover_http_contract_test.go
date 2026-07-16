package fiberhouse

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	adaptorerrorhandler "github.com/lamxy/fiberhouse/adaptor/errorhandler"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type task5RecoverCase struct {
	name          string
	kind          string
	debugMode     bool
	wantStatus    int
	wantCode      int
	wantMessage   string
	wantData      interface{}
	wantDataShown bool
	wantStack     int32
}

func task5RecoverCases() []task5RecoverCase {
	return []task5RecoverCase{
		{name: "panic error production", kind: "panic-error", wantStatus: 500, wantCode: constant.UnknownErrCode, wantMessage: constant.UnknownErrMsg, wantStack: 1},
		{name: "panic error debug", kind: "panic-error", debugMode: true, wantStatus: 500, wantCode: constant.UnknownErrCode, wantMessage: "panic detail", wantStack: 1},
		{name: "panic exception hides data", kind: "panic-exception", wantStatus: 400, wantCode: 4101, wantMessage: "business rule", wantStack: 1},
		{name: "panic exception debug shows data", kind: "panic-exception", debugMode: true, wantStatus: 400, wantCode: 4101, wantMessage: "business rule", wantData: "business data", wantDataShown: true, wantStack: 1},
		{name: "panic validation always shows data", kind: "panic-validation", wantStatus: 400, wantCode: 4201, wantMessage: "invalid input", wantData: "field data", wantDataShown: true, wantStack: 1},
		{name: "runtime panic production", kind: "panic-runtime", wantStatus: 500, wantCode: constant.UnknownErrCode, wantMessage: "NullPointerException", wantStack: 1},
		{name: "runtime panic debug", kind: "panic-runtime", debugMode: true, wantStatus: 500, wantCode: constant.UnknownErrCode, wantMessage: "RuntimeError", wantDataShown: true, wantStack: 1},
		{name: "string panic production", kind: "panic-string", wantStatus: 500, wantCode: constant.UnknownErrCode, wantMessage: constant.UnknownErrMsg, wantStack: 1},
		{name: "string panic debug", kind: "panic-string", debugMode: true, wantStatus: 500, wantCode: constant.UnknownErrCode, wantMessage: constant.UnknownErrMsg, wantDataShown: true, wantStack: 1},
		{name: "next bypass", kind: "next", wantStatus: 204},
		{name: "not found", kind: "http-404", wantStatus: 404, wantCode: 404, wantMessage: "route missing"},
		{name: "method not allowed", kind: "http-405", wantStatus: 405, wantCode: 405, wantMessage: "method rejected"},
		{name: "ordinary returned error", kind: "returned-error", wantStatus: 500, wantCode: constant.UnknownErrCode, wantMessage: constant.UnknownErrMsg},
	}
}

func task5RunFiberContract(t *testing.T, testCase task5RecoverCase, cfg RecoverConfig, handler *ErrorHandler, downstream *atomic.Int32) (*http.Response, adaptorctx.ICoreContext) {
	t.Helper()
	var middlewareContext adaptorctx.ICoreContext
	cfg.Next = func(ctx adaptorctx.ICoreContext) bool {
		middlewareContext = ctx
		return testCase.kind == "next"
	}
	app := fiber.New(fiber.Config{ErrorHandler: adaptorerrorhandler.FiberErrorHandler(handler.ErrorHandler)})
	app.Use(NewFiberRecovery(handler.AppCtx).RecoverPanic(cfg).(fiber.Handler))
	app.Get("/contract", func(c *fiber.Ctx) error {
		downstream.Add(1)
		switch testCase.kind {
		case "panic-error":
			panic(errors.New("panic detail"))
		case "panic-exception":
			panic(exception.New(4101, "business rule", "business data"))
		case "panic-validation":
			panic(exception.NewVE(4201, "invalid input", "field data"))
		case "panic-runtime":
			var value *int
			_ = *value
		case "panic-string":
			panic("string detail")
		case "http-404":
			return fiber.NewError(http.StatusNotFound, "route missing")
		case "http-405":
			return fiber.NewError(http.StatusMethodNotAllowed, "method rejected")
		case "returned-error":
			return errors.New("returned detail")
		case "next":
			return c.SendStatus(http.StatusNoContent)
		default:
			return nil
		}
		return nil
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/contract", nil))
	require.NoError(t, err)
	return response, middlewareContext
}

func task5RunGinContract(t *testing.T, testCase task5RecoverCase, cfg RecoverConfig, handler *ErrorHandler, downstream *atomic.Int32) (*httptest.ResponseRecorder, adaptorctx.ICoreContext) {
	t.Helper()
	preserveTask4GinMode(t)
	gin.SetMode(gin.TestMode)
	var middlewareContext adaptorctx.ICoreContext
	cfg.Next = func(ctx adaptorctx.ICoreContext) bool {
		middlewareContext = ctx
		return testCase.kind == "next"
	}
	engine := gin.New()
	recoverHandler := NewGinRecovery(handler.AppCtx).RecoverPanic(cfg).(func(*gin.Context))
	engine.Use(gin.HandlerFunc(recoverHandler))
	engine.Use(adaptorerrorhandler.GinErrorHandler(handler.ErrorHandler))
	engine.GET("/contract", func(c *gin.Context) {
		downstream.Add(1)
		switch testCase.kind {
		case "panic-error":
			panic(errors.New("panic detail"))
		case "panic-exception":
			panic(exception.New(4101, "business rule", "business data"))
		case "panic-validation":
			panic(exception.NewVE(4201, "invalid input", "field data"))
		case "panic-runtime":
			var value *int
			_ = *value
		case "panic-string":
			panic("string detail")
		case "http-404":
			c.Set("error", fiber.NewError(http.StatusNotFound, "route missing"))
		case "http-405":
			c.Set("error", fiber.NewError(http.StatusMethodNotAllowed, "method rejected"))
		case "returned-error":
			c.Set("error", errors.New("returned detail"))
		case "next":
			c.Status(http.StatusNoContent)
		}
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/contract", nil))
	return recorder, middlewareContext
}

func TestRecoverHTTPContract_FiberAndGin(t *testing.T) {
	for _, testCase := range task5RecoverCases() {
		t.Run(testCase.name, func(t *testing.T) {
			for _, core := range []string{"fiber", "gin"} {
				t.Run(core, func(t *testing.T) {
					ctx := newTask5AppContext(t, testCase.debugMode, false)
					installTask5ResponseManager(t, ctx)
					installTask5Exceptions(t, ctx)
					var stackCalls atomic.Int32
					cfg := RecoverConfig{
						EnableStackTrace: true,
						StackTraceHandler: func(adaptorctx.ICoreContext, interface{}) {
							stackCalls.Add(1)
						},
						JsonCodec: json.Marshal,
						DebugMode: testCase.debugMode,
					}
					var downstream atomic.Int32
					var status int
					var contentType string
					var body []byte
					var middlewareContext adaptorctx.ICoreContext
					switch core {
					case "fiber":
						handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))
						response, captured := task5RunFiberContract(t, testCase, cfg, handler, &downstream)
						defer response.Body.Close()
						var err error
						body, err = io.ReadAll(response.Body)
						require.NoError(t, err)
						status, contentType, middlewareContext = response.StatusCode, response.Header.Get("Content-Type"), captured
					case "gin":
						handler := newTask5ErrorHandler(ctx, NewGinRecovery(ctx))
						recorder, captured := task5RunGinContract(t, testCase, cfg, handler, &downstream)
						status, contentType, body, middlewareContext = recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.Bytes(), captured
					}

					assert.Equal(t, testCase.wantStatus, status)
					assert.Equal(t, int32(1), downstream.Load(), "downstream must execute exactly once")
					assert.Equal(t, testCase.wantStack, stackCalls.Load())
					require.NotNil(t, middlewareContext)
					assert.Nil(t, middlewareContext.GetCtx(), "middleware wrapper must be released on normal, bypass, error, and panic paths")
					if testCase.wantStatus == http.StatusNoContent {
						assert.Empty(t, body)
						return
					}
					assert.Contains(t, contentType, "application/json")
					var envelope map[string]interface{}
					require.NoError(t, json.Unmarshal(body, &envelope), "body: %s", body)
					assert.EqualValues(t, testCase.wantCode, envelope["code"])
					assert.Equal(t, testCase.wantMessage, envelope["msg"])
					if testCase.wantDataShown {
						assert.NotNil(t, envelope["data"])
						if testCase.wantData != nil {
							assert.Equal(t, testCase.wantData, envelope["data"])
						}
					} else {
						assert.Nil(t, envelope["data"])
					}
				})
			}
		})
	}
}

func TestRecoverHTTPContract_RealRouterNotFoundAndMethodMismatch(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		method     string
		target     string
		wantStatus int
	}{
		{name: "missing route", method: http.MethodGet, target: "/missing", wantStatus: http.StatusNotFound},
		{name: "method mismatch", method: http.MethodPost, target: "/registered", wantStatus: http.StatusMethodNotAllowed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			for _, core := range []string{"fiber", "gin"} {
				t.Run(core, func(t *testing.T) {
					ctx := newTask5AppContext(t, false, false)
					installTask5ResponseManager(t, ctx)
					installTask5Exceptions(t, ctx)
					cfg := RecoverConfig{JsonCodec: json.Marshal}
					var status int

					switch core {
					case "fiber":
						handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))
						app := fiber.New(fiber.Config{ErrorHandler: adaptorerrorhandler.FiberErrorHandler(handler.ErrorHandler)})
						app.Use(NewFiberRecovery(ctx).RecoverPanic(cfg).(fiber.Handler))
						app.Get("/registered", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
						response, err := app.Test(httptest.NewRequest(testCase.method, testCase.target, nil))
						require.NoError(t, err)
						defer response.Body.Close()
						status = response.StatusCode
					case "gin":
						preserveTask4GinMode(t)
						gin.SetMode(gin.TestMode)
						engine := gin.New()
						engine.HandleMethodNotAllowed = true
						engine.Use(gin.HandlerFunc(NewGinRecovery(ctx).RecoverPanic(cfg).(func(*gin.Context))))
						engine.GET("/registered", func(c *gin.Context) { c.Status(http.StatusNoContent) })
						recorder := httptest.NewRecorder()
						engine.ServeHTTP(recorder, httptest.NewRequest(testCase.method, testCase.target, nil))
						status = recorder.Code
					}

					assert.Equal(t, testCase.wantStatus, status)
				})
			}
		})
	}
}
