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

// 本文件锁定 PRD C2 的 8 处同构 panic 的裁决结论。
//
// provider_impl.go:89,238,241 与 provider_manager_impl.go:52,227,241,290,293
// 在结构上与已判定合规的 provider_type.go Must* 系列相同——皆为装配期的程序
// 设计错误（"type is not set"、"cannot bind unique provider"、MountToParent
// 参数错误）。
//
// C2 的验收标准是「必须给出与同构代码一致的结论」。provider_type.go 的结论是
// 「保留 panic，但提供返回 error 的替代入口」（Default/Custom/Type ←
// MustDefault/MustCustom/MustType）。因此这 8 处采取同一结论：原方法保留
// panic 与签名不变，另补 *WithError 姊妹。
//
// 之所以补姊妹而非改造原方法：这六个方法都在链式调用中使用
// （NewXxx().SetName().SetType()、son.MountToParent(son)），改签名会波及全部
// 构造子。与 NewMysqlModelWithError / NewMongoModelWithError 同一模式。

// --- Provider.Check ---------------------------------------------------------

// TestProviderCheckWithError_ReportsMissingType 未设置类型时返回 error 而非 panic。
func TestProviderCheckWithError_ReportsMissingType(t *testing.T) {
	provider := NewProvider().SetName("no-type")

	proceed, err := provider.CheckWithError()

	require.Error(t, err, "a provider without a type must report the error")
	assert.ErrorIs(t, err, ErrProviderTypeNotSet)
	assert.ErrorContains(t, err, "no-type", "the error must name the provider")
	assert.False(t, proceed, "a failed check must not tell the caller to proceed")
}

// TestProviderCheckWithError_MatchesCheckOnSuccess 正常路径与 Check 结论一致。
func TestProviderCheckWithError_MatchesCheckOnSuccess(t *testing.T) {
	provider := NewProvider().
		SetName("typed").
		SetType(&PType{id: CustomTypeStart, name: "custom"})

	proceed, err := provider.CheckWithError()

	require.NoError(t, err)
	assert.True(t, proceed, "a pending typed provider must be told to proceed")
	assert.Equal(t, provider.Check(), proceed,
		"the WithError sibling must agree with Check on the success path")
}

// TestProviderCheckWithError_AgreesOnTerminalStates 终态下两者同样一致。
func TestProviderCheckWithError_AgreesOnTerminalStates(t *testing.T) {
	for _, state := range []State{StateLoaded, StateFailed, StateSkipped} {
		provider := NewProvider().
			SetName("terminal").
			SetType(&PType{id: CustomTypeStart, name: "custom"}).(*Provider)
		provider.setLifecycleStatus(state)

		proceed, err := provider.CheckWithError()

		require.NoError(t, err, "a terminal state is not an error")
		assert.False(t, proceed, "state %s must not proceed", state.Name())
		assert.Equal(t, provider.Check(), proceed)
	}
}

// TestProviderCheck_KeepsPanicking 原方法的 panic 行为不变。
func TestProviderCheck_KeepsPanicking(t *testing.T) {
	provider := NewProvider().SetName("no-type")
	assert.Panics(t, func() { provider.Check() },
		"the original entry point must keep its fail-fast behaviour")
}

// --- Provider.MountToParent -------------------------------------------------

// TestProviderMountToParentWithError_RejectsEmptyAndSelf 两种参数错误都返回 error。
func TestProviderMountToParentWithError_RejectsEmptyAndSelf(t *testing.T) {
	provider := NewProvider().SetName("mount").(*Provider)

	_, err := provider.MountToParentWithError()
	require.Error(t, err, "an empty son list must be reported")
	assert.ErrorIs(t, err, ErrProviderMountInvalid)
	assert.ErrorContains(t, err, "mount")

	_, err = provider.MountToParentWithError(provider)
	require.Error(t, err, "mounting onto itself must be reported")
	assert.ErrorIs(t, err, ErrProviderMountInvalid)
}

// TestProviderMountToParentWithError_MountsOnSuccess 正常路径挂载并返回自身。
func TestProviderMountToParentWithError_MountsOnSuccess(t *testing.T) {
	parent := NewProvider().SetName("parent").(*Provider)
	son := NewProvider().SetName("son")

	returned, err := parent.MountToParentWithError(son)

	require.NoError(t, err)
	assert.Same(t, parent, returned, "the sibling must still return the receiver for chaining")
	assert.Same(t, son, parent.sonProvider)
}

// TestProviderMountToParent_KeepsPanicking 原方法保留 panic。
func TestProviderMountToParent_KeepsPanicking(t *testing.T) {
	provider := NewProvider().SetName("mount")
	assert.Panics(t, func() { provider.MountToParent() })
	assert.Panics(t, func() { provider.MountToParent(provider) })
}

// --- ProviderManager.Check --------------------------------------------------

// TestManagerCheckWithError_ReportsMissingType 未设置类型时返回 error。
func TestManagerCheckWithError_ReportsMissingType(t *testing.T) {
	manager := NewProviderManager(nil).SetName("no-type")

	err := manager.CheckWithError()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderTypeNotSet)
	assert.ErrorContains(t, err, "no-type")
}

// TestManagerCheckWithError_PassesWhenTyped 已设置类型时返回 nil。
func TestManagerCheckWithError_PassesWhenTyped(t *testing.T) {
	manager := NewProviderManager(nil).
		SetName("typed").
		SetType(&PType{id: CustomTypeStart, name: "custom"})

	assert.NoError(t, manager.CheckWithError())
	assert.NotPanics(t, manager.Check)
}

// TestManagerCheck_KeepsPanicking 原方法保留 panic。
func TestManagerCheck_KeepsPanicking(t *testing.T) {
	manager := NewProviderManager(nil).SetName("no-type")
	assert.Panics(t, manager.Check)
}

// --- ProviderManager.BindToUniqueProvider -----------------------------------

// TestBindToUniqueProviderWithError_RejectsMultipleProviders 已有多个提供者时返回 error。
func TestBindToUniqueProviderWithError_RejectsMultipleProviders(t *testing.T) {
	manager := NewProviderManager(nil).SetName("multiple")
	first := NewProvider().SetName("first")
	require.NoError(t, manager.Register(first))
	require.NoError(t, manager.Register(NewProvider().SetName("second")))

	_, err := manager.BindToUniqueProviderWithError(first)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderIsUnique)
	assert.ErrorContains(t, err, "multiple")
}

// TestBindToUniqueProviderWithError_RejectsDifferentProvider 已有不同提供者时返回 error。
func TestBindToUniqueProviderWithError_RejectsDifferentProvider(t *testing.T) {
	manager := NewProviderManager(nil).SetName("different")
	require.NoError(t, manager.Register(NewProvider().SetName("existing")))

	_, err := manager.BindToUniqueProviderWithError(NewProvider().SetName("other"))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderIsUnique)
}

// TestBindToUniqueProviderWithError_BindsOnSuccess 正常路径绑定并置位唯一模式。
func TestBindToUniqueProviderWithError_BindsOnSuccess(t *testing.T) {
	manager := NewProviderManager(nil).SetName("unique")
	provider := NewProvider().SetName("only")

	returned, err := manager.BindToUniqueProviderWithError(provider)

	require.NoError(t, err)
	assert.Same(t, manager, returned)
	assert.True(t, manager.IsUnique())

	// 重复绑定同一提供者视为成功，与原方法语义一致。
	_, err = manager.BindToUniqueProviderWithError(provider)
	assert.NoError(t, err, "re-binding the same provider stays a success")
}

// TestBindToUniqueProvider_KeepsPanicking 原方法保留 panic。
func TestBindToUniqueProvider_KeepsPanicking(t *testing.T) {
	manager := NewProviderManager(nil).SetName("different")
	require.NoError(t, manager.Register(NewProvider().SetName("existing")))

	assert.Panics(t, func() { manager.BindToUniqueProvider(NewProvider().SetName("other")) })
}

// --- ProviderManager.MountToParent ------------------------------------------

// TestManagerMountToParentWithError_RejectsEmptyAndSelf 两种参数错误都返回 error。
func TestManagerMountToParentWithError_RejectsEmptyAndSelf(t *testing.T) {
	manager := NewProviderManager(nil).SetName("mount").(*ProviderManager)

	_, err := manager.MountToParentWithError()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderMountInvalid)
	assert.ErrorContains(t, err, "mount")

	_, err = manager.MountToParentWithError(manager)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderMountInvalid)
}

// TestManagerMountToParentWithError_MountsOnSuccess 正常路径挂载并返回自身。
func TestManagerMountToParentWithError_MountsOnSuccess(t *testing.T) {
	parent := NewProviderManager(nil).SetName("parent").(*ProviderManager)
	son := NewProviderManager(nil).SetName("son")

	returned, err := parent.MountToParentWithError(son)

	require.NoError(t, err)
	assert.Same(t, parent, returned)
	assert.Same(t, son, parent.sonManager)
}

// TestManagerMountToParent_KeepsPanicking 原方法保留 panic。
func TestManagerMountToParent_KeepsPanicking(t *testing.T) {
	manager := NewProviderManager(nil).SetName("mount").(*ProviderManager)
	assert.Panics(t, func() { manager.MountToParent() })
	assert.Panics(t, func() { manager.MountToParent(manager) })
}

// --- 哨兵可分类性 -----------------------------------------------------------

// TestProviderAssemblySentinels_AreDistinct 两个哨兵必须可区分。
//
// 调用方据此分流：类型未设置是「忘了 SetType」，挂载错误是「MountToParent 用法
// 不对」，两者的修复动作完全不同。
func TestProviderAssemblySentinels_AreDistinct(t *testing.T) {
	assert.NotErrorIs(t, ErrProviderTypeNotSet, ErrProviderMountInvalid)
	assert.NotErrorIs(t, ErrProviderMountInvalid, ErrProviderTypeNotSet)
	assert.NotEmpty(t, ErrProviderTypeNotSet.Error())
	assert.NotEmpty(t, ErrProviderMountInvalid.Error())
}
