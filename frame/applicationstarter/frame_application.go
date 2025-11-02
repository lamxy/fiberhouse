// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package applicationstarter 提供基于 Fiber 框架的应用启动器实现，负责应用的完整生命周期管理和启动流程编排。
package applicationstarter

import (
	"errors"
	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/frame"
	"github.com/lamxy/fiberhouse/frame/component/validate"
	frameRecover "github.com/lamxy/fiberhouse/frame/middleware/recover"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// RunApplicationStarter 接受实现了ApplicationStarter接口的实例，执行应用启动流程
// coreCfg 应用启动器包装的底层http服务对象初始化配置，可选参数
func RunApplicationStarter(starter frame.ApplicationStarter) {
	// 应用启动流程，保持执行顺序
	starter.RegisterToCtx(starter)
	starter.RegisterApplicationGlobals()
	starter.InitCoreApp(starter.GetFrameApp())
	starter.RegisterAppHooks(starter.GetFrameApp())
	starter.RegisterAppMiddleware(starter.GetFrameApp())
	starter.RegisterModuleInitialize(starter.GetFrameApp())
	starter.RegisterModuleSwagger(starter.GetFrameApp())
	starter.RegisterTaskServer()
	starter.RegisterGlobalsKeepalive()
	starter.AppCoreRun()
}

type WebApplication struct {
	frame.FrameStarter
	frame.CoreStarter
}

// FrameApplication 框架应用启动器实现，实现了 frame.ApplicationStarter 接口
type FrameApplication struct {
	Ctx         frame.ContextFramer
	coreCfg     *fiber.Config
	coreApp     *fiber.App
	application frame.ApplicationRegister
	module      frame.ModuleRegister
	task        frame.TaskRegister
	appStarted  bool // 应用启动状态
}

// NewFrameApplication 创建一个应用启动器对象
func NewFrameApplication(ctx frame.ContextFramer, opts ...frame.FrameStarterOption) frame.FrameStarter {
	fApp := &FrameApplication{
		Ctx: ctx,
	}
	if len(opts) == 0 {
		ctx.GetLogger().FatalWith(ctx.GetConfig().LogOriginFrame()).Msg("no registrar option available for injection into the application starter via NewFrameApplication")
	}

	for _, opt := range opts {
		opt(fApp)
	}

	return fApp
}

type CoreFiber struct {
	ctx     frame.ContextFramer
	coreCfg *fiber.Config
	coreApp *fiber.App
}

func NewCoreFiber(ctx frame.ContextFramer, opts ...frame.CoreStarterOption) frame.CoreStarter {
	core := &CoreFiber{
		ctx: ctx,
	}

	if len(opts) > 0 {
		for _, opt := range opts {
			opt(core)
		}
	}

	return core
}

// GetContext 获取应用上下文
func (fa *FrameApplication) GetContext() frame.ContextFramer {
	return fa.Ctx
}

// GetFrameApp 获取框架启动器实例
func (fa *FrameApplication) GetFrameApp() frame.FrameStarter {
	return fa
}

// GetAppContext 获取应用上下文
func (cf *CoreFiber) GetAppContext() frame.ContextFramer {
	return cf.ctx
}

// RegisterApplication 注入应用注册器实例到应用启动器的application属性
func (fa *FrameApplication) RegisterApplication(application frame.ApplicationRegister) {
	fa.application = application
}

// RegisterModule 注入应用模块/子系统注册器实例到应用启动器的module属性
func (fa *FrameApplication) RegisterModule(module frame.ModuleRegister) {
	fa.module = module
}

// RegisterTask 注入异步任务注册器实例到应用启动器的task属性
func (fa *FrameApplication) RegisterTask(task frame.TaskRegister) {
	fa.task = task
}

// GetApplication 获取实现IApplication接口的应用对象（ApplicationRegister）
func (fa *FrameApplication) GetApplication() frame.IApplication {
	return fa.application
}

// GetModule 获取ModuleRegister对象
func (fa *FrameApplication) GetModule() frame.ModuleRegister {
	return fa.module
}

// GetTask 获取TaskRegister对象
func (fa *FrameApplication) GetTask() frame.TaskRegister {
	return fa.task
}

// GetCoreApp 获取应用核心启动器实例
func (cf *CoreFiber) GetCoreApp() frame.CoreStarter {
	return cf
}

// InitCoreApp 初始化应用核心（框架应用基于 fiber.App）
func (cf *CoreFiber) InitCoreApp(fs frame.FrameStarter) {
	if cf.GetAppContext().GetAppState() {
		return
	}
	cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("InitCoreApp starting...")

	// 自定义核心配置
	if cf.coreCfg != nil {
		cf.coreApp = fiber.New(*cf.coreCfg)
		return
	}

	cfg := cf.GetAppContext().GetConfig()
	// frame.JsonWrapper序列化反序列化接口，默认编解码器实例
	json := frame.GetMustInstance[frame.JsonWrapper](fs.GetApplication().GetDefaultJsonCodecKey())
	// IRecover接口实例
	rc := frameRecover.NewRecoverCatch(cf.GetAppContext())
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
		ErrorHandler: rc.ErrorHandler,
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
		JSONEncoder:       json.Marshal,
		JSONDecoder:       json.Unmarshal,
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

// RegisterCoreCfg 注册应用核心配置对象到应用启动器
func (cf *CoreFiber) RegisterCoreCfg(coreCfg interface{}) {
	if cfg, ok := coreCfg.(*fiber.Config); ok {
		cf.coreCfg = cfg
	} else {
		cf.GetAppContext().GetLogger().WarnWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Msg("RegisterCoreCfg coreCfg isn't a fiber.Config")
	}
}

// RegisterToCtx 注册应用启动器对象到应用上下文
func (fa *FrameApplication) RegisterToCtx(as frame.ApplicationStarter) {
	if fa.GetContext().GetAppState() {
		return
	}
	fa.GetContext().RegisterStarterApp(as)
}

// RegisterApplicationGlobals 注册应用全局初始化逻辑
//
// 注册全局对象初始化器
// 初始化必要的全局对象和组件
// 注册自定义新增语言的验证器实例到验证其包装器中
// 注册自定义验证器tag和tag的语言翻译
func (fa *FrameApplication) RegisterApplicationGlobals() {
	if fa.GetContext().GetAppState() {
		return
	}
	fa.GetContext().GetLogger().InfoWith(fa.GetContext().GetConfig().LogOriginFrame()).
		Str("applicationStarter", "FrameApplication").Msg("RegisterApplicationGlobals")

	// 注册配置文件预定义LogOrigin不同来源的子日志器初始化器到全局管理容器
	fa.RegisterLoggerWithOriginToContainer()

	// 注册自定义全局对象初始化器、初始化预启动对象、初始化自定义语言验证器、注册自定义验证器tag和tag的语言翻译
	fa.RegisterGlobalInitializers()
	fa.InitializeGlobalRequired()
	fa.InitializeCustomValidateInitializers()
	fa.RegisterValidatorCustomTags()

	if fa.GetTask() != nil {
		// 注册异步任务客户端和服务端初始化器到全局管理容器
		fa.GetTask().RegisterTaskServerToContainer()     // 异步任务服务器/服务端
		fa.GetTask().RegisterTaskDispatcherToContainer() // 异步任务分发器/客户端
	}
}

// RegisterGlobalInitializers 注册全局对象初始化器
func (fa *FrameApplication) RegisterGlobalInitializers() {
	if fa.GetContext().GetAppState() {
		return
	}

	if fa.GetApplication() == nil {
		panic(errors.New("application that implements the ApplicationRegister interface is nil. Please RegisterApplication first"))
	}

	appRegister := fa.GetApplication().(frame.ApplicationRegister)
	fa.GetContext().GetContainer().Registers(appRegister.ConfigGlobalInitializers())
}

// InitializeGlobalRequired 初始化应用启动时必要的全局对象
func (fa *FrameApplication) InitializeGlobalRequired() {
	if fa.GetContext().GetAppState() {
		return
	}
	if fa.GetApplication() != nil {
		appRegister := fa.GetApplication().(frame.ApplicationRegister)
		gm := fa.GetContext().GetContainer()
		for _, name := range appRegister.ConfigRequiredGlobalKeys() {
			_, err := gm.Get(name)
			if err != nil {
				fa.GetContext().GetLogger().ErrorWith(fa.GetContext().GetConfig().LogOriginFrame()).Err(err).Msgf("ApplicationRegister InitializeGlobalRequired error, keyName: %s", name)
				//panic(err)
			}
		}
	}
}

// InitializeCustomValidateInitializers 初始化自定义新增语言的验证器到验证包装器
func (fa *FrameApplication) InitializeCustomValidateInitializers() {
	if fa.GetContext().GetAppState() {
		return
	}
	if fa.GetApplication() != nil {
		fa.GetContext().GetLogger().InfoWith(fa.GetContext().GetConfig().LogOriginFrame()).Msg("InitializeCustomValidateInitializers starting...")
		appRegister := fa.GetApplication().(frame.ApplicationRegister)
		validateInitializers := appRegister.ConfigCustomValidateInitializers()
		if len(validateInitializers) > 0 {
			for i := range validateInitializers {
				validateInitializers[i]().RegisterToWrap(fa.GetContext().GetValidateWrap().(*validate.Wrap))
			}
		}
	}
}

// RegisterValidatorCustomTags 注册验证器自定义的tag及翻译，详细使用见 https://github.com/go-playground/validator README & _examples
func (fa *FrameApplication) RegisterValidatorCustomTags() {
	if fa.GetContext().GetAppState() {
		return
	}
	if fa.GetApplication() != nil {
		appRegister := fa.GetApplication().(frame.ApplicationRegister)
		// 初始化验证器以及注册验证器公共验证和自定义tag及其多语言翻译
		if errs := fa.GetContext().GetValidateWrap().RegisterCustomTags(appRegister.ConfigValidatorCustomTags()); errs != nil {
			var errBuilder = strings.Builder{}
			errBuilder.Grow(len(errs))
			for i := range errs {
				errBuilder.WriteString(errs[i].Error())
				errBuilder.WriteString(" \t\n ")
			}
			msg := errBuilder.String()
			fa.GetContext().GetLogger().ErrorWith(fa.GetContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("RegisterValidatorCustomTags errors: " + msg)
			//panic(msg)
		}
	}
}

// RegisterLoggerWithOriginToContainer 注册配置文件预定义LogOrigin不同来源的子日志器初始化器到容器
// 获取已初始化好日志来源标记的子日志器：
//
//	e.g. IContext.GetLoggerWithOrigin(originFormCfg appconfig.LogOrigin)
//
// 方便直接使用已附加来源标记的子日志器记录日志
func (fa *FrameApplication) RegisterLoggerWithOriginToContainer() {
	if fa.GetContext().GetAppState() {
		return
	}
	logOriginMap := fa.GetContext().GetConfig().GetLogOriginMap()
	gm := fa.GetContext().GetContainer()
	for originKey, logOriginVal := range logOriginMap {
		if originKey != "" {
			gm.Register(logOriginVal.InstanceKey(), func() (interface{}, error) {
				log := fa.GetContext().GetLogger().With().Str("Origin", logOriginVal.String()).Logger()
				return &log, nil
			})
		}
	}
}

// RegisterGlobalsKeepalive 注册需要保活的全局对象后台健康检测
func (fa *FrameApplication) RegisterGlobalsKeepalive() {
	if fa.GetContext().GetAppState() {
		return
	}
	// 全局对象健康检测和保活
	if fa.GetContext().GetConfig().Bool("application.globalManage.keepAlive") {
		d := fa.GetContext().GetConfig().Duration("application.globalManage.interval", 180) * time.Second
		fa.startHealthCheck(d)
	}
}

// StartHealthCheck 异步检查全局对象是否健康和重建
func (fa *FrameApplication) startHealthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func(app *FrameApplication, t *time.Ticker) {
		gm, log, cfg := app.GetContext().GetContainer(), app.GetContext().GetLogger(), app.GetContext().GetConfig()
		defer func(t *time.Ticker) {
			t.Stop()
			if r := recover(); r != nil {
				switch re := r.(type) {
				case error:
					log.Error(cfg.LogOriginFrame()).Err(re).Str("from", "global manager").Msg("StartHealthCheck recover Error")
				default:
					log.Error(cfg.LogOriginFrame()).Str("from", "global manager").Msgf("StartHealthCheck recover Error: %v", re)
				}
			}
		}(t)
		for range t.C {
			gm.Range(func(key, value interface{}) bool {
				name := key.(string)
				ret, err := gm.CheckHealth(name)
				if err != nil {
					log.Error(cfg.LogOriginFrame()).Err(err).Msgf("global object from key: '%s', health check failure", name) // return false to stop iteration
					return true
				}
				if !ret {
					log.Error(cfg.LogOriginFrame()).Msgf("global resource '%s' is unhealthy, rebuilding...", name)
					err = gm.Rebuild(name)
					if err != nil {
						log.Error(cfg.LogOriginFrame()).Err(err).Msgf("global resource '%s' rebuild failed.", name)
					}
					log.Info(cfg.LogOriginFrame()).Err(err).Msgf("global resource '%s' rebuild success.", name)
				}
				return true
			})
		}
	}(fa, ticker)
}

// RegisterAppMiddleware 注册应用级的中间件
func (cf *CoreFiber) RegisterAppMiddleware(fs frame.FrameStarter) {
	if cf.GetAppContext().GetAppState() {
		return
	}
	cf.GetAppContext().GetLogger().Info(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("RegisterAppMiddleware")
	debugMode := cf.GetAppContext().GetConfig().GetRecover().DebugMode
	// IRecover接口实例
	rc := frameRecover.NewRecoverCatch(cf.GetAppContext())

	// 注册核心应用(coreApp/fiber App)全局错误捕获中间件
	cf.coreApp.Use(frameRecover.New(frameRecover.Config{
		EnableStackTrace:  true,
		StackTraceHandler: rc.DefaultStackTraceHandler,
		Logger:            cf.GetAppContext().GetLogger(),
		AppContext:        cf.GetAppContext(),
		Stdout:            false,
		DebugMode:         debugMode, // true开启调试模式，将详细错误信息显示给客户端，否则隐藏细节，只能通过日志文件查看。生产环境关闭该调式模式。
	}))

	// 注册核心应用(coreApp/fiber App)http请求日志中间件
	cf.coreApp.Use(fiberzerolog.New(fiberzerolog.Config{
		Logger: func() *zerolog.Logger {
			log, err := cf.GetAppContext().GetContainer().Get(cf.GetAppContext().GetConfig().LogOriginCoreHttp().InstanceKey())
			if err != nil {
				// 获取http类子日志器错误
				cf.GetAppContext().GetLogger().Error(cf.GetAppContext().GetConfig().LogOriginFrame()).Err(err).Str("applicationStarter", "FrameApplication").Msg("RegisterAppMiddleware register fiberzerolog middleware to get http logger error")
				return nil // 使用默认日志器
			}
			return log.(*zerolog.Logger)
		}(),
		Next: func(c *fiber.Ctx) bool {
			ms := cf.GetAppContext().GetConfig().GetMiddlewareSwitch("coreHttp")
			return !ms
		},
	}))

	if fs.GetApplication() != nil {
		// 注册项目应用注册器全局中间件
		fs.GetApplication().(frame.ApplicationRegister).RegisterAppMiddleware(cf.coreApp)
	}
}

// RegisterModuleInitialize 注册应用模块/子系统级的中间件、路由处理器、swagger、etc...
func (cf *CoreFiber) RegisterModuleInitialize(fs frame.FrameStarter) {
	if cf.GetAppContext().GetAppState() {
		return
	}
	if fs.GetModule() != nil {
		// 注册模块/子系统中间件
		fs.GetModule().RegisterModuleMiddleware(cf.coreApp)
		// 注册模块/子系统路由处理器
		fs.GetModule().RegisterModuleRouteHandlers(cf.coreApp)
	}
}

// RegisterModuleSwagger 注册模块/子系统级的swagger
func (cf *CoreFiber) RegisterModuleSwagger(fs frame.FrameStarter) {
	if cf.GetAppContext().GetAppState() {
		return
	}
	registerOrNot := cf.GetAppContext().GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		if fs.GetModule() != nil {
			// 注册模块系统的swagger
			fs.GetModule().RegisterSwagger(cf.coreApp)
		}
	}
}

// RegisterTaskServer 注册启动异步任务服务器后台工作器服务
func (fa *FrameApplication) RegisterTaskServer() {
	if fa.GetContext().GetAppState() {
		return
	}
	enable := fa.GetContext().GetConfig().Bool("application.task.enableServer")
	if enable {
		if fa.GetTask() == nil {
			return
		}
		// 从容器获取任务工作者实例
		worker, err := fa.GetTask().GetTaskWorker(fa.GetContext().GetStarter().GetApplication().GetTaskServerKey())
		if err != nil {
			panic(err)
		}
		// 获取并注册批量任务处理器
		worker.RegisterHandlers(fa.GetTask().GetTaskHandlerMap())
		// 启动异步任务处理服务
		worker.RunServer()
	}
}

// RegisterAppHooks 注册核心应用的生命周期钩子函数（如果存在）
func (cf *CoreFiber) RegisterAppHooks(fs frame.FrameStarter) {
	if cf.GetAppContext().GetAppState() {
		return
	}

	// 注册应用注册器的钩子函数
	if fs.GetApplication() != nil {
		fs.GetApplication().(frame.ApplicationRegister).RegisterCoreHook(cf.coreApp)
	}

	cf.coreApp.Hooks().OnListen(func(listenData fiber.ListenData) error {
		if fiber.IsChild() {
			return nil
		}
		scheme := "http"
		if listenData.TLS {
			scheme = "https"
		}
		cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Str("appListen", listenData.Host+":"+listenData.Port).Msg(scheme + "://" + listenData.Host + ":" + listenData.Port)
		return nil
	})

	cf.coreApp.Hooks().OnShutdown(func() error {
		// 应用Shutdown时回调，回收/关闭相关资源，如后台程序(等待关闭信号)、异步任务(等待关闭信号)、连接池（关闭连接池）、中间件（封装实现Closable接口）等
		cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Str("appShutdown", "ok").Msg("")

		//fa.GetContext().GetContainer().ReleaseAll(true) // 释放资源
		cf.GetAppContext().GetContainer().ClearAll(true) // 将全局容器初始化，清空全局对象
		_ = cf.GetAppContext().GetLogger().Close()       // 日志器Close
		return nil
	})
}

// AppCoreRun 监听核心应用套接字
func (cf *CoreFiber) AppCoreRun() {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM) // 监听信号

	go func(app *CoreFiber) {
		app.GetAppContext().GetLogger().InfoWith(app.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("App listening...")
		host, port := app.GetAppContext().GetConfig().String("application.server.host"), app.GetAppContext().GetConfig().String("application.server.port")
		if err := app.coreApp.Listen(host + ":" + port); err != nil {
			app.GetAppContext().GetLogger().FatalWith(app.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("App listen failed")
		}
		app.GetAppContext().SetAppState(true)
	}(cf)

	<-stopCh

	cf.GetAppContext().GetLogger().InfoWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Msg("Fiber app Shutting down...")
	err := cf.coreApp.Shutdown()
	if err != nil {
		cf.GetAppContext().GetLogger().FatalWith(cf.GetAppContext().GetConfig().LogOriginFrame()).Str("applicationStarter", "FrameApplication").Err(err).Msg("Fiber app Shutdown failed.")
	}
}
