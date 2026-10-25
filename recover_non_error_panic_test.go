package fiberhouse

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTask5AppContextWithLogger 与 newTask5AppContext 相同，但使用调用方提供的 logger 以便断言日志内容。
func newTask5AppContextWithLogger(t *testing.T, logger *zerolog.Logger, printStack bool) IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.recover.debugMode":        false,
		"application.recover.enablePrintStack": printStack,
		"application.recover.enableDebugFlag":  false,
		"application.recover.debugFlag":        "X-Debug",
		"application.recover.debugFlagValue":   "enabled",
		"application.trace.requestID":          "X-Trace-ID",
	}).Initialize()
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(logger))
	ctx.RegisterBootConfig(&BootConfig{})
	ctx.RegisterStarterApp(&task5Starter{application: &task5Application{}})
	ctx.GetContainer().Unregister(task5CodecKey)
	require.True(t, ctx.GetContainer().Register(task5CodecKey, func() (interface{}, error) {
		return jsoncodec.StdJsonDefault(), nil
	}))
	t.Cleanup(func() { ctx.GetContainer().Unregister(task5CodecKey) })
	return ctx
}

// TestErrorHandler_LogsNonErrorPanicValue 校验 panic("...") 这类非 error 的 panic 值会被记录，
// 而不是因类型 switch 没有对应分支被静默丢弃。
func TestErrorHandler_LogsNonErrorPanicValue(t *testing.T) {
	for _, printStack := range []bool{false, true} {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)
		ctx := newTask5AppContextWithLogger(t, &logger, printStack)
		handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))

		app := fiber.New()
		app.Get("/log", func(c *fiber.Ctx) error {
			wrapped := adaptorctx.WithFiberContext(c)
			handler.DefaultStackTraceHandler(wrapped, "string detail")
			wrapped.(*adaptorctx.FiberContext).Release()
			return c.SendStatus(http.StatusNoContent)
		})
		response, err := app.Test(httptest.NewRequest(http.MethodGet, "/log", nil))
		require.NoError(t, err)
		response.Body.Close()

		out := buf.String()
		assert.Contains(t, out, `"level":"error"`, "printStack=%v", printStack)
		assert.Contains(t, out, "panic: string detail", "printStack=%v", printStack)
		if printStack {
			assert.Contains(t, out, "stack trace:", "printStack=%v", printStack)
		}
	}
}
