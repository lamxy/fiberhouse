package module

import (
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
	exampleGinApi "github.com/lamxy/fiberhouse/example_application/module/example-ginapi-module/api"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterRouteHandlers 注册各业务模块的路由处理器
func RegisterGinRouteHandlers(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(gin.IRouter)
	// 注册example模块的路由处理器
	exampleGinApi.RegisterRouteHandlers(ctx, app)

	// TODO 注册更多业务模块路由处理器 ...

}

// RegisterGinSwagger 注册Swagger UI route
func RegisterGinSwagger(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(*gin.Engine)
	registerOrNot := ctx.GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		app.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// todo 设置安全访问配置

	}
}
