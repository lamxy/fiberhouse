// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// 本文件锁定 AppPhase 有序状态机的契约。
//
// 取代的旧实现是 `appState bool` + `sync.Once`：单一布尔承载三种互斥语义
// （「尚未装配」「正在服务」「已关闭」），且 sync.Once 使其首次写入即定值、无法
// 回退。后果有二：Shutdown 的守卫读到提前 pin 的 true 会直接返回并跳过资源回收；
// 「已开始接受请求」与「装配已完成」无法区分。
//
// AppPhase 把这三种语义拆成有序阶段，只允许正向迁移。

func newPhaseTestContext(t *testing.T) IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	cfg.Initialize()
	logger := zerolog.Nop()
	return NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

// TestAppPhase_StartsAtCreated 新建的上下文必须停在起始阶段。
func TestAppPhase_StartsAtCreated(t *testing.T) {
	ctx := newPhaseTestContext(t)

	assert.Equal(t, PhaseCreated, ctx.Phase())
	assert.False(t, ctx.IsServing(), "a freshly created context is not serving")
}

// TestAppPhase_ForwardTransitionsAreAccepted 覆盖完整的正向链路。
func TestAppPhase_ForwardTransitionsAreAccepted(t *testing.T) {
	ctx := newPhaseTestContext(t)

	for _, target := range []AppPhase{
		PhaseInitializing,
		PhaseServing,
		PhaseShuttingDown,
		PhaseTerminated,
	} {
		require.NoError(t, ctx.TransitionTo(target),
			"forward transition to %s must be accepted", target)
		assert.Equal(t, target, ctx.Phase())
	}
}

// TestAppPhase_SkippingForwardIsAllowed 允许跳跃式正向迁移。
//
// 装配失败时应用会从 PhaseInitializing 直接进入 PhaseTerminated，从不经过
// PhaseServing；若强制逐级递进，这条失败路径将无法表达。
func TestAppPhase_SkippingForwardIsAllowed(t *testing.T) {
	ctx := newPhaseTestContext(t)

	require.NoError(t, ctx.TransitionTo(PhaseInitializing))
	require.NoError(t, ctx.TransitionTo(PhaseTerminated),
		"a failed startup must be able to skip straight to Terminated")
	assert.Equal(t, PhaseTerminated, ctx.Phase())
}

// TestAppPhase_BackwardTransitionsAreRejected 逆向迁移必须返回 error 且不改变状态。
//
// 这是旧 sync.Once latch 最关键的替换点：错误要显式暴露，而不是静默吞掉。
func TestAppPhase_BackwardTransitionsAreRejected(t *testing.T) {
	ctx := newPhaseTestContext(t)
	require.NoError(t, ctx.TransitionTo(PhaseServing))

	for _, target := range []AppPhase{PhaseCreated, PhaseInitializing} {
		err := ctx.TransitionTo(target)

		require.Error(t, err, "backward transition to %s must be rejected", target)
		assert.Equal(t, PhaseServing, ctx.Phase(),
			"a rejected transition must leave the phase untouched")
	}
}

// TestAppPhase_SelfTransitionIsRejected 重复置位同一阶段同样视为非正向迁移。
//
// 保留这条断言是为了让「置位一次」成为调用方可依赖的事实：若自迁移静默成功，
// 重复调用 TransitionTo(PhaseServing) 就无法被测试区分。
func TestAppPhase_SelfTransitionIsRejected(t *testing.T) {
	ctx := newPhaseTestContext(t)
	require.NoError(t, ctx.TransitionTo(PhaseServing))

	require.Error(t, ctx.TransitionTo(PhaseServing),
		"re-entering the current phase is not a forward transition")
	assert.Equal(t, PhaseServing, ctx.Phase())
}

// TestAppPhase_IsServingOnlyDuringServing IsServing 必须严格对应 PhaseServing。
//
// 关闭开始后它必须转 false，否则关闭链会把「正在服务」误读成仍可接受流量。
func TestAppPhase_IsServingOnlyDuringServing(t *testing.T) {
	ctx := newPhaseTestContext(t)

	require.NoError(t, ctx.TransitionTo(PhaseInitializing))
	assert.False(t, ctx.IsServing(), "initializing is not serving")

	require.NoError(t, ctx.TransitionTo(PhaseServing))
	assert.True(t, ctx.IsServing(), "PhaseServing must report serving")

	require.NoError(t, ctx.TransitionTo(PhaseShuttingDown))
	assert.False(t, ctx.IsServing(), "shutting down is no longer serving")

	require.NoError(t, ctx.TransitionTo(PhaseTerminated))
	assert.False(t, ctx.IsServing(), "terminated is not serving")
}

// TestAppPhase_ConcurrentTransitionsElectSingleWinner 并发迁移只能有一个成功。
//
// 以 -race 执行。旧实现的 appState 是无保护的普通布尔读写，本用例在其上既会报
// data race 也无法保证单一胜者。
func TestAppPhase_ConcurrentTransitionsElectSingleWinner(t *testing.T) {
	ctx := newPhaseTestContext(t)

	const racers = 16
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
	)

	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func() {
			defer wg.Done()
			if err := ctx.TransitionTo(PhaseServing); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, succeeded,
		"exactly one racer may win the transition to PhaseServing")
	assert.Equal(t, PhaseServing, ctx.Phase())
}

// TestAppPhase_ConcurrentReadsAreRaceFree 读写并发必须无 data race。
func TestAppPhase_ConcurrentReadsAreRaceFree(t *testing.T) {
	ctx := newPhaseTestContext(t)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = ctx.Phase()
			_ = ctx.IsServing()
		}
	}()
	go func() {
		defer wg.Done()
		_ = ctx.TransitionTo(PhaseInitializing)
		_ = ctx.TransitionTo(PhaseServing)
		_ = ctx.TransitionTo(PhaseShuttingDown)
	}()
	wg.Wait()
}

// TestAppPhase_StringIsDiagnosable 阶段值出现在日志与 error 里，必须可读。
func TestAppPhase_StringIsDiagnosable(t *testing.T) {
	for phase, expected := range map[AppPhase]string{
		PhaseCreated:      "created",
		PhaseInitializing: "initializing",
		PhaseServing:      "serving",
		PhaseShuttingDown: "shutting_down",
		PhaseTerminated:   "terminated",
	} {
		assert.Equal(t, expected, phase.String())
	}

	assert.NotEmpty(t, AppPhase(99).String(),
		"an out-of-range phase must still render something diagnosable")
}

// TestAppPhase_TransitionErrorNamesBothPhases 迁移失败的 error 必须点名两端。
//
// 这类失败出现在启动/关闭链深处，只说「transition rejected」无法定位现场。
func TestAppPhase_TransitionErrorNamesBothPhases(t *testing.T) {
	ctx := newPhaseTestContext(t)
	require.NoError(t, ctx.TransitionTo(PhaseShuttingDown))

	err := ctx.TransitionTo(PhaseInitializing)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutting_down", "the error must name the current phase")
	assert.Contains(t, err.Error(), "initializing", "the error must name the rejected target")
}

// --- Deprecated 转接层 ---------------------------------------------------
//
// RegisterAppState/GetAppState 保留为转接层：仓库是已发布的 v1.x 模块，
// 移除两个导出方法会让下游实现 IApplicationContext 的代码编译失败，而
// .codegraph-qa-out/v1-production-readiness-tasklist.md 已把「AppState 语义重构」
// 划入 v2.0.0 范围。转接层把旧语义映射到新状态机，令二者不会各自为政。

// TestAppState_ShimReflectsPhase GetAppState 必须由 AppPhase 推导，而非独立字段。
func TestAppState_ShimReflectsPhase(t *testing.T) {
	ctx := newPhaseTestContext(t)

	assert.False(t, ctx.GetAppState(), "PhaseCreated maps to false")

	require.NoError(t, ctx.TransitionTo(PhaseInitializing))
	assert.False(t, ctx.GetAppState(), "PhaseInitializing is not yet 'started'")

	require.NoError(t, ctx.TransitionTo(PhaseServing))
	assert.True(t, ctx.GetAppState(), "PhaseServing maps to true")

	require.NoError(t, ctx.TransitionTo(PhaseShuttingDown))
	assert.True(t, ctx.GetAppState(),
		"the legacy boolean stays true once serving was reached: it means 'no longer assembling'")
}

// TestAppState_ShimRegistersServingPhase RegisterAppState(true) 必须驱动状态机。
func TestAppState_ShimRegistersServingPhase(t *testing.T) {
	ctx := newPhaseTestContext(t)

	ctx.RegisterAppState(true)

	assert.Equal(t, PhaseServing, ctx.Phase(),
		"the legacy setter must move the state machine, not a detached field")
	assert.True(t, ctx.IsServing())
}

// TestAppState_ShimIgnoresFalse RegisterAppState(false) 不得使状态倒退。
func TestAppState_ShimIgnoresFalse(t *testing.T) {
	ctx := newPhaseTestContext(t)
	require.NoError(t, ctx.TransitionTo(PhaseServing))

	ctx.RegisterAppState(false)

	assert.Equal(t, PhaseServing, ctx.Phase(),
		"the legacy setter must never drive the machine backwards")
}

// TestAppState_ShimLatchNoLongerSkipsShutdown 锁定旧 latch 缺陷已消失。
//
// 旧实现里使用方提前 pin RegisterAppState(true) 会让 Shutdown 的守卫直接返回、
// 跳过全部资源回收。改用有序阶段后，「已服务」不再等价于「可跳过关闭」——
// 关闭守卫读的是 PhaseShuttingDown/PhaseTerminated，而提前置位只能到达
// PhaseServing，因此关闭链照常执行。
func TestAppState_ShimLatchNoLongerSkipsShutdown(t *testing.T) {
	ctx := newPhaseTestContext(t)

	ctx.RegisterAppState(true)
	require.Equal(t, PhaseServing, ctx.Phase())

	require.NoError(t, ctx.TransitionTo(PhaseShuttingDown),
		"an early pin must not block the shutdown transition")
	assert.Equal(t, PhaseShuttingDown, ctx.Phase())
}
