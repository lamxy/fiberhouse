package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/lamxy/fiberhouse"
)

func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app fiber.Router) {
	exampleAPI, _ := InjectExampleApi(ctx)
	registerExampleRoutes(app, exampleAPI)

	healthAPI, _ := InjectHealthApi(ctx)
	healthGroup := app.Group("/health", limiter.New(limiter.Config{
		Max:        5,
		Expiration: 30 * time.Second,
		KeyGenerator: func(*fiber.Ctx) string {
			return "limiter_key_unique"
		},
		LimiterMiddleware: limiter.SlidingWindow{},
	}))
	healthGroup.Get("/livez", healthAPI.Liveness)

	commonAPI := NewCommonHandler(ctx)
	commonGroup := app.Group("/common", limiter.New(limiter.Config{}))
	commonGroup.Get("/test/get-instance", commonAPI.TestGetInstance).Name("common_get_instance")
	commonGroup.Get("/test/get-must-instance", commonAPI.TestGetMustInstance).Name("common_get_must_instance")
	commonGroup.Get("/test/get-must-instance-failed", commonAPI.TestGetMustInstanceFailed).Name("common_get_must_instance_failed")
}

func registerExampleRoutes(router fiber.Router, handler *ExampleHandler) {
	router.Post("/examples", handler.Create)
	router.Get("/examples/:id", handler.Get)
	router.Get("/examples", handler.List)
	router.Put("/examples/:id", handler.Update)
	router.Delete("/examples/:id", handler.Delete)
}
