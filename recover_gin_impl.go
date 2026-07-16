// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// GinRecovery Gin 框架的请求数据实现
type GinRecovery struct {
	AppCtx IApplicationContext
}

// NewGinRecovery 创建 Gin 请求数据实例
func NewGinRecovery(ctx IApplicationContext) *GinRecovery {
	return &GinRecovery{
		AppCtx: ctx,
	}
}

func (g *GinRecovery) GetParamsJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return nil
	}
	params := make(map[string]string)
	for _, p := range c.Params {
		params[p.Key] = p.Value
	}
	j, err := jsonEncoder(params)
	if err != nil {
		log.Warn(g.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqParamsErr", err.Error()).Msg("getParamsJson error")
		return nil
	}
	return j
}

func (g *GinRecovery) GetQueriesJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return nil
	}
	queries := make(map[string][]string)
	for key, values := range c.Request.URL.Query() {
		queries[key] = values
	}
	j, err := jsonEncoder(queries)
	if err != nil {
		log.Warn(g.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqQueriesErr", err.Error()).Msg("getQueriesJson error")
		return nil
	}
	return j
}

func (g *GinRecovery) GetHeadersJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return nil
	}
	headers := make(map[string][]string)
	for key, values := range c.Request.Header {
		headers[key] = values
	}
	sanitizedHeaders := sanitizeHeaders(headers)
	j, err := jsonEncoder(sanitizedHeaders)
	if err != nil {
		log.Warn(g.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqHeadersErr", err.Error()).Msg("getHeadersJson error")
		return nil
	}
	return j
}

func (g *GinRecovery) GetHeader(ctx adaptorctx.ICoreContext, key string) string {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return ""
	}
	return c.GetHeader(key)
}

func (g *GinRecovery) RecoverPanic(config ...RecoverConfig) any {
	// 使用恢复中间件提供者来创建和返回相应核心的恢复中间件函数，
	//通过恢复中间件管理器内部，依据核心类型自动返回相应的恢复中间件的any类型
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *gin.Context) {
		pCtx := adaptorctx.WithGinContext(c)
		completed := false
		func() {
			defer recoverPanicInternal(pCtx, cfg)
			// Don't execute middleware if Cfg Next returns true
			if cfg.Next != nil && cfg.Next(pCtx) {
				c.Next()
				completed = true
				return
			}

			c.Next()
			completed = true
		}()
		if completed {
			releaseCoreContext(pCtx)
		}
	}
}

func (g *GinRecovery) TraceID(ctx adaptorctx.ICoreContext, flag ...string) string {
	// 原生上下文
	var (
		c  *gin.Context
		ok bool
	)
	if c, ok = ctx.GetCtx().(*gin.Context); !ok {
		panic("ContextProvider is not *gin.Context")
	}
	// 请求requestId
	return requestid.Get(c)
}
