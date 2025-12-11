package adaptor

import (
	"github.com/gin-gonic/gin"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

func GinErrorHandler(fn func(providerCtx.ContextProvider, error) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := fn(providerCtx.WithGinContext(c), c.Errors.Last())
		if err != nil {
			// 如果处理函数返回错误，则记录该错误
			_ = c.Error(err)
		}
	}
}
