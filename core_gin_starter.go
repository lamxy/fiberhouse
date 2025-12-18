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

// CoreGin 基于Gin的核心应用启动器
type CoreGin struct {
	ctx            IApplicationContext
	OptionFuncList []gin.OptionFunc
	coreApp        *gin.Engine
	httpServer     *http.Server
}

// NewCoreGin 创建一个基于Gin的应用核心启动器对象
func NewCoreGin(ctx IApplicationContext, opts ...CoreStarterOption) CoreStarter {
	core := &CoreGin{
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
func (cg *CoreGin) InitCoreApp(fs FrameStarter, manager ...IProviderManager) {
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
	if len(manager) == 0 {
		// 使用默认的JSON编解码提供者
		cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).Msg("No JSON codec manager provided, using default JSON codec")
		ginJson.API = GetMustInstance[ginJson.Core](fs.GetApplication().GetDefaultJsonCodecKey())
	} else {
		jsonCodecManager := manager[0]

		if jsonCodecManager.Type().GetTypeID() != ProviderTypeDefault().GroupJsonCodec.GetTypeID() {
			panic("json codec manager type mismatch")
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
func (cg *CoreGin) initHttpServer(cfg appconfig.IAppConfig) {
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
func (cg *CoreGin) RegisterAppMiddleware(fs FrameStarter) {
	if cg.GetAppContext().GetAppState() {
		return
	}
	cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
		Str("applicationStarter", "GinApplication").
		Msg("RegisterAppMiddleware")

	debugMode := cg.GetAppContext().GetConfig().GetRecover().DebugMode

	// 注册错误恢复中间件
	cg.coreApp.Use(cg.recoverMiddleware(debugMode))

	// 注册HTTP请求日志中间件
	cg.coreApp.Use(cg.loggerMiddleware())

	// 注册项目应用注册器全局中间件
	if fs.GetApplication() != nil {
		// TODO: 需要适配ApplicationRegister接口,创建Gin版本的RegisterAppMiddleware
		// 当前ApplicationRegister.RegisterAppMiddleware接受*fiber.App参数
		// 需要改造为泛型接口或创建GinApplicationRegister接口
		cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
			Msg("TODO: Adapt ApplicationRegister.RegisterAppMiddleware(*fiber.App) to support *gin.Engine")

		// 临时方案:如果应用注册器实现了特定接口,可以调用
		// if ginRegister, ok := fs.GetApplication().(GinApplicationRegister); ok {
		//     ginRegister.RegisterGinAppMiddleware(cg.coreApp)
		// }
	}
}

// recoverMiddleware 错误恢复中间件
func (cg *CoreGin) recoverMiddleware(debugMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// TODO: 完整集成frame/middleware/recover包的RecoverCatch逻辑
				// 当前RecoverCatch依赖*fiber.Ctx,需要适配到*gin.Context
				//rc := frameRecover.NewRecoverCatch(cg.GetAppContext())

				cg.GetAppContext().GetLogger().ErrorWith(cg.GetAppContext().GetConfig().LogOriginCoreHttp()).
					Interface("panic", err).
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Str("ip", c.ClientIP()).
					Msg("Panic recovered")

				// TODO: 使用frame/response包统一响应格式
				// 需要创建Gin版本的响应包装器
				if debugMode {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    http.StatusInternalServerError,
						"message": "Internal Server Error",
						"error":   fmt.Sprintf("%v", err),
					})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    http.StatusInternalServerError,
						"message": "Internal Server Error",
					})
				}

				c.Abort()
			}
		}()
		c.Next()
	}
}

// loggerMiddleware HTTP请求日志中间件
func (cg *CoreGin) loggerMiddleware() gin.HandlerFunc {
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
func (cg *CoreGin) RegisterModuleInitialize(fs FrameStarter) {
	if cg.GetAppContext().GetAppState() {
		return
	}
	if fs.GetModule() != nil {
		// TODO: 适配ModuleRegister接口到Gin
		// 当前ModuleRegister方法接受*fiber.App参数
		// 方案1: 创建GinModuleRegister接口
		// 方案2: 将ModuleRegister改造为泛型接口
		cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
			Msg("TODO: Adapt ModuleRegister interface methods to support *gin.Engine")

		// 临时方案:
		// if ginModule, ok := fs.GetModule().(GinModuleRegister); ok {
		//     ginModule.RegisterGinModuleMiddleware(cg.coreApp)
		//     ginModule.RegisterGinModuleRouteHandlers(cg.coreApp)
		// }
	}
}

// RegisterModuleSwagger 注册模块/子系统级的swagger
func (cg *CoreGin) RegisterModuleSwagger(fs FrameStarter) {
	if cg.GetAppContext().GetAppState() {
		return
	}
	registerOrNot := cg.GetAppContext().GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		if fs.GetModule() != nil {
			// TODO: 集成gin-swagger
			// import ginSwagger "github.com/swaggo/gin-swagger"
			// import "github.com/swaggo/files"
			// cg.coreApp.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
			cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
				Msg("TODO: Integrate gin-swagger package for API documentation")
		}
	}
}

// RegisterAppHooks 注册核心应用的生命周期钩子函数
func (cg *CoreGin) RegisterAppHooks(fs FrameStarter) {
	if cg.GetAppContext().GetAppState() {
		return
	}

	// 注册应用注册器的钩子函数
	if fs.GetApplication() != nil {
		// TODO: ApplicationRegister.RegisterCoreHook接受*fiber.App参数
		// 需要适配到*gin.Engine或创建通用钩子接口
		cg.GetAppContext().GetLogger().InfoWith(cg.GetAppContext().GetConfig().LogOriginFrame()).
			Msg("TODO: Adapt ApplicationRegister.RegisterCoreHook to support Gin")
	}
}

// AppCoreRun 启动Gin应用并监听信号
func (cg *CoreGin) AppCoreRun() {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// 启动HTTP服务器
	go func(app *CoreGin) {
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
func (cg *CoreGin) GetAppContext() IApplicationContext {
	return cg.ctx
}

// GetCoreApp 获取核心Gin引擎实例
func (cg *CoreGin) GetCoreApp() interface{} {
	return cg.coreApp
}
