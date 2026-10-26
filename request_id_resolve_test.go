package fiberhouse

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestResolveRequestID_Priority(t *testing.T) {
	headers := func(h map[string]string) func(string) string {
		return func(k string) string { return h[k] }
	}
	for _, tc := range []struct {
		name       string
		middleware string
		headers    map[string]string
		key        string
		want       string
	}{
		{"middleware wins", "mw", map[string]string{"X-Request-ID": "std", "requestId": "legacy"}, "requestId", "mw"},
		{"standard header before legacy", "", map[string]string{"X-Request-ID": "std", "requestId": "legacy"}, "requestId", "std"},
		{"legacy header still honored", "", map[string]string{"requestId": "legacy"}, "requestId", "legacy"},
		{"nothing available", "", map[string]string{}, "requestId", ""},
		{"key equal to standard header is not read twice", "", map[string]string{}, "x-request-id", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveRequestID(tc.middleware, headers(tc.headers), tc.key))
		})
	}
}

// 没有 requestid 中间件、上游带 X-Request-ID 时，Recover 日志与访问日志必须记录同一个 ID。
func TestFiber_NoMiddleware_RecoverAndAccessLogShareClientRequestID(t *testing.T) {
	var buf bytes.Buffer
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.middleware.coreHttp":      true,
		"application.recover.enablePrintStack": false,
		"application.trace.requestID":          "requestId",
	}).Initialize()
	logger := zerolog.New(&buf)
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&BootConfig{})
	ctx.RegisterStarterApp(&task5Starter{application: &task5Application{}})
	ctx.GetContainer().Unregister(task5CodecKey)
	require.True(t, ctx.GetContainer().Register(task5CodecKey, func() (interface{}, error) {
		return jsoncodec.StdJsonDefault(), nil
	}))
	t.Cleanup(func() { ctx.GetContainer().Unregister(task5CodecKey) })
	httpKey := cfg.LogOriginCoreHttp().InstanceKey()
	ctx.GetContainer().Unregister(httpKey)
	require.True(t, ctx.GetContainer().Register(httpKey, func() (interface{}, error) {
		l := logger.With().Str("Origin", "access").Logger()
		return &l, nil
	}))
	t.Cleanup(func() { ctx.GetContainer().Unregister(httpKey) })

	handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	app := fiber.New()
	app.Use(core.accessLogMiddleware())
	app.Get("/", func(c *fiber.Ctx) error {
		wrapped := adaptorctx.WithFiberContext(c)
		defer wrapped.(*adaptorctx.FiberContext).Release()
		handler.DefaultStackTraceHandler(wrapped, errors.New("boom"))
		return c.SendStatus(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "upstream-id")
	resp, err := app.Test(req)
	require.NoError(t, err)
	resp.Body.Close()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.Len(t, lines, 2, buf.String())
	for _, line := range lines {
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		assert.Equal(t, "upstream-id", record["requestId"], string(line))
	}
}
