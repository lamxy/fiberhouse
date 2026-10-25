package fiberhouse

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestid 中间件把 id 存进 Locals 并写到响应头，请求头里并没有；
// 错误日志的请求 ID 必须取中间件生成的值，而不是空字符串。
func TestErrorHandler_DefaultStackTraceHandler_TraceIDFromRequestIDMiddleware(t *testing.T) {
	var buf bytes.Buffer
	ctx := newStackFieldAppContext(t, &buf) // application.trace.requestID = X-Trace-ID
	handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))

	app := fiber.New()
	app.Use(requestid.New(requestid.Config{ContextKey: "X-Trace-ID"}))
	app.Get("/", func(c *fiber.Ctx) error {
		wrapped := adaptorctx.WithFiberContext(c)
		defer wrapped.(*adaptorctx.FiberContext).Release()
		handler.DefaultStackTraceHandler(wrapped, errors.New("boom"))
		return c.SendStatus(http.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	resp.Body.Close()

	generated := resp.Header.Get(fiber.HeaderXRequestID)
	require.NotEmpty(t, generated)
	var record map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record), buf.String())
	assert.Equal(t, generated, record["X-Trace-ID"])
}

// 没有 requestid 中间件时，保留原有行为：从同名请求头读取。
func TestErrorHandler_DefaultStackTraceHandler_TraceIDFallsBackToRequestHeader(t *testing.T) {
	var buf bytes.Buffer
	ctx := newStackFieldAppContext(t, &buf)
	handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		wrapped := adaptorctx.WithFiberContext(c)
		defer wrapped.(*adaptorctx.FiberContext).Release()
		handler.DefaultStackTraceHandler(wrapped, errors.New("boom"))
		return c.SendStatus(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "from-header")
	resp, err := app.Test(req)
	require.NoError(t, err)
	resp.Body.Close()

	var record map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record), buf.String())
	assert.Equal(t, "from-header", record["X-Trace-ID"])
}
