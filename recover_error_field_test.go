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
	"github.com/lamxy/fiberhouse/exception"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 错误写入 zerolog 标准的 error 字段，错误追踪平台（如 sentryzerolog）据此生成带堆栈的 Exception。
func TestErrorHandler_DefaultStackTraceHandler_WritesErrorField(t *testing.T) {
	var typedNilFiberErr *fiber.Error
	cases := []struct {
		name      string
		e         interface{}
		wantError interface{} // nil 表示不应出现 error 字段
	}{
		{"plain error", errors.New("boom"), "boom"},
		{"exception", exception.New(4101, "biz failed"), "biz failed"},
		{"validate exception", exception.NewVE(4201, "bad input"), "bad input"},
		{"fiber error", fiber.NewError(http.StatusNotFound, "route missing"), "route missing"},
		{"string panic", "boom", "panic: boom"},
		{"typed-nil fiber error", typedNilFiberErr, nil},
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
				assert.NotPanics(t, func() { handler.DefaultStackTraceHandler(wrapped, tc.e) })
				return c.SendStatus(http.StatusNoContent)
			})
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
			require.NoError(t, err)
			resp.Body.Close()

			var record map[string]any
			require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record), buf.String())
			assert.Equal(t, tc.wantError, record["error"])
			wantCount := 1
			if tc.wantError == nil {
				wantCount = 0
			}
			assert.Equal(t, wantCount, strings.Count(buf.String(), `"error":`), "error field must be written at most once")
		})
	}
}
