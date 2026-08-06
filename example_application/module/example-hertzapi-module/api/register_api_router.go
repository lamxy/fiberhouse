package api

import (
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/lamxy/fiberhouse"
)

// RegisterRouteHandlers 註冊 example 模組的 hertz 路由處理器
//
// 路由路徑與 gin 適配器保持一致，差異僅在於 hertz 的路徑參數語法為 :id（與 gin 相同）。
func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, router route.IRouter) {
	exampleAPI, _ := InjectExampleApi(ctx)
	registerExampleRoutes(router, exampleAPI)

	commonAPI := NewCommonHandler(ctx)
	registerCommonRoutes(router, commonAPI)
}

func registerExampleRoutes(router route.IRouter, handler *ExampleHandler) {
	router.POST("/examples", handler.Create)
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
