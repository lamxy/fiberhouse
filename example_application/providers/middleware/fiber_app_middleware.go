package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
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

	// 就绪探针：依赖不健康时返回 503，供 K8s readinessProbe 或负载均衡器摘除流量。
	//
	// 刻意放在 basicauth 保护的 /uuid 前缀之外——探针由编排系统发起，不带凭证。
	// 注意不要把该路由同时用作 livenessProbe：liveness 探测外部依赖会在数据库
	// 抖动时导致全体 Pod 重启，把依赖故障放大成雪崩；liveness 只需检查进程自身。
	app.Get("/readyz", adaptor.HTTPHandlerFunc(
		fiberhouse.NewReadinessHandler(ctx.GetContainer()),
	))

	// etc...
}
