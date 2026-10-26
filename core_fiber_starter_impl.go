// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	adaptorerrorhandler "github.com/lamxy/fiberhouse/adaptor/errorhandler"
	"github.com/rs/zerolog"
)

// ErrCoreInitFailed 标记核心装配失败。
//
// 装配失败此前只能以 panic 表达，调用方无从选择。本哨兵使失败可被 errors.Is
// 分类：生命周期方法短路后，AppCoreRun / Shutdown 会把它作为错误返回。
var ErrCoreInitFailed = errors.New("fiberhouse: core initialization failed")

// CoreWithFiber 应用核心启动器
type CoreWithFiber struct {
	ctx     IApplicationContext
	CoreCfg *fiber.Config
	coreApp *fiber.App
	json    JsonWrapper
	// initErr 记录装配期失败，与 Gin 的同名字段同义。
	//
	// Fiber 此前完全没有这套机制（Gin 有 13 处使用、hertzcore 有 15 处）：
	// JSON codec 解析失败时直接 panic，既无法短路后续装配，也无法让调用方
	// 观测到「核心没起来」。现在失败被记录，后续生命周期方法一律短路，
	// 运行与关闭入口把它作为错误返回。
	//
	// 原 panic 行为保留在 InitCoreAppMust：条件 4 要求 panic 入口与返回 error
	// 的入口成对出现，而不是消除 panic。
	initErr error
	// globalsCleared 保证全局对象只回收一次。
	//
	// 回收有两个入口：Shutdown 顶部的 defer，以及 fiber 的 OnShutdown hook（应用
	// 直接调用 coreApp.Shutdown() 时只有后者会触发）。两者都委派到
	// clearApplicationGlobalsOnce，由本标志把重复执行折叠成一次。
	//
	// 这不是优化而是必需：LoggerWrap.Close 只委派给 wCloser.Close，本身没有幂等
	// 保护，重复关闭的安全性完全取决于注入的 writer。
	//
	// 挂在实例上而非包级别——包级标志会让同进程内的第二个应用实例拿不到回收。
	globalsCleared atomic.Bool
	// shutdownCoordinated 标记本次关闭由 Shutdown 主导。
	//
	// fiber 在 coreApp.Shutdown() 内**同步**触发 OnShutdown hook。若 hook 无条件回收，
	// 日志器会在 ServerShutdownAfter 位点加载之前就被关掉——正是本次要修的顺序缺陷。
	// 因此经由 Shutdown 关闭时置位本标志，hook 让位，由 Shutdown 顶部的 defer 在所有
	// 位点执行完毕后统一回收。
	//
	// 应用绕过 Shutdown 直接调用 coreApp.Shutdown() 时本标志为 false，hook 照常回收，
	// 那条路径的既有行为不受影响。与 Gin 的 shutdownCoordinated 同义（见
	// core_gin_starter_impl.go:35）。
	shutdownCoordinated atomic.Bool
}

// NewCoreWithFiber 创建一个应用核心启动器对象
func NewCoreWithFiber(ctx IApplicationContext, opts ...CoreStarterOption) CoreStarter {
	core := &CoreWithFiber{
		ctx: ctx,
	}

	if len(opts) > 0 {
		for _, opt := range opts {
			opt(core)
		}
	}

	return core
}

// GetAppContext 获取应用上下文
func (cf *CoreWithFiber) GetAppContext() IApplicationContext {
	return cf.ctx
}

// initializationFailed 报告核心装配是否已失败。
//
// 与 core_gin_starter_impl.go 的同名方法同义：为真时全部生命周期方法短路，
// 运行与关闭入口把 initErr 作为错误返回。
func (cf *CoreWithFiber) initializationFailed() bool {
	return cf.initErr != nil
}

// recordInitFailure 记录装配失败并写入框架日志。
//
// 统一入口保证三件事一致：错误链上始终带 ErrCoreInitFailed 以便 errors.Is
// 分类、原始错误得以保留、失败必定留下日志。
func (cf *CoreWithFiber) recordInitFailure(err error) {
	if err == nil || cf.initErr != nil {
		return
	}
	cf.initErr = fmt.Errorf("%w: %w", ErrCoreInitFailed, err)
	if ctx := cf.GetAppContext(); ctx != nil && ctx.GetLogger() != nil && ctx.GetConfig() != nil {
		ctx.GetLogger().ErrorWith(ctx.GetConfig().LogOriginFrame()).
			Str("applicationStarter", "FrameApplication").
			Err(cf.initErr).
			Msg("InitCoreApp failed")
	}
}

// GetCoreApp 获取应用核心实例
func (cf *CoreWithFiber) GetCoreApp() interface{} {
	return cf.coreApp
}

// InitCoreApp 初始化应用核心（框架应用基于 fiber.App）
// InitCoreAppMust 初始化应用核心，装配失败时 panic。
//
// 与 InitCoreApp 成对出现，遵循 stdlib 的 regexp.MustCompile / template.Must
// 惯例：需要 fail-fast 的调用方用本入口，需要处理错误的调用方用 InitCoreApp
// 并检查 AppCoreRun / Shutdown 的返回值。
//
// 装配失败在启动期不可恢复，因此 panic 本身是合理的默认；条件 4 要求的不是
// 消除 panic，而是让调用方**有的选**。
func (cf *CoreWithFiber) InitCoreAppMust(fs FrameStarter, managers ...IProviderManager) {
	cf.InitCoreApp(fs, managers...)
	if cf.initializationFailed() {
		panic(cf.initErr)
	}
}

func (cf *CoreWithFiber) InitCoreApp(fs FrameStarter, managers ...IProviderManager) {
	if cf.initializationFailed() {
		return
	}
	if cf.GetAppContext().Phase() >= PhaseServing {
		return
	}

	_, replaced, err := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationCoreEngineInit,
		cf,
	)
	if err != nil {
		cf.GetAppContext().GetLogger().ErrorWith(cf.GetAppContext().GetConfig().LogOriginFrame()).
			Err(err).
			Msg("InitCoreApp providers failed")
		return
	}
	if replaced {
		return
	}

	cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("InitCoreApp starting...")

	// 自定义核心配置
	if cf.CoreCfg != nil {
		cf.coreApp = fiber.New(*cf.CoreCfg)
		// fs 为 nil 时（例如仅验证自定义配置的单元测试）无法解析编解码器，跳过；
		// 标准启动链会传入非 nil fs，此时仍需完成编解码器装配，避免 cf.json 保持 nil
		// 导致后续 RegisterAppMiddleware 使用 cf.json.Marshal 时 panic。
		if fs != nil {
			json, codecErr := cf.resolveJSONCodec(fs, managers...)
			if codecErr != nil {
				cf.recordInitFailure(codecErr)
				cf.coreApp = nil
				return
			}
			cf.json = json
		}
		return
	}

	cfg := cf.GetAppContext().GetConfig()
	json, codecErr := cf.resolveJSONCodec(fs, managers...)
	if codecErr != nil {
		cf.recordInitFailure(codecErr)
		return
	}
	cf.json = json

	// IErrorHandler接口实例
	eh := NewErrorHandlerOnce(cf.GetAppContext())

	// 默认核心配置
	cf.coreApp = fiber.New(fiber.Config{
		// 设置应用名称
		AppName:       cfg.String("application.appName"),
		CaseSensitive: cfg.Bool("application.server.caseSensitive"),
		// 启用严格路由匹配，要求路由必须完全匹配请求路径
		StrictRouting: cfg.Bool("application.server.strictRouting"),
		// 设置服务器头部信息
		ServerHeader: cfg.String("application.server.appServerHeader"),
		// 设置自定义错误处理函数
		// 该函数会在请求处理过程中发生错误时被调用
		ErrorHandler: adaptorerrorhandler.FiberErrorHandler(eh.ErrorHandler),
		// 设置并发处理请求的数量
		Concurrency: cfg.Int("application.server.appConcurrency"),
		// 设置是否启用长连接
		DisableKeepalive: cfg.Bool("application.server.disableKeepalive"),
		// 设置读取和写入缓冲区大小
		ReadBufferSize:  cfg.Int("application.server.readBufferSize", 4096),
		WriteBufferSize: cfg.Int("application.server.writeBufferSize", 4096),
		// 设置请求体大小限制，单位为KB
		BodyLimit: cfg.Int("application.server.bodyLimit", 4096),
		// 设置空闲连接超时时间
		IdleTimeout: cfg.Duration("application.server.idleTimeout", 60) * time.Second,
		// 设置读取和写入超时时间
		ReadTimeout:  cfg.Duration("application.server.readTimeout", 30) * time.Second,
		WriteTimeout: cfg.Duration("application.server.writeTimeout", 30) * time.Second,
		// 打印路由列表信息
		EnablePrintRoutes: cfg.Bool("application.server.enablePrintRoutes"), // 默认false
		JSONEncoder:       cf.json.Marshal,
		JSONDecoder:       cf.json.Unmarshal,
		// true: /api?foo=bar,baz == foo[]=bar&foo[]=baz
		EnableSplittingOnParsers: true,
		// http://127.0.0.1:3000/exchange/name/adas%20ahdsa+asldas,反转空格、+加号等特殊字符
		UnescapePath: true,
		// When set to true, it will not print out debug information
		DisableStartupMessage: false,
		// Limit supported http methods
		RequestMethods: cfg.Strings("application.server.requestMethods", []string{}), // 默认支持全部方法
		// enables request body streaming, and calls the handler sooner when given body is larger than the current limit
		StreamRequestBody: cfg.Bool("application.server.streamRequestBody"), // 默认false
		// more...
	})
}

// resolveJSONCodec 解析JSON编解码器实例。
// 该逻辑在默认核心配置路径和自定义 CoreCfg 路径下均需要执行，
// 抽取为独立方法避免自定义 CoreCfg 路径遗漏 cf.json 赋值。
func (cf *CoreWithFiber) resolveJSONCodec(fs FrameStarter, managers ...IProviderManager) (JsonWrapper, error) {
	// fiberhouse.JsonWrapper序列化反序列化接口，默认编解码器实例
	// JSON := GetMustInstance[JsonWrapper](fs.GetApplication().GetDefaultTrafficCodecKey())

	// 配置JSON序列化器
	var (
		json JsonWrapper
		jOk  bool
	)

	if len(managers) == 0 {
		// 默认编解码器实例
		cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Msg("No JSON codec manager provided, using default JSON codec.")
		json = GetMustInstance[JsonWrapper](fs.GetApplication().GetDefaultTrafficCodecKey())
	} else {
		var jsonCodecManager IProviderManager
		for _, manager := range managers {
			if manager.Location().GetLocationID() == ProviderLocationDefault().LocationCoreCodecInit.GetLocationID() &&
				manager.Type().GetTypeID() == ProviderTypeDefault().GroupTrafficCodecChoose.GetTypeID() {
				jsonCodecManager = manager
				break
			}
		}

		if jsonCodecManager == nil {
			return nil, errors.New("no JSON codec manager found in the provided managers")
		}

		jcodec, err := jsonCodecManager.LoadProvider()

		if err != nil {
			return nil, fmt.Errorf("load JSON codec provider: %w", err)
		}

		if json, jOk = jcodec.(JsonWrapper); !jOk {
			return nil, fmt.Errorf(
				"loaded JSON codec provider (%T) does not implement JsonWrapper", jcodec)
		}
	}

	return json, nil
}

// RegisterAppMiddleware 注册应用级的中间件
func (cf *CoreWithFiber) RegisterAppMiddleware(fs FrameStarter, managers ...IProviderManager) {
	if cf.initializationFailed() {
		return
	}
	if cf.GetAppContext().Phase() >= PhaseServing {
		return
	}

	_, replaced, err := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationAppMiddlewareInit,
		cf,
	)
	if err != nil {
		cf.GetAppContext().GetLogger().ErrorWith(cf.GetAppContext().GetConfig().LogOriginFrame()).
			Err(err).
			Msg("RegisterAppMiddleware providers failed")
		return
	}
	if replaced {
		return
	}

	cf.GetAppContext().GetLogger().Info(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("RegisterAppMiddleware")
	debugMode := cf.GetAppContext().GetConfig().GetRecover().DebugMode

	// 遍历管理器切片，筛选类型为GroupRecoverMiddlewareChoose管理器，然后加载对应的恢复中间件提供者
	//或者，直接从NewErrorHandlerOnce单例获取恢复中间件

	// IErrorHandler接口实例
	eh := NewErrorHandlerOnce(cf.GetAppContext())

	recoverHandler := eh.RecoverMiddleware(RecoverConfig{
		AppCtx:            cf.GetAppContext(),
		EnableStackTrace:  true,
		StackTraceHandler: eh.DefaultStackTraceHandler,
		Logger:            cf.GetAppContext().GetLogger(),
		Stdout:            false,
		JsonCodec:         cf.json.Marshal,
		DebugMode:         debugMode, // true开启调试模式，将详细错误信息显示给客户端，否则隐藏细节，只能通过日志文件查看。生产环境关闭该调式模式。
	})

	// 注册核心应用(coreApp/fiber App)全局错误捕获中间件
	cf.coreApp.Use(MustRecoverMiddleware[fiber.Handler](recoverHandler))

	// 注册核心应用(coreApp/fiber App)http请求日志中间件
	cf.coreApp.Use(cf.accessLogMiddleware())

	if fs.GetApplication() != nil {
		// 注册项目应用注册器全局中间件
		fs.GetApplication().(ApplicationRegister).RegisterAppMiddleware(cf)
	}
}

// accessLogMiddleware 返回 http 请求访问日志中间件，由 coreHttp 开关控制
//
// 访问日志只记录流量：5xx、4xx 记 WARN，其余记 INFO。错误本身由 Recover
// （DefaultStackTraceHandler）按 HTTP 状态记录，5xx 在那里已是 ERROR；
// 这里再记 ERROR 会让同一个错误向错误追踪平台上报两次。
//
// 请求ID取自响应头 X-Request-ID（requestid 中间件写入），字段名跟随 trace 配置，
// 与 Recover 日志一致，便于按同一个 ID 串联访问日志和错误日志。
func (cf *CoreWithFiber) accessLogMiddleware() fiber.Handler {
	base := func() *zerolog.Logger {
		log, err := cf.GetAppContext().GetContainer().Get(cf.GetAppContext().GetConfig().LogOriginCoreHttp().InstanceKey())
		if err != nil {
			// 获取http类子日志器错误
			cf.GetAppContext().GetLogger().Error(cf.GetAppContext().GetConfig().LogOriginFrame()).Err(err).Str("applicationStarter", "FrameApplication").Msg("RegisterAppMiddleware register fiberzerolog middleware to get http logger error")
			return fiberzerolog.ConfigDefault.Logger // 使用默认日志器
		}
		return log.(*zerolog.Logger)
	}()
	ridKey := requestIDKey(cf.GetAppContext())

	return fiberzerolog.New(fiberzerolog.Config{
		Levels: []zerolog.Level{zerolog.WarnLevel, zerolog.WarnLevel, zerolog.InfoLevel},
		// GetLogger 在 c.Next() 之后调用，此时 requestid 中间件已写入响应头
		GetLogger: func(c *fiber.Ctx) zerolog.Logger {
			if rid := c.GetRespHeader(fiber.HeaderXRequestID); rid != "" {
				return base.With().Str(ridKey, rid).Logger()
			}
			return *base
		},
		Next: func(c *fiber.Ctx) bool {
			ms := cf.GetAppContext().GetConfig().GetMiddlewareSwitch("coreHttp")
			return !ms
		},
	})
}

// RegisterModuleInitialize 注册应用模块/子系统级的中间件、路由处理器、etc...
func (cf *CoreWithFiber) RegisterModuleInitialize(fs FrameStarter, managers ...IProviderManager) {
	if cf.initializationFailed() {
		return
	}
	if cf.GetAppContext().Phase() >= PhaseServing {
		return
	}

	_, _, moduleErr := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationModuleMiddlewareInit,
		cf,
	)
	if moduleErr != nil {
		fs.GetContext().GetLogger().ErrorWith(fs.GetContext().GetConfig().LogOriginFrame()).
			Err(moduleErr).
			Msg("RegisterModuleInitialize module middleware providers failed")
	}

	routeHandled, _, routeErr := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationRouteRegisterInit,
		cf,
	)
	if routeErr != nil {
		fs.GetContext().GetLogger().ErrorWith(fs.GetContext().GetConfig().LogOriginFrame()).
			Err(routeErr).
			Msg("RegisterModuleInitialize route providers failed")
	}

	if !routeHandled && fs.GetModule() != nil {
		// 注册模块/子系统路由处理器
		fs.GetModule().RegisterModuleRouteHandlers(cf)
	}
}

// RegisterModuleSwagger 注册模块/子系统级的swagger
func (cf *CoreWithFiber) RegisterModuleSwagger(fs FrameStarter, managers ...IProviderManager) {
	if cf.initializationFailed() {
		return
	}
	if cf.GetAppContext().Phase() >= PhaseServing {
		return
	}

	_, replaced, err := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationModuleSwaggerInit,
		cf,
	)
	if err != nil {
		cf.GetAppContext().GetLogger().ErrorWith(cf.GetAppContext().GetConfig().LogOriginFrame()).
			Err(err).
			Msg("RegisterModuleSwagger providers failed")
		return
	}
	if replaced {
		return
	}

	registerOrNot := cf.GetAppContext().GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		if fs.GetModule() != nil {
			// 注册模块系统的swagger
			fs.GetModule().RegisterSwagger(cf)
		}
	}
}

// RegisterAppHooks 注册核心应用的生命周期钩子函数（如果存在）
func (cf *CoreWithFiber) RegisterAppHooks(fs FrameStarter, managers ...IProviderManager) {
	if cf.initializationFailed() {
		return
	}
	if cf.GetAppContext().Phase() >= PhaseServing {
		return
	}

	_, replaced, err := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationCoreHookInit,
		cf,
	)
	if err != nil {
		cf.GetAppContext().GetLogger().ErrorWith(cf.GetAppContext().GetConfig().LogOriginFrame()).
			Err(err).
			Msg("RegisterAppHooks providers failed")
		return
	}
	if replaced {
		return
	}

	// 注册应用注册器的钩子函数
	if fs.GetApplication() != nil {
		fs.GetApplication().(ApplicationRegister).RegisterCoreHook(cf)
	}

	cf.coreApp.Hooks().OnListen(func(listenData fiber.ListenData) error {
		if fiber.IsChild() {
			return nil
		}

		// 监听已真正建立，此刻才是 PhaseServing 的置位时机。
		//
		// 不能等 Listen 返回后再置位：Listen 会一直阻塞到服务器停止，那时置位
		// 恰好把语义颠倒——「正在服务」写在了已经停止服务之后。迁移失败（例如
		// 关闭已先行开始）不应阻断监听，因此只记录不返回错误。
		if err := cf.GetAppContext().TransitionTo(PhaseServing); err != nil {
			cf.GetAppContext().GetLogger().WarnWith(cf.GetAppContext().GetConfig().LogOriginFrame()).
				Str("applicationStarter", "FrameApplication").
				Err(err).
				Msg("App phase transition to serving was rejected")
		}

		scheme := "http"
		if listenData.TLS {
			scheme = "https"
		}
		cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Str("appListen", listenData.Host+":"+listenData.Port).Msg(scheme + "://" + listenData.Host + ":" + listenData.Port)
		return nil
	})

	// 保留 OnShutdown hook，只覆盖「绕过 Shutdown」这条路径。
	//
	// 应用可以直接调用 coreApp.Shutdown()，那条路径上只有本 hook 会触发；移除它会让
	// 那些调用方彻底失去回收。
	//
	// 但经由 Shutdown 关闭时本 hook 必须让位：fiber 在 coreApp.Shutdown() 内**同步**
	// 执行 hook（v2.52.14 app.go:888，`defer app.hooks.executeOnShutdownHooks()` 位于
	// `app.server == nil` 检查之上，故无条件触发），若此时回收，日志器会在
	// ServerShutdownAfter 位点加载之前就被关掉——该位点的 provider 于是写进已关闭的
	// writer，日志静默丢失。这正是本次要修的顺序缺陷，仓库内的消费者是 pprof 关闭
	// 提供者。
	cf.coreApp.Hooks().OnShutdown(func() error {
		if cf.shutdownCoordinated.Load() {
			return nil
		}
		cf.clearApplicationGlobalsOnce()
		return nil
	})
}

// clearApplicationGlobalsOnce 回收全局对象并关闭日志器，重复调用为空操作。
//
// 顺序：先停健康检查、释放并清空容器，再记录关闭日志，最后关闭日志器。日志器必须
// 最后关——释放失败要靠框架日志器逐 key 上报，日志器已关则这些错误不可见。
func (cf *CoreWithFiber) clearApplicationGlobalsOnce() {
	if !cf.globalsCleared.CompareAndSwap(false, true) {
		return
	}

	appCtx := cf.GetAppContext()
	if appCtx == nil {
		return
	}

	// 回收/关闭相关资源：后台健康检查、连接池、缓存、实现 Closable 的中间件等。
	clearApplicationGlobals(appCtx)
	appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).
		Str("applicationStarter", "FrameApplication").
		Str("appShutdown", "ok").
		Msg("")
	_ = appCtx.GetLogger().Close()
}

// AppCoreRun 监听核心应用套接字
func (cf *CoreWithFiber) AppCoreRun(managers ...IProviderManager) error {
	// 装配失败的核心不能运行。返回 initErr 使失败在调用方（boot.go 的
	// coordinateServerRun）可观测，而不是交出一个静默不工作的核心。
	if cf.initializationFailed() {
		return cf.initErr
	}
	if cf.GetAppContext().Phase() >= PhaseServing {
		return nil
	}

	_, replaced, err := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationServerRun,
		cf,
	)
	if err != nil {
		return fmt.Errorf("failed to load server run providers: %w", err)
	}
	if replaced {
		return nil
	}

	cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("App listening...")
	cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("App: Manager for application processing server runtime")

	host, port := cf.GetAppContext().GetConfig().String("application.server.host"), cf.GetAppContext().GetConfig().String("application.server.port")

	if err = cf.coreApp.Listen(host + ":" + port); err != nil {
		cf.GetAppContext().GetLogger().ErrorWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("App listen failed")
		return err
	}

	// 这里不再置位「已启动」：Listen 返回意味着服务器**已经停止**，此处置位会把
	// 语义颠倒。PhaseServing 由 RegisterAppHooks 安装的 OnListen hook 在监听真正
	// 建立的那一刻置位。
	return nil
}

// Shutdown 关闭应用
//
// 回收写在顶部的 defer 里，而不是方法尾部：本方法有六处提前返回（AppState 守卫、
// 三处位点加载失败、replaced 短路、引擎关闭失败），尾部语句会被它们全部跳过。
// Gin 侧能用尾部写法，是因为它另有 `defer cg.releaseGinLogger()` 配套。
//
// 回收本身经 once 守卫，因此与 OnShutdown hook 中的那次调用只会生效一次：正常路径
// 由 hook 先执行（fiber 在 coreApp.Shutdown() 内同步触发），此处的 defer 成为空操作；
// 提前返回时 hook 未触发，则由此处兜底。
func (cf *CoreWithFiber) Shutdown(managers ...IProviderManager) error {
	defer cf.clearApplicationGlobalsOnce()

	// 关闭结束时推进到 PhaseTerminated。迁移失败说明已有其他路径先行终结，
	// 不影响本次回收（回收本身经 once 守卫）。
	defer func() { _ = cf.GetAppContext().TransitionTo(PhaseTerminated) }()

	// 与 Gin 一致：装配失败时把 initErr 作为错误返回。顶部的 defer 仍会执行，
	// 因此即使核心从未装配成功，已注册的全局对象依旧得到回收。
	if cf.initializationFailed() {
		return cf.initErr
	}

	// 守卫读的是「关闭是否已经开始」，而不再是旧的 GetAppState()。
	//
	// 旧守卫的缺陷：GetAppState() 在使用方提前 pin 之后恒为 true，Shutdown 于是
	// 直接返回并跳过全部资源回收。改读阶段之后，只有真正进入关闭的调用才会被
	// 挡住，重复关闭仍是幂等的。
	if cf.GetAppContext().Phase() >= PhaseShuttingDown {
		return nil
	}
	_ = cf.GetAppContext().TransitionTo(PhaseShuttingDown)

	_, replaced, err := LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationServerShutdown,
		cf,
	)
	if err != nil {
		return fmt.Errorf("failed to load server shutdown providers: %w", err)
	}
	if replaced {
		return nil
	}

	_, _, err = LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationServerShutdownBefore,
		cf,
	)
	if err != nil {
		return fmt.Errorf("failed to load pre-shutdown providers: %w", err)
	}

	cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("Fiber app Shutting down...")

	// 置位后 OnShutdown hook 让位，回收改由顶部的 defer 在 ServerShutdownAfter 之后执行。
	cf.shutdownCoordinated.Store(true)
	err = cf.coreApp.Shutdown()
	if err != nil {
		cf.GetAppContext().GetLogger().ErrorWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Err(err).Msg("Fiber app Shutdown failed.")
		return err
	}

	_, _, err = LoadProviderManagersAtLocation(
		managers,
		ProviderLocationDefault().LocationServerShutdownAfter,
		cf,
	)
	if err != nil {
		return fmt.Errorf("failed to load post-shutdown providers: %w", err)
	}

	// 关闭 provider 持有的资源。位置夹在两端之间：ServerShutdownAfter 之后
	// （该位点的 provider 仍可能使用这些资源），全局对象释放与日志器关闭之前
	// （关闭失败要靠框架日志器上报）。
	closeShutdownChainProviders(cf.GetAppContext(), managers)

	// 日志器由顶部 defer 的 clearApplicationGlobalsOnce 关闭：它必须在全局对象释放
	// 之后才关，否则逐 key 上报的释放失败会写进已关闭的 writer。此处不再重复关闭
	// ——LoggerWrap.Close 没有幂等保护，重复关闭是否安全完全取决于注入的 writer。
	return nil
}
