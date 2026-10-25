// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package exception

import "github.com/lamxy/fiberhouse/response"

// Exception 定义业务异常，按20、40、50开头+系统+模块+功能点规范响应码
type Exception response.RespInfo

// ValidateException 定义接口参数校验异常
type ValidateException response.RespInfo

type ExceptionMap map[string]Exception

type ErrorData map[string]string

// Exception Error 实现 error 接口
func (e *Exception) Error() string {
	return e.Msg
}

// ValidateException Error 实现 error 接口
func (e *ValidateException) Error() string {
	return e.Msg
}
