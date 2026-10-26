package fiberhouse

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	fiberrequestid "github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 访问日志要带上 requestid 中间件回写到响应头的请求ID，字段名跟随 trace 配置，
// 与 Recover 日志一致，才能用同一个 ID 串联访问日志和错误日志。
func TestCoreFiber_AccessLogRecordsRequestID(t *testing.T) {
	ctx, output := newTask4LoggingAppContext(t, map[string]interface{}{
		"application.middleware.coreHttp": true,
		"application.trace.requestID":     "requestId",
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
	// 访问日志在 requestid 之前注册，与框架实际顺序一致（应用中间件后注册）
	app.Use(core.accessLogMiddleware())
	app.Use(fiberrequestid.New(fiberrequestid.Config{ContextKey: "requestId"}))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	resp.Body.Close()

	generated := resp.Header.Get(fiber.HeaderXRequestID)
	require.NotEmpty(t, generated, "requestid middleware must echo the id in the response header")
	records := decodeTask4LogRecords(t, output)
	require.Len(t, records, 1)
	assert.Equal(t, generated, records[0]["requestId"])
	assert.NotContains(t, records[0], "traceId")
}

// 没有 requestid 中间件时不写空字段
func TestCoreFiber_AccessLogOmitsEmptyRequestID(t *testing.T) {
	ctx, output := newTask4LoggingAppContext(t, map[string]interface{}{
		"application.middleware.coreHttp": true,
	})
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	app := fiber.New()
	app.Use(core.accessLogMiddleware())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	resp.Body.Close()

	records := decodeTask4LogRecords(t, output)
	require.Len(t, records, 1)
	assert.NotContains(t, records[0], "requestId")
}

func TestCoreGin_AccessLogRecordsRequestID(t *testing.T) {
	preserveTask4GinMode(t)
	ctx, output := newTask4LoggingAppContext(t, map[string]interface{}{
		"application.middleware.coreHttp": true,
		"application.trace.requestID":     "requestId",
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)

	core.coreApp.Use(core.loggerMiddleware())
	core.coreApp.Use(requestid.New())
	core.coreApp.GET("/access", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	core.coreApp.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/access", nil))

	generated := recorder.Header().Get("X-Request-ID")
	require.NotEmpty(t, generated, "requestid middleware must echo the id in the response header")
	records := task4LogRecordsWithMessage(t, output, "HTTP Request")
	require.Len(t, records, 1)
	assert.Equal(t, generated, records[0]["requestId"])
}
