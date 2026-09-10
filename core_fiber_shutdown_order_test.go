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

// 本文件锁定 Fiber 关闭链的顺序契约：ServerShutdownAfter 位点必须在日志器关闭
// **之前**执行。
//
// 修复前的缺陷：clearApplicationGlobals 与 logger.Close() 写在 OnShutdown hook 里，
// 而 fiber 在 coreApp.Shutdown() 内同步执行该 hook（v2.52.14 app.go:888，
// `defer app.hooks.executeOnShutdownHooks()` 位于 `app.server == nil` 检查之上，
// 因此无条件触发）。Shutdown 方法随后才加载 ServerShutdownAfter，于是该位点运行在
// 「容器已清空、日志器已关闭」的环境里——任何在此记录日志的 provider 都写进了已
// 关闭的 writer，日志静默丢失。仓库内的消费者就是 pprof 关闭提供者。
//
// 观测手段是注入一个记录关闭时序的 io.WriteCloser，而不是断言容器状态。
// AppContext.GetContainer() 返回 NewGlobalManagerOnce() 这个进程级单例，没有 reset
// 入口也没有注入点；驱动真实容器回收会污染同包其他测试（T5 已经踩过一次）。
// bootstrap.NewLoggerWrap 接受 io.WriteCloser，因此顺序可以完全在本地观测。

// orderRecordingWriter 记录自己被关闭的时刻，以及关闭时已发生的事件序号。
type orderRecordingWriter struct {
	mu         sync.Mutex
	seq        *int
	closedAt   int
	closeCalls int
}

func (w *orderRecordingWriter) Write(p []byte) (int, error) { return len(p), nil }

func (w *orderRecordingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closeCalls++
	if w.closedAt == 0 {
		*w.seq++
		w.closedAt = *w.seq
	}
	return nil
}

func (w *orderRecordingWriter) snapshot() (closedAt, closeCalls int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closedAt, w.closeCalls
}

// TestCoreFiber_ShutdownAfterRunsBeforeLoggerClose 是 T2 的核心回归。
//
// 在修复前，ServerShutdownAfter 的 loadCalls 会记录在日志器关闭之后，断言必然失败。
func TestCoreFiber_ShutdownAfterRunsBeforeLoggerClose(t *testing.T) {
	var seq int
	writer := &orderRecordingWriter{seq: &seq}

	cfg := appconfig.NewAppConfig()
	cfg.Initialize()
	zl := zerolog.New(writer)
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&zl, writer))

	seedTask7FiberErrorHandler(t, ctx)
	frame := &task4Frame{application: &task7Application{}}
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.InitCoreApp(frame, task4GoodCodecManager())
	require.NotNil(t, core.GetCoreApp())

	// 装上 OnShutdown hook：正是它在修复前抢先关掉了日志器。
	core.RegisterAppHooks(frame)

	var afterRanAt int
	after := &orderProbeManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdownAfter,
		onLoad: func() {
			seq++
			afterRanAt = seq
		},
	}

	require.NoError(t, core.Shutdown(after))

	closedAt, closeCalls := writer.snapshot()

	require.NotZero(t, afterRanAt, "ServerShutdownAfter must run")
	require.NotZero(t, closedAt, "the logger must be closed during shutdown")
	assert.Less(t, afterRanAt, closedAt,
		"ServerShutdownAfter must run before the logger is closed; "+
			"otherwise providers at that location log into a closed writer")
	assert.Equal(t, 1, closeCalls,
		"the logger must be closed exactly once: LoggerWrap.Close has no idempotency guard")
}

// TestCoreFiber_CleanupRunsOnEarlyReturn 锁定 defer 放置。
//
// Shutdown 有六处提前返回。写在方法尾部的回收会被它们全部跳过；用顶部 defer 才不会。
// 这里用位点加载失败触发提前返回。
func TestCoreFiber_CleanupRunsOnEarlyReturn(t *testing.T) {
	var seq int
	writer := &orderRecordingWriter{seq: &seq}

	cfg := appconfig.NewAppConfig()
	cfg.Initialize()
	zl := zerolog.New(writer)
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&zl, writer))

	seedTask7FiberErrorHandler(t, ctx)
	frame := &task4Frame{application: &task7Application{}}
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.InitCoreApp(frame, task4GoodCodecManager())

	failing := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdown,
		err:      errTask7Provider,
	}

	require.Error(t, core.Shutdown(failing))

	closedAt, closeCalls := writer.snapshot()
	assert.NotZero(t, closedAt, "cleanup must still run when Shutdown returns early")
	assert.Equal(t, 1, closeCalls, "cleanup must remain single-shot on the early-return path")
}

// TestCoreFiber_CleanupIsSingleShotAcrossBothEntrypoints 确认 once 守卫把两个入口
// 折叠成一次回收。
//
// 回收有两个入口：Shutdown 顶部的 defer，以及 OnShutdown hook（应用直接调用
// coreApp.Shutdown() 时只有它触发）。守卫不是装饰——LoggerWrap.Close 只委派给
// wCloser.Close，本身没有幂等保护。
func TestCoreFiber_CleanupIsSingleShotAcrossBothEntrypoints(t *testing.T) {
	var seq int
	writer := &orderRecordingWriter{seq: &seq}

	cfg := appconfig.NewAppConfig()
	cfg.Initialize()
	zl := zerolog.New(writer)
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&zl, writer))

	seedTask7FiberErrorHandler(t, ctx)
	frame := &task4Frame{application: &task7Application{}}
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.InitCoreApp(frame, task4GoodCodecManager())
	core.RegisterAppHooks(frame)

	require.NoError(t, core.Shutdown())
	core.clearApplicationGlobalsOnce()
	core.clearApplicationGlobalsOnce()

	_, closeCalls := writer.snapshot()
	assert.Equal(t, 1, closeCalls, "the once guard must collapse every entrypoint into one cleanup")
}

// TestCoreFiber_CleanupGuardIsPerInstance 确认守卫挂在实例上。
//
// 包级守卫会让同进程内的第二个应用实例永远拿不到回收。
func TestCoreFiber_CleanupGuardIsPerInstance(t *testing.T) {
	newCore := func(t *testing.T) (*CoreWithFiber, *orderRecordingWriter) {
		t.Helper()
		var seq int
		writer := &orderRecordingWriter{seq: &seq}
		cfg := appconfig.NewAppConfig()
		cfg.Initialize()
		zl := zerolog.New(writer)
		ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&zl, writer))
		seedTask7FiberErrorHandler(t, ctx)
		core := NewCoreWithFiber(ctx).(*CoreWithFiber)
		core.InitCoreApp(&task4Frame{application: &task7Application{}}, task4GoodCodecManager())
		return core, writer
	}

	first, firstWriter := newCore(t)
	first.clearApplicationGlobalsOnce()
	firstClosedAt, _ := firstWriter.snapshot()
	require.NotZero(t, firstClosedAt)

	second, secondWriter := newCore(t)
	second.clearApplicationGlobalsOnce()

	secondClosedAt, _ := secondWriter.snapshot()
	assert.NotZero(t, secondClosedAt,
		"a second core instance must still get its own cleanup")
}

// orderProbeManager 在位点被分派时记录一个事件序号。
type orderProbeManager struct {
	IProviderManager
	typ      IProviderType
	location IProviderLocation
	onLoad   func()
}

func (m *orderProbeManager) Type() IProviderType         { return m.typ }
func (m *orderProbeManager) Location() IProviderLocation { return m.location }
func (m *orderProbeManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	if m.onLoad != nil {
		m.onLoad()
	}
	if len(loadFunc) > 0 {
		return loadFunc[0](m)
	}
	return nil, nil
}
