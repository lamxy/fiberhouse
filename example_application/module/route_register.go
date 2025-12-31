package module

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	exampleApi "github.com/lamxy/fiberhouse/example_application/module/example-module/api"
)

// RegisterRouteHandlers 注册各业务模块的路由处理器
func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(*fiber.App)
	// 注册example模块的路由处理器
	exampleApi.RegisterRouteHandlers(ctx, app)

	// TODO 注册更多业务模块路由处理器 ...

}
