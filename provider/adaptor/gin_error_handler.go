package adaptor

import (
	"github.com/gin-gonic/gin"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

// GinErrorHandler 创建一个 Gin 框架的错误处理适配器
func GinErrorHandler(fn func(providerCtx.ICoreContext, error) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := fn(providerCtx.WithGinContext(c), c.Errors.Last())
		if err != nil {
			// 如果处理函数返回错误，则记录该错误
			_ = c.Error(err)
		}
	}
}
