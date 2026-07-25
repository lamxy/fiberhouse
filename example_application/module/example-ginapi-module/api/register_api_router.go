package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
)

func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app gin.IRouter) {
	exampleAPI, _ := InjectExampleApi(ctx)
	registerExampleRoutes(app, exampleAPI)

	commonAPI := NewCommonHandler(ctx)
	registerCommonRoutes(app, commonAPI)
}

func registerExampleRoutes(router gin.IRouter, handler *ExampleHandler) {
	router.POST("/examples", handler.Create)
	router.GET("/examples/:id", handler.Get)
	router.GET("/examples", handler.List)
	router.PUT("/examples/:id", handler.Update)
	router.DELETE("/examples/:id", handler.Delete)
}

func registerCommonRoutes(router gin.IRouter, handler *CommonHandler) {
	commonGroup := router.Group("/common")
	{
		commonGroup.GET("/test/get-instance", handler.TestGetInstance)
		commonGroup.GET("/test/get-must-instance", handler.TestGetMustInstance)
		commonGroup.GET("/test/get-must-instance-failed", handler.TestGetMustInstanceFailed)
	}
}
