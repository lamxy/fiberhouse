package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
)

func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app gin.IRouter) {
	// 获取exampleApi处理器
	exampleApi, _ := InjectExampleApi(ctx) // 由wire编译依赖注入获取

	// 获取CommonApi处理器，直接NewCommonHandler
	commonApi := NewCommonHandler(ctx) // 直接New，无需依赖注入(Wire)，内部依赖走全局管理器延迟获取依赖组件，见 common_api.go: api.CommonHandler

	// get more api handlers ...
	// 获取注册更多api处理器并注册相应路由

	// 注册Example模块的路由
	// Example Controller
	exampleGroup := app.Group("/gin/example")
	{
		exampleGroup.GET("/hello/world", exampleApi.HelloWorld)
		exampleGroup.GET("/get/:id", exampleApi.GetExample)
		exampleGroup.GET("/on-async-task/get/:id", exampleApi.GetExampleWithTaskDispatcher)
		exampleGroup.POST("/create", exampleApi.CreateExample)
		exampleGroup.GET("/list", exampleApi.GetExamples)
	}

	// 注册Common公共模块路由
	// Common Controller
	commonGroup := app.Group("/gin/common")
	{
		commonGroup.GET("/test/get-instance", commonApi.TestGetInstance)
		commonGroup.GET("/test/get-must-instance", commonApi.TestGetMustInstance)
		commonGroup.GET("/test/get-must-instance-failed", commonApi.TestGetMustInstanceFailed)
	}
}
