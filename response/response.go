// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package response 提供了统一的HTTP响应格式和高性能的响应对象管理功能。
package response

import (
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
	"net/http"
	"sync"
)

// 响应对象池
var respPool = sync.Pool{
	New: func() interface{} {
		return &RespInfo{}
	},
}

// GetRespInfo 从对象池获取 RespInfo 实例
func GetRespInfo() *RespInfo {
	return respPool.Get().(*RespInfo)
}

type RespInfo struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// Release 释放 RespInfo 实例回对象池
func (r *RespInfo) Release() {
	// 重置字段避免数据泄露
	r.Code = 0
	r.Msg = ""
	r.Data = nil

	respPool.Put(r)
}

// Reset 重置 RespInfo 字段
func (r *RespInfo) Reset(code int, msg string, data interface{}) *RespInfo {
	r.Code = code
	r.Msg = msg
	r.Data = data
	return r
}

// NewRespInfo 创建新的 RespInfo 实例（使用对象池）
func NewRespInfo(code int, msg string, data ...interface{}) *RespInfo {
	resp := GetRespInfo()
	if len(data) > 0 {
		return resp.Reset(code, msg, data[0])
	}
	return resp.Reset(code, msg, nil)
}

// RespSuccess 创建成功响应（使用对象池）
func RespSuccess(data ...interface{}) *RespInfo {
	return NewRespInfo(0, "ok", data...)
}

// RespSuccessWithoutPool 创建成功响应（直接创建实例）
func RespSuccessWithoutPool(data ...interface{}) *RespInfo {
	return NewRespInfoWithoutPool(0, "ok", data...)
}

// RespError 创建错误响应（使用对象池）
func RespError(code int, msg string) *RespInfo {
	return NewRespInfo(code, msg, nil)
}

// RespErrorWithoutPool 创建错误响应（直接创建实例）
func RespErrorWithoutPool(code int, msg string) *RespInfo {
	return NewRespInfoWithoutPool(code, msg, nil)
}

// NewRespInfoWithoutPool 直接创建实例（不使用对象池，用于特殊场景）
func NewRespInfoWithoutPool(code int, msg string, data ...interface{}) *RespInfo {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	return &RespInfo{
		Code: code,
		Msg:  msg,
		Data: d,
	}
}

// SuccessWithoutPool 创建成功响应（使用对象池）
func SuccessWithoutPool(data ...interface{}) *RespInfo {
	return NewRespInfoWithoutPool(0, "ok", nil)
}

// ErrorWithoutPool 创建错误响应（使用对象池）
func ErrorWithoutPool(code int, msg string) *RespInfo {
	return NewRespInfoWithoutPool(code, msg, nil)
}

// JsonWithCtx 使用 Fiber 上下文返回 JSON 响应，并释放对象回池
//func (r *RespInfo) JsonWithCtx(c *fiber.Ctx, status ...int) error {
//	defer r.Release()
//	if len(status) > 0 {
//		return c.Status(status[0]).JSON(r)
//	}
//	return c.JSON(r)
//}

// JsonWithCtx 使用 ContextProvider 上下文提供者返回 JSON 响应，并释放对象回池  // TODO
func (r *RespInfo) JsonWithCtx(c providerCtx.ContextProvider, status ...int) error {
	defer r.Release()
	statusCode := http.StatusOK
	if len(status) > 0 {
		statusCode = status[0]
	}
	return c.JSON(statusCode, r)
}

// NewExceptionResp 异常专用的池化创建方法
func NewExceptionResp(code int, msg string, data ...interface{}) *RespInfo {
	return NewRespInfo(code, msg, data...)
}

// NewValidateExceptionResp 异常验证专用的池化创建方法
func NewValidateExceptionResp(code int, msg string, data ...interface{}) *RespInfo {
	return NewRespInfo(code, msg, data...)
}
