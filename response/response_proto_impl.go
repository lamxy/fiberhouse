// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package response

import (
	"google.golang.org/protobuf/proto"
	"sync"

	providerctx "github.com/lamxy/fiberhouse/provider/context"
	pb "github.com/lamxy/fiberhouse/rpc/protosrc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"net/http"
)

type RespInfoPB struct {
	pb *pb.RespInfoProto
}

// respInfoPBPool 响应对象池
var respInfoPBPool = sync.Pool{
	New: func() interface{} {
		return &RespInfoPB{
			pb: &pb.RespInfoProto{},
		}
	},
}

// GetRespInfoPB 从对象池获取 RespInfoPB 实例
func GetRespInfoPB() IResponse {
	return respInfoPBPool.Get().(IResponse)
}

// ReleaseRespInfoPB 释放 RespInfoPB 实例回对象池
func ReleaseRespInfoPB(r *RespInfoPB) {
	r.pb.Code = 0
	r.pb.Msg = ""
	r.pb.Data = nil
	respInfoPBPool.Put(r)
}

// GetCode 获取响应码
func (r *RespInfoPB) GetCode() int {
	return int(r.pb.Code)
}

// GetMsg 获取响应消息
func (r *RespInfoPB) GetMsg() string {
	return r.pb.Msg
}

// GetData 获取响应数据
func (r *RespInfoPB) GetData() interface{} {
	if r.pb.Data != nil {
		var value structpb.Value
		if err := r.pb.Data.UnmarshalTo(&value); err == nil {
			return value.AsInterface()
		}
	}
	return nil
}

// SendWithCtx 返回 Protobuf 响应
func (r *RespInfoPB) SendWithCtx(c providerctx.ICoreContext, status ...int) error {
	defer r.Release()

	statusCode := http.StatusOK
	if len(status) > 0 {
		statusCode = status[0]
	}

	body, err := proto.Marshal(r.pb)
	if err != nil {
		return err
	}

	return c.Send(statusCode, body)
}

// JsonWithCtx 返回 JSON 响应
func (r *RespInfoPB) JsonWithCtx(c providerctx.ICoreContext, status ...int) error {
	defer r.Release()

	statusCode := http.StatusOK
	if len(status) > 0 {
		statusCode = status[0]
	}

	result := map[string]interface{}{
		"code": r.GetCode(),
		"msg":  r.GetMsg(),
		"data": r.GetData(),
	}

	return c.JSON(statusCode, result)
}

// Reset 重置响应内容
func (r *RespInfoPB) Reset(code int, msg string, data interface{}) IResponse {
	r.pb.Code = int32(code)
	r.pb.Msg = msg

	if data != nil {
		value, err := structpb.NewValue(data)
		if err == nil {
			anyData, err := anypb.New(value)
			if err == nil {
				r.pb.Data = anyData
			}
		}
	} else {
		r.pb.Data = nil
	}

	return r
}

// Release 释放资源回对象池
func (r *RespInfoPB) Release() {
	ReleaseRespInfoPB(r)
}

// From 从另一个 IResponse 复制数据
func (r *RespInfoPB) From(resp IResponse, needToRelease bool) IResponse {
	r.Reset(resp.GetCode(), resp.GetMsg(), resp.GetData())

	if needToRelease {
		resp.Release()
	}
	return r
}

// SuccessWithData 成功时的响应，重置data字段
func (r *RespInfoPB) SuccessWithData(data ...interface{}) IResponse {
	if len(data) > 0 {
		d := data[0]
		if d != nil {
			value, err := structpb.NewValue(d)
			if err == nil {
				anyData, err := anypb.New(value)
				if err == nil {
					r.pb.Data = anyData
				}
			}
		} else {
			r.pb.Data = nil
		}
	}
	return r
}

// ErrorCustom 错误时的响应，重置code和msg字段
func (r *RespInfoPB) ErrorCustom(code int, msg string) IResponse {
	r.pb.Code = int32(code)
	r.pb.Msg = msg
	return r
}

// ToProto 将 RespInfo 转换为 RespInfoProto (支持标量、map、list)
//func (r *RespInfo) ToProto() (*pb.RespInfoProto, error) {
//	pbResp := GetPbRespInfo()
//	pbResp.Code = int32(r.Code)
//	pbResp.Msg = r.Msg
//
//	if r.Data != nil {
//		// 使用 structpb.NewValue 支持所有类型
//		value, err := structpb.NewValue(r.Data)
//		if err != nil {
//			return pbResp, err
//		}
//
//		// 将 structpb.Value 包装到 Any
//		anyData, err := anypb.New(value)
//		if err != nil {
//			return pbResp, err
//		}
//		pbResp.Data = anyData
//	}
//
//	return pbResp, nil
//}
//
//// FromProto 从 RespInfoProto 转换为 RespInfo
//func FromProto(pbResp *pb.RespInfoProto) (*RespInfo, error) {
//	resp := GetRespInfo()
//	resp.Code = int(pbResp.Code)
//	resp.Msg = pbResp.Msg
//
//	if pbResp.Data != nil {
//		var value structpb.Value
//		if err := pbResp.Data.UnmarshalTo(&value); err != nil {
//			return resp, err
//		}
//		resp.Data = value.AsInterface()
//	}
//
//	return resp, nil
//}
//
//// ProtoWithCtx 使用 ICoreContext 返回 Protobuf 响应
//func (r *RespInfo) ProtoWithCtx(c providerctx.ICoreContext, status ...int) error {
//	defer r.Release()
//
//	pbResp, err := r.ToProto()
//	if err != nil {
//		return err
//	}
//	defer ReleasePbRespInfo(pbResp)
//
//	statusCode := http.StatusOK
//	if len(status) > 0 {
//		statusCode = status[0]
//	}
//
//	return c.Proto(statusCode, pbResp)
//}
