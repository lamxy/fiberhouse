// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package applicationstarter 提供基于 Fiber 框架的应用启动器实现，负责应用的完整生命周期管理和启动流程编排。
package applicationstarter

import (
	"errors"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/validate"
	"strings"
	"time"
)

// RunApplicationStarter 接受实现了ApplicationStarter接口的实例，执行应用启动流程
func RunApplicationStarter(starter fiberhouse.ApplicationStarter) {
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

// FrameApplication 框架应用启动器实现，实现了 fiberhouse.ApplicationStarter 接口
type FrameApplication struct {
	Ctx         fiberhouse.ContextFramer
	application fiberhouse.ApplicationRegister
	module      fiberhouse.ModuleRegister
	task        fiberhouse.TaskRegister
}

// NewFrameApplication 创建一个应用启动器对象
func NewFrameApplication(ctx fiberhouse.ContextFramer, opts ...fiberhouse.FrameStarterOption) fiberhouse.FrameStarter {
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

// GetContext 获取应用上下文
func (fa *FrameApplication) GetContext() fiberhouse.ContextFramer {
	return fa.Ctx
}

// GetFrameApp 获取框架启动器实例
func (fa *FrameApplication) GetFrameApp() fiberhouse.FrameStarter {
	return fa
}

// GetAppContext 获取应用上下文
func (cf *CoreFiber) GetAppContext() fiberhouse.ContextFramer {
	return cf.ctx
}

// RegisterApplication 注入应用注册器实例到应用启动器的application属性
func (fa *FrameApplication) RegisterApplication(application fiberhouse.ApplicationRegister) {
	fa.application = application
}

// RegisterModule 注入应用模块/子系统注册器实例到应用启动器的module属性
func (fa *FrameApplication) RegisterModule(module fiberhouse.ModuleRegister) {
	fa.module = module
}

// RegisterTask 注入异步任务注册器实例到应用启动器的task属性
func (fa *FrameApplication) RegisterTask(task fiberhouse.TaskRegister) {
	fa.task = task
}

// GetApplication 获取实现IApplication接口的应用对象（ApplicationRegister）
func (fa *FrameApplication) GetApplication() fiberhouse.IApplication {
	return fa.application
}

// GetModule 获取ModuleRegister对象
func (fa *FrameApplication) GetModule() fiberhouse.ModuleRegister {
	return fa.module
}

// GetTask 获取TaskRegister对象
func (fa *FrameApplication) GetTask() fiberhouse.TaskRegister {
	return fa.task
}

// RegisterToCtx 注册应用启动器对象到应用上下文
func (fa *FrameApplication) RegisterToCtx(as fiberhouse.ApplicationStarter) {
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

	appRegister := fa.GetApplication().(fiberhouse.ApplicationRegister)
	fa.GetContext().GetContainer().Registers(appRegister.ConfigGlobalInitializers())
}

// InitializeGlobalRequired 初始化应用启动时必要的全局对象
func (fa *FrameApplication) InitializeGlobalRequired() {
	if fa.GetContext().GetAppState() {
		return
	}
	if fa.GetApplication() != nil {
		appRegister := fa.GetApplication().(fiberhouse.ApplicationRegister)
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
		appRegister := fa.GetApplication().(fiberhouse.ApplicationRegister)
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
		appRegister := fa.GetApplication().(fiberhouse.ApplicationRegister)
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
