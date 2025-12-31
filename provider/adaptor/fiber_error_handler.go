package adaptor

import (
	"github.com/gofiber/fiber/v2"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
)

// FiberErrorHandler 创建一个 Fiber 框架的错误处理适配器
func FiberErrorHandler(fn func(providerctx.ICoreContext, error) error) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		handlerErr := fn(providerctx.WithFiberContext(c), err)
		if handlerErr != nil {
			return handlerErr
		}
		return err
	}
}
