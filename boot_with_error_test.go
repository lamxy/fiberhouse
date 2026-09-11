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

// 本文件锁定 boot.go 10 处装配期 panic 的替代入口（PRD C1）。
//
// 四个 resolve* 辅助方法在位点加载失败或类型不符时 panic，调用方（RunServer）
// 无从选择。现在每个都有 *WithError 姊妹，RunServerWithError 把它们的错误串起来
// 向调用方返回。
//
// 原入口的行为完全不变：RunServer 委派给 RunServerWithError，出错时 panic。
// 这正是 boot.go 自己的 GetValue / GetMustValue 惯例（:100 / :113），
// 也是条件 4 要求的成对模式。

// bootWithErrorManager 是绑定到指定位点的管理器替身，可注入加载错误或错误类型。
type bootWithErrorManager struct {
	IProviderManager
	typ      IProviderType
	location IProviderLocation
	result   any
	err      error
	unique   bool
}

func (m *bootWithErrorManager) Type() IProviderType         { return m.typ }
func (m *bootWithErrorManager) Location() IProviderLocation { return m.location }
func (m *bootWithErrorManager) IsUnique() bool              { return m.unique }
func (m *bootWithErrorManager) LoadProvider(...ProviderLoadFunc) (any, error) {
	return m.result, m.err
}

// bindBootManager 把管理器绑定到位点，并在用例结束后解绑。
//
// 位点是进程级单例（ProviderLocationDefault() 返回的对象在整个进程内共享），
// 不清理会污染同包其他用例——boot.go 的四个 resolve* 都读取这些位点。
func bindBootManager(t *testing.T, location IProviderLocation, manager IProviderManager) {
	t.Helper()
	require.NoError(t, location.Bind(manager))
	t.Cleanup(func() { unbindBootManager(t, location, manager) })
}

// unbindBootManager 把管理器从位点摘除。
//
// IProviderLocation 没有 Unbind——绑定在设计上是启动期的一次性动作。测试需要
// 复原进程级状态，因此在同包内直接操作 PLocation.managers，并持有它自己的锁。
func unbindBootManager(t *testing.T, location IProviderLocation, manager IProviderManager) {
	t.Helper()
	concrete, ok := location.(*PLocation)
	if !ok {
		return
	}
	concrete.mu.Lock()
	defer concrete.mu.Unlock()
	kept := concrete.managers[:0]
	for _, bound := range concrete.managers {
		if !sameProviderManagerInstance(bound, manager) {
			kept = append(kept, bound)
		}
	}
	concrete.managers = kept
}

func newBootWithErrorHouse(t *testing.T) *FiberHouse {
	t.Helper()
	ctx := newTask4InternalAppContext(t, nil)
	ctx.RegisterBootConfig(&BootConfig{CoreType: "fiber"})
	return &FiberHouse{AppCtx: ctx}
}

// --- resolveFrameStarterOptsWithError ---------------------------------------

// TestResolveFrameStarterOptsWithError_ReportsLoadFailure 位点加载失败返回 error。
func TestResolveFrameStarterOptsWithError_ReportsLoadFailure(t *testing.T) {
	fh := newBootWithErrorHouse(t)
	bindBootManager(t, ProviderLocationDefault().LocationFrameStarterOptionInit,
		&bootWithErrorManager{
			typ:      ProviderTypeDefault().GroupProviderAutoRun,
			location: ProviderLocationDefault().LocationFrameStarterOptionInit,
			err:      errTask7Provider,
		})

	opts, err := fh.resolveFrameStarterOptsWithError()

	require.Error(t, err)
	assert.ErrorIs(t, err, errTask7Provider, "the provider error must stay in the chain")
	assert.Nil(t, opts)
}

// TestResolveFrameStarterOptsWithError_ReportsTypeMismatch 类型不符返回 error。
func TestResolveFrameStarterOptsWithError_ReportsTypeMismatch(t *testing.T) {
	fh := newBootWithErrorHouse(t)
	bindBootManager(t, ProviderLocationDefault().LocationFrameStarterOptionInit,
		&bootWithErrorManager{
			typ:      ProviderTypeDefault().GroupProviderAutoRun,
			location: ProviderLocationDefault().LocationFrameStarterOptionInit,
			result:   "not options",
		})

	_, err := fh.resolveFrameStarterOptsWithError()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBootAssemblyFailed)
	assert.ErrorContains(t, err, "FrameStarterOption")
}

// TestResolveFrameStarterOpts_KeepsPanicking 原入口保留 panic。
func TestResolveFrameStarterOpts_KeepsPanicking(t *testing.T) {
	fh := newBootWithErrorHouse(t)
	bindBootManager(t, ProviderLocationDefault().LocationFrameStarterOptionInit,
		&bootWithErrorManager{
			typ:      ProviderTypeDefault().GroupProviderAutoRun,
			location: ProviderLocationDefault().LocationFrameStarterOptionInit,
			err:      errTask7Provider,
		})

	assert.Panics(t, func() { fh.resolveFrameStarterOpts() },
		"the original entry point must keep its fail-fast behaviour")
}

// --- resolveCoreStarterOptsWithError ----------------------------------------

// TestResolveCoreStarterOptsWithError_ReportsFailures 两种失败都返回 error。
func TestResolveCoreStarterOptsWithError_ReportsFailures(t *testing.T) {
	t.Run("load failure", func(t *testing.T) {
		fh := newBootWithErrorHouse(t)
		bindBootManager(t, ProviderLocationDefault().LocationCoreStarterOptionInit,
			&bootWithErrorManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationCoreStarterOptionInit,
				err:      errTask7Provider,
			})

		_, err := fh.resolveCoreStarterOptsWithError()

		require.Error(t, err)
		assert.ErrorIs(t, err, errTask7Provider)
	})

	t.Run("type mismatch", func(t *testing.T) {
		fh := newBootWithErrorHouse(t)
		bindBootManager(t, ProviderLocationDefault().LocationCoreStarterOptionInit,
			&bootWithErrorManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationCoreStarterOptionInit,
				result:   42,
			})

		_, err := fh.resolveCoreStarterOptsWithError()

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrBootAssemblyFailed)
		assert.ErrorContains(t, err, "CoreStarterOption")
	})
}

// --- resolveAndReturnFrameStarterWithError ----------------------------------

// TestResolveFrameStarterWithError_ReportsMissingManager 位点无管理器时返回 error。
//
// 这是旧实现里最容易踩到的一处：未装配 FrameStarterCreate 管理器时直接 panic，
// 而这恰恰是新用户最常见的配置遗漏。
func TestResolveFrameStarterWithError_ReportsMissingManager(t *testing.T) {
	fh := newBootWithErrorHouse(t)

	starter, err := fh.resolveAndReturnFrameStarterWithError(nil)

	require.Error(t, err, "an unconfigured FrameStarterCreate location must be reported")
	assert.ErrorIs(t, err, ErrBootAssemblyFailed)
	assert.ErrorContains(t, err, "LocationFrameStarterCreate")
	assert.Nil(t, starter)
}

// TestResolveFrameStarterWithError_ReportsLoadAndTypeFailures 加载与类型失败。
func TestResolveFrameStarterWithError_ReportsLoadAndTypeFailures(t *testing.T) {
	t.Run("load failure", func(t *testing.T) {
		fh := newBootWithErrorHouse(t)
		bindBootManager(t, ProviderLocationDefault().LocationFrameStarterCreate,
			&bootWithErrorManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationFrameStarterCreate,
				err:      errTask7Provider,
			})

		_, err := fh.resolveAndReturnFrameStarterWithError(nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errTask7Provider)
	})

	t.Run("type mismatch", func(t *testing.T) {
		fh := newBootWithErrorHouse(t)
		bindBootManager(t, ProviderLocationDefault().LocationFrameStarterCreate,
			&bootWithErrorManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationFrameStarterCreate,
				result:   "not a starter",
			})

		_, err := fh.resolveAndReturnFrameStarterWithError(nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrBootAssemblyFailed)
		assert.ErrorContains(t, err, "FrameStarter")
	})
}

// --- resolveAndReturnCoreStarterWithError -----------------------------------

// TestResolveCoreStarterWithError_ReportsMissingManager 位点无管理器时返回 error。
func TestResolveCoreStarterWithError_ReportsMissingManager(t *testing.T) {
	fh := newBootWithErrorHouse(t)

	starter, err := fh.resolveAndReturnCoreStarterWithError(nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBootAssemblyFailed)
	assert.ErrorContains(t, err, "LocationCoreStarterCreate")
	assert.Nil(t, starter)
}

// TestResolveCoreStarterWithError_ReportsLoadAndTypeFailures 加载与类型失败。
func TestResolveCoreStarterWithError_ReportsLoadAndTypeFailures(t *testing.T) {
	t.Run("load failure", func(t *testing.T) {
		fh := newBootWithErrorHouse(t)
		bindBootManager(t, ProviderLocationDefault().LocationCoreStarterCreate,
			&bootWithErrorManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationCoreStarterCreate,
				err:      errTask7Provider,
			})

		_, err := fh.resolveAndReturnCoreStarterWithError(nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, errTask7Provider)
	})

	t.Run("type mismatch", func(t *testing.T) {
		fh := newBootWithErrorHouse(t)
		bindBootManager(t, ProviderLocationDefault().LocationCoreStarterCreate,
			&bootWithErrorManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationCoreStarterCreate,
				result:   struct{}{},
			})

		_, err := fh.resolveAndReturnCoreStarterWithError(nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrBootAssemblyFailed)
		assert.ErrorContains(t, err, "CoreStarter")
	})
}

// --- RunServerWithError -----------------------------------------------------

// TestRunServerWithError_ReturnsAssemblyFailure 装配失败经由返回值传出。
//
// 这是本组改动的价值所在：此前装配失败只能让整个进程 panic，调用方既无法
// 记录自己的诊断，也无法执行自己的清理。
func TestRunServerWithError_ReturnsAssemblyFailure(t *testing.T) {
	fh := newBootWithErrorHouse(t)

	// 未绑定 FrameStarterCreate 管理器：装配必然失败，且不会走到监听。
	err := fh.RunServerWithError()

	require.Error(t, err, "RunServerWithError must surface assembly failures")
	assert.ErrorIs(t, err, ErrBootAssemblyFailed)
}

// TestRunServer_KeepsPanickingOnAssemblyFailure 原入口保留 panic。
//
// example_main/main.go 是唯一调用方，依赖装配失败时进程立即退出；
// 静默返回会让它带着半装配的应用继续运行。
func TestRunServer_KeepsPanickingOnAssemblyFailure(t *testing.T) {
	fh := newBootWithErrorHouse(t)

	assert.Panics(t, func() { fh.RunServer() },
		"RunServer must keep failing fast on assembly errors")
}

// TestErrBootAssemblyFailed_IsClassifiable 哨兵必须可分类。
func TestErrBootAssemblyFailed_IsClassifiable(t *testing.T) {
	assert.NotEmpty(t, ErrBootAssemblyFailed.Error())
}
