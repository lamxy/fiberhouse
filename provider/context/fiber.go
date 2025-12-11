package context

import "github.com/gofiber/fiber/v2"

// FiberContext Fiber 框架适配器
type FiberContext struct {
	Ctx *fiber.Ctx
}

// WithFiberContext 创建 Fiber 上下文适配器
func WithFiberContext(c *fiber.Ctx) ContextProvider {
	return &FiberContext{Ctx: c}
}

// JSON 以 JSON 格式响应数据
func (f *FiberContext) JSON(statusCode int, data interface{}) error {
	return f.Ctx.Status(statusCode).JSON(data)
}

// GetCtx 获取原生上下文
func (f *FiberContext) GetCtx() any {
	return f.Ctx
}
