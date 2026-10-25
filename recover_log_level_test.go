package fiberhouse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpStatusOf(t *testing.T) {
	var typedNilFiberErr *fiber.Error
	cases := []struct {
		name string
		e    interface{}
		want int
	}{
		{"validate exception", exception.NewVE(400001, "bad input"), http.StatusBadRequest},
		// 业务 Code 只给客户端分类用，不影响 HTTP 状态
		{"exception 40 code", exception.New(400002, "not found"), http.StatusBadRequest},
		{"exception 50 code", exception.New(500001, "internal"), http.StatusBadRequest},
		{"exception 20 code", exception.New(200001, "no update"), http.StatusBadRequest},
		{"wrapped exception", fmt.Errorf("query: %w", exception.New(400002, "not found")), http.StatusBadRequest},
		{"fiber 404", fiber.NewError(http.StatusNotFound, "Cannot GET /x"), http.StatusNotFound},
		{"fiber 503", fiber.NewError(http.StatusServiceUnavailable), http.StatusServiceUnavailable},
		{"fiber zero code", &fiber.Error{}, http.StatusInternalServerError},
		{"typed-nil fiber error", typedNilFiberErr, http.StatusInternalServerError},
		{"fiber error value", fiber.Error{Code: http.StatusNotFound}, http.StatusInternalServerError},
		{"plain error", errors.New("boom"), http.StatusInternalServerError},
		{"runtime error", runtimeErrorForTest(), http.StatusInternalServerError},
		{"string panic", "boom", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, httpStatusOf(tc.e))
		})
	}
}

// 日志级别跟随 HTTP 状态：4xx 记 warn，5xx 记 error。
func TestErrorHandler_DefaultStackTraceHandler_LevelFollowsHTTPStatus(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		level string
	}{
		{"validate exception", exception.NewVE(400001, "bad input"), "warn"},
		{"exception", exception.New(500001, "internal"), "warn"},
		{"fiber 404", fiber.NewError(http.StatusNotFound), "warn"},
		{"fiber 503", fiber.NewError(http.StatusServiceUnavailable), "error"},
		{"plain error", errors.New("boom"), "error"},
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
			assert.Equal(t, tc.level, record["level"])
		})
	}
}

func runtimeErrorForTest() (err error) {
	defer func() { err = recover().(error) }()
	var m map[string]int
	m["x"] = 1
	return nil
}
