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
			if err := fn(adaptorctx.WithGinContext(c), c.Errors.Last()); err != nil {
				reportHandlerFailure(c, err)
			}
			return
		}
		if err, ok := c.Get("error"); ok { // 自定义错误处理
			if errObj, isErr := err.(error); isErr {
				if err := fn(adaptorctx.WithGinContext(c), errObj); err != nil {
					reportHandlerFailure(c, err)
				}
				return
			}
		}
	}
}

// reportHandlerFailure 把错误处理器自身的失败登记回 Gin 的错误链。
//
// 此前这里 panic。那是**运行期** panic：本中间件在 c.Next() 之后执行，即请求
// 已经处理完毕、正在收尾错误响应的阶段。此时 panic 会被 recovery 中间件捕获
// 并转成 500，掩盖掉真正的原始错误；若 recovery 未装配则直接打断连接。
//
// 条件 4 只豁免「装配期 panic + 有 error 替代入口」，运行期 panic 始终不满足
// （feature-status.md:124）。本函数签名是 gin.HandlerFunc，没有 error 通道，
// 因此正确处置是登记到 c.Errors——Gin 自身的错误收集机制，后续中间件与访问
// 日志都能看到它。
func reportHandlerFailure(c *gin.Context, err error) {
	_ = c.Error(err)
}
