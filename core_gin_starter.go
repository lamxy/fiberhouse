// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy/fiberhouse

package fiberhouse

import (
	"context"
	"errors"
	"fmt"
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse/appconfig"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// CoreWithGin 基于Gin的核心应用启动器
type CoreWithGin struct {
	ctx            IApplicationContext
	OptionFuncList []gin.OptionFunc
	coreApp        *gin.Engine
	httpServer     *http.Server
}

// NewCoreWithGin 创建一个基于Gin的应用核心启动器对象
func NewCoreWithGin(ctx IApplicationContext, opts ...CoreStarterOption) CoreStarter {
	core := &CoreWithGin{
		ctx: ctx,
	}

	if len(opts) > 0 {
		core.OptionFuncList = make([]gin.OptionFunc, 0, len(opts))
		for _, opt := range opts {
			opt(core)
		}
	}

	return core
}

// InitCoreApp 初始化应用核心
func (cg *CoreWithGin) InitCoreApp(fs FrameStarter, managers ...IProviderManager) {
	if cg.GetAppContext().GetAppState() {
		return
	}
	cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
		Str("applicationStarter", "GinApplication").
		Msg("InitCoreApp starting...")

	cfg := cg.GetAppContext().GetConfig()

	// 设置Gin运行模式
	mode := cfg.String("application.plugins.server.gin.mode", gin.ReleaseMode)
	if cfg.GetRecover().DebugMode {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)

	// 创建Gin引擎
	cg.coreApp = gin.New(cg.OptionFuncList...)

	// 配置JSON序列化器
	if len(managers) == 0 {
		// 使用默认的JSON编解码提供者
		cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).Msg("No JSON codec manager provided, using default JSON codec")
		ginJson.API = GetMustInstance[ginJson.Core](fs.GetApplication().GetDefaultJsonCodecKey())
	} else {
		var jsonCodecManager IProviderManager

		for _, manager := range managers {
			if manager.Type().GetTypeID() == ProviderTypeDefault().GroupJsonCodecChoose.GetTypeID() {
				jsonCodecManager = manager
				break
			}
		}

		if jsonCodecManager == nil {
			msg := "No JSON codec manager provided, using default JSON codec"
			cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).Msg(msg)
			panic(msg)
		}

		_, err := jsonCodecManager.LoadProvider()

		if err != nil {
			cg.GetAppContext().GetLogger().ErrorWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
				Str("applicationStarter", "GinApplication").
				Err(err).
				Msg("Failed to load providers into Gin application starter")
			panic(err)
		}
	}

	// 初始化HTTP Server
	cg.initHttpServer(cfg)
}

// initHttpServer 初始化HTTP服务器
func (cg *CoreWithGin) initHttpServer(cfg appconfig.IAppConfig) {
	host := cfg.String("application.plugins.server.gin.host")
	port := cfg.String("application.plugins.server.gin.port")

	cg.httpServer = &http.Server{
		Addr:           host + ":" + port,
		Handler:        cg.coreApp,
		ReadTimeout:    cfg.Duration("application.plugins.server.gin.readTimeout", 30) * time.Second,
		WriteTimeout:   cfg.Duration("application.plugins.server.gin.writeTimeout", 30) * time.Second,
		IdleTimeout:    cfg.Duration("application.plugins.server.gin.idleTimeout", 60) * time.Second,
		MaxHeaderBytes: cfg.Int("application.plugins.server.gin.bodyLimit", 4096) * 1024,
	}
}

// RegisterAppMiddleware 注册应用级的中间件
func (cg *CoreWithGin) RegisterAppMiddleware(fs FrameStarter, managers ...IProviderManager) {
	if cg.GetAppContext().GetAppState() {
		return
	}
	cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
		Str("applicationStarter", "GinApplication").
		Msg("RegisterAppMiddleware")

	debugMode := cg.GetAppContext().GetConfig().GetRecover().DebugMode

	// 遍历管理器切片，筛选类型为GroupRecoverMiddlewareChoose管理器，然后加载对应的恢复中间件提供者
	//或者，直接从NewErrorHandlerOnce单例获取恢复中间件

	// IErrorHandler接口实例
	eh := NewErrorHandlerOnce(cg.GetAppContext())

	recoverHandler := eh.RecoverMiddleware(Config{
		AppCtx:            cg.GetAppContext(),
		EnableStackTrace:  true,
		StackTraceHandler: eh.DefaultStackTraceHandler,
		Logger:            cg.GetAppContext().GetLogger(),
		Stdout:            false,
		DebugMode:         debugMode, // true开启调试模式，将详细错误信息显示给客户端，否则隐藏细节，只能通过日志文件查看。生产环境关闭该调式模式。
	})

	// 注册错误恢复中间件
	cg.coreApp.Use(MustRecoverMiddleware[gin.HandlerFunc](recoverHandler))

	// 注册HTTP请求日志中间件
	cg.coreApp.Use(cg.loggerMiddleware())

	// 注册项目应用注册器全局中间件
	if fs.GetApplication() != nil {
		// 注册项目应用注册器全局中间件
		fs.GetApplication().(ApplicationRegister).RegisterAppMiddleware(cg)
	}
}

// loggerMiddleware HTTP请求日志中间件
func (cg *CoreWithGin) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ms := cg.GetAppContext().GetConfig().GetMiddlewareSwitch("coreHttp")
		if !ms {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		logEvent := cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginCoreHttp()).
			Str("method", method).
			Str("path", path).
			Int("status", statusCode).
			Dur("latency", latency).
			Str("ip", clientIP).
			Int("bodySize", c.Writer.Size())

		if query != "" {
			logEvent.Str("query", query)
		}

		if len(c.Errors) > 0 {
			logEvent.Str("errors", c.Errors.String())
		}

		logEvent.Msg("HTTP Request")
	}
}

// RegisterModuleInitialize 注册应用模块/子系统级的中间件、路由处理器等
func (cg *CoreWithGin) RegisterModuleInitialize(fs FrameStarter, managers ...IProviderManager) {
	if cg.GetAppContext().GetAppState() {
		return
	}
	if fs.GetModule() != nil {
		// 注册模块/子系统路由处理器
		fs.GetModule().RegisterModuleRouteHandlers(cg)
	}
}

// RegisterModuleSwagger 注册模块/子系统级的swagger
func (cg *CoreWithGin) RegisterModuleSwagger(fs FrameStarter, managers ...IProviderManager) {
	if cg.GetAppContext().GetAppState() {
		return
	}
	registerOrNot := cg.GetAppContext().GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		if fs.GetModule() != nil {
			// 注册模块系统的swagger
			fs.GetModule().RegisterSwagger(cg)
		}
	}
}

// RegisterAppHooks 注册核心应用的生命周期钩子函数
func (cg *CoreWithGin) RegisterAppHooks(fs FrameStarter, managers ...IProviderManager) {
	if cg.GetAppContext().GetAppState() {
		return
	}

	// 注册应用注册器的钩子函数
	if fs.GetApplication() != nil {
		fs.GetApplication().(ApplicationRegister).RegisterCoreHook(cg)
	}
}

// AppCoreRun 启动Gin应用并监听信号
func (cg *CoreWithGin) AppCoreRun(managers ...IProviderManager) {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// 启动HTTP服务器
	go func(app *CoreWithGin) {
		cfg := app.GetAppContext().GetConfig()
		scheme := "http"
		if app.httpServer.TLSConfig != nil {
			scheme = "https"
		}

		app.GetAppContext().GetLogger().InfoWith(cfg.LogOriginFrame()).
			Str("applicationStarter", "GinApplication").
			Str("scheme", scheme).
			Str("addr", app.httpServer.Addr).
			Msg(fmt.Sprintf("Gin app listening on %s://%s", scheme, app.httpServer.Addr))

		if err := app.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.GetAppContext().GetLogger().FatalWith(cfg.LogOriginFrame()).
				Str("applicationStarter", "GinApplication").
				Err(err).
				Msg("Failed to start Gin server")
		}
		app.GetAppContext().RegisterAppState(true)
	}(cg)

	// 等待信号
	<-stopCh

	// 执行优雅关闭
	cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
		Str("applicationStarter", "GinApplication").
		Msg("Shutting down Gin server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := cg.httpServer.Shutdown(shutdownCtx); err != nil {
		cg.GetAppContext().GetLogger().ErrorWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
			Str("applicationStarter", "GinApplication").
			Err(err).
			Msg("Gin server forced to shutdown")
	}

	// 清理资源
	cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
		Str("applicationStarter", "GinApplication").
		Msg("Cleaning up resources...")

	cg.GetAppContext().GetContainer().ClearAll(true)
	_ = cg.GetAppContext().GetLogger().Close()

	cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
		Str("applicationStarter", "GinApplication").
		Msg("Gin server shutdown complete")
}

// GetAppContext 获取应用上下文
func (cg *CoreWithGin) GetAppContext() IApplicationContext {
	return cg.ctx
}

// GetCoreApp 获取核心Gin引擎实例
func (cg *CoreWithGin) GetCoreApp() interface{} {
	return cg.coreApp
}
