// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件锁定 CoreWithFiber 的 initErr 错误状态机（PRD C3）。
//
// Gin 有 13 处 initErr 使用、hertzcore 有 15 处，Fiber **完全没有**：装配失败时
// 只能 panic，调用方无从选择。这不是增量补齐而是新建一套机制。
//
// 契约与 Gin 对齐：
//   - 装配失败记录 initErr，而非 panic。
//   - initializationFailed() 为真时，后续生命周期方法一律短路。
//   - AppCoreRun / Shutdown 把 initErr 作为错误返回，使失败可被调用方观测。
//
// 原 panic 行为经 InitCoreAppMust 保留，满足条件 4 的 Must*/普通入口成对惯例。

// TestCoreFiber_InitCoreAppRecordsCodecFailureAsInitErr 装配期找不到 codec 管理器
// 时必须记录 initErr，而不是 panic。
func TestCoreFiber_InitCoreAppRecordsCodecFailureAsInitErr(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)

	// 传入一个位点不匹配的管理器：resolveJSONCodec 找不到 codec 管理器，
	// 这是旧实现三处 panic 中的第一处。
	mismatched := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerRun,
	}

	require.NotPanics(t, func() {
		core.InitCoreApp(&task4Frame{}, mismatched)
	}, "a missing JSON codec manager must not panic the assembly path")

	require.True(t, core.initializationFailed(),
		"the failure must be recorded as initErr")
	assert.ErrorIs(t, core.initErr, ErrCoreInitFailed)
	assert.ErrorContains(t, core.initErr, "JSON codec",
		"the error must say what part of assembly failed")
}

// TestCoreFiber_InitCoreAppRecordsCodecLoadError codec 管理器加载失败同样记录。
func TestCoreFiber_InitCoreAppRecordsCodecLoadError(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)

	failing := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupTrafficCodecChoose,
		location: ProviderLocationDefault().LocationCoreCodecInit,
		err:      errTask7Provider,
	}

	require.NotPanics(t, func() {
		core.InitCoreApp(&task4Frame{}, failing)
	})

	require.True(t, core.initializationFailed())
	assert.ErrorIs(t, core.initErr, errTask7Provider,
		"the provider's original error must stay in the chain")
}

// TestCoreFiber_InitCoreAppRecordsCodecTypeMismatch 加载结果类型不符同样记录。
func TestCoreFiber_InitCoreAppRecordsCodecTypeMismatch(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)

	wrongType := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupTrafficCodecChoose,
		location: ProviderLocationDefault().LocationCoreCodecInit,
	}
	// task4LifecycleManager 的 LoadProvider 在无 loadFunc 时返回 nil，
	// 不满足 JsonWrapper，命中第三处旧 panic。

	require.NotPanics(t, func() {
		core.InitCoreApp(&task4Frame{}, wrongType)
	})

	require.True(t, core.initializationFailed())
	assert.ErrorContains(t, core.initErr, "JsonWrapper",
		"the error must name the interface the provider failed to satisfy")
}

// TestCoreFiber_FailedInitShortCircuitsLifecycle 装配失败后，后续生命周期方法
// 必须全部短路——与 Gin 的 initializationFailed() 守卫语义一致。
func TestCoreFiber_FailedInitShortCircuitsLifecycle(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.initErr = ErrCoreInitFailed

	application := &task7Application{}
	frame := &task4Frame{application: application}

	require.NotPanics(t, func() {
		core.InitCoreApp(frame, task4GoodCodecManager())
		core.RegisterAppMiddleware(frame)
		core.RegisterModuleInitialize(frame)
		core.RegisterModuleSwagger(frame)
		core.RegisterAppHooks(frame)
	})

	assert.Nil(t, core.coreApp,
		"a failed core must not go on to build the Fiber app")
	assert.Zero(t, application.middlewareCalls,
		"registrars must not run after assembly failed")
	assert.Zero(t, application.hookCalls)
}

// TestCoreFiber_AppCoreRunReturnsInitErr 运行入口必须把装配失败作为错误返回。
//
// 这是 initErr 机制的价值所在：调用方（boot.go 的 coordinateServerRun）因此能
// 观测到失败，而不是拿到一个静默不工作的核心。
func TestCoreFiber_AppCoreRunReturnsInitErr(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.initErr = ErrCoreInitFailed

	err := core.AppCoreRun()

	require.Error(t, err, "AppCoreRun must surface the assembly failure")
	assert.ErrorIs(t, err, ErrCoreInitFailed)
}

// TestCoreFiber_ShutdownReturnsInitErr 关闭入口同样返回 initErr。
func TestCoreFiber_ShutdownReturnsInitErr(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.initErr = ErrCoreInitFailed

	err := core.Shutdown()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCoreInitFailed)
}

// TestCoreFiber_InitCoreAppMustPreservesPanicBehaviour 原 panic 行为保留在 Must 入口。
//
// 条件 4 的要求是「成对出现」：panic 入口可以存在，但必须同时有返回 error 的
// 替代入口。这里确认 Must 入口确实仍然 panic——否则依赖 fail-fast 的调用方会
// 在装配失败时静默继续。
func TestCoreFiber_InitCoreAppMustPreservesPanicBehaviour(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)

	mismatched := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerRun,
	}

	assert.Panics(t, func() {
		core.InitCoreAppMust(&task4Frame{}, mismatched)
	}, "the Must entry point must keep the fail-fast behaviour")
}

// TestCoreFiber_InitCoreAppMustSucceedsOnHappyPath Must 入口在正常路径不 panic。
func TestCoreFiber_InitCoreAppMustSucceedsOnHappyPath(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)

	require.NotPanics(t, func() {
		core.InitCoreAppMust(&task4Frame{}, task4GoodCodecManager())
	})
	assert.NotNil(t, core.coreApp)
	assert.False(t, core.initializationFailed())
}

// TestCoreFiber_SuccessfulInitLeavesNoError 正常装配不得留下 initErr。
func TestCoreFiber_SuccessfulInitLeavesNoError(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)

	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())

	assert.False(t, core.initializationFailed())
	assert.NoError(t, core.initErr)
	assert.NotNil(t, core.coreApp)
}
