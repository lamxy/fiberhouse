package adaptor

import (
	"github.com/gin-gonic/gin"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

// GinErrorHandler 创建一个 Gin 框架的错误处理适配器
func GinErrorHandler(fn func(providerCtx.ICoreContext, error) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := fn(providerCtx.WithGinContext(c), c.Errors.Last())
			if err != nil {
				panic(err)
			}
			return
		}
		if err, ok := c.Get("error"); ok {
			if errObj, isErr := err.(error); isErr {
				err := fn(providerCtx.WithGinContext(c), errObj)
				if err != nil {
					panic(err)
				}
				return
			}
		}
	}
}
