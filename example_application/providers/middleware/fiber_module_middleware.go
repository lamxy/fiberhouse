package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/lamxy/fiberhouse"
	"time"
)

// RegisterMiddleware 注册中间件
//
// 注意：pprof 性能剖析已由框架层提供，通过 application.pprof.enable 配置开关控制，
// 并独立监听诊断端口（默认 127.0.0.1:6060），不再挂载到业务引擎路由上。
// 详见框架的 pprof_server.go 与 pprof_config.go。
func RegisterModuleMiddleware(cs fiberhouse.CoreStarter) fiber.Router {
	app := cs.GetCoreApp().(*fiber.App)

	// csrf post请求验证
	app.Use(csrf.New(csrf.Config{
		Next: func(c *fiber.Ctx) bool {
			// todo 定义条件
			return true // 默认不拦截
		},
		Expiration: 1 * time.Hour,
	}))

	return app
}
