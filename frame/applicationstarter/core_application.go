package applicationstarter

import (
	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/frame"
	frameRecover "github.com/lamxy/fiberhouse/frame/middleware/recover"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// CoreFiber 应用核心启动器
type CoreFiber struct {
	ctx     frame.ContextFramer
	coreCfg *fiber.Config
	coreApp *fiber.App
}

// NewCoreFiber 创建一个应用核心启动器对象
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
