// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errTask7Provider 是注入到位点管理器的哨兵错误，用于断言失败沿调用链原样传回。
var errTask7Provider = errors.New("task7 provider failed")

// 本文件补齐 Fiber 内核的生命周期测试。
//
// Gin 内核有 14 个专项用例（logger lease、TLS 握手、listener 失败、优雅排空等），
// Fiber 作为 Default() 选中的内核却只有 1 个专项用例，其余都是 Fiber+Gin 共用的
// 表驱动用例。结果是几条只属于 Fiber 的路径长期无人覆盖：
//
//	RegisterAppMiddleware   0.0%
//	AppCoreRun             33.3%（Listen 之后的分支全部未覆盖）
//	RegisterAppHooks       39.1%（OnShutdown hook 本体在此）
//	Shutdown               63.2%
//
// 这里照搬 Gin 已验证的 loopback listener 策略，不另起一套做法。

// seedTask7FiberErrorHandler 预置 ErrorHandler 单例，使其带有可用的 recoverManager。
//
// RegisterAppMiddleware 走 NewErrorHandlerOnce，而该单例若由框架自行构造则不带
// recoverManager，RecoverMiddleware 会直接 panic("Recovery: recover manager is
// not set")。因此必须在调用前把预置实例塞进单例并消费掉 sync.Once。
//
// 复用 recover_error_handler_test.go 的 newTask5ErrorHandler：它包装真实的
// FiberRecovery，RecoverPanic 返回真正的 fiber.Handler，能通过
// MustRecoverMiddleware[fiber.Handler] 的类型断言。
func seedTask7FiberErrorHandler(t *testing.T, ctx IApplicationContext) {
	t.Helper()
	isolateTask4ErrorHandlerSingleton(t)
	errorHandlerInstance = newTask5ErrorHandler(ctx, NewFiberRecovery(ctx))
	errorHandlerOnce = sync.Once{}
	errorHandlerOnce.Do(func() {})
}

// newTask7FiberCore 构造一个已完成 InitCoreApp 的 Fiber 内核。
func newTask7FiberCore(t *testing.T, ctx IApplicationContext, frame FrameStarter) *CoreWithFiber {
	t.Helper()
	core := NewCoreWithFiber(ctx).(*CoreWithFiber)
	core.InitCoreApp(frame, task4GoodCodecManager())
	require.NotNil(t, core.GetCoreApp(), "InitCoreApp must create the Fiber app")
	return core
}

// TestCoreFiber_RegisterAppMiddlewareInstallsRecoveryAndDelegates 覆盖此前 0% 的
// RegisterAppMiddleware：确认它装上恢复与访问日志中间件，并把注册权交给应用注册器。
//
// 断言落在可观察结果上（应用注册器被调用、装配后引擎仍能处理请求），而不是中间件
// 的内部数量——后者属于实现细节，锁死它只会让正常重构失败。
func TestCoreFiber_RegisterAppMiddlewareInstallsRecoveryAndDelegates(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	seedTask7FiberErrorHandler(t, ctx)

	application := &task7Application{}
	frame := &task4Frame{application: application}
	core := newTask7FiberCore(t, ctx, frame)

	require.NotPanics(t, func() {
		core.RegisterAppMiddleware(frame)
	})

	assert.Equal(t, 1, application.middlewareCalls,
		"the application registrar must get its chance to add middleware")

	// 中间件链装好之后引擎仍须可用：注册一条路由并用 fiber 自带的 Test 驱动。
	core.GetCoreApp().(*fiber.App).Get("/probe", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	resp, err := core.GetCoreApp().(*fiber.App).Test(
		httptest.NewRequest(http.MethodGet, "/probe", nil), 3000)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCoreFiber_RegisterAppMiddlewareRespectsLocationOutcomes 覆盖位点分派的两条
// 短路分支：provider 失败时记录并返回，GroupExtendReplace 替代时不再执行默认逻辑。
//
// 两种情况下应用注册器都不应被调用——替代的语义就是「顶掉默认行为」，若仍然调用，
// 替代就形同虚设。
func TestCoreFiber_RegisterAppMiddlewareRespectsLocationOutcomes(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		manager func() *task4LifecycleManager
	}{
		{
			name: "provider error short-circuits",
			manager: func() *task4LifecycleManager {
				return &task4LifecycleManager{
					typ:      ProviderTypeDefault().GroupProviderAutoRun,
					location: ProviderLocationDefault().LocationAppMiddlewareInit,
					err:      errTask7Provider,
				}
			},
		},
		{
			name: "replacement suppresses default registration",
			manager: func() *task4LifecycleManager {
				return &task4LifecycleManager{
					typ:      ProviderTypeDefault().GroupExtendReplace,
					location: ProviderLocationDefault().LocationAppMiddlewareInit,
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := newTask4InternalAppContext(t, nil)
			seedTask7FiberErrorHandler(t, ctx)

			application := &task7Application{}
			frame := &task4Frame{application: application}
			core := newTask7FiberCore(t, ctx, frame)

			manager := testCase.manager()
			require.NotPanics(t, func() {
				core.RegisterAppMiddleware(frame, manager)
			})

			assert.Equal(t, 1, manager.loadCalls, "the location manager must be consulted")
			assert.Zero(t, application.middlewareCalls,
				"default registration must not run once the location handled it")
		})
	}
}

// TestCoreFiber_AppCoreRunReturnsListenError 覆盖 AppCoreRun 的失败分支。
//
// Gin 侧用 httpServer.Addr = "bad address" 达成同样目的；Fiber 的监听地址来自配置，
// 因此这里配一个不可绑定的端口。
func TestCoreFiber_AppCoreRunReturnsListenError(t *testing.T) {
	ctx := newTask4InternalAppContext(t, map[string]interface{}{
		"application.server.host": "127.0.0.1",
		"application.server.port": "-1",
	})
	seedTask7FiberErrorHandler(t, ctx)

	frame := &task4Frame{application: &task7Application{}}
	core := newTask7FiberCore(t, ctx, frame)

	err := core.AppCoreRun()

	require.Error(t, err, "an unbindable address must surface as an error")
	assert.False(t, ctx.GetAppState(),
		"AppState must stay false when the listener never came up")
}

// TestCoreFiber_AppCoreRunReplacementSkipsListen 确认 GroupExtendReplace 顶掉运行
// 位点时 AppCoreRun 直接返回，不会真的去监听套接字。
func TestCoreFiber_AppCoreRunReplacementSkipsListen(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	seedTask7FiberErrorHandler(t, ctx)

	frame := &task4Frame{application: &task7Application{}}
	core := newTask7FiberCore(t, ctx, frame)

	manager := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupExtendReplace,
		location: ProviderLocationDefault().LocationServerRun,
	}

	done := make(chan error, 1)
	go func() { done <- core.AppCoreRun(manager) }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("AppCoreRun blocked on Listen despite a replacement provider")
	}
	assert.Equal(t, 1, manager.loadCalls)
}

// TestCoreFiber_ShutdownRunsAllLocations 用 loopback listener 驱动一次真实的
// 启动—关闭往返，锁定 Shutdown 会依次消费三个关闭位点。
//
// 刻意**不**调用 RegisterAppHooks：它安装的 OnShutdown hook 会执行
// clearApplicationGlobals，而容器来自 NewGlobalManagerOnce() —— 进程级单例。
// 在同一进程内重复运行时，上一轮的 hook 会拆掉下一轮依赖的共享状态。
// hook 本身的行为（以及 ServerShutdownAfter 相对于资源释放的顺序）由
// core_fiber_shutdown_order_test.go 用注入 writer 的方式锁定。
//
// 也刻意**不**起真实监听。早期版本会 Listen 再 Shutdown，并等待 AppCoreRun 返回；
// 那个版本在 -run 过滤执行时约 1/15 卡住（-race 与非 -race 都会），整包执行则从不
// 复现。goroutine dump 显示 Shutdown 已返回 nil，但 fasthttp 的 accept 循环仍停在
// acceptConn——即「已可拨号」与「已进入 accept 循环」之间存在窗口，此时的 Shutdown
// 收不干净。那是测试自身的启停竞态，与位点编排无关；曾两次放宽期限（5s→30s）都仍会
// 失败，说明放宽期限是错的方向。位点编排不需要真实套接字即可验证。
func TestCoreFiber_ShutdownRunsAllLocations(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	seedTask7FiberErrorHandler(t, ctx)

	frame := &task4Frame{application: &task7Application{}}
	core := newTask7FiberCore(t, ctx, frame)

	before := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdownBefore,
	}
	main := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdown,
	}
	after := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdownAfter,
	}

	require.NoError(t, core.Shutdown(before, main, after))

	assert.Equal(t, 1, before.loadCalls, "ServerShutdownBefore must run")
	assert.Equal(t, 1, main.loadCalls, "ServerShutdown must run")
	assert.Equal(t, 1, after.loadCalls, "ServerShutdownAfter must run")
}

// TestCoreFiber_ShutdownPropagatesLocationErrors 确认关闭位点的 provider 失败会被
// 包装后返回，而不是被吞掉。
func TestCoreFiber_ShutdownPropagatesLocationErrors(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	seedTask7FiberErrorHandler(t, ctx)

	frame := &task4Frame{application: &task7Application{}}
	core := newTask7FiberCore(t, ctx, frame)

	failing := &task4LifecycleManager{
		typ:      ProviderTypeDefault().GroupProviderAutoRun,
		location: ProviderLocationDefault().LocationServerShutdown,
		err:      errTask7Provider,
	}

	err := core.Shutdown(failing)

	require.Error(t, err)
	assert.ErrorIs(t, err, errTask7Provider)
}

// task7Application 记录应用注册器被调用的次数。
type task7Application struct {
	ApplicationRegister
	middlewareCalls int
	hookCalls       int
}

func (a *task7Application) RegisterAppMiddleware(CoreStarter) { a.middlewareCalls++ }
func (a *task7Application) RegisterCoreHook(CoreStarter)      { a.hookCalls++ }
func (a *task7Application) GetDefaultTrafficCodecKey() string { return "" }
