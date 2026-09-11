// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// 本文件锁定关闭链消费 CloseProviders 契约的位置。
//
// 顺序约束有两端，两端都必须成立：
//
//   - **在 ServerShutdownAfter 位点之后**：该位点的 provider 仍可能使用 provider
//     持有的资源，过早关闭会让它们打在已关闭的资源上。
//   - **在全局对象释放与日志器关闭之前**：provider 关闭失败要靠框架日志器上报，
//     日志器已关则这些错误不可见——这正是 T2 修过的那类缺陷。
//
// 观测手段沿用 core_fiber_shutdown_order_test.go 的注入 writer：不触碰
// NewGlobalManagerOnce 这个进程级单例。

// closeChainProvider 记录自身 Close 被调用时的事件序号。
type closeChainProvider struct {
	*Provider
	seq      *int
	closedAt int
	closeErr error
}

func newCloseChainProvider(name string, seq *int) *closeChainProvider {
	base := NewProvider().
		SetName(name).
		SetType(ProviderTypeDefault().GroupProviderAutoRun).(*Provider)
	p := &closeChainProvider{Provider: base, seq: seq}
	base.MountToParent(p)
	return p
}

func (p *closeChainProvider) Close(context.Context) error {
	*p.seq++
	p.closedAt = *p.seq
	return p.closeErr
}

// newCloseChainManager 构造一个绑定到指定位点、内含一个可关闭 provider 的管理器。
func newCloseChainManager(
	t *testing.T,
	ctx IContext,
	location IProviderLocation,
	provider IProvider,
	onLoad func(),
) IProviderManager {
	t.Helper()
	manager := &orderProbeManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("close-chain-manager").
			SetType(ProviderTypeDefault().GroupProviderAutoRun),
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: location,
		onLoad:   onLoad,
	}
	require.NoError(t, manager.Register(provider))
	// provider 必须处于 StateLoaded，CloseProviders 才会关闭它。
	_, _ = provider.ReturnAndSetSuccessInitialize(struct{}{}, nil)
	return manager
}

// newCloseChainContext 构造一个日志写入注入 writer 的应用上下文。
func newCloseChainContext(t *testing.T, seq *int) (IApplicationContext, *orderRecordingWriter) {
	t.Helper()
	writer := &orderRecordingWriter{seq: seq}
	cfg := appconfig.NewAppConfig()
	cfg.Initialize()
	zl := zerolog.New(writer)
	return NewAppContext(cfg, bootstrap.NewLoggerWrap(&zl, writer)), writer
}

// TestCoreFiber_CloseProvidersRunsAfterLocationBeforeLoggerClose 锁定 Fiber 侧顺序。
func TestCoreFiber_CloseProvidersRunsAfterLocationBeforeLoggerClose(t *testing.T) {
	var seq int
	ctx, writer := newCloseChainContext(t, &seq)

	seedTask7FiberErrorHandler(t, ctx)
	frame := &task4Frame{application: &task7Application{}}
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.InitCoreApp(frame, task4GoodCodecManager())
	require.NotNil(t, core.GetCoreApp())

	provider := newCloseChainProvider("closable", &seq)
	var afterRanAt int
	manager := newCloseChainManager(
		t, ctx,
		ProviderLocationDefault().LocationServerShutdownAfter,
		provider,
		func() { seq++; afterRanAt = seq },
	)

	require.NoError(t, core.Shutdown(manager))

	closedAt, _ := writer.snapshot()

	require.NotZero(t, afterRanAt, "the ServerShutdownAfter location must run")
	require.NotZero(t, provider.closedAt, "CloseProviders must close the provider")
	require.NotZero(t, closedAt, "the logger must be closed during shutdown")

	assert.Less(t, afterRanAt, provider.closedAt,
		"providers must close AFTER the ServerShutdownAfter location: "+
			"that location's providers may still use the resources being closed")
	assert.Less(t, provider.closedAt, closedAt,
		"providers must close BEFORE the logger: "+
			"close failures are reported through the framework logger")
}

// TestCoreGin_CloseProvidersRunsAfterLocationBeforeLoggerClose 锁定 Gin 侧同一顺序。
func TestCoreGin_CloseProvidersRunsAfterLocationBeforeLoggerClose(t *testing.T) {
	preserveTask4GinMode(t)

	var seq int
	ctx, writer := newCloseChainContext(t, &seq)

	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	require.NotNil(t, core.coreApp)

	provider := newCloseChainProvider("closable", &seq)
	var afterRanAt int
	manager := newCloseChainManager(
		t, ctx,
		ProviderLocationDefault().LocationServerShutdownAfter,
		provider,
		func() { seq++; afterRanAt = seq },
	)

	require.NoError(t, core.Shutdown(manager))

	closedAt, _ := writer.snapshot()

	require.NotZero(t, afterRanAt, "the ServerShutdownAfter location must run")
	require.NotZero(t, provider.closedAt, "CloseProviders must close the provider")
	require.NotZero(t, closedAt, "the logger must be closed during shutdown")

	assert.Less(t, afterRanAt, provider.closedAt,
		"providers must close AFTER the ServerShutdownAfter location")
	assert.Less(t, provider.closedAt, closedAt,
		"providers must close BEFORE the logger")
}

// TestCoreFiber_CloseProvidersFailureDoesNotBreakShutdown provider 关闭失败不得
// 让 Shutdown 提前返回，否则后续的全局对象释放与日志器关闭全部落空。
func TestCoreFiber_CloseProvidersFailureDoesNotBreakShutdown(t *testing.T) {
	var seq int
	ctx, writer := newCloseChainContext(t, &seq)

	seedTask7FiberErrorHandler(t, ctx)
	frame := &task4Frame{application: &task7Application{}}
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.InitCoreApp(frame, task4GoodCodecManager())

	provider := newCloseChainProvider("failing", &seq)
	provider.closeErr = errors.New("provider close failed")
	manager := newCloseChainManager(
		t, ctx,
		ProviderLocationDefault().LocationServerShutdownAfter,
		provider,
		nil,
	)

	// 关闭失败只上报不中断：Shutdown 仍须完成全部回收。
	require.NoError(t, core.Shutdown(manager))

	closedAt, closeCalls := writer.snapshot()
	assert.NotZero(t, provider.closedAt, "the failing provider must still have been closed")
	assert.NotZero(t, closedAt, "cleanup must still run after a provider close failure")
	assert.Equal(t, 1, closeCalls, "cleanup must remain single-shot")
}

// TestCoreFiber_CloseProvidersSkipsUnboundManagers 只关闭关闭链上的管理器。
//
// 传给 Shutdown 的管理器集合同时包含运行位点的管理器（boot.go 把 before/main/after
// 三组拼在一起传入）。关闭运行位点的 provider 会误伤仍在使用的资源。
func TestCoreFiber_CloseProvidersSkipsUnboundManagers(t *testing.T) {
	var seq int
	ctx, writer := newCloseChainContext(t, &seq)

	seedTask7FiberErrorHandler(t, ctx)
	frame := &task4Frame{application: &task7Application{}}
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.InitCoreApp(frame, task4GoodCodecManager())

	shutdownProvider := newCloseChainProvider("shutdown-scoped", &seq)
	shutdownManager := newCloseChainManager(
		t, ctx,
		ProviderLocationDefault().LocationServerShutdownAfter,
		shutdownProvider,
		nil,
	)

	runProvider := newCloseChainProvider("run-scoped", &seq)
	runManager := newCloseChainManager(
		t, ctx,
		ProviderLocationDefault().LocationServerRun,
		runProvider,
		nil,
	)

	require.NoError(t, core.Shutdown(shutdownManager, runManager))

	_, _ = writer.snapshot()
	assert.NotZero(t, shutdownProvider.closedAt,
		"a provider on a shutdown location must be closed")
	assert.Zero(t, runProvider.closedAt,
		"a provider bound to a run location must not be closed by the shutdown chain")
}

// --- pprof 经由新契约关闭 ---------------------------------------------------

// TestPprofStartProvider_ClosesServerThroughClosableContract pprof 诊断服务必须
// 可经 ClosableProvider 关闭。
//
// 此前关闭只有一条路径：ServerShutdownAfter 位点上的 PprofShutdownProvider。
// 实现 ClosableProvider 之后，持有服务的 PprofStartProvider 自己也能在关闭链上
// 交出资源，与「谁持有谁负责关闭」的所有权契约一致。
func TestPprofStartProvider_ClosesServerThroughClosableContract(t *testing.T) {
	resetPprofServerInstance()
	t.Cleanup(resetPprofServerInstance)
	t.Cleanup(runtimeResetProfileRates)

	port := freePort(t)
	ctx := newPprofTestContext(t)
	ctx.GetConfig().LoadDefault(map[string]interface{}{
		"application.pprof.enable": true,
		"application.pprof.host":   "127.0.0.1",
		"application.pprof.port":   port,
	})

	provider := NewPprofStartProvider()
	_, err := provider.Initialize(ctx)
	require.NoError(t, err)

	server := resolvePprofServer(ctx)
	require.True(t, server.started, "the provider must have started the pprof server")

	closable, ok := any(provider).(ClosableProvider)
	require.True(t, ok, "PprofStartProvider must implement ClosableProvider")

	require.NoError(t, closable.Close(context.Background()))
	assert.False(t, server.started,
		"closing through the contract must stop the pprof server")
}

// TestPprofStartProvider_CloseIsIdempotent 契约要求 Close 幂等。
//
// pprof 现在有两条关闭路径（关闭位点的 PprofShutdownProvider 与本契约），
// 二者都可能触发，重复关闭必须安全。
func TestPprofStartProvider_CloseIsIdempotent(t *testing.T) {
	resetPprofServerInstance()
	t.Cleanup(resetPprofServerInstance)
	t.Cleanup(runtimeResetProfileRates)

	port := freePort(t)
	ctx := newPprofTestContext(t)
	ctx.GetConfig().LoadDefault(map[string]interface{}{
		"application.pprof.enable": true,
		"application.pprof.host":   "127.0.0.1",
		"application.pprof.port":   port,
	})

	provider := NewPprofStartProvider()
	_, err := provider.Initialize(ctx)
	require.NoError(t, err)

	closable := any(provider).(ClosableProvider)
	require.NoError(t, closable.Close(context.Background()))
	assert.NoError(t, closable.Close(context.Background()),
		"a second Close must return safely")
}

// TestPprofStartProvider_CloseWhenDisabledIsNoOp 未启用时关闭是空操作。
func TestPprofStartProvider_CloseWhenDisabledIsNoOp(t *testing.T) {
	resetPprofServerInstance()
	t.Cleanup(resetPprofServerInstance)

	ctx := newPprofTestContext(t) // 未设置 application.pprof.enable，默认关闭

	provider := NewPprofStartProvider()
	_, err := provider.Initialize(ctx)
	require.NoError(t, err)

	closable := any(provider).(ClosableProvider)
	assert.NoError(t, closable.Close(context.Background()),
		"closing a never-started pprof server must be a no-op")
}
