package fiberhouse

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 5xx 已由 Recover（DefaultStackTraceHandler）记一次 ERROR，访问日志再记 ERROR 会重复上报；
// 访问日志只记录流量：5xx、4xx 记 WARN，其余记 INFO。
func TestCoreFiber_AccessLogLevels(t *testing.T) {
	ctx, output := newTask4LoggingAppContext(t, map[string]interface{}{
		"application.middleware.coreHttp": true,
	})
	key := ctx.GetConfig().LogOriginCoreHttp().InstanceKey()
	ctx.GetContainer().Unregister(key)
	require.True(t, ctx.GetContainer().Register(key, func() (interface{}, error) {
		log := ctx.GetLogger().With().Str("Origin", ctx.GetConfig().LogOriginCoreHttp().String()).Logger()
		return &log, nil
	}))
	t.Cleanup(func() { ctx.GetContainer().Unregister(key) })

	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	app := fiber.New()
	app.Use(core.accessLogMiddleware())
	app.Get("/status/:code", func(c *fiber.Ctx) error {
		code, _ := c.ParamsInt("code")
		return c.SendStatus(code)
	})

	for _, tc := range []struct {
		code  int
		level zerolog.Level
	}{
		{http.StatusInternalServerError, zerolog.WarnLevel},
		{http.StatusServiceUnavailable, zerolog.WarnLevel},
		{http.StatusNotFound, zerolog.WarnLevel},
		{http.StatusOK, zerolog.InfoLevel},
	} {
		output.Reset()
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/status/"+strconv.Itoa(tc.code), nil))
		require.NoError(t, err)
		resp.Body.Close()
		records := decodeTask4LogRecords(t, output)
		require.Len(t, records, 1, "status %d", tc.code)
		assert.Equal(t, tc.level.String(), records[0]["level"], "status %d", tc.code)
	}
}

// Gin 访问日志与 Fiber 同一规则：5xx、4xx 记 WARN，其余记 INFO。
func TestCoreGin_AccessLogLevels(t *testing.T) {
	preserveTask4GinMode(t)
	ctx, output := newTask4LoggingAppContext(t, map[string]interface{}{
		"application.middleware.coreHttp": true,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)

	core.coreApp.Use(core.loggerMiddleware())
	core.coreApp.GET("/status/:code", func(c *gin.Context) {
		code, _ := strconv.Atoi(c.Param("code"))
		c.Status(code)
	})

	for _, tc := range []struct {
		code  int
		level zerolog.Level
	}{
		{http.StatusInternalServerError, zerolog.WarnLevel},
		{http.StatusServiceUnavailable, zerolog.WarnLevel},
		{http.StatusNotFound, zerolog.WarnLevel},
		{http.StatusOK, zerolog.InfoLevel},
	} {
		output.Reset()
		core.coreApp.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/status/"+strconv.Itoa(tc.code), nil))
		records := task4LogRecordsWithMessage(t, output, "HTTP Request")
		require.Len(t, records, 1, "status %d", tc.code)
		assert.Equal(t, tc.level.String(), records[0]["level"], "status %d", tc.code)
	}
}
