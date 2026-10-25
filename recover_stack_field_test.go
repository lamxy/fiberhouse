package fiberhouse

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 开启堆栈打印时，message 必须保留错误文本（GlitchTip 等以 message 作 issue 标题），
// 堆栈由独立的 stack 字段承载。
func TestErrorHandler_DefaultStackTraceHandler_StackInSeparateField(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		message string
	}{
		{"plain error", errors.New("boom"), "boom"},
		{"exception", &exception.Exception{Code: 1001, Msg: "biz failed"}, (&exception.Exception{Code: 1001, Msg: "biz failed"}).Error()},
		{"validate exception", &exception.ValidateException{Code: 1002, Msg: "bad input"}, (&exception.ValidateException{Code: 1002, Msg: "bad input"}).Error()},
		{"fiber error", fiber.NewError(http.StatusTeapot, "teapot"), "teapot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := newStackFieldAppContext(t, &buf)
			handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))

			app := fiber.New()
			app.Get("/", func(c *fiber.Ctx) error {
				wrapped := adaptorctx.WithFiberContext(c)
				defer wrapped.(*adaptorctx.FiberContext).Release()
				handler.DefaultStackTraceHandler(wrapped, tc.err)
				return c.SendStatus(http.StatusNoContent)
			})
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
			require.NoError(t, err)
			resp.Body.Close()

			var record map[string]any
			require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record), buf.String())
			assert.Equal(t, tc.message, record["message"])
			stack, _ := record["stack"].(string)
			assert.True(t, strings.HasPrefix(stack, "stack trace:\n"), "stack field: %q", stack)
			// zerolog 不去重字段，json.Unmarshal 又只保留最后一个，重复写入须直接数原文
			assert.Equal(t, 1, strings.Count(buf.String(), `"stack":`), "stack field must be written once")
		})
	}
}

func newStackFieldAppContext(t *testing.T, buf *bytes.Buffer) IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.recover.debugMode":        false,
		"application.recover.enablePrintStack": true,
		"application.trace.requestID":          "X-Trace-ID",
	}).Initialize()
	logger := zerolog.New(buf)
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&BootConfig{})
	ctx.RegisterStarterApp(&task5Starter{application: &task5Application{}})
	ctx.GetContainer().Unregister(task5CodecKey)
	require.True(t, ctx.GetContainer().Register(task5CodecKey, func() (interface{}, error) {
		return jsoncodec.StdJsonDefault(), nil
	}))
	t.Cleanup(func() { ctx.GetContainer().Unregister(task5CodecKey) })
	return ctx
}
