// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/lamxy/fiberhouse/response"
)

var (
	respInfoPManagerOnce     sync.Once
	respInfoPManagerInstance *RespInfoPManager
	// respInfoPManagerErr 保存单例解析失败的原因。
	//
	// 必须与实例一同缓存：sync.Once 只执行一次闭包，失败之后的调用不会重跑，
	// 若不记录错误，后续调用只能看到一个 nil 实例而不知为何为 nil。
	respInfoPManagerErr error
)

// ErrRespInfoPManagerUnavailable 标记响应信息管理器单例不可用。
//
// 两种装配失败：位点上没有任何管理器（应用未装配响应 provider 集合），
// 或位点首个管理器不是 *RespInfoPManager（装配了错误的管理器）。
var ErrRespInfoPManagerUnavailable = errors.New("fiberhouse: RespInfoPManager is unavailable")

// NewRespInfoPManagerOnce 获取响应信息提供者管理器单例
//
// 装配失败时 panic。需要处理该错误的调用方请改用
// NewRespInfoPManagerOnceWithError——这正是 feature-status.md 判定
// MsgPack / Protobuf 卡住条件 4 的那处缺口：此前唯一入口即 panic。
func NewRespInfoPManagerOnce() *RespInfoPManager {
	manager, err := NewRespInfoPManagerOnceWithError()
	if err != nil {
		panic(err)
	}
	return manager
}

// NewRespInfoPManagerOnceWithError 与 NewRespInfoPManagerOnce 相同，但装配失败时
// 返回错误而非 panic。
//
// 失败结果同样被 sync.Once 缓存：第一次调用失败后，后续调用返回同一个错误，
// 而不是重新尝试解析。这与容器的 Get 不同——位点绑定是启动期的一次性动作，
// 重试不会改变结果。
func NewRespInfoPManagerOnceWithError() (*RespInfoPManager, error) {
	respInfoPManagerOnce.Do(func() {
		managers := ProviderLocationDefault().LocationResponseInfoInit.GetManagers()
		if len(managers) == 0 {
			respInfoPManagerErr = fmt.Errorf(
				"%w: no RespInfoPManager found in ProviderLocationDefault().LocationResponseInfoInit",
				ErrRespInfoPManagerUnavailable)
			return
		}
		manager, ok := managers[0].(*RespInfoPManager)
		if !ok {
			respInfoPManagerErr = fmt.Errorf(
				"%w: first manager in ProviderLocationDefault().LocationResponseInfoInit is not a RespInfoPManager (got %T)",
				ErrRespInfoPManagerUnavailable, managers[0])
			return
		}
		respInfoPManagerInstance = manager
	})
	return respInfoPManagerInstance, respInfoPManagerErr
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
	if r.IResponse == nil {
		// 从新从对象池获取一个 IResponse 实例，确保是干净的对象
		r.IResponse = response.GetRespInfo()
	}
	return r
}

// SendWithCtx 使用核心上下文接口响应 JSON 数据
func (r *ResponseWrap) SendWithCtx(c adaptorctx.ICoreContext, status ...int) error {
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
				rpb, err := m.InitializeProvider(p)
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

// JsonWithCtx 使用核心上下文接口响应 JSON 数据
func (r *ResponseWrap) JsonWithCtx(c adaptorctx.ICoreContext, status ...int) error {
	defer r.Release()
	return r.IResponse.JsonWithCtx(c, status...)
}

// Release 释放 ResponseWrap 回对象池
func (r *ResponseWrap) Release() {
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
