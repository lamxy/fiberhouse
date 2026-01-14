// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package response

import (
	"sync"

	"github.com/lamxy/fiberhouse/provider/context"
	"github.com/vmihailenco/msgpack/v5"
)

// RespInfoMagPack MessagePack格式的响应实现
type RespInfoMagPack struct {
	ri *RespInfo
}

// magPackPool MessagePack响应对象池
var magPackPool = sync.Pool{
	New: func() interface{} {
		return &RespInfoMagPack{
			ri: &RespInfo{},
		}
	},
}

// GetRespInfoMsgPack 从对象池创建MessagePack响应实例
func GetRespInfoMsgPack() IResponse {
	return magPackPool.Get().(*RespInfoMagPack)
}

// GetCode 获取响应码
func (r *RespInfoMagPack) GetCode() int {
	return r.ri.Code
}

// GetMsg 获取响应消息
func (r *RespInfoMagPack) GetMsg() string {
	return r.ri.Msg
}

// GetData 获取响应数据
func (r *RespInfoMagPack) GetData() interface{} {
	return r.ri.Data
}

// SendWithCtx 使用MessagePack格式发送响应
func (r *RespInfoMagPack) SendWithCtx(c context.ICoreContext, status ...int) error {
	statusCode := 200
	if len(status) > 0 {
		statusCode = status[0]
	}

	msgpackData := map[string]interface{}{
		"code": r.ri.Code,
		"msg":  r.ri.Msg,
	}
	if r.ri.Data != nil {
		msgpackData["data"] = r.ri.Data
	}

	data, err := msgpack.Marshal(msgpackData)
	if err != nil {
		return err
	}

	return c.Send(statusCode, data)
}

// JsonWithCtx 使用JSON格式发送响应
func (r *RespInfoMagPack) JsonWithCtx(c context.ICoreContext, status ...int) error {
	statusCode := 200
	if len(status) > 0 {
		statusCode = status[0]
	}

	jsonData := map[string]interface{}{
		"code": r.ri.Code,
		"msg":  r.ri.Msg,
	}
	if r.ri.Data != nil {
		jsonData["data"] = r.ri.Data
	}

	return c.JSON(statusCode, jsonData)
}

// Reset 重置响应内容
func (r *RespInfoMagPack) Reset(code int, msg string, data interface{}) IResponse {
	r.ri.Code = code
	r.ri.Msg = msg
	r.ri.Data = data
	return r
}

// Release 释放资源并放回对象池
func (r *RespInfoMagPack) Release() {
	// 重置内部数据
	r.ri.Code = 0
	r.ri.Msg = ""
	r.ri.Data = nil
	// 放回对象池
	magPackPool.Put(r)
}

// From 从另一个IResponse复制数据
func (r *RespInfoMagPack) From(resp IResponse, needToRelease bool) IResponse {
	r.Reset(resp.GetCode(), resp.GetMsg(), resp.GetData())

	if needToRelease {
		resp.Release()
	}
	return r
}

// ParseMsgPackResponse Go客户端解析MessagePack响应
func ParseMsgPackResponse(data []byte) (*RespInfo, error) {
	var result map[string]interface{}
	if err := msgpack.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	resp := &RespInfo{
		Code: int(result["code"].(int64)),
		Msg:  result["msg"].(string),
	}

	if dataValue, ok := result["data"]; ok {
		resp.Data = dataValue
	}

	return resp, nil
}

// ParseMsgPackResponseWithType Go客户端解析MessagePack响应并指定Data类型
func ParseMsgPackResponseWithType(data []byte, dataType interface{}) (*RespInfo, error) {
	var result map[string]interface{}
	if err := msgpack.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	resp := &RespInfo{
		Code: int(result["code"].(int64)),
		Msg:  result["msg"].(string),
	}

	if dataValue, ok := result["data"]; ok && dataType != nil {
		dataBytes, err := msgpack.Marshal(dataValue)
		if err != nil {
			return nil, err
		}
		if err := msgpack.Unmarshal(dataBytes, dataType); err != nil {
			return nil, err
		}
		resp.Data = dataType
	} else if ok {
		resp.Data = dataValue
	}

	return resp, nil
}

// SuccessWithData 成功时的响应，重置data字段
func (r *RespInfoMagPack) SuccessWithData(data ...interface{}) IResponse {
	if len(data) > 0 {
		r.ri.Data = data[0]
	}
	return r
}

// ErrorCustom 错误时的响应，重置code和msg字段
func (r *RespInfoMagPack) ErrorCustom(code int, msg string) IResponse {
	r.ri.Code = code
	r.ri.Msg = msg
	return r
}
