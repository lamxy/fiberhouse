// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package errorhandler

import (
	"github.com/gin-gonic/gin"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
)

// GinErrorHandler 创建一个 Gin 框架的错误处理适配器
func GinErrorHandler(fn func(adaptorctx.ICoreContext, error) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := fn(adaptorctx.WithGinContext(c), c.Errors.Last())
			if err != nil {
				panic(err)
			}
			return
		}
		if err, ok := c.Get("error"); ok { // 自定义错误处理
			if errObj, isErr := err.(error); isErr {
				err := fn(adaptorctx.WithGinContext(c), errObj)
				if err != nil {
					panic(err)
				}
				return
			}
		}
	}
}
