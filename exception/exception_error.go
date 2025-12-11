// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package exception 提供异常处理和错误响应功能，支持业务异常和验证异常两种类型。
// 支持从全局管理器中获取预定义异常，也支持自定义异常创建和抛出。
package exception

import (
	"fmt"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
	"github.com/lamxy/fiberhouse/response"
	"net/http"
)

/*
基本用法：

app.Get("/recover", func(c *fiber.Ctx) error {
	p := c.Query("ok", "")
	if p == "" {
		panic(exception.Get("Some exception key")) // panic抛出业务错误
	} else if p == "custom" {
		// panic(exception.Throw("InputParamError"))
		exception.Panic("InputParamError") // panic抛出业务异常
		// or
		exception.Throw("InputParamError") // panic抛出业务异常
	} else if p == "default" {
		return errors.New("default error return")  // 直接返回错误
	}
	return c.JSON(fiber.Map{"code":0, "msg":"ok"})  // 成功返回
	// or
	response.RespSuccess().JsonWithCtx(c) // 成功返回
})
*/

func (e *Exception) Error() string {
	return e.Msg
}

func New(c int, m string, d ...interface{}) *Exception {
	if len(d) > 0 {
		data := d[0]
		if errData, ok := data.(error); ok {
			resp := response.NewExceptionResp(c, m, errData.Error())
			return (*Exception)(resp)
		}
		resp := response.NewExceptionResp(c, m, data)
		return (*Exception)(resp)
	}
	resp := response.NewExceptionResp(c, m, nil)
	return (*Exception)(resp)
}

func (e *Exception) Release() {
	(*response.RespInfo)(e).Release()
}

// Get 方法用于获取业务异常（直接 panic 后可被 recover 捕获）。
func Get(key string) *Exception {
	//exceptions := GetGlobalExceptions()
	gm := globalmanager.NewGlobalManagerOnce()
	k := constant.RegisterKeyPrefix + "exceptions"
	v, err := gm.Get(k)
	if err != nil || v == nil {
		panic(fmt.Errorf("exceptions: %s not found, please make sure you have registered exceptions in global manager", k))
	}
	exceptions := v.(ExceptionMap)
	if respInfo, ok := exceptions[key]; ok {
		return New(respInfo.Code, respInfo.Msg, respInfo.Data)
	}
	return New(constant.UnknownErrCode, constant.UnknownErrMsg)
}

// Throw 方法用于抛出业务异常（直接 panic）。
func Throw(key string, d ...interface{}) {
	gm := globalmanager.NewGlobalManagerOnce()
	k := constant.RegisterKeyPrefix + "exceptions"
	v, err := gm.Get(k)
	if err != nil || v == nil {
		panic(fmt.Errorf("exceptions: %s not found, please make sure you have registered exceptions in global manager", k))
	}
	exceptions := v.(ExceptionMap)
	if respInfo, ok := exceptions[k]; ok {
		if len(d) > 0 {
			if errData, ok := d[0].(error); ok {
				respInfo.Data = errData.Error()
			} else {
				respInfo.Data = d[0]
			}
		}
		panic(New(respInfo.Code, respInfo.Msg, respInfo.Data))
	}
	panic(New(constant.UnknownErrCode, constant.UnknownErrMsg))
}

// RespError 方法用于响应错误，并可添加数据参数
func (e *Exception) RespError(d ...interface{}) *Exception {
	if len(d) > 0 {
		if errData, ok := d[0].(error); ok {
			e.Data = errData.Error()
		} else {
			e.Data = d[0]
		}
	}
	return e
}

// Panic Exception 直接panic
func (e *Exception) Panic() {
	panic(e)
}

// JsonWithCtx Exception 使用 fiber 上下文响应 JSON
//func (e *Exception) JsonWithCtx(c *fiber.Ctx, status ...int) error {
//	defer e.Release()
//	if len(status) > 0 {
//		return c.Status(status[0]).JSON(e)
//	}
//	return c.JSON(e)
//}

// JsonWithCtx Exception 使用 ContextProvider 上下文响应 JSON
func (e *Exception) JsonWithCtx(c providerCtx.ContextProvider, status ...int) error {
	defer e.Release()
	statusCode := http.StatusOK
	if len(status) > 0 {
		statusCode = status[0]
	}
	return c.JSON(statusCode, e)
}

// GetInputError 常见异常错误方法
func GetInputError() *Exception {
	key := "InputParamError"
	return Get(key)
}

// GetNotFoundDocument 常见异常错误方法
func GetNotFoundDocument() *Exception {
	key := "NotFoundDocument"
	return Get(key)
}

// GetIllegalRequest 常见异常错误方法
func GetIllegalRequest() *Exception {
	key := "IllegalRequest"
	return Get(key)
}

func GetInternalError() *Exception {
	key := "InternalError"
	return Get(key)
}

func GetUnknownError() *Exception {
	key := "UnknownError"
	return Get(key)
}

// ----------- ValidateException ------------------

func (e *ValidateException) Error() string {
	return e.Msg
}

// Panic ValidateException 直接panic
func (e *ValidateException) Panic() {
	panic(e)
}

func NewVE(c int, m string, d ...interface{}) *ValidateException {
	if len(d) > 0 {
		data := d[0]
		if errData, ok := data.(error); ok {
			resp := response.NewValidateExceptionResp(c, m, errData.Error())
			return (*ValidateException)(resp)
		}
		resp := response.NewValidateExceptionResp(c, m, data)
		return (*ValidateException)(resp)
	}
	resp := response.NewValidateExceptionResp(c, m, nil)
	return (*ValidateException)(resp)
}

// VeGet 方法用于获取验证异常（直接 panic 后可被 recover 捕获）。
func VeGet(key string) *ValidateException {
	gm := globalmanager.NewGlobalManagerOnce()
	k := constant.RegisterKeyPrefix + "exceptions"
	v, err := gm.Get(k)
	if err != nil || v == nil {
		panic(fmt.Errorf("exceptions: %s not found, please make sure you have registered exceptions in global manager", k))
	}
	exceptions := v.(ExceptionMap)
	if respInfo, ok := exceptions[key]; ok {
		return NewVE(respInfo.Code, respInfo.Msg, respInfo.Data)
	}
	return NewVE(constant.UnknownErrCode, constant.UnknownErrMsg)
}

// VeThrow 方法用于抛出验证异常（直接 panic）。
func VeThrow(key string, d ...interface{}) {
	gm := globalmanager.NewGlobalManagerOnce()
	k := constant.RegisterKeyPrefix + "exceptions"
	v, err := gm.Get(k)
	if err != nil || v == nil {
		panic(fmt.Errorf("exceptions: %s not found, please make sure you have registered exceptions in global manager", k))
	}
	exceptions := v.(ExceptionMap)
	if respInfo, ok := exceptions[k]; ok {
		if len(d) > 0 {
			if errData, ok := d[0].(error); ok {
				respInfo.Data = errData.Error()
			} else {
				respInfo.Data = d[0]
			}
		}
		panic(NewVE(respInfo.Code, respInfo.Msg, respInfo.Data))
	}
	panic(NewVE(constant.UnknownErrCode, constant.UnknownErrMsg))
}

func (e *ValidateException) Release() {
	(*response.RespInfo)(e).Release()
}

// RespError 方法用于响应错误，并可添加数据参数
func (e *ValidateException) RespError(d ...interface{}) *ValidateException {
	if len(d) > 0 {
		if errData, ok := d[0].(error); ok {
			e.Data = errData.Error()
		} else {
			e.Data = d[0]
		}
	}
	return e
}

// JsonWithCtx ValidateException 使用 fiber 上下文响应 JSON
//func (e *ValidateException) JsonWithCtx(c *fiber.Ctx, status ...int) error {
//	defer e.Release()
//	if len(status) > 0 {
//		return c.Status(status[0]).JSON(e)
//	}
//	return c.JSON(e)
//}

// JsonWithCtx ValidateException 使用 ContextProvider 上下文响应 JSON
func (e *ValidateException) JsonWithCtx(c providerCtx.ContextProvider, status ...int) error {
	defer e.Release()
	statusCode := http.StatusOK
	if len(status) > 0 {
		statusCode = status[0]
	}
	return c.JSON(statusCode, e)
}

// VeGetInputError 常见验证类错误方法
func VeGetInputError() *ValidateException {
	key := "InputParamError"
	return VeGet(key)
}

func VeGetNotFoundError() *ValidateException {
	key := "NotFoundDocument"
	return VeGet(key)
}

func VeGetInternalError() *ValidateException {
	key := "InternalError"
	return VeGet(key)
}

func VeGetUnknownError() *ValidateException {
	key := "UnknownError"
	return VeGet(key)
}
