package module

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"github.com/lamxy/fiberhouse"
)

// RegisterSwagger 注册Swagger UI route
func RegisterSwagger(ctx fiberhouse.ContextFramer, app fiber.Router) fiber.Router {
	registerOrNot := ctx.GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		app.Get("/swagger/*", swagger.HandlerDefault) //  Route: /{uuid}/swagger/*

		// todo 设置安全访问配置

	}

	return app
}
