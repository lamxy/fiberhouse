package context

import (
	"sync"

	"github.com/gin-gonic/gin"
)

// ginContextPool GinContext 对象池
var ginContextPool = sync.Pool{
	New: func() interface{} {
		return &GinContext{}
	},
}

// GinContext Gin 框架适配器
type GinContext struct {
	Ctx *gin.Context
}

// WithGinContext 从对象池获取 Gin 上下文适配器
func WithGinContext(c *gin.Context) ICoreContext {
	ctx := ginContextPool.Get().(*GinContext)
	ctx.Ctx = c
	return ctx
}

// Release 释放 GinContext 回对象池
func (g *GinContext) Release() {
	g.Ctx = nil
	ginContextPool.Put(g)
}

// JSON 以 JSON 格式响应数据
func (g *GinContext) JSON(statusCode int, data interface{}) error {
	defer g.Release()
	g.Ctx.JSON(statusCode, data)
	return nil
}

// GetCtx 获取原生上下文
func (g *GinContext) GetCtx() any {
	return g.Ctx
}
