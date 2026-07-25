package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/lamxy/fiberhouse"
)

func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app fiber.Router) {
	exampleAPI, _ := InjectExampleApi(ctx) // 存在构造注入依赖，内部依赖定位层组件，由 wire 编译期解决依赖后注入
	registerExampleRoutes(app, exampleAPI)

	healthAPI, _ := InjectHealthApi(ctx) // 同上。备注：框架根下 component/container/ 目录中提供了基于 dig 封装的依赖注入容器组件，可以替换 wire 注入。但 dig 推荐仅用于应用启动阶段
	registerHealthRoutes(app, healthAPI)

	commonAPI := NewCommonHandler(ctx) // 直接 New 构造，无需依赖注入(Wire)，内部依赖走全局管理器延迟初始化和获取依赖组件，见 common_api.go: api.CommonHandler
	registerCommonRoutes(app, commonAPI)
}

// registerExampleRoutes 注册样例模块路由
func registerExampleRoutes(router fiber.Router, handler *ExampleHandler) {
	router.Post("/examples", handler.Create)
	router.Get("/examples/:id", handler.Get)
	router.Get("/examples", handler.List)
	router.Put("/examples/:id", handler.Update)
	router.Delete("/examples/:id", handler.Delete)
}

// registerHealthRoutes 注册与应用健康状态的路由
func registerHealthRoutes(router fiber.Router, handler *HealthHandler) {
	healthGroup := router.Group("/health")
	healthGroup.Get("/livez", handler.Liveness).Name("health_liveness")
}

// registerCommonRoutes 注册公共部分的路由
func registerCommonRoutes(router fiber.Router, handler *CommonHandler) {
	// 限流中间件
	commonGroup := router.Group("/common", limiter.New(limiter.Config{
		Max:        5,
		Expiration: 30 * time.Second,
		KeyGenerator: func(*fiber.Ctx) string {
			return "limiter_key_unique"
		},
		LimiterMiddleware: limiter.SlidingWindow{},
	}))
	commonGroup.Get("/test/get-instance", handler.TestGetInstance).Name("common_get_instance")
	commonGroup.Get("/test/get-must-instance", handler.TestGetMustInstance).Name("common_get_must_instance")
	commonGroup.Get("/test/get-must-instance-failed", handler.TestGetMustInstanceFailed).Name("common_get_must_instance_failed")
}
