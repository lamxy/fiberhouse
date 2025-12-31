package module

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"github.com/lamxy/fiberhouse"
)

// RegisterSwagger 注册Swagger UI route
func RegisterSwagger(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(*fiber.App)
	registerOrNot := ctx.GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		app.Get("/swagger/*", swagger.HandlerDefault) //  Route: /{uuid}/swagger/*

		// todo 设置安全访问配置

	}
}
