// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"github.com/lamxy/fiberhouse/bootstrap"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
)

// IErrorHandler 错误处理接口，用于统一定义堆栈日志记录及错误处理器的方法
type IErrorHandler interface {
	DefaultStackTraceHandler(providerctx.ICoreContext, interface{})
	ErrorHandler(providerctx.ICoreContext, error) error
	GetContext() IApplicationContext
	RecoverMiddleware(...RecoverConfig) any
}

// IRecover 恢复惊慌接口，用于获取不同框架的请求上下文中的参数、查询参数、获取tranceID以及定义恢复中间件方法
type IRecover interface {
	// GetParamsJson 获取路由参数的 JSON 编码字节切片
	GetParamsJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
	// GetQueriesJson 获取查询参数的 JSON 编码字节切片
	GetQueriesJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
	// GetHeadersJson 获取请求头的 JSON 编码字节切片（敏感信息脱敏）
	GetHeadersJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
	// RecoverPanic 返回恢复中间件函数，根据核心类型（如 fiber、gin）返回对应的中间件
	// 通过恢复中间件管理器依据启动配置选择相应的提供者自动返回对应的恢复中间件
	RecoverPanic(...RecoverConfig) any
	TraceID(ctx providerctx.ICoreContext, flag ...string) string
	GetHeader(ctx providerctx.ICoreContext, key string) string
}
