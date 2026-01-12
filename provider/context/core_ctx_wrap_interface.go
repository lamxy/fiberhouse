// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package context

// ICoreContext 统一核心的上下文接口
type ICoreContext interface {
	GetCtx() interface{}
	GetHeader(key string) string
	SetHeader(key string, value string)
	JSON(statusCode int, data interface{}) error
	Send(statusCode int, body []byte) error
	// TODO 支持更多的方法： JSONP、XML、SendProto...
}
