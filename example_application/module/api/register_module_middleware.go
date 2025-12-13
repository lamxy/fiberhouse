package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"time"
)

// RegisterMiddleware 注册中间件
func RegisterMiddleware(app fiber.Router) fiber.Router {
	// Pprof中间件 /monitor/debug/pprof/
	app.Use(pprof.New(pprof.Config{
		Next: func(c *fiber.Ctx) bool {
			// todo 定义条件
			return false
		},
		Prefix: "/uuid",
	}))

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
