// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package context

// ICoreContext 统一核心的上下文包装器接口
type ICoreContext interface {
	// GetCtx 获取底层的原生上下文对象
	GetCtx() interface{}
	// GetHeader 获取请求头信息
	GetHeader(key string) string
	// SetHeader 设置响应头信息
	SetHeader(key string, value string)
	// JSON 以 JSON 格式响应数据
	JSON(statusCode int, data interface{}) error
	// Send 发送原始字节数据
	Send(statusCode int, body []byte) error
	// TODO 支持更多的方法： JSONP、XML、SendProto...
}
