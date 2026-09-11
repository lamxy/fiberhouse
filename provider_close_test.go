// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件锁定 provider 关闭契约：ClosableProvider opt-in 接口与
// IProviderManager.CloseProviders。
//
// 此前 IProvider 完全没有关闭方法，provider 持有的资源（连接、后台 goroutine、
// 文件句柄）在关闭链上无人回收——feature-status.md 把这记为「没有统一的 provider
// 关闭契约」。沿用 globalmanager.Closable 的 opt-in 惯例：不修改 IProvider，
// 只有确实持有资源的 provider 才实现 ClosableProvider。

// closeRecordingProvider 记录自身被关闭的顺序号。
type closeRecordingProvider struct {
	*Provider
	seq        *int
	closedAt   int
	closeCalls int
	closeErr   error
	sawCtx     context.Context
}

func newCloseRecordingProvider(name string, seq *int) *closeRecordingProvider {
	base := NewProvider().
		SetName(name).
		SetType(&PType{id: CustomTypeStart, name: "closable"}).(*Provider)
	p := &closeRecordingProvider{Provider: base, seq: seq}
	base.MountToParent(p)
	return p
}

func (p *closeRecordingProvider) Close(ctx context.Context) error {
	p.closeCalls++
	p.sawCtx = ctx
	*p.seq++
	p.closedAt = *p.seq
	return p.closeErr
}

// plainProvider 不实现 ClosableProvider，用于确认关闭时被安全跳过。
func newPlainProvider(name string) IProvider {
	base := NewProvider().
		SetName(name).
		SetType(&PType{id: CustomTypeStart, name: "plain"}).(*Provider)
	son := &struct{ *Provider }{Provider: base}
	base.MountToParent(son)
	return son
}

// markLoaded 把 provider 推到 StateLoaded：只有已初始化的 provider 才持有资源。
func markLoaded(t *testing.T, p IProvider) {
	t.Helper()
	_, _ = p.ReturnAndSetSuccessInitialize(struct{}{}, nil)
	require.Equal(t, StateLoaded, p.Status())
}

// TestCloseProviders_ClosesInReverseRegistrationOrder 关闭必须按注册逆序。
//
// 后注册的 provider 可能依赖先注册的（例如缓存 provider 依赖 Redis provider），
// 因此拆除顺序必须与装配顺序相反。map 遍历顺序不确定，所以管理器必须自己记录
// 注册序——这正是本用例要锁定的。
func TestCloseProviders_ClosesInReverseRegistrationOrder(t *testing.T) {
	var seq int
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	first := newCloseRecordingProvider("first", &seq)
	second := newCloseRecordingProvider("second", &seq)
	third := newCloseRecordingProvider("third", &seq)
	for _, p := range []IProvider{first, second, third} {
		require.NoError(t, manager.Register(p))
		markLoaded(t, p)
	}

	require.NoError(t, manager.CloseProviders(context.Background()))

	assert.Equal(t, 1, third.closedAt, "the last registered provider must close first")
	assert.Equal(t, 2, second.closedAt)
	assert.Equal(t, 3, first.closedAt, "the first registered provider must close last")
}

// TestCloseProviders_OneFailureDoesNotStopTheRest 单个失败不得中断其余关闭。
//
// 关闭链上「提前返回」意味着后续 provider 永远拿不到回收。错误必须聚合后一次性
// 返回，且每个错误都要能被调用方识别。
func TestCloseProviders_OneFailureDoesNotStopTheRest(t *testing.T) {
	var seq int
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	firstErr := errors.New("first close failed")
	secondErr := errors.New("second close failed")

	first := newCloseRecordingProvider("first", &seq)
	first.closeErr = firstErr
	middle := newCloseRecordingProvider("middle", &seq)
	last := newCloseRecordingProvider("last", &seq)
	last.closeErr = secondErr

	for _, p := range []IProvider{first, middle, last} {
		require.NoError(t, manager.Register(p))
		markLoaded(t, p)
	}

	err := manager.CloseProviders(context.Background())

	require.Error(t, err)
	assert.ErrorIs(t, err, firstErr, "each failure must stay identifiable through errors.Join")
	assert.ErrorIs(t, err, secondErr)
	assert.Equal(t, 1, first.closeCalls, "a failing sibling must not skip this provider")
	assert.Equal(t, 1, middle.closeCalls)
	assert.Equal(t, 1, last.closeCalls)
}

// TestCloseProviders_SkipsNonClosableAndUninitialized 只关闭该关闭的。
//
// 未实现 ClosableProvider 的 provider 没有资源要放；StateFailed / StateSkipped /
// StatePending 的 provider 从未产出实例，对它们调用 Close 等于把关闭动作打在
// 不存在的资源上。
func TestCloseProviders_SkipsNonClosableAndUninitialized(t *testing.T) {
	var seq int
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	loaded := newCloseRecordingProvider("loaded", &seq)
	pending := newCloseRecordingProvider("pending", &seq)
	failed := newCloseRecordingProvider("failed", &seq)
	plain := newPlainProvider("plain")

	for _, p := range []IProvider{loaded, pending, failed, plain} {
		require.NoError(t, manager.Register(p))
	}
	markLoaded(t, loaded)
	_, _ = failed.ReturnAndSetFailInitialize(nil, errors.New("init failed"))
	require.Equal(t, StatePending, pending.Status())

	require.NoError(t, manager.CloseProviders(context.Background()))

	assert.Equal(t, 1, loaded.closeCalls, "an initialized closable provider must be closed")
	assert.Zero(t, pending.closeCalls, "a provider that never initialized holds nothing to close")
	assert.Zero(t, failed.closeCalls, "a failed provider produced no instance to close")
}

// TestCloseProviders_PassesContextThrough ctx 必须原样传到 Close。
//
// 关闭超时靠它传播；吞掉 ctx 会让上游的 30 秒关闭预算失效。
func TestCloseProviders_PassesContextThrough(t *testing.T) {
	var seq int
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	provider := newCloseRecordingProvider("provider", &seq)
	require.NoError(t, manager.Register(provider))
	markLoaded(t, provider)

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	require.NoError(t, manager.CloseProviders(ctx))

	require.NotNil(t, provider.sawCtx)
	assert.Equal(t, "sentinel", provider.sawCtx.Value(ctxKey{}),
		"the caller's context must reach Close unchanged")
}

// TestCloseProviders_EmptyManagerIsNoOp 空管理器关闭是空操作，不得报错。
func TestCloseProviders_EmptyManagerIsNoOp(t *testing.T) {
	manager := NewProviderManager(nil).SetName("empty").(*ProviderManager)
	assert.NoError(t, manager.CloseProviders(context.Background()))
}

// TestProviderManager_ListPreservesRegistrationOrder List() 顺序必须稳定。
//
// 旧实现直接遍历 map，顺序每次运行都可能不同。关闭顺序依赖它，因此稳定性不是
// 锦上添花而是正确性前提。
func TestProviderManager_ListPreservesRegistrationOrder(t *testing.T) {
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	names := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}
	for _, name := range names {
		require.NoError(t, manager.Register(NewProvider().SetName(name)))
	}

	for attempt := 0; attempt < 8; attempt++ {
		listed := make([]string, 0, len(names))
		for _, p := range manager.List() {
			listed = append(listed, p.Name())
		}
		require.Equal(t, names, listed,
			"List() must return providers in registration order on every call")
	}
}

// TestProviderManager_ListOrderSurvivesUnregister 注销后顺序仍须连续。
//
// 注销必须把 provider 从顺序记录里摘掉，否则关闭时会拿到已注销的条目。
func TestProviderManager_ListOrderSurvivesUnregister(t *testing.T) {
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		require.NoError(t, manager.Register(NewProvider().SetName(name)))
	}

	require.NoError(t, manager.Unregister("bravo"))

	listed := make([]string, 0, 2)
	for _, p := range manager.List() {
		listed = append(listed, p.Name())
	}
	assert.Equal(t, []string{"alpha", "charlie"}, listed,
		"Unregister must remove the provider from the ordering, keeping the rest in order")

	// 重新注册的 provider 排到队尾，而不是回到原位。
	require.NoError(t, manager.Register(NewProvider().SetName("bravo")))
	listed = listed[:0]
	for _, p := range manager.List() {
		listed = append(listed, p.Name())
	}
	assert.Equal(t, []string{"alpha", "charlie", "bravo"}, listed)
}

// TestProviderManager_BindToUniqueProviderKeepsOrdering 唯一绑定也要进入顺序记录。
//
// BindToUniqueProvider 绕开 Register 直接写 map，若不同时记录顺序，唯一模式下的
// provider 就永远不会被 CloseProviders 关闭。
func TestProviderManager_BindToUniqueProviderKeepsOrdering(t *testing.T) {
	var seq int
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)
	provider := newCloseRecordingProvider("unique", &seq)

	manager.BindToUniqueProvider(provider)
	markLoaded(t, provider)

	require.Len(t, manager.List(), 1)
	require.NoError(t, manager.CloseProviders(context.Background()))
	assert.Equal(t, 1, provider.closeCalls,
		"a uniquely bound provider must still be reachable for closing")
}

// --- SetOrBindToLocationWithError -----------------------------------------
//
// 原 SetOrBindToLocation 丢弃 Bind 的返回值。Bind 会在 manager 为 nil 或同一实例
// 重复绑定时返回错误，静默丢弃使这两种装配错误不可观测。
//
// 原方法签名保持不变（29 处 production 调用点都写在构造子里的链式调用中），
// 另提供取错变体——与仓库既有的 NewMysqlModel / NewMysqlModelWithError 同一模式。

// TestSetOrBindToLocationWithError_ReportsDuplicateBind 重复绑定必须可观测。
func TestSetOrBindToLocationWithError_ReportsDuplicateBind(t *testing.T) {
	location := &PLocation{id: CustomLocationStart, name: "dup-location"}
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	returned, err := manager.SetOrBindToLocationWithError(location, true)
	require.NoError(t, err, "the first bind must succeed")
	assert.Same(t, manager, returned, "the variant must still return the manager for chaining")

	_, err = manager.SetOrBindToLocationWithError(location, true)
	require.Error(t, err, "binding the same manager instance twice must be reported")
	assert.ErrorContains(t, err, "already bound")
	assert.Len(t, location.GetManagers(), 1,
		"a rejected bind must not append a duplicate entry")
}

// TestSetOrBindToLocationWithError_SetsLocationEvenWithoutBinding 不绑定时只设位点。
func TestSetOrBindToLocationWithError_SetsLocationEvenWithoutBinding(t *testing.T) {
	location := &PLocation{id: CustomLocationStart, name: "set-only"}
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	returned, err := manager.SetOrBindToLocationWithError(location)

	require.NoError(t, err)
	assert.Same(t, manager, returned)
	assert.Same(t, location, manager.Location())
	assert.Empty(t, location.GetManagers(), "without bind=true nothing may be bound")
}

// TestSetOrBindToLocation_StillChainsOnFailure 原方法的链式契约不变。
//
// 即使绑定失败，原方法仍须返回管理器本身，否则构造子里的链式调用会崩。
func TestSetOrBindToLocation_StillChainsOnFailure(t *testing.T) {
	location := &PLocation{id: CustomLocationStart, name: "chain-location"}
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	require.Same(t, manager, manager.SetOrBindToLocation(location, true))
	require.Same(t, manager, manager.SetOrBindToLocation(location, true),
		"the fluent method must keep chaining even when the bind is rejected")
	assert.Len(t, location.GetManagers(), 1)
}

// TestSetOrBindToLocationWithError_BindsSonManager 有子实例时绑定子实例。
//
// 与原方法一致：子实例存在则绑定子实例，否则绑定父实例。
func TestSetOrBindToLocationWithError_BindsSonManager(t *testing.T) {
	location := &PLocation{id: CustomLocationStart, name: "son-location"}
	child := newTask2Manager(nil)

	returned, err := child.ProviderManager.SetOrBindToLocationWithError(location, true)

	require.NoError(t, err)
	assert.Same(t, child.ProviderManager, returned)
	require.Equal(t, []IProviderManager{child}, location.GetManagers(),
		"the mounted son manager is what must reach the location")
}

// TestCloseProviders_ThroughInterface 契约必须经由接口可达。
//
// CloseProviders 挂在 IProviderManager 上，关闭链消费的是接口而非具体类型。
func TestCloseProviders_ThroughInterface(t *testing.T) {
	var seq int
	var manager IProviderManager = NewProviderManager(nil).SetName("manager")

	provider := newCloseRecordingProvider("provider", &seq)
	require.NoError(t, manager.Register(provider))
	markLoaded(t, provider)

	require.NoError(t, manager.CloseProviders(context.Background()))
	assert.Equal(t, 1, provider.closeCalls)
}

// TestCloseProviders_ErrorNamesTheProvider 聚合错误必须点名失败的 provider。
//
// 关闭链上同时有多个 provider，只说「close failed」无法定位是谁。
func TestCloseProviders_ErrorNamesTheProvider(t *testing.T) {
	var seq int
	manager := NewProviderManager(nil).SetName("manager").(*ProviderManager)

	provider := newCloseRecordingProvider("redis-provider", &seq)
	provider.closeErr = fmt.Errorf("connection reset")
	require.NoError(t, manager.Register(provider))
	markLoaded(t, provider)

	err := manager.CloseProviders(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "redis-provider",
		"the aggregated error must identify which provider failed to close")
}
