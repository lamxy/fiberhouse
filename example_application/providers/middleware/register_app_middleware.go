package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/lamxy/fiberhouse"
)

// RegisterMiddleware 注册全局中间件
func RegisterAppMiddleware(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	// 获取 fiber.App 实例
	app := cs.GetCoreApp().(*fiber.App)
	// 全局 requestId中间件(uuid追查一个链路的日志)
	app.Use(requestid.New(requestid.Config{
		Next: func(c *fiber.Ctx) bool {
			ms := ctx.GetConfig().GetMiddlewareSwitch("requestId")
			return !ms
		},
		ContextKey: "traceId",
	}))

	app.Use("/uuid", basicauth.New(basicauth.Config{
		Users: map[string]string{
			"admin": "123456",
		},
	}))

	// 性能指标监控 uuid占位前缀
	app.Get("/uuid/metrics", monitor.New(monitor.Config{
		Title: "Your App Monitor",
		Next: func(c *fiber.Ctx) bool {
			ms := ctx.GetConfig().GetMiddlewareSwitch("monitor")
			return !ms
		},
	}))

	// etc...
}
