package fiberhouse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panic 路径与 ErrorHandler 必须对同一个错误给出同一个 HTTP 状态，
// 这样 DefaultStackTraceHandler 按 httpStatusOf 选出的日志级别在两条路径上都准确。
func TestRecoverPanic_StatusMatchesHttpStatusOf(t *testing.T) {
	cases := []struct {
		name     string
		value    func() interface{}
		wantCode int // 响应体中的业务 code
	}{
		{"exception", func() interface{} { return exception.New(4101, "business rule") }, 4101},
		{"wrapped exception", func() interface{} { return fmt.Errorf("query: %w", exception.New(4101, "business rule")) }, 4101},
		{"wrapped validate exception", func() interface{} { return fmt.Errorf("bind: %w", exception.NewVE(4201, "invalid input")) }, 4201},
		{"fiber error", func() interface{} { return fiber.NewError(http.StatusNotFound, "route missing") }, http.StatusNotFound},
		{"wrapped fiber error", func() interface{} { return fmt.Errorf("proxy: %w", fiber.NewError(http.StatusServiceUnavailable)) }, http.StatusServiceUnavailable},
		{"plain error", func() interface{} { return errors.New("boom") }, 0},
	}
	for _, tc := range cases {
		for _, core := range []string{"fiber", "gin"} {
			t.Run(tc.name+"/"+core, func(t *testing.T) {
				ctx := newTask5AppContext(t, false, false)
				installTask5ResponseManager(t, ctx)
				installTask5Exceptions(t, ctx)
				cfg := RecoverConfig{JsonCodec: json.Marshal}

				var status int
				var body []byte
				switch core {
				case "fiber":
					app := fiber.New()
					app.Use(NewFiberRecovery(ctx).RecoverPanic(cfg).(fiber.Handler))
					app.Get("/", func(*fiber.Ctx) error { panic(tc.value()) })
					resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
					require.NoError(t, err)
					defer resp.Body.Close()
					body, err = io.ReadAll(resp.Body)
					require.NoError(t, err)
					status = resp.StatusCode
				case "gin":
					preserveTask4GinMode(t)
					gin.SetMode(gin.TestMode)
					engine := gin.New()
					engine.Use(gin.HandlerFunc(NewGinRecovery(ctx).RecoverPanic(cfg).(func(*gin.Context))))
					engine.GET("/", func(*gin.Context) { panic(tc.value()) })
					recorder := httptest.NewRecorder()
					engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
					status, body = recorder.Code, recorder.Body.Bytes()
				}

				assert.Equal(t, httpStatusOf(tc.value()), status)
				if tc.wantCode != 0 {
					var envelope map[string]interface{}
					require.NoError(t, json.Unmarshal(body, &envelope), "body: %s", body)
					assert.EqualValues(t, tc.wantCode, envelope["code"])
				}
			})
		}
	}
}
