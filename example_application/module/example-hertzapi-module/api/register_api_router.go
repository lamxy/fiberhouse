package api

import (
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/lamxy/fiberhouse"
)

// RegisterRouteHandlers 注册 example 模块的 Hertz 路由处理器。
//
// 路由路径与 Gin 适配器保持一致，路径参数语法同样使用 :id。
func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, router route.IRouter) {
	exampleAPI, _ := InjectExampleApi(ctx)
	registerExampleRoutes(router, exampleAPI)

	healthAPI, _ := InjectHealthApi(ctx)
	registerHealthRoutes(router, healthAPI)

	commonAPI := NewCommonHandler(ctx)
	registerCommonRoutes(router, commonAPI)
}

// registerHealthRoutes 注册与应用健康状态的路由
//
// 路径与 Fiber 适配器保持一致；CI 冒烟测试探测 GET /health/livez，
// 任何作为 example_main 默认核心的适配器都必须提供该路由。
func registerHealthRoutes(router route.IRouter, handler *HealthHandler) {
	healthGroup := router.Group("/health")
	healthGroup.GET("/livez", handler.Liveness)
}

func registerExampleRoutes(router route.IRouter, handler *ExampleHandler) {
	router.POST("/examples", handler.Create)
	router.GET("/examples/cache/:id", handler.GetWithCache)
	router.GET("/examples/:id", handler.Get)
	router.GET("/examples", handler.List)
	router.PUT("/examples/:id", handler.Update)
	router.DELETE("/examples/:id", handler.Delete)
}

func registerCommonRoutes(router route.IRouter, handler *CommonHandler) {
	commonGroup := router.Group("/common")
	{
		commonGroup.GET("/test/get-instance", handler.TestGetInstance)
		commonGroup.GET("/test/get-must-instance", handler.TestGetMustInstance)
		commonGroup.GET("/test/get-must-instance-failed", handler.TestGetMustInstanceFailed)
	}
}
