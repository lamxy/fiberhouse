package module

import (
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
	exampleGinApi "github.com/lamxy/fiberhouse/example_application/module/example-ginapi-module/api"
)

// RegisterRouteHandlers 注册各业务模块的路由处理器
func RegisterGinRouteHandlers(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(gin.IRouter)
	// 注册example模块的路由处理器
	exampleGinApi.RegisterRouteHandlers(ctx, app)

	// TODO 注册更多业务模块路由处理器 ...

}
