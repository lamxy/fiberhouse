package context

import (
	"sync"

	"github.com/gofiber/fiber/v2"
)

// fiberContextPool FiberContext 对象池
var fiberContextPool = sync.Pool{
	New: func() interface{} {
		return &FiberContext{}
	},
}

// FiberContext Fiber 框架适配器
type FiberContext struct {
	Ctx *fiber.Ctx
}

// WithFiberContext 从对象池获取 Fiber 上下文适配器
func WithFiberContext(c *fiber.Ctx) ICoreContext {
	ctx := fiberContextPool.Get().(*FiberContext)
	ctx.Ctx = c
	return ctx
}

// Release 释放 FiberContext 回对象池
func (f *FiberContext) Release() {
	f.Ctx = nil
	fiberContextPool.Put(f)
}

// JSON 以 JSON 格式响应数据
func (f *FiberContext) JSON(statusCode int, data interface{}) error {
	defer f.Release()
	return f.Ctx.Status(statusCode).JSON(data)
}

// GetCtx 获取原生上下文
func (f *FiberContext) GetCtx() any {
	return f.Ctx
}
