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

// 本文件锁定 frame_starter_impl.go 两处 panic 的裁决结论（PRD C1）。
//
// 两处的处置方式不同：
//
//   - :242 RegisterGlobalInitializers 是装配期的顺序错误（未先
//     RegisterApplication），按条件 4 补 *WithError 姊妹，原入口保留 panic。
//   - :367 的 worker 解析失败改为记录日志并返回，不再 panic：task server 是
//     可选能力（application.task.enableServer 控制），它起不来不该连带打断
//     整个应用启动——Web 服务本身仍可正常对外提供。

// TestRegisterGlobalInitializersWithError_ReportsMissingApplication 未注册应用时
// 返回错误。
func TestRegisterGlobalInitializersWithError_ReportsMissingApplication(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	frame := &FrameApplication{Ctx: ctx}

	err := frame.RegisterGlobalInitializersWithError()

	require.Error(t, err, "an unregistered application must be reported")
	assert.ErrorIs(t, err, ErrApplicationNotRegistered)
}

// TestRegisterGlobalInitializers_KeepsPanicking 原入口保留 panic，且文本逐字不变。
//
// 既有测试 TestFrameApplication_GuardsAvoidStartupSideEffects 以
// PanicsWithError 断言具体文本；委派给姊妹方法之后必须仍然匹配，否则等于
// 静默改变了调用方可见的行为。
func TestRegisterGlobalInitializers_KeepsPanicking(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	frame := &FrameApplication{Ctx: ctx}

	assert.PanicsWithError(t,
		"application that implements the ApplicationRegister interface is nil. Please RegisterApplication first",
		frame.RegisterGlobalInitializers,
		"the original panic text must survive the refactor verbatim")
}

// TestRegisterGlobalInitializersWithError_ShortCircuitsWhenServing 已进入服务阶段
// 时直接返回，不报错。
//
// 与原实现的守卫语义一致：装配阶段已过，重复注册是空操作而非错误。
func TestRegisterGlobalInitializersWithError_ShortCircuitsWhenServing(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	require.NoError(t, ctx.TransitionTo(PhaseServing))
	frame := &FrameApplication{Ctx: ctx}

	assert.NoError(t, frame.RegisterGlobalInitializersWithError(),
		"a serving application must short-circuit without reporting an error")
}

// TestFrameSentinels_AreDistinct 三个哨兵必须可区分。
func TestFrameSentinels_AreDistinct(t *testing.T) {
	assert.NotErrorIs(t, ErrApplicationNotRegistered, ErrNoFrameStarterOption)
	assert.NotErrorIs(t, ErrTaskWorkerUnavailable, ErrApplicationNotRegistered)
	for _, sentinel := range []error{
		ErrApplicationNotRegistered,
		ErrNoFrameStarterOption,
		ErrTaskWorkerUnavailable,
	} {
		assert.NotEmpty(t, sentinel.Error())
	}
}
