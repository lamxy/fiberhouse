// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"net/http"
	"testing"

	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// facadeCoreContextRecorder 是 adaptorctx.ICoreContext 的最小记录实现，
// 供不需要真实 HTTP 内核的门面用例使用。真实内核协商路径由
// response_facade_test.go 的 Fiber/Gin 用例覆盖，这里只钉住门面自身的转发契约。
type facadeCoreContextRecorder struct {
	status     int
	body       []byte
	jsonValue  interface{}
	headers    map[string]string
	sendCalled bool
	jsonCalled bool
}

func newFacadeCoreContextRecorder() *facadeCoreContextRecorder {
	return &facadeCoreContextRecorder{headers: make(map[string]string)}
}

func (c *facadeCoreContextRecorder) GetCtx() interface{} { return nil }
func (c *facadeCoreContextRecorder) GetHeader(key string) string {
	return c.headers[key]
}
func (c *facadeCoreContextRecorder) SetHeader(key string, value string) {
	c.headers[key] = value
}
func (c *facadeCoreContextRecorder) JSON(statusCode int, data interface{}) error {
	c.jsonCalled = true
	c.status = statusCode
	c.jsonValue = data
	return nil
}
func (c *facadeCoreContextRecorder) Send(statusCode int, body []byte) error {
	c.sendCalled = true
	c.status = statusCode
	c.body = append([]byte(nil), body...)
	return nil
}

var _ adaptorctx.ICoreContext = (*facadeCoreContextRecorder)(nil)

// TestResponseFacade_JsonWithCtxBypassesNegotiationAndReleases 钉住 JsonWithCtx 的契约：
// 它是"绕过二进制协商、强制 JSON"的显式入口，即便请求头要求 msgpack 也必须走 JSON，
// 且返回后必须把 ResponseWrap 归还对象池。
func TestResponseFacade_JsonWithCtxBypassesNegotiationAndReleases(t *testing.T) {
	ctx := newTask5AppContext(t, false, true)
	installTask5ResponseManager(t, ctx)

	recorder := newFacadeCoreContextRecorder()
	recorder.SetHeader("Accept", "application/msgpack")

	wrap := Response().Reset(703, "json-forced", map[string]interface{}{"ok": true})
	responseWrap, ok := wrap.(*ResponseWrap)
	require.True(t, ok)

	require.NoError(t, responseWrap.JsonWithCtx(recorder, http.StatusCreated))

	assert.True(t, recorder.jsonCalled, "JsonWithCtx must use the JSON path")
	assert.False(t, recorder.sendCalled, "JsonWithCtx must not emit a binary body")
	assert.Equal(t, http.StatusCreated, recorder.status)
	assert.Nil(t, responseWrap.IResponse, "JsonWithCtx must release the wrap back to the pool")
}

// TestResponseFacade_SuccessWithDataAndErrorCustomForwardToInner 覆盖门面的两个
// 便捷构造方法：它们必须把调用转发给内层 IResponse 并返回门面自身以支持链式调用。
func TestResponseFacade_SuccessWithDataAndErrorCustomForwardToInner(t *testing.T) {
	t.Run("SuccessWithData forwards the payload", func(t *testing.T) {
		wrap := Response()
		returned := wrap.SuccessWithData(map[string]interface{}{"id": 7})
		t.Cleanup(wrap.Release)

		assert.Same(t, wrap, returned, "SuccessWithData must return the facade for chaining")
		data, ok := wrap.GetData().(map[string]interface{})
		require.True(t, ok)
		assert.EqualValues(t, 7, data["id"])
	})

	t.Run("SuccessWithData with no argument clears stale data", func(t *testing.T) {
		wrap := Response()
		t.Cleanup(wrap.Release)
		wrap.SuccessWithData(map[string]interface{}{"stale": true})
		wrap.SuccessWithData()
		assert.Nil(t, wrap.GetData(), "a no-arg call must not leak the previous payload")
	})

	t.Run("ErrorCustom forwards code and message", func(t *testing.T) {
		wrap := Response()
		returned := wrap.ErrorCustom(4001, "bad input")
		t.Cleanup(wrap.Release)

		assert.Same(t, wrap, returned, "ErrorCustom must return the facade for chaining")
		assert.Equal(t, 4001, wrap.GetCode())
		assert.Equal(t, "bad input", wrap.GetMsg())
	})
}

// TestResponseFacade_ConstructorsReturnDistinctPooledInstances 覆盖三个此前 0% 的
// 构造入口。它们各自从对应的对象池取实例，调用方负责 Release；这里同时钉住
// "不同入口返回不同实现"这一事实，避免将来误把三者合并。
func TestResponseFacade_ConstructorsReturnDistinctPooledInstances(t *testing.T) {
	respInfo := RespInfo()
	require.NotNil(t, respInfo)
	t.Cleanup(respInfo.Release)

	respProto := RespProto()
	require.NotNil(t, respProto)
	t.Cleanup(respProto.Release)

	respMsgpack := RespMsgpack()
	require.NotNil(t, respMsgpack)
	t.Cleanup(respMsgpack.Release)

	assert.IsType(t, &response.RespInfo{}, respInfo)
	assert.NotSame(t, any(respInfo), any(respProto))
	assert.NotSame(t, any(respProto), any(respMsgpack))

	// 三者都实现同一个响应接口，可在同一条发送路径上互换。
	for name, candidate := range map[string]response.IResponse{
		"RespInfo":    respInfo,
		"RespProto":   respProto,
		"RespMsgpack": respMsgpack,
	} {
		t.Run(name+" satisfies IResponse", func(t *testing.T) {
			candidate.Reset(700, name, nil)
			assert.Equal(t, 700, candidate.GetCode())
			assert.Equal(t, name, candidate.GetMsg())
		})
	}
}
