// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"testing"

	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件锁定 recover 相关 5 处 panic 的裁决结论（PRD C1）。
//
// 分两类，处置方式不同：
//
//   - recover_error_handler_impl.go:107,116,123 是**装配期**（中间件注册期）
//     失败，按条件 4 补 RecoverMiddlewareWithError 姊妹，原入口保留 panic。
//   - recover_gin_impl.go:123 / recover_fiber_impl.go:133 的 TraceID 是
//     **运行期** panic。条件 4 明言运行期 panic 始终不满足本条
//     （feature-status.md:124），因此不是补姊妹，而是改为降级返回。

// --- 装配期：RecoverMiddlewareWithError -------------------------------------

// TestRecoverMiddlewareWithError_ReportsMissingManager 管理器未设置时返回错误。
func TestRecoverMiddlewareWithError_ReportsMissingManager(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	handler := &ErrorHandler{AppCtx: ctx}

	middleware, err := handler.RecoverMiddlewareWithError()

	require.Error(t, err, "a missing recover manager must be reported")
	assert.ErrorIs(t, err, ErrRecoverMiddlewareUnavailable)
	assert.Nil(t, middleware)
}

// TestRecoverMiddlewareWithError_ReportsNonIRecoverProvider 加载结果类型不符。
func TestRecoverMiddlewareWithError_ReportsNonIRecoverProvider(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	handler := &ErrorHandler{
		AppCtx: ctx,
		recoverManager: &task5RecoverManager{
			IProviderManager: NewProviderManager(ctx),
			recovery:         nil,
		},
	}

	middleware, err := handler.RecoverMiddlewareWithError()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRecoverMiddlewareUnavailable)
	assert.ErrorContains(t, err, "IRecover")
	assert.Nil(t, middleware)
}

// TestRecoverMiddleware_PreservesOriginalPanicValues 原入口的 panic 值逐字不变。
//
// 委派给姊妹方法之后，panic 值必须还原成旧实现抛出的那个具体值——调用方
// （含既有测试 TestErrorHandler_RejectsMissingOrInvalidRecoveryManager）依赖
// 它。统一抛出包装后的 error 会静默改变这些调用方的行为。
func TestRecoverMiddleware_PreservesOriginalPanicValues(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)

	assert.PanicsWithValue(t, msgRecoverManagerNotSet, func() {
		(&ErrorHandler{AppCtx: ctx}).RecoverMiddleware()
	}, "the missing-manager panic value must stay verbatim")

	assert.PanicsWithValue(t, msgRecoverNotIRecover, func() {
		(&ErrorHandler{
			AppCtx: ctx,
			recoverManager: &task5RecoverManager{
				IProviderManager: NewProviderManager(ctx),
				recovery:         nil,
			},
		}).RecoverMiddleware()
	}, "the wrong-provider panic value must stay verbatim")
}

// TestRecoverMiddlewareWithError_ReturnsMiddlewareOnSuccess 正常路径返回中间件。
func TestRecoverMiddlewareWithError_ReturnsMiddlewareOnSuccess(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	handler := newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))

	middleware, err := handler.RecoverMiddlewareWithError()

	require.NoError(t, err)
	assert.NotNil(t, middleware, "a successful resolution must return the middleware")
}

// TestErrRecoverMiddlewareUnavailable_IsClassifiable 哨兵必须可分类。
func TestErrRecoverMiddlewareUnavailable_IsClassifiable(t *testing.T) {
	assert.NotEmpty(t, ErrRecoverMiddlewareUnavailable.Error())
	assert.True(t, errors.Is(
		errors.Join(ErrRecoverMiddlewareUnavailable, errors.New("other")),
		ErrRecoverMiddlewareUnavailable))
}

// --- 运行期：TraceID 降级而非 panic ------------------------------------------

// wrongCoreContext 是携带错误原生上下文的 ICoreContext 替身。
type wrongCoreContext struct {
	adaptorctx.ICoreContext
}

func (c *wrongCoreContext) GetCtx() any { return "not a framework context" }

// TestTraceID_DowngradesInsteadOfPanicking 上下文类型不符时返回空串。
//
// TraceID 在 panic 恢复路径上被调用：此时进程正在处理一个已经出错的请求，
// 再抛一次 panic 只会让恢复中间件自身崩溃。返回空值降级，与 hertzcore 的
// 同名实现一致（hertz_recovery.go:121-123）。
func TestTraceID_DowngradesInsteadOfPanicking(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	wrong := &wrongCoreContext{}

	for name, recovery := range map[string]IRecover{
		"fiber": NewFiberRecovery(ctx),
		"gin":   NewGinRecovery(ctx),
	} {
		t.Run(name, func(t *testing.T) {
			var traceID string
			require.NotPanics(t, func() { traceID = recovery.TraceID(wrong) },
				"a runtime type mismatch must not panic inside the recovery path")
			assert.Empty(t, traceID, "an unusable context must yield an empty trace id")
		})
	}
}
