// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package errorhandler

import (
	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
)

// FiberErrorHandler 创建一个 Fiber 框架的错误处理适配器
func FiberErrorHandler(fn func(adaptorctx.ICoreContext, error) error) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		return fn(adaptorctx.WithFiberContext(c), err)
	}
}
