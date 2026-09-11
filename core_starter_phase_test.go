// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// 本文件是任务 1「关键修正」的红线：PhaseServing 必须在**监听真正建立时**置位，
// 而不是在阻塞的 Listen/Serve 返回之后。
//
// 旧实现把 RegisterAppState(true) 写在 Listen 返回之后，而 Listen 一直阻塞到
// 服务器停止——于是「已启动」这个标记恰好写在服务结束之后。整个服务期间状态都是
// 「未启动」，关闭守卫据此放行，两条语义同时错位。
//
// 下面两个用例分别用真实 loopback 监听驱动 Gin 与 Fiber，在服务仍在接受流量的
// 那一刻断言阶段。

// TestCoreGin_PhaseIsServingWhileListening 在 Gin 真正接受请求时断言 PhaseServing。
func TestCoreGin_PhaseIsServingWhileListening(t *testing.T) {
	preserveTask4GinMode(t)
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)

	// 借一个空闲端口再立刻归还，复用仓库既有做法（core_starter_init_test.go:847）。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	core.httpServer.Addr = listener.Addr().String()
	require.NoError(t, listener.Close())

	require.Equal(t, PhaseCreated, ctx.Phase(),
		"the phase must not advance before the listener exists")

	runResult := make(chan error, 1)
	go func() { runResult <- core.AppCoreRun() }()

	// 等到端口真的可拨——此时服务器正在接受流量。
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, dialErr := net.DialTimeout("tcp", core.httpServer.Addr, 50*time.Millisecond)
		if dialErr == nil {
			require.NoError(t, conn.Close())
			break
		}
		if time.Now().After(deadline) {
			require.NoError(t, dialErr, "Gin listener never came up")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 红线断言：服务进行中，阶段必须已经是 PhaseServing。
	assert.Equal(t, PhaseServing, ctx.Phase(),
		"PhaseServing must be set while the server is actually listening")
	assert.True(t, ctx.IsServing(), "IsServing must be true during listening")

	require.NoError(t, core.Shutdown())

	select {
	case runErr := <-runResult:
		if runErr != nil {
			require.ErrorIs(t, runErr, http.ErrServerClosed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("AppCoreRun did not return after shutdown")
	}

	// 关闭之后不得再自称正在服务。
	assert.False(t, ctx.IsServing(), "a stopped server must not report serving")
	assert.GreaterOrEqual(t, ctx.Phase(), PhaseShuttingDown,
		"shutdown must advance the phase past serving")
}

// TestCoreFiber_PhaseIsServingFromListenHook 确认 Fiber 的 PhaseServing 由
// 监听建立时执行的 hook 置位，而非等阻塞的 Listen 返回。
//
// fiber 的 executeOnListenHooks 未导出，无法直接触发；改用真实 loopback 监听，
// 并在 Listen 仍在阻塞（即服务进行中）的窗口内断言阶段。这与
// core_fiber_starter_lifecycle_test.go:192 记录的启停竞态不同：那里的问题出在
// **关闭**——「已可拨号」与「已进入 accept 循环」之间的窗口会让 Shutdown 收不
// 干净。这里不调用 Shutdown，只等端口可拨后读一次阶段，然后让 t.Cleanup 关掉
// 应用，因此不触及那个竞态。
func TestCoreFiber_PhaseIsServingFromListenHook(t *testing.T) {
	ctx := newTask4InternalAppContext(t, map[string]interface{}{
		"application.server.host": "127.0.0.1",
		"application.server.port": "0",
	})
	seedTask7FiberErrorHandler(t, ctx)

	frame := &task4Frame{application: &task7Application{}}
	core := newTask7FiberCore(t, ctx, frame)
	core.RegisterAppHooks(frame)

	require.Equal(t, PhaseCreated, ctx.Phase(),
		"registering hooks must not advance the phase on its own")

	runResult := make(chan error, 1)
	go func() { runResult <- core.AppCoreRun() }()
	t.Cleanup(func() {
		_ = core.coreApp.Shutdown()
		select {
		case <-runResult:
		case <-time.After(5 * time.Second):
		}
	})

	// 等待 OnListen hook 执行——它只在监听真正建立时触发。
	deadline := time.Now().Add(5 * time.Second)
	for ctx.Phase() != PhaseServing {
		if time.Now().After(deadline) {
			t.Fatal("Fiber never reached PhaseServing while listening")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 红线断言：Listen 仍在阻塞（服务进行中），阶段已经是 PhaseServing。
	assert.Equal(t, PhaseServing, ctx.Phase(),
		"the OnListen hook must set PhaseServing at the moment the listener is established")
	assert.True(t, ctx.IsServing())

	select {
	case <-runResult:
		t.Fatal("AppCoreRun returned while the server was expected to keep listening")
	default:
	}
}

// TestCoreFiber_EarlyPinNoLongerSkipsShutdownCleanup 锁定 latch 缺陷已消失。
//
// 旧实现里 RegisterAppState 是 sync.Once latch：使用方（或任何早于 Listen 的
// 调用）先 pin 一次 true，Shutdown 顶部的 `if GetAppState() { return nil }` 就会
// 直接返回，全部资源回收被跳过。改用有序阶段后，关闭守卫读的是
// PhaseShuttingDown，而提前置位只能到达 PhaseServing，因此关闭链照常执行。
//
// 以注入 writer 观测日志器是否真的被关闭——这是回收确实发生过的可见证据，
// 手法与 core_fiber_shutdown_order_test.go 一致，不触碰全局容器单例。
func TestCoreFiber_EarlyPinNoLongerSkipsShutdownCleanup(t *testing.T) {
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

	// 提前 pin：这正是旧 latch 下会让 Shutdown 变成空操作的动作。
	ctx.RegisterAppState(true)
	require.True(t, ctx.GetAppState())

	after := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdownAfter,
	}
	require.NoError(t, core.Shutdown(after))

	assert.Equal(t, 1, after.loadCalls,
		"an early pin must not let Shutdown skip the shutdown locations")

	closedAt, _ := writer.snapshot()
	assert.NotZero(t, closedAt,
		"an early pin must not let Shutdown skip resource cleanup")
	assert.Equal(t, PhaseTerminated, ctx.Phase(),
		"a completed shutdown must land on PhaseTerminated")
}

// TestCoreFiber_ShutdownIsIdempotentAcrossPhases 重复关闭仍须是幂等的。
//
// 新守卫读 PhaseShuttingDown，因此第二次调用要被挡在位点分派之前。
func TestCoreFiber_ShutdownIsIdempotentAcrossPhases(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	seedTask7FiberErrorHandler(t, ctx)

	frame := &task4Frame{application: &task7Application{}}
	core := newTask7FiberCore(t, ctx, frame)

	first := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdown,
	}
	require.NoError(t, core.Shutdown(first))
	require.Equal(t, 1, first.loadCalls)

	second := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdown,
	}
	require.NoError(t, core.Shutdown(second))

	assert.Zero(t, second.loadCalls,
		"a second Shutdown must short-circuit: the phase already reached shutting down")
	assert.Equal(t, PhaseTerminated, ctx.Phase())
}
