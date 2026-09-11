// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package response

import (
	"testing"
)

// 本文件锁定两种二进制响应实现的对象池所有权**对称性**（PRD 任务 10 F2）。
//
// response-and-serialization.md 曾记载「Protobuf SendWithCtx 会释放自身，
// MsgPack 不会，因此两种二进制实现的所有权不对称」。复核后该描述已过时：
// 四个发送方法现在都以 defer r.Release() 开头（response_msgpack_impl.go:53,77、
// response_proto_impl.go:69,86）。
//
// 不对称是真实的风险来源——调用方无法对两种实现采用同一套释放约定，必然有一边
// 出现重复释放或未归还。因此把对称性转成可执行断言，避免再次漂移。
//
// 判据是「释放之后对象回到零值」：Release 把字段清空并归还池（:103、:123），
// 因此发送后读到零值即证明释放确实发生。

// binaryResponseFactories 两种二进制实现的构造入口。
//
// 用表驱动而非两段重复代码：对称性正是本文件要断言的东西，让两者跑**同一段**
// 断言，比分别写两份更能防止将来只改一边。
func binaryResponseFactories() map[string]func() IResponse {
	return map[string]func() IResponse{
		"msgpack":  GetRespInfoMsgPack,
		"protobuf": GetRespInfoPB,
	}
}

// TestBinaryResponses_ReleaseOnSendSymmetrically 两种实现的 SendWithCtx 都必须
// 释放自身。
func TestBinaryResponses_ReleaseOnSendSymmetrically(t *testing.T) {
	for name, newResponse := range binaryResponseFactories() {
		t.Run(name, func(t *testing.T) {
			resp := newResponse()
			if resp == nil {
				t.Fatal("the constructor must return a response")
			}
			resp.Reset(700, "ownership probe", nil)

			recorder := &responseContextRecorder{}
			if err := resp.SendWithCtx(recorder); err != nil {
				t.Fatalf("SendWithCtx() error = %v", err)
			}

			if len(recorder.body) == 0 {
				t.Fatal("the response body must have been written")
			}
			if got := resp.GetCode(); got != 0 {
				t.Fatalf("GetCode() after SendWithCtx = %d, want 0: "+
					"SendWithCtx must release the response back to the pool", got)
			}
		})
	}
}

// TestBinaryResponses_ReleaseOnJsonSymmetrically JsonWithCtx 同样对称。
func TestBinaryResponses_ReleaseOnJsonSymmetrically(t *testing.T) {
	for name, newResponse := range binaryResponseFactories() {
		t.Run(name, func(t *testing.T) {
			resp := newResponse()
			resp.Reset(701, "json ownership probe", nil)

			recorder := &responseContextRecorder{}
			if err := resp.JsonWithCtx(recorder); err != nil {
				t.Fatalf("JsonWithCtx() error = %v", err)
			}

			if got := resp.GetCode(); got != 0 {
				t.Fatalf("GetCode() after JsonWithCtx = %d, want 0: "+
					"JsonWithCtx must release the response back to the pool", got)
			}
		})
	}
}

// TestBinaryResponses_ReleaseIsIdempotent 重复释放必须安全。
//
// 发送方法已经释放过一次；facade 层还可能再释放一次（见
// response-and-serialization.md 记载的重复释放风险）。两种实现在这一点上同样
// 必须一致，否则调用方无法采用统一的释放约定。
func TestBinaryResponses_ReleaseIsIdempotent(t *testing.T) {
	for name, newResponse := range binaryResponseFactories() {
		t.Run(name, func(t *testing.T) {
			resp := newResponse()
			resp.Reset(702, "double release probe", nil)

			recorder := &responseContextRecorder{}
			if err := resp.SendWithCtx(recorder); err != nil {
				t.Fatalf("SendWithCtx() error = %v", err)
			}

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("a second Release panicked: %v", recovered)
				}
			}()
			resp.Release()
		})
	}
}
