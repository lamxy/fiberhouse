// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package response

import adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"

type IResponse interface {
	GetCode() int
	GetMsg() string
	GetData() interface{}
	SendWithCtx(c adaptorctx.ICoreContext, status ...int) error
	JsonWithCtx(c adaptorctx.ICoreContext, status ...int) error
	Reset(code int, msg string, data interface{}) IResponse
	Release()
	From(resp IResponse, needToRelease bool) IResponse
	SuccessWithData(data ...interface{}) IResponse
	ErrorCustom(code int, msg string) IResponse
}
