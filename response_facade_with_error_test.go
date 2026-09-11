// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件锁定 response_facade.go 两处装配 panic 的替代入口（PRD C1）。
//
// feature-status.md 把 MsgPack / Protobuf 判定为「卡住条件 4」，理由正是
// response_facade.go:33,36 在装配期 panic 而**没有任何返回 error 的替代入口**
// ——与已晋级的 MySQL / MongoDB 在结构上相同，区别只在有没有 *WithError 姊妹。
// 补上 NewRespInfoPManagerOnceWithError 即闭合该缺口。

// isolateRespInfoPManagerSingleton 隔离单例状态。
//
// 单例是包级变量且由 sync.Once 守卫，用例之间必须复原，否则第一个用例的结果
// 会决定其余所有用例——包括 response_facade_test.go 里依赖正常单例的那些。
// sync.Once 不可复制（内含 noCopy），因此这里不保存再还原原对象，而是在用例
// 前后各重置一次。重置本身是赋一个全新的零值，不涉及复制已有的 Once。
func isolateRespInfoPManagerSingleton(t *testing.T) {
	t.Helper()
	originalInstance := respInfoPManagerInstance
	originalErr := respInfoPManagerErr
	t.Cleanup(func() {
		respInfoPManagerOnce = sync.Once{}
		respInfoPManagerInstance = originalInstance
		respInfoPManagerErr = originalErr
		// 还原后重新消费 Once，使后续调用直接复用已还原的实例，
		// 而不是再次执行解析闭包。
		if originalInstance != nil || originalErr != nil {
			respInfoPManagerOnce.Do(func() {})
		}
	})
	respInfoPManagerOnce = sync.Once{}
	respInfoPManagerInstance = nil
	respInfoPManagerErr = nil
}

// TestNewRespInfoPManagerOnceWithError_ReportsMissingManager 位点无管理器时返回 error。
func TestNewRespInfoPManagerOnceWithError_ReportsMissingManager(t *testing.T) {
	isolateRespInfoPManagerSingleton(t)
	clearRespInfoLocation(t)

	manager, err := NewRespInfoPManagerOnceWithError()

	require.Error(t, err, "an unconfigured response location must be reported")
	assert.ErrorIs(t, err, ErrRespInfoPManagerUnavailable)
	assert.Nil(t, manager)
}

// TestNewRespInfoPManagerOnceWithError_ReportsWrongManagerType 类型不符时返回 error。
func TestNewRespInfoPManagerOnceWithError_ReportsWrongManagerType(t *testing.T) {
	isolateRespInfoPManagerSingleton(t)
	clearRespInfoLocation(t)

	wrong := &bootWithErrorManager{
		typ:      ProviderTypeDefault().GroupResponseInfoChoose,
		location: ProviderLocationDefault().LocationResponseInfoInit,
	}
	bindBootManager(t, ProviderLocationDefault().LocationResponseInfoInit, wrong)

	manager, err := NewRespInfoPManagerOnceWithError()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRespInfoPManagerUnavailable)
	assert.ErrorContains(t, err, "RespInfoPManager",
		"the error must name the expected type")
	assert.Nil(t, manager)
}

// TestNewRespInfoPManagerOnceWithError_CachesFailure 失败结果同样被 once 缓存。
//
// sync.Once 只执行一次闭包。若失败时不记录错误，后续调用会返回
// (nil, nil)——调用方拿到 nil 管理器却看到 nil 错误，随后解引用即 panic。
// 这与 globalmanager.Get 此前的缺陷同源。
func TestNewRespInfoPManagerOnceWithError_CachesFailure(t *testing.T) {
	isolateRespInfoPManagerSingleton(t)
	clearRespInfoLocation(t)

	_, firstErr := NewRespInfoPManagerOnceWithError()
	require.Error(t, firstErr)

	manager, secondErr := NewRespInfoPManagerOnceWithError()

	require.Error(t, secondErr,
		"a cached failure must keep reporting, never degrade to (nil, nil)")
	assert.ErrorIs(t, secondErr, ErrRespInfoPManagerUnavailable)
	assert.Nil(t, manager)
}

// TestNewRespInfoPManagerOnce_KeepsPanicking 原入口保留 panic。
func TestNewRespInfoPManagerOnce_KeepsPanicking(t *testing.T) {
	isolateRespInfoPManagerSingleton(t)
	clearRespInfoLocation(t)

	assert.Panics(t, func() { NewRespInfoPManagerOnce() },
		"the original entry point must keep its fail-fast behaviour")
}

// TestNewRespInfoPManagerOnceWithError_ReturnsManagerOnSuccess 正常路径返回管理器。
func TestNewRespInfoPManagerOnceWithError_ReturnsManagerOnSuccess(t *testing.T) {
	isolateRespInfoPManagerSingleton(t)
	clearRespInfoLocation(t)

	ctx := newTask4InternalAppContext(t, nil)
	expected := NewRespInfoPManager(ctx)
	t.Cleanup(func() {
		unbindBootManager(t, ProviderLocationDefault().LocationResponseInfoInit, expected)
	})

	manager, err := NewRespInfoPManagerOnceWithError()

	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.Same(t, expected, manager)

	// 原入口在成功路径上返回同一实例。
	assert.Same(t, manager, NewRespInfoPManagerOnce())
}

// clearRespInfoLocation 清空响应信息位点上已绑定的管理器，并在用例结束后复原。
//
// 位点是进程级单例：包内其他文件（response_providers_and_manager.go 的构造子）
// 会在 init 或构造时绑定管理器，不清空则「位点无管理器」这条路径无法被触发。
func clearRespInfoLocation(t *testing.T) {
	t.Helper()
	location, ok := ProviderLocationDefault().LocationResponseInfoInit.(*PLocation)
	if !ok {
		return
	}
	location.mu.Lock()
	original := location.managers
	location.managers = nil
	location.mu.Unlock()

	t.Cleanup(func() {
		location.mu.Lock()
		location.managers = original
		location.mu.Unlock()
	})
}
