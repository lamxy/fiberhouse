// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// webStarter 提供基于 Fiber 框架的应用启动器实现，负责应用的完整生命周期管理和启动流程编排。

package fiberhouse

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/validate"
	"github.com/lamxy/fiberhouse/constant"
)

// FrameApplication 框架应用启动器实现，实现了 fiberhouse.ApplicationStarter 接口
type FrameApplication struct {
	Ctx          IApplicationContext
	application  ApplicationRegister
	module       ModuleRegister
	task         TaskRegister
	healthMu     sync.Mutex
	healthCancel context.CancelFunc
	healthWG     sync.WaitGroup
}

// 全局对象健康检查的配置键。
//
// configKeyHealthCheckLegacy 是 healthCheck 的旧名。该开关原名 "keepAlive"（保活），
// 但其行为只是周期性观测上报，从不保持任何连接存活，也不再触发重建，
// 名实不符易致误解，故更名为 healthCheck。旧键保留读取以兼容既有部署。
const (
	configKeyHealthCheck         = "application.globalManage.healthCheck"
	configKeyHealthCheckLegacy   = "application.globalManage.keepAlive"
	configKeyHealthCheckInterval = "application.globalManage.interval"
)

// GlobalsHealthCheckEnabled 判断是否启用全局对象健康检查。
//
// 新键 application.globalManage.healthCheck 优先；未启用时回退读取已废弃的
// application.globalManage.keepAlive，并记录一条 deprecation 警告。
// 两个键的默认值均为 false，因此"未配置"与"显式关闭"在此语义等价。
func GlobalsHealthCheckEnabled(cfg appconfig.IAppConfig, log bootstrap.LoggerWrapper) bool {
	if cfg == nil {
		return false
	}
	if cfg.Bool(configKeyHealthCheck) {
		return true
	}
	if cfg.Bool(configKeyHealthCheckLegacy) {
		if log != nil {
			log.WarnWith(cfg.LogOriginFrame()).
				Msgf("config key '%s' is deprecated, please rename it to '%s'", configKeyHealthCheckLegacy, configKeyHealthCheck)
		}
		return true
	}
	return false
}

type healthCheckStopper interface {
	stopHealthCheck()
}

func stopFrameHealthCheck(ctx IApplicationContext) {
	if ctx == nil {
		return
	}
	starter := ctx.GetStarterApp()
	if starter == nil {
		return
	}
	frame := starter.GetFrameApp()
	if stopper, ok := frame.(healthCheckStopper); ok {
		stopper.stopHealthCheck()
	}
}

func clearApplicationGlobals(ctx IApplicationContext) {
	stopFrameHealthCheck(ctx)
	ctx.GetContainer().ClearAll(true)
}

// NewFrameApplication 创建一个应用启动器对象
func NewFrameApplication(ctx IApplicationContext, opts ...FrameStarterOption) FrameStarter {
	fApp := &FrameApplication{
		Ctx: ctx,
	}
	if len(opts) == 0 {
		msg := "no registrar option available for injection into the application starter via NewFrameApplication"
		ctx.GetLogger().ErrorWith(ctx.GetConfig().LogOriginFrame()).Msg(msg)
		panic(msg)
	}

	for _, opt := range opts {
		opt(fApp)
	}

	return fApp
}

// GetContext 获取应用上下文
func (fa *FrameApplication) GetContext() IApplicationContext {
	return fa.Ctx
}

// GetFrameApp 获取框架启动器实例
func (fa *FrameApplication) GetFrameApp() FrameStarter {
	return fa
}

// RegisterApplication 注入应用注册器实例到应用启动器的application属性
func (fa *FrameApplication) RegisterApplication(application ApplicationRegister) {
	fa.application = application
}

// RegisterModule 注入应用模块/子系统注册器实例到应用启动器的module属性
func (fa *FrameApplication) RegisterModule(module ModuleRegister) {
	fa.module = module
}

// RegisterTask 注入异步任务注册器实例到应用启动器的task属性
func (fa *FrameApplication) RegisterTask(task TaskRegister) {
	fa.task = task
}

// GetApplication 获取实现IApplication接口的应用对象（ApplicationRegister）
func (fa *FrameApplication) GetApplication() IApplication {
	return fa.application
}

// GetModule 获取ModuleRegister对象
func (fa *FrameApplication) GetModule() ModuleRegister {
	return fa.module
}

// GetTask 获取TaskRegister对象
func (fa *FrameApplication) GetTask() TaskRegister {
	return fa.task
}

// RegisterToCtx 注册应用启动器对象到应用上下文
func (fa *FrameApplication) RegisterToCtx(as ApplicationStarter) {
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
func (fa *FrameApplication) RegisterApplicationGlobals(managers ...IProviderManager) {
	if fa.GetContext().GetAppState() {
		return
	}

	if len(managers) > 0 {
		for _, manager := range managers {
			if manager != nil && manager.Location().GetLocationID() == ProviderLocationDefault().LocationGlobalInit.GetLocationID() {
				_, err := manager.LoadProvider(func(manager IProviderManager) (any, error) {
					return fa, nil
				})
				if err != nil {
					fa.GetContext().GetLogger().ErrorWith(fa.GetContext().GetConfig().LogOriginFrame()).Err(err).Msgf("RegisterApplicationGlobals error, manager: %s", manager.Location().GetLocationName())
				}
			}
		}
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

	appRegister := fa.GetApplication().(ApplicationRegister)
	fa.GetContext().GetContainer().Registers(appRegister.ConfigGlobalInitializers())
}

// InitializeGlobalRequired 初始化应用启动时必要的全局对象
func (fa *FrameApplication) InitializeGlobalRequired() {
	if fa.GetContext().GetAppState() {
		return
	}
	if fa.GetApplication() != nil {
		appRegister := fa.GetApplication().(ApplicationRegister)
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
		appRegister := fa.GetApplication().(ApplicationRegister)
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
		appRegister := fa.GetApplication().(ApplicationRegister)
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
func (fa *FrameApplication) RegisterTaskServer(managers ...IProviderManager) {
	if fa.GetContext().GetAppState() {
		return
	}

	if len(managers) > 0 {
		for _, manager := range managers {
			if manager != nil && manager.Location().GetLocationID() == ProviderLocationDefault().LocationTaskServerInit.GetLocationID() {
				_, err := manager.LoadProvider(func(manager IProviderManager) (any, error) {
					return fa, nil
				})
				if err != nil {
					fa.GetContext().GetLogger().ErrorWith(fa.GetContext().GetConfig().LogOriginFrame()).Err(err).Msgf("RegisterTaskServer error, manager: %s", manager.Location().GetLocationName())
				}
			}
		}
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

// RegisterGlobalsKeepalive 注册全局对象的后台健康检测（仅观测上报，不做重建）。
//
// 默认关闭（application.globalManage.keepAlive）。连接池型对象的失效恢复依赖驱动层
// 与 connMaxLifetime/connMaxIdleTime 配置，通常无需开启；开启后仅用于日志观测。
func (fa *FrameApplication) RegisterGlobalsKeepalive(managers ...IProviderManager) {
	if fa.GetContext().GetAppState() {
		return
	}

	if len(managers) > 0 {
		for _, manager := range managers {
			if manager != nil && manager.Location().GetLocationID() == ProviderLocationDefault().LocationGlobalKeepaliveInit.GetLocationID() {
				_, err := manager.LoadProvider(func(manager IProviderManager) (any, error) {
					return fa, nil
				})
				if err != nil {
					fa.GetContext().GetLogger().ErrorWith(fa.GetContext().GetConfig().LogOriginFrame()).Err(err).Msgf("RegisterGlobalsKeepalive error, manager: %s", manager.Location().GetLocationName())
				}
			}
		}
		return
	}

	// 全局对象健康检查（仅观测上报，不做重建）
	if GlobalsHealthCheckEnabled(fa.GetContext().GetConfig(), fa.GetContext().GetLogger()) {
		d := fa.GetContext().GetConfig().Duration(configKeyHealthCheckInterval, constant.DefaultHealthCheckInterval) * time.Second
		fa.startHealthCheck(d)
	}
}

// startHealthCheck 异步检查全局对象是否健康和重建
func (fa *FrameApplication) startHealthCheck(interval time.Duration) {
	if interval <= 0 {
		fa.GetContext().GetLogger().ErrorWith(fa.GetContext().GetConfig().LogOriginFrame()).
			Msg("health check interval must be positive")
		return
	}

	fa.healthMu.Lock()
	if fa.healthCancel != nil {
		fa.healthMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	fa.healthCancel = cancel
	fa.healthWG.Add(1)
	fa.healthMu.Unlock()

	go func() {
		defer fa.healthWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer fa.recoverHealthCheckPanic()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fa.checkGlobalsHealthOnce()
			}
		}
	}()
}

func (fa *FrameApplication) stopHealthCheck() {
	fa.healthMu.Lock()
	cancel := fa.healthCancel
	fa.healthMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	fa.healthWG.Wait()
}

func (fa *FrameApplication) recoverHealthCheckPanic() {
	if r := recover(); r != nil {
		log, cfg := fa.GetContext().GetLogger(), fa.GetContext().GetConfig()
		switch re := r.(type) {
		case error:
			log.Error(cfg.LogOriginFrame()).Err(re).Str("from", "global manager").Msg("StartHealthCheck recover Error")
		default:
			log.Error(cfg.LogOriginFrame()).Str("from", "global manager").Msgf("StartHealthCheck recover Error: %v", re)
		}
	}
}

// checkGlobalsHealthOnce 遍历全局对象执行一轮健康检查，仅观测与上报，不做任何重建。
//
// 此处不会调用 Rebuild：连接池型对象（MySQL/Redis/MongoDB）的驱动层已具备失效连接
// 自愈能力，配合 connMaxLifetime/connMaxIdleTime 即可覆盖中间设备静默掐断的场景。
// 健康状态应交由外部编排系统（K8s readiness探针、负载均衡器）决策摘流量；
// 重建仅保留给配置热更新等场景手动调用。
func (fa *FrameApplication) checkGlobalsHealthOnce() {
	gm, log, cfg := fa.GetContext().GetContainer(), fa.GetContext().GetLogger(), fa.GetContext().GetConfig()
	gm.Range(func(key, value interface{}) bool {
		name := key.(string)
		ret, err := gm.CheckHealth(name)
		if err != nil {
			log.Error(cfg.LogOriginFrame()).Err(err).Msgf("global object from key: '%s', health check failure", name) // return false to stop iteration
			return true
		}
		if !ret {
			log.Error(cfg.LogOriginFrame()).Msgf("global resource '%s' is unhealthy, waiting for driver-level recovery", name)
		}
		return true
	})
}
