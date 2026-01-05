package module

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"github.com/lamxy/fiberhouse"
	exampleApi "github.com/lamxy/fiberhouse/example_application/module/example-module/api"
)

// RegisterRouteHandlers 注册各业务模块的路由处理器
func RegisterFiberRouteHandlers(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(*fiber.App)
	// 注册example模块的路由处理器
	exampleApi.RegisterRouteHandlers(ctx, app)

	// TODO 注册更多业务模块路由处理器 ...

}

// RegisterFiberSwagger 注册Swagger UI route
func RegisterFiberSwagger(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(*fiber.App)
	registerOrNot := ctx.GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		app.Get("/swagger/*", swagger.HandlerDefault) //  Route: /{uuid}/swagger/*

		// todo 设置安全访问配置

	}
}
