package context

import "github.com/gin-gonic/gin"

// GinContext Gin 框架适配器
type GinContext struct {
	Ctx *gin.Context
}

// WithGinContext 创建 Gin 上下文适配器
func WithGinContext(c *gin.Context) ICoreContext {
	return &GinContext{Ctx: c}
}

// JSON 以 JSON 格式响应数据
func (g *GinContext) JSON(statusCode int, data interface{}) error {
	g.Ctx.JSON(statusCode, data)
	return nil
}

// GetCtx 获取原生上下文
func (g *GinContext) GetCtx() any {
	return g.Ctx
}
