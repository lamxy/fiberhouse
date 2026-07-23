package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
)

func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app gin.IRouter) {
	exampleAPI, _ := InjectExampleApi(ctx)
	registerExampleRoutes(app, exampleAPI)

	commonAPI := NewCommonHandler(ctx)
	commonGroup := app.Group("/gin/common")
	{
		commonGroup.GET("/test/get-instance", commonAPI.TestGetInstance)
		commonGroup.GET("/test/get-must-instance", commonAPI.TestGetMustInstance)
		commonGroup.GET("/test/get-must-instance-failed", commonAPI.TestGetMustInstanceFailed)
	}
}

func registerExampleRoutes(router gin.IRouter, handler *ExampleHandler) {
	router.POST("/examples", handler.Create)
	router.GET("/examples/:id", handler.Get)
	router.GET("/examples", handler.List)
	router.PUT("/examples/:id", handler.Update)
	router.DELETE("/examples/:id", handler.Delete)
}
