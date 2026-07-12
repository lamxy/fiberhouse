// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

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

// GetCtx 获取原生上下文
func (g *GinContext) GetCtx() any {
	return g.Ctx
}

// JSON 以 JSON 格式响应数据
func (g *GinContext) JSON(statusCode int, data interface{}) error {
	defer g.Release()
	g.Ctx.JSON(statusCode, data)
	return nil
}

// Send 发送原始字节数据
func (g *GinContext) Send(statusCode int, body []byte) error {
	defer g.Release()
	g.Ctx.Status(statusCode)
	_, err := g.Ctx.Writer.Write(body)
	return err
}

// GetHeader 获取请求头
func (g *GinContext) GetHeader(key string) string {
	return g.Ctx.GetHeader(key)
}

// SetHeader 设置响应头
func (g *GinContext) SetHeader(key string, value string) {
	g.Ctx.Writer.Header()[key] = []string{value}
}
