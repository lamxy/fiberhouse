package fiberhouse

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

type task5RecoverManager struct {
	IProviderManager
	recovery IRecover
}

func (m *task5RecoverManager) LoadProvider(...ProviderLoadFunc) (any, error) {
	return m.recovery, nil
}

func newTask5ErrorHandler(ctx IApplicationContext, recovery IRecover) *ErrorHandler {
	return &ErrorHandler{
		AppCtx: ctx,
		recoverManager: &task5RecoverManager{
			IProviderManager: NewProviderManager(ctx),
			recovery:         recovery,
		},
	}
}

func installTask5Exceptions(t *testing.T, ctx IApplicationContext) {
	t.Helper()
	key := constant.RegisterKeyPrefix + "exceptions"
	wasRegistered := ctx.GetContainer().IsRegistered(key)
	var previous interface{}
	if wasRegistered {
		var err error
		previous, err = ctx.GetContainer().Get(key)
		require.NoError(t, err)
	}
	ctx.GetContainer().Unregister(key)
	require.True(t, ctx.GetContainer().Register(key, func() (interface{}, error) {
		return exception.ExceptionMap{
			"UnknownError": {Code: constant.UnknownErrCode, Msg: constant.UnknownErrMsg},
		}, nil
	}))
	t.Cleanup(func() {
		ctx.GetContainer().Unregister(key)
		if wasRegistered {
			require.True(t, ctx.GetContainer().Register(key, func() (interface{}, error) {
				return previous, nil
			}))
		}
	})
}

func TestErrorHandler_PreservesFiberHTTPStatusAcrossCores(t *testing.T) {
	for _, core := range []string{"fiber", "gin"} {
		t.Run(core, func(t *testing.T) {
			ctx := newTask5AppContext(t, false, false)
			installTask5ResponseManager(t, ctx)
			installTask5Exceptions(t, ctx)

			var result task5HTTPResponse
			switch core {
			case "fiber":
				recovery := NewFiberRecovery(ctx)
				handler := newTask5ErrorHandler(ctx, recovery)
				app := fiber.New(fiber.Config{ErrorHandler: adaptorerrorhandler.FiberErrorHandler(handler.ErrorHandler)})
				app.Get("/not-found", func(*fiber.Ctx) error {
					return fiber.NewError(http.StatusNotFound, "route missing")
				})
				response, err := app.Test(httptest.NewRequest(http.MethodGet, "/not-found", nil))
				require.NoError(t, err)
				defer response.Body.Close()
				var envelope map[string]interface{}
				require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
				result.status = response.StatusCode
				assert.EqualValues(t, http.StatusNotFound, envelope["code"])
				assert.Equal(t, "route missing", envelope["msg"])
			case "gin":
				preserveTask4GinMode(t)
				gin.SetMode(gin.TestMode)
				recovery := NewGinRecovery(ctx)
				handler := newTask5ErrorHandler(ctx, recovery)
				engine := gin.New()
				engine.Use(adaptorerrorhandler.GinErrorHandler(handler.ErrorHandler))
				engine.GET("/not-found", func(c *gin.Context) {
					c.Set("error", fiber.NewError(http.StatusNotFound, "route missing"))
				})
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/not-found", nil))
				var envelope map[string]interface{}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
				result.status = recorder.Code
				assert.EqualValues(t, http.StatusNotFound, envelope["code"])
				assert.Equal(t, "route missing", envelope["msg"])
			}
			assert.Equal(t, http.StatusNotFound, result.status)
		})
	}
}

func TestErrorHandler_OrdinaryErrorDebugVisibility(t *testing.T) {
	for _, debugMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "production", true: "debug"}[debugMode], func(t *testing.T) {
			ctx := newTask5AppContext(t, debugMode, false)
			installTask5ResponseManager(t, ctx)
			installTask5Exceptions(t, ctx)
			recovery := NewFiberRecovery(ctx)
			handler := newTask5ErrorHandler(ctx, recovery)
			app := fiber.New(fiber.Config{ErrorHandler: adaptorerrorhandler.FiberErrorHandler(handler.ErrorHandler)})
			app.Get("/error", func(*fiber.Ctx) error { return errors.New("database detail") })

			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/error", nil))
			require.NoError(t, err)
			defer response.Body.Close()
			var envelope map[string]interface{}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			assert.EqualValues(t, constant.UnknownErrCode, envelope["code"])
			assert.Equal(t, constant.UnknownErrMsg, envelope["msg"])
			if debugMode {
				assert.Equal(t, "database detail", envelope["data"])
			} else {
				assert.Nil(t, envelope["data"])
			}
		})
	}
}

func TestErrorHandler_GetJsonIndentAndErrorStack(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	assert.Nil(t, GetJsonIndent(ctx, "", ctx.GetLogger(), json.Marshal, "trace"))
	assert.NotEmpty(t, ErrorStack())

	stack := "goroutine 1 [running]:\nmain.one()\n\t/tmp/main.go:10 +0x1\nmain.two()\n\t/tmp/main.go:20 +0x2"
	encoded := GetJsonIndent(ctx, stack, ctx.GetLogger(), json.Marshal, "trace")
	if encoded != nil {
		assert.True(t, json.Valid(encoded))
	}
}

func TestErrorHandler_LogsFiberErrorValueAndPointer(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))
	app := fiber.New()
	app.Get("/log", func(c *fiber.Ctx) error {
		wrapped := adaptorctx.WithFiberContext(c)
		assert.NotPanics(t, func() {
			handler.DefaultStackTraceHandler(wrapped, fiber.Error{Code: http.StatusNotFound, Message: "value error"})
		})
		pointerError := &fiber.Error{Message: "pointer error"}
		assert.NotPanics(t, func() {
			handler.DefaultStackTraceHandler(wrapped, pointerError)
		})
		assert.Equal(t, http.StatusInternalServerError, pointerError.Code)
		wrapped.(*adaptorctx.FiberContext).Release()
		return c.SendStatus(http.StatusNoContent)
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/log", nil))
	require.NoError(t, err)
	response.Body.Close()
	assert.Equal(t, http.StatusNoContent, response.StatusCode)
}

func TestErrorHandler_TypedNilFiberErrorFallsBackToUnknown(t *testing.T) {
	for _, core := range []string{"fiber", "gin"} {
		t.Run(core, func(t *testing.T) {
			ctx := newTask5AppContext(t, false, false)
			installTask5ResponseManager(t, ctx)
			installTask5Exceptions(t, ctx)
			handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))
			var typedNil *fiber.Error

			var status int
			var body []byte
			switch core {
			case "fiber":
				app := fiber.New(fiber.Config{ErrorHandler: adaptorerrorhandler.FiberErrorHandler(handler.ErrorHandler)})
				app.Get("/typed-nil", func(*fiber.Ctx) error { return typedNil })
				response, err := app.Test(httptest.NewRequest(http.MethodGet, "/typed-nil", nil))
				require.NoError(t, err)
				defer response.Body.Close()
				status = response.StatusCode
				body, err = io.ReadAll(response.Body)
				require.NoError(t, err)
			case "gin":
				preserveTask4GinMode(t)
				gin.SetMode(gin.TestMode)
				engine := gin.New()
				engine.Use(adaptorerrorhandler.GinErrorHandler(handler.ErrorHandler))
				engine.GET("/typed-nil", func(c *gin.Context) { c.Set("error", typedNil) })
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/typed-nil", nil))
				status, body = recorder.Code, recorder.Body.Bytes()
			}

			assert.Equal(t, http.StatusInternalServerError, status)
			var envelope map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &envelope))
			assert.EqualValues(t, constant.UnknownErrCode, envelope["code"])
			assert.Equal(t, constant.UnknownErrMsg, envelope["msg"])
		})
	}
}

func TestErrorHandler_RejectsMissingOrInvalidRecoveryManager(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	withoutManager := &ErrorHandler{AppCtx: ctx}
	assert.PanicsWithValue(t, "Recovery: recover manager is not set", func() {
		withoutManager.RecoverMiddleware()
	})

	invalid := &ErrorHandler{
		AppCtx: ctx,
		recoverManager: &task5RecoverManager{
			IProviderManager: NewProviderManager(ctx),
			recovery:         nil,
		},
	}
	assert.PanicsWithValue(t, "Recovery: loaded recover provider does not implement IRecover", func() {
		invalid.RecoverMiddleware()
	})
}

var _ adaptorctx.ICoreContext = (*task5WrongCoreContext)(nil)
