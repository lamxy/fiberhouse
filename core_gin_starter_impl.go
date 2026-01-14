// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/provider/adaptor"
	"net"
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
		ginJson.API = GetMustInstance[ginJson.Core](fs.GetApplication().GetDefaultTrafficCodecKey())
	} else {
		var jsonCodecManager IProviderManager

		for _, manager := range managers {
			if manager.Type().GetTypeID() == ProviderTypeDefault().GroupTrafficCodecChoose.GetTypeID() {
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
	// NewCoreWithGin的选项参数已初始化httpServer,此处无需重复初始化
	if cg.httpServer != nil {
		return
	}

	host := cfg.String("application.plugins.engine.servers.gin.host", "0.0.0.0")
	port := cfg.String("application.plugins.engine.servers.gin.port", "8080")

	cg.httpServer = &http.Server{
		Addr:    host + ":" + port,
		Handler: cg.coreApp,

		// 读取超时时间（默认30秒）
		ReadTimeout: cfg.Duration("application.plugins.engine.servers.gin.readTimeout", 30) * time.Second,

		// 写入超时时间（默认30秒）
		WriteTimeout: cfg.Duration("application.plugins.engine.servers.gin.writeTimeout", 30) * time.Second,

		// 空闲超时时间（默认120秒）
		IdleTimeout: cfg.Duration("application.plugins.engine.servers.gin.idleTimeout", 120) * time.Second,

		// 请求头最大字节数（默认1MB）
		MaxHeaderBytes: cfg.Int("application.plugins.engine.servers.gin.maxHeaderBytes", 1024) * 1024,

		// 读取请求头超时时间（默认10秒）
		ReadHeaderTimeout: cfg.Duration("application.plugins.engine.servers.gin.readHeaderTimeout", 10) * time.Second,

		// TLS配置（如果启用HTTPS）
		TLSConfig: nil, // 可通过配置或选项函数设置

		// 错误日志记录器（使用应用的日志记录器）
		ErrorLog: nil, // 可通过选项函数自定义

		// 连接状态回调函数
		ConnState: nil, // 可通过选项函数自定义

		// 连接上下文函数
		ConnContext: nil, // 可通过选项函数自定义

		// 基础上下文函数
		BaseContext: func(listener net.Listener) context.Context {
			return context.Background()
		},
	}

	// 配置TLS/HTTPS（如果启用）
	if cg.httpServer.TLSConfig == nil && cfg.Bool("application.plugins.engine.servers.gin.tls.enable") {
		// 如果配置了HTTPS相关参数，则启用TLS
		certFile := cfg.String("application.plugins.engine.servers.gin.tls.certFile", "")
		keyFile := cfg.String("application.plugins.engine.servers.gin.tls.keyFile", "")

		if certFile != "" && keyFile != "" {
			msg := fmt.Sprintf("Enabling TLS/HTTPS with certFile: %s and keyFile: %s", certFile, keyFile)
			cg.GetAppContext().GetLogger().InfoWith(cfg.LogOriginFrame()).
				Str("applicationStarter", "GinApplication").
				Msg(msg)
			panic(msg)
		}
		// 加载TLS证书配置
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			cg.GetAppContext().GetLogger().ErrorWith(cfg.LogOriginFrame()).
				Str("applicationStarter", "GinApplication").
				Err(err).
				Msg("Failed to load TLS certificates")
		} else {
			cg.httpServer.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				// 最小TLS版本（默认TLS 1.2）
				MinVersion: tls.VersionTLS12,
				// 密码套件配置
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				},
			}

			cg.GetAppContext().GetLogger().InfoWith(cfg.LogOriginFrame()).
				Str("applicationStarter", "GinApplication").
				Msg("TLS/HTTPS enabled")
		}
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

	recoverHandler := eh.RecoverMiddleware(RecoverConfig{
		AppCtx:            cg.GetAppContext(),
		EnableStackTrace:  true,
		StackTraceHandler: eh.DefaultStackTraceHandler,
		Logger:            cg.GetAppContext().GetLogger(),
		Stdout:            false,
		JsonCodec:         ginJson.API.Marshal,
		DebugMode:         debugMode, // true开启调试模式，将详细错误信息显示给客户端，否则隐藏细节，只能通过日志文件查看。生产环境关闭该调式模式。
	})

	// 注册panic恢复中间件
	//cg.coreApp.Use(recoverHandler.(func(ctx *gin.Context)))
	cg.coreApp.Use(MustRecoverMiddleware[func(ctx *gin.Context)](recoverHandler))

	// 注册错误处理器中间件
	cg.coreApp.Use(adaptor.GinErrorHandler(eh.ErrorHandler))

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
