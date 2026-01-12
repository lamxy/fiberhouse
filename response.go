// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
	"github.com/lamxy/fiberhouse/response"
	"strings"
	"sync"
)

var (
	respInfoPManagerOnce     sync.Once
	respInfoPManagerInstance *RespInfoPManager
)

// NewRespInfoPManagerOnce 获取响应信息提供者管理器单例
func NewRespInfoPManagerOnce() *RespInfoPManager {
	respInfoPManagerOnce.Do(func() {
		managers := ProviderLocationDefault().LocationResponseInfoInit.GetManagers()
		if len(managers) > 0 {
			if manager, ok := managers[0].(*RespInfoPManager); ok {
				respInfoPManagerInstance = manager
				return
			} else {
				panic("first manager in ProviderLocationDefault().LocationResponseInfoInit is not a RespInfoPManager")
			}
		}
		panic("no RespInfoPManager found in ProviderLocationDefault().LocationResponseInfoInit")
	})
	return respInfoPManagerInstance
}

// ------------------------------------------------------------------------------------------------------------------

var _ response.IResponse = &ResponseWrap{}

// 响应包装对象池
var responseWrapPool = sync.Pool{
	New: func() interface{} {
		return &ResponseWrap{
			IResponse: response.GetRespInfo(),
		}
	},
}

// ResponseWrap 响应包装结构体
type ResponseWrap struct {
	response.IResponse
}

// Response 从对象池获取 ResponseWrap
func Response() *ResponseWrap {
	r := responseWrapPool.Get().(*ResponseWrap)
	// 从新从对象池获取一个 IResponse 实例，确保是干净的对象
	r.IResponse = response.GetRespInfo()
	return r
}

// SendWithCtx 使用核心上下文接口响应 JSON 数据
func (r *ResponseWrap) SendWithCtx(c providerctx.ICoreContext, status ...int) error {
	defer r.Release()

	// 获取响应信息提供者管理器单例
	m := NewRespInfoPManagerOnce()
	// 开启了二进制协议支持,则尝试使用对应的协议进行响应
	support := m.GetContext().(IApplicationContext).GetBootConfig().EnableBinaryProtocolSupport
	if support {
		// 获取Content-Type头部
		ct := c.GetHeader("Content-Type")
		if ct == "" {
			ct = c.GetHeader("Accept")
		}
		if ct != "" {
			// 处理多个 Accept 值和权重,提取主要的 MIME 类型
			ct = extractPrimaryMimeType(ct)
			if ct == "application/json" || ct == "application/*" || ct == "*/*" {
				return r.IResponse.JsonWithCtx(c, status...)
			}
			// 获取指定协议的响应信息提供者
			p, err := m.GetProvider(ct)
			if err == nil && p != nil {
				rpb, err := p.Initialize(m.GetContext())
				if err == nil && rpb != nil {
					if resp, ok := rpb.(response.IResponse); ok {
						// 设置响应内容类型
						c.SetHeader("Content-Type", ct)
						return resp.From(r.IResponse, true).SendWithCtx(c, status...)
					}
				}
			}
		}
	}
	// 默认使用 JSON 进行响应
	return r.IResponse.JsonWithCtx(c, status...)
}

// Release 释放 ResponseWrap 回对象池
func (r *ResponseWrap) Release() {
	if r.IResponse != nil {
		r.IResponse.Release()
	}
	r.IResponse = nil
	responseWrapPool.Put(r)
}

// Reset 重置 ResponseWrap
func (r *ResponseWrap) Reset(code int, msg string, data interface{}) response.IResponse {
	if r.IResponse != nil {
		r.IResponse.Reset(code, msg, data)
	}
	return r
}

// SuccessWithData 设置成功响应数据
func (r *ResponseWrap) SuccessWithData(data ...interface{}) response.IResponse {
	if r.IResponse != nil {
		r.IResponse.SuccessWithData(data...)
	}
	return r
}

// ErrorCustom 设置自定义错误响应
func (r *ResponseWrap) ErrorCustom(code int, message string) response.IResponse {
	if r.IResponse != nil {
		r.IResponse.ErrorCustom(code, message)
	}
	return r
}

// From 从另一个响应对象复制数据
func (r *ResponseWrap) From(resp response.IResponse, needToRelease bool) response.IResponse {
	if r.IResponse != nil {
		r.IResponse.From(resp, needToRelease)
	}
	return r
}

// extractPrimaryMimeType 从 Accept 头部提取主要的 MIME 类型
// 例如: "application/json;q=0.9, text/html" -> "application/json"
func extractPrimaryMimeType(accept string) string {
	// 取第一个值(通常是优先级最高的)
	if idx := strings.IndexByte(accept, ','); idx > 0 {
		accept = accept[:idx]
	}
	// 移除参数(如 charset, q值)
	if idx := strings.IndexByte(accept, ';'); idx > 0 {
		accept = accept[:idx]
	}
	return strings.TrimSpace(accept)
}

// -----------------------------------------------------------------------------------------------------------------

// Exception 获取异常响应对象
func Exception() *exception.Exception {
	return (*exception.Exception)(response.NewExceptionResp(constant.DefaultErrCode, "", nil))
}

// ValidateException 获取验证异常响应对象
func ValidateException() *exception.ValidateException {
	return (*exception.ValidateException)(response.NewExceptionResp(constant.DefaultErrCode, "", nil))
}

// RespInfo 获取 RespInfo 对象
func RespInfo() *response.RespInfo {
	return response.GetRespInfo()
}

// RespProto 获取 ProtoBuf 响应对象
func RespProto() response.IResponse {
	return response.GetRespInfoPB()
}

// RespMsgpack 获取 MsgPack 响应对象
func RespMsgpack() response.IResponse {
	return response.GetRespInfoMsgPack()
}
