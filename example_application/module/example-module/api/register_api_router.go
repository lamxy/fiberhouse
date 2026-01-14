package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/lamxy/fiberhouse"
	"time"
)

func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app fiber.Router) {
	// 获取exampleApi处理器
	exampleApi, _ := InjectExampleApi(ctx) // 由wire编译依赖注入获取

	// 获取HealthApi健康检查处理器
	healthApi, _ := InjectHealthApi(ctx) // 由wire编译依赖注入获取

	// 获取CommonApi处理器，直接NewCommonHandler
	commonApi := NewCommonHandler(ctx) // 直接New，无需依赖注入(Wire)，内部依赖走全局管理器延迟获取依赖组件，见 common_api.go: api.CommonHandler

	// get more api handlers ...
	// 获取注册更多api处理器并注册相应路由

	// 注册Example模块的路由
	// Example Controller
	exampleGroup := app.Group("/example")
	exampleGroup.Get("/hello/world", exampleApi.HelloWorld).Name("ex_get_example_test")
	exampleGroup.Get("/get/:id", exampleApi.GetExample).Name("ex_get_example")
	exampleGroup.Get("/on-async-task/get/:id", exampleApi.GetExampleWithTaskDispatcher).Name("ex_get_example_on_task")
	exampleGroup.Post("/create", exampleApi.CreateExample).Name("ex_create_example")
	exampleGroup.Get("/list", exampleApi.GetExamples).Name("ex_get_examples")

	// 注册Health路由
	// Health Controller
	healthGroup := app.Group("/health", limiter.New(limiter.Config{
		Max:        5,
		Expiration: 30 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return "limiter_key_unique" // 唯一的key（非ip）接口整体滑动窗口限流
		},
		LimiterMiddleware: limiter.SlidingWindow{},
	}))
	healthGroup.Get("/livez", healthApi.Liveness)

	// 注册Common公共模块路由
	// Common Controller
	commonGroup := app.Group("/common", limiter.New(limiter.Config{}))
	commonGroup.Get("/test/get-instance", commonApi.TestGetInstance).Name("common_get_instance")
	commonGroup.Get("/test/get-must-instance", commonApi.TestGetMustInstance).Name("common_get_must_instance")
	commonGroup.Get("/test/get-must-instance-failed", commonApi.TestGetMustInstanceFailed).Name("common_get_must_instance_failed")
}
