// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// FiberRecovery Fiber 框架的请求数据实现
type FiberRecovery struct {
	AppCtx IApplicationContext
}

// NewFiberRecovery 创建 Fiber 恢复实例
func NewFiberRecovery(ctx IApplicationContext) *FiberRecovery {
	return &FiberRecovery{
		AppCtx: ctx,
	}
}

func (f *FiberRecovery) GetParamsJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return nil
	}
	params := c.AllParams()
	j, err := jsonEncoder(params)
	if err != nil {
		log.Warn(f.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqParamsErr", err.Error()).Msg("getParamsJson error")
		return nil
	}
	return j
}

func (f *FiberRecovery) GetQueriesJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return nil
	}
	queries := c.Queries()
	j, err := jsonEncoder(queries)
	if err != nil {
		log.Warn(f.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqQueriesErr", err.Error()).Msg("getQueriesJson error")
		return nil
	}
	return j
}

func (f *FiberRecovery) GetHeadersJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return nil
	}
	headers := c.GetReqHeaders()
	sanitizedHeaders := sanitizeHeaders(headers)
	j, err := jsonEncoder(sanitizedHeaders)
	if err != nil {
		log.Warn(f.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqHeadersErr", err.Error()).Msg("getHeadersJson error")
		return nil
	}
	return j
}

func (f *FiberRecovery) GetHeader(ctx adaptorctx.ICoreContext, key string) string {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return ""
	}
	return c.Get(key)
}

func (f *FiberRecovery) RecoverPanic(config ...RecoverConfig) any {
	// 使用恢复中间件提供者来返回相关核心引擎的恢复中间件函数，
	//通过恢复中间件管理器依据核心类型配置选择相应的提供者自动返回对应的恢复中间件，返回一个any
	//同时结合全局的泛型方法获取指定核心的恢复函数

	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *fiber.Ctx) error {
		pCtx := adaptorctx.WithFiberContext(c)
		var err error
		completed := false
		func() {
			defer recoverPanicInternal(pCtx, cfg)
			// Don't execute middleware if Next returns true
			if cfg.Next != nil && cfg.Next(pCtx) {
				err = c.Next()
				completed = true
				return
			}

			// Return err if existed, else move to next handler
			err = c.Next()
			completed = true
		}()
		if completed {
			releaseCoreContext(pCtx)
		}
		return err
	}
}

func releaseCoreContext(ctx adaptorctx.ICoreContext) {
	if releasable, ok := ctx.(interface{ Release() }); ok {
		releasable.Release()
	}
}

func (f *FiberRecovery) TraceID(ctx adaptorctx.ICoreContext, flag ...string) string {
	// 原生上下文
	var (
		c  *fiber.Ctx
		ok bool
	)
	if c, ok = ctx.GetCtx().(*fiber.Ctx); !ok {
		panic("ContextProvider is not *fiber.Ctx")
	}
	// 请求requestId
	var (
		traceId string
		rID     = requestID
	)
	if len(flag) > 0 {
		rID = flag[0]
	}
	if c.Locals(rID) != nil {
		traceId = c.Locals(rID).(string) // 请求类错误，从本地变量获取请求ID
	} else {
		traceId = "" // 非请求接口出现的错误，请求ID空值
	}
	return traceId
}

// sanitizeHeaders 对敏感头部信息进行脱敏处理
func sanitizeHeaders(headers map[string][]string) map[string][]string {
	sanitized := make(map[string][]string, len(headers))
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if isSensitiveHeader(lowerKey) {
			maskedValues := make([]string, len(values))
			for i, v := range values {
				maskedValues[i] = maskValue(v)
			}
			sanitized[key] = maskedValues
		} else {
			sanitized[key] = values
		}
	}
	return sanitized
}

func isSensitiveHeader(key string) bool {
	return key == "authorization" ||
		key == "cookie" ||
		key == "proxy-authorization" ||
		key == "x-auth-token" ||
		key == "x-api-key" ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password")
}

func maskValue(v string) string {
	l := len(v)
	if l == 0 {
		return ""
	}
	if l <= 8 {
		return "***"
	}
	return v[:4] + "...***"
}
