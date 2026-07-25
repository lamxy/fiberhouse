// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package fiberhouse provides a web application framework built on top of Fiber,
// combining frame and core starters to simplify application bootstrapping.
package fiberhouse

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
)

// WebApplication Web应用启动器，框架和核心启动器组合体，实现了 fiberhouse.FrameStarter 和 fiberhouse.CoreStarter 接口
type WebApplication struct {
	FrameStarter
	CoreStarter
}

// RunApplicationStarter 接受实现了ApplicationStarter接口的实例，执行应用启动流程
func RunApplicationStarter(starter ApplicationStarter, managers ...IProviderManager) error {
	// 应用启动流程，保持执行顺序
	starter.RegisterToCtx(starter)
	starter.RegisterApplicationGlobals(managers...)                      // 内部筛选出符合当前执行位点的管理器，按需执行加载
	starter.InitCoreApp(starter.GetFrameApp(), managers...)              // 同上
	starter.RegisterAppHooks(starter.GetFrameApp(), managers...)         // 同上
	starter.RegisterAppMiddleware(starter.GetFrameApp(), managers...)    // 同上
	starter.RegisterModuleInitialize(starter.GetFrameApp(), managers...) // 同上
	starter.RegisterModuleSwagger(starter.GetFrameApp(), managers...)    // 同上
	starter.RegisterTaskServer(managers...)                              // 同上
	starter.RegisterGlobalsKeepalive(managers...)                        // 同上
	return starter.AppCoreRun(managers...)                               // 同上
}

// BootConfig 启动配置
type BootConfig struct {
	// AppId 应用唯一标识符
	AppId string
	// AppName 应用名称
	AppName string
	// Version 应用版本
	Version string
	// BuildDate 应用构建日期
	Date string
	// FrameType 框架启动器的类型标识，由提供者的Target属性区分，如FiberHouse默认提供的"DefaultFrameStarter"、其他更多FrameStarter实现的标识
	// 见constant.ProviderTypeDefaultFrameStarter
	FrameType string
	// CoreType 核心启动器的类型标识，由提供者的target属性区分，如FiberHouse提供的"fiber"、"gin"、其他选择
	CoreType string
	// TrafficCodec 传输编解码器类型标识，由提供者的name属性区分，如"std_json_codec"、"sonic_json_codec"、"go_json_codec"、其他选择如protobuf等
	TrafficCodec string
	// 是否启用二进制协议支持，如Protobuf、MsgPack等
	EnableBinaryProtocolSupport bool
	// ConfigPath 全局应用配置文件的路径
	ConfigPath string
	// LogPath 全局应用日志文件的路径
	LogPath string
	// kvStorage 键值存储映射，用于存储额外自定义的属性
	kvStorage map[string]any
	// sealed 是否已封闭，封闭后不可再添加键值
	sealed bool
	// mu 读写锁
	mu sync.RWMutex
}

// WithCustom 初始化时设置键值对，仅在未封闭前有效，支持链式调用
func (bc *BootConfig) WithCustom(key string, value any) *BootConfig {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.sealed {
		return bc
	}
	if bc.kvStorage == nil {
		bc.kvStorage = make(map[string]any)
	}
	bc.kvStorage[key] = value
	return bc
}

// Finally 封闭配置，封闭后不可再添加键值
func (bc *BootConfig) Finally() *BootConfig {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.sealed = true
	return bc
}

// GetValue 获取键值存储中的值
func (bc *BootConfig) GetValue(key string) (any, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if bc.kvStorage == nil {
		return nil, errors.New("BootConfig kvStorage is nil")
	}
	if v, ok := bc.kvStorage[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("BootConfig kvStorage not found key: %s", key)
}

// GetMustValue 获取键值存储中的值，键不存在时panic
func (bc *BootConfig) GetMustValue(key string) any {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if bc.kvStorage == nil {
		panic("BootConfig kvStorage is nil")
	}
	if v, ok := bc.kvStorage[key]; ok {
		return v
	}
	panic(fmt.Sprintf("BootConfig kvStorage not found key: %s", key))
}

// FiberHouse FiberHouse应用运行器，用于配置和运行基于底层可切换框架的Web应用
type FiberHouse struct {
	AppCtx           IApplicationContext
	container        *globalmanager.GlobalManager
	bootCfg          *BootConfig
	frameStarterOpts []FrameStarterOption
	coreStarterOpts  []CoreStarterOption
	providers        []IProvider
	managers         []IProviderManager
}

// New 创建FiberHouse实例
func New(cfg *BootConfig) *FiberHouse {
	fh := &FiberHouse{
		container:        globalmanager.NewGlobalManagerOnce(),
		frameStarterOpts: make([]FrameStarterOption, 0, 3),
		coreStarterOpts:  make([]CoreStarterOption, 0),
		providers:        make([]IProvider, 0),
		managers:         make([]IProviderManager, 0),
	}
	fh.bootCfg = cfg

	// bootstrap 初始化启动配置(全局配置、全局日志器)，配置目录默认为当前工作目录"."下的`example_config/`
	appCfg := bootstrap.NewConfigOnce(fh.bootCfg.ConfigPath)
	// 日志目录默认为当前工作目录"."下的`example_main/logs`
	logger := bootstrap.NewLoggerOnce(appCfg, fh.bootCfg.LogPath)

	// 初始化全局应用上下文
	appContext := NewAppContextOnce(appCfg, logger)

	if cfg.AppId != "" {
		appCfg.SetAppId(cfg.AppId)
	}

	if cfg.AppName != "" {
		appCfg.SetAppName(cfg.AppName)
	}
	if cfg.Version != "" {
		appCfg.SetVersion(cfg.Version)
	}

	// 注册全局应用上下文到全局管容器
	fh.container.Register(constant.GlobalAppIContext, func() (interface{}, error) {
		return appContext, nil
	})

	// 注册启动配置到全局应用上下文
	appContext.RegisterBootConfig(cfg)
	fh.AppCtx = appContext

	return fh
}

// Default 创建默认的FiberHouse实例，支持通过函数选项修改默认配置
func Default(opts ...BootConfigOption) *FiberHouse {
	// 默认启动配置
	cfg := &BootConfig{
		AppId:        "",
		AppName:      "FiberHouse Application",
		Version:      "0.0.1",
		Date:         "",
		FrameType:    constant.FrameTypeWithDefaultFrameStarter,
		CoreType:     constant.CoreTypeWithFiber,
		TrafficCodec: constant.TrafficCodecWithSonic,
		ConfigPath:   "./config",
		LogPath:      "./logs",
	}

	// 应用函数选项
	for _, opt := range opts {
		opt(cfg)
	}

	return New(cfg)
}

// WithAppId 设置应用ID
func WithAppId(appId string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.AppId = appId
	}
}

// WithAppName 设置应用名称
func WithAppName(appName string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.AppName = appName
	}
}

// WithVersion 设置应用版本
func WithVersion(version string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.Version = version
	}
}

// WithDate 设置应用构建日期
func WithDate(date string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.Date = date
	}
}

// WithFrameType 设置框架启动器类型
func WithFrameType(frameType string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.FrameType = frameType
	}
}

// WithCoreType 设置核心启动器类型
func WithCoreType(coreType string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.CoreType = coreType
	}
}

// WithTrafficCodec 设置JSON编解码器类型
func WithTrafficCodec(codec string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.TrafficCodec = codec
	}
}

// WithConfigPath 设置配置文件路径
func WithConfigPath(configPath string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.ConfigPath = configPath
	}
}

// WithLogPath 设置日志文件路径
func WithLogPath(logPath string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.LogPath = logPath
	}
}

// WithCustomKV 设置自定义键值对
func WithCustomKV(key string, value any) BootConfigOption {
	return func(boot *BootConfig) {
		boot.WithCustom(key, value)
	}
}

// WithFrameStarterOptions 添加框架启动器选项: 用于fiberhouse.NewFrameApplication(appContext, opts...)创建框架启动器时传入的选项
func (fh *FiberHouse) WithFrameStarterOptions(opts ...FrameStarterOption) *FiberHouse {
	fh.frameStarterOpts = append(fh.frameStarterOpts, opts...)
	return fh
}

// WithCoreStarterOptions 添加核心启动器选项: 用于fiberhouse.NewCoreWithFiber(appContext, opts...)创建核心启动器时传入的选项
func (fh *FiberHouse) WithCoreStarterOptions(opts ...CoreStarterOption) *FiberHouse {
	fh.coreStarterOpts = append(fh.coreStarterOpts, opts...)
	return fh
}

// WithProviders 添加服务提供者，启动时初始化的全局服务提供者: 框架默认的提供者、用户自定义的提供者
func (fh *FiberHouse) WithProviders(providers ...IProvider) *FiberHouse {
	fh.providers = append(fh.providers, providers...)
	return fh
}

// WithPManagers 添加服务提供者管理器，启动时初始化的全局服务提供者管理器: 框架默认的提供者管理器、用户自定义的提供者管理器
func (fh *FiberHouse) WithPManagers(managers ...IProviderManager) *FiberHouse {
	fh.managers = append(fh.managers, managers...)
	return fh
}

// RunServer 运行应用服务器
// 提供者状态/日志: pending、loaded、skipped、failed ???
func (fh *FiberHouse) RunServer(manager ...IProviderManager) {
	// 引导配置完成位置点，获取该位点的提供者管理器列表并加载提供者
	ms := ProviderLocationDefault().LocationBootStrapConfig.GetManagers()
	if len(ms) > 0 {
		for _, m := range ms {
			if m.IsUnique() { // 只允许唯一绑定单一提供者的管理器
				_, _ = m.LoadProvider(func(manager IProviderManager) (any, error) {
					return fh, nil // 向当前管理器加载提供者函数中注入当前执行位点的FiberHouse实例
				})
			}
			break
		}
	}

	// 收集提供者并注册到同类型组的管理器中，管理器加載提供者并執行提供者的初始化逻辑
	fh.resolveManagerWithProviders(manager...)

	// 框架启动器选项初始化位置点，获取创建框架启动器所需的选项参数列表
	fso := fh.resolveFrameStarterOpts()

	// 核心启动器选项初始化位置点，获取创建核心启动器所需的选项参数列表
	cso := fh.resolveCoreStarterOpts()

	// 框架启动器创建位置点，加载并获取框架启动器对象
	frameStarter := fh.resolveAndReturnFrameStarter(fso)

	// 核心启动器创建位置点，加载并获取核心启动器对象
	coreStarter := fh.resolveAndReturnCoreStarter(cso)

	// 创建应用启动器
	appStarter := &WebApplication{
		FrameStarter: frameStarter,
		CoreStarter:  coreStarter,
	}

	// ======== 应用启动流程，保持执行顺序 =========

	// 将应用启动器注册到全局应用上下文
	appStarter.RegisterToCtx(appStarter)

	// 注册全局应用对象执行位置点，完成应用自定义全局对象注册和必要的对象初始化
	appStarter.RegisterApplicationGlobals(ProviderLocationDefault().LocationGlobalInit.GetManagers()...)

	// engine初始化位置点 & json编解码初始化位置点，返回合并管理器列表，完成核心引擎和编解码初始化
	initCoreManagers := fh.resolverAndReturnInitCoreManagers()

	// 初始化核心应用执行位置点，完成核心应用监听服务前的配置初始化
	appStarter.InitCoreApp(appStarter.GetFrameApp(), initCoreManagers...)

	// 应用钩子函数注册执行位置点，完成注册核心应用的声明周期钩子函数注册
	appStarter.RegisterAppHooks(appStarter.GetFrameApp(), ProviderLocationDefault().LocationCoreHookInit.GetManagers()...)

	// 应用中间件注册执行位置点，完成应用级的中间件注册
	appStarter.RegisterAppMiddleware(appStarter.GetFrameApp(), ProviderLocationDefault().LocationAppMiddlewareInit.GetManagers()...)

	// 模块初始化执行位置点，完成模块级中间件和应用路由的注册
	appStarter.RegisterModuleInitialize(appStarter.GetFrameApp(), fh.resolveAndReturnModuleInitManagers()...)

	// Swagger模块初始化执行位置点，完成注册 swagger 组件
	appStarter.RegisterModuleSwagger(appStarter.GetFrameApp(), ProviderLocationDefault().LocationModuleSwaggerInit.GetManagers()...)

	// 异步任务服务器注册执行位置点，完成异步任务注册
	appStarter.RegisterTaskServer(ProviderLocationDefault().LocationTaskServerInit.GetManagers()...)

	// 全局对象保活注册执行位置点，完成全局对象探测和保活机制
	appStarter.RegisterGlobalsKeepalive(ProviderLocationDefault().LocationGlobalKeepaliveInit.GetManagers()...)

	// 运行前执行位置点，完成核心应用服务器监听前的必要逻辑（如有）
	runBeforeManagers := ProviderLocationDefault().LocationServerRunBefore.GetManagers()
	if len(runBeforeManagers) > 0 {
		for _, m := range runBeforeManagers {
			if m.IsUnique() { // 只允许唯一绑定单一提供者的管理器
				_, _ = m.LoadProvider(func(manager IProviderManager) (any, error) {
					return appStarter, nil // 向当前管理器加载提供者函数中注入当前执行位点的应用启动器实例
				})
				break
			}
		}
	}

	// 监听系统信号，处理应用优雅关闭逻辑
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// 应用核心运行时执行位置点和关机执行位置点，完成核心应用监听套接字时的附加逻辑和注册关机后的清理逻辑
	runManagers, allShutdownManagers := fh.resolveRunAndShutdownManagers()

	// 启动应用服务监听套接字
	// select 监听中断信号和错误信号
	// 等待程序优雅退出
	runErr, shutdownErr, shutdownRequested := coordinateServerRun(appStarter, runManagers, allShutdownManagers, stopCh)

	// 应用正常退出，关闭系统信号通道
	signal.Stop(stopCh)

	if runErr != nil {
		fmt.Printf("Application run server error: %v\n", runErr)
	}

	if shutdownRequested {
		fmt.Printf("Application shutdown gracefully: %v\n", shutdownErr)
	}

	fmt.Println("Application RunServer exited")
}

// resolveGlobalTools 获取全局应用上下文、全局配置器和全局日志器
func (fh *FiberHouse) resolveGlobalTools() (IApplicationContext, appconfig.IAppConfig, bootstrap.LoggerWrapper) {
	// 全局应用上下文
	appContext := fh.AppCtx
	// 全局配置器
	cfg := appContext.GetConfig()
	// 全局日志器
	logger := appContext.GetLogger()

	return appContext, cfg, logger
}

// resolveAndReturnModuleInitManagers 获取模块级中间件和路由注册的提供者管理器列表
func (fh *FiberHouse) resolveAndReturnModuleInitManagers() []IProviderManager {
	moduleMS := ProviderLocationDefault().LocationModuleMiddlewareInit.GetManagers()
	routeMS := ProviderLocationDefault().LocationRouteRegisterInit.GetManagers()
	ms := make([]IProviderManager, 0, len(moduleMS)+len(routeMS))
	ms = append(ms, moduleMS...)
	ms = append(ms, routeMS...)
	return ms
}

// resolverAndReturnInitCoreManagers 获取初始化核心的提供者管理器列表
func (fh *FiberHouse) resolverAndReturnInitCoreManagers() []IProviderManager {
	engineInitManagers := ProviderLocationDefault().LocationCoreEngineInit.GetManagers()
	engineCodecManagers := ProviderLocationDefault().LocationCoreCodecInit.GetManagers()
	initCoreManagers := make([]IProviderManager, 0, len(engineInitManagers)+len(engineCodecManagers))
	initCoreManagers = append(initCoreManagers, engineInitManagers...)
	initCoreManagers = append(initCoreManagers, engineCodecManagers...)
	return initCoreManagers
}

// resolveRunAndShutdownManagers 获取运行时和关闭时的提供者管理器列表
func (fh *FiberHouse) resolveRunAndShutdownManagers() ([]IProviderManager, []IProviderManager) {
	runManagers := ProviderLocationDefault().LocationServerRun.GetManagers()
	shutdownBeforeManagers := ProviderLocationDefault().LocationServerShutdownBefore.GetManagers()
	shutdownManagers := ProviderLocationDefault().LocationServerShutdown.GetManagers()
	shutdownAfterManagers := ProviderLocationDefault().LocationServerShutdownAfter.GetManagers()
	allShutdownManagers := make(
		[]IProviderManager,
		0,
		len(shutdownBeforeManagers)+len(shutdownManagers)+len(shutdownAfterManagers),
	)
	allShutdownManagers = append(allShutdownManagers, shutdownBeforeManagers...)
	allShutdownManagers = append(allShutdownManagers, shutdownManagers...)
	allShutdownManagers = append(allShutdownManagers, shutdownAfterManagers...)
	return runManagers, allShutdownManagers
}

// resolveFrameStarterOpts 创建并返回框架启动器选项参数
func (fh *FiberHouse) resolveFrameStarterOpts() []FrameStarterOption {
	_, cfg, logger := fh.resolveGlobalTools()
	if len(fh.frameStarterOpts) == 0 {
		logger.WarnWith(cfg.LogOriginFrame()).Msg("FiberHouse: frameStarterOpts not set, loading from FrameStarterOptionInit location point")
		// 配置项未设置，从框架启动器选项位置点加载
		ms := ProviderLocationDefault().LocationFrameStarterOptionInit.GetManagers()
		if len(ms) > 0 {
			anyFrameOpts, err := ms[0].LoadProvider()
			if err != nil {
				logger.ErrorWith(cfg.LogOriginFrame()).Err(err).Msg("FrameStarterOptionInit provider load failed")
				panic(err)
			}
			opts, ok := anyFrameOpts.([]FrameStarterOption)
			if !ok {
				msg := "loaded FrameStarterOptionInit provider is not []FrameStarterOption type"
				logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
				panic(errors.New(msg))
			}
			fh.frameStarterOpts = opts
		}
	}
	return fh.frameStarterOpts
}

// resolveCoreStarterOpts 创建并返回核心启动器选项参数
func (fh *FiberHouse) resolveCoreStarterOpts() []CoreStarterOption {
	_, cfg, logger := fh.resolveGlobalTools()
	if len(fh.coreStarterOpts) == 0 {
		logger.WarnWith(cfg.LogOriginFrame()).Msg("FiberHouse: coreStarterOpts not set, loading from CoreStarterOptionInit location point")
		// 配置项未设置，从核心启动器选项位置点加载
		ms := ProviderLocationDefault().LocationCoreStarterOptionInit.GetManagers()
		if len(ms) > 0 {
			anyCoreOpts, err := ms[0].LoadProvider()
			if err != nil {
				msg := "CoreStarterOptionInit provider load failed"
				logger.ErrorWith(cfg.LogOriginFrame()).Err(err).Msg(msg)
				panic(errors.New(msg))
			}
			opts, ok := anyCoreOpts.([]CoreStarterOption)
			if !ok {
				msg := "loaded CoreStarterOptionInit provider is not []CoreStarterOption type"
				logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
				panic(errors.New(msg))
			}
			fh.coreStarterOpts = opts
		}
	}
	return fh.coreStarterOpts
}

// resolveAndReturnFrameStarter 创建并返回框架启动器
func (fh *FiberHouse) resolveAndReturnFrameStarter(fs []FrameStarterOption) FrameStarter {
	_, cfg, logger := fh.resolveGlobalTools()

	// 通过框架启动器创建位置点加载获取框架启动器对象
	ms := ProviderLocationDefault().LocationFrameStarterCreate.GetManagers()
	if len(ms) == 0 {
		msg := "Location point:LocationFrameStarterCreate， no FrameStarterCreate provider manager found"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		panic(errors.New(msg))
	}
	// 通过提供者加载回调函数(ProviderLoadFunc)参数注入框架启动器选项
	anyStarter, err := ms[0].LoadProvider(func(manager IProviderManager) (any, error) {
		return fs, nil
	})
	if err != nil {
		msg := "FrameStarterCreate provider load failed"
		logger.ErrorWith(cfg.LogOriginFrame()).Err(err).Msg(msg)
		panic(errors.New(msg))
	}
	// 初始化框架启动器
	frameStarter, ok := anyStarter.(FrameStarter)
	if !ok {
		msg := "loaded FrameStarterCreate provider is not FrameStarter type"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		panic(errors.New(msg))
	}

	return frameStarter
}

// resolveAndReturnCoreStarter 创建并返回核心启动器
func (fh *FiberHouse) resolveAndReturnCoreStarter(cs []CoreStarterOption) CoreStarter {
	_, cfg, logger := fh.resolveGlobalTools()

	// 通过核心启动器创建位置点加载获取核心启动器对象
	ms := ProviderLocationDefault().LocationCoreStarterCreate.GetManagers()
	if len(ms) == 0 {
		msg := "Location point: LocationCoreStarterCreate, no CoreStarterCreate provider manager found"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		panic(errors.New(msg))
	}
	// 通过提供者加载回调函数(ProviderLoadFunc)参数注入核心启动器选项
	anyCoreStarter, err := ms[0].LoadProvider(func(manager IProviderManager) (any, error) {
		return cs, nil
	})
	if err != nil {
		logger.ErrorWith(cfg.LogOriginFrame()).Err(err).Msg("CoreStarterCreate provider load failed")
		panic(err)
	}
	// 初始化核心启动器
	coreStarter, ok := anyCoreStarter.(CoreStarter)
	if !ok {
		msg := "loaded CoreStarterCreate provider is not CoreStarter type"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		panic(errors.New(msg))
	}
	return coreStarter
}

// resolveManagerWithProviders 收集提供者并注册到同类型组的管理器中，加載 providers 執行 initialize 初始化
// 排除已绑定到特定位置点的管理器，这些管理器将在后续对应位置点被单独加载，不在此处解决
func (fh *FiberHouse) resolveManagerWithProviders(manager ...IProviderManager) {
	appContext, cfg, logger := fh.resolveGlobalTools()

	var defaultManager IProviderManager
	if len(manager) == 0 {
		// 使用默认提供者管理器
		defaultManager = NewDefaultPManager(appContext)
		fh.managers = append(fh.managers, defaultManager)
	} else {
		defaultManager = manager[0]
		fh.managers = append(fh.managers, defaultManager)
	}
	var leftProviders = make([]IProvider, 0)
	for _, pdr := range fh.providers {
		matched := false
		for _, mgr := range fh.managers {
			if pdr.Type().GetTypeID() == mgr.Type().GetTypeID() {
				matched = true
				err := mgr.Register(pdr) // 注册子类提供者实例
				if err != nil {
					// 注册失败（如已注册同名提供者）记录日志即可，不影响匹配状态
					logger.Error(cfg.LogOriginFrame()).
						Err(err).
						Msgf("provider %s register failed", pdr.Type().GetTypeName())
				}
				break
			}
		}
		// 未找到匹配类型的管理器，收集到leftProviders中
		if !matched {
			leftProviders = append(leftProviders, pdr)
		}
	}

	// 将未匹配的提供者注册到默认管理器中
	for _, pdr := range leftProviders {
		//err := pdr.RegisterTo(defaultManager)
		err := defaultManager.Register(pdr)
		if err != nil {
			logger.Error(cfg.LogOriginFrame()).
				Err(err).
				Msgf("provider %s register to default manager failed", pdr.Type().GetTypeName())
		}
	}

	// 加载所有管理器中的提供者，排除已绑定到特定位置点的管理器，这些管理器将在对应位置点被单独加载
	if len(fh.managers) > 0 {
		for _, mgr := range fh.managers {
			// 排除已设置位点的管理器，未设置位点的管理器直接加载
			if mgr.Location().GetLocationID() == ProviderLocationDefault().ZeroLocation.GetLocationID() {
				_, err := mgr.LoadProvider()
				if err != nil {
					logger.Error(cfg.LogOriginFrame()).Err(err).Msg("manager load provider failed")
				}
			}
		}
	}

	// 默认管理器加载
	if len(defaultManager.List()) > 0 {
		_, err := defaultManager.LoadProvider()
		if err != nil {
			logger.Error(cfg.LogOriginFrame()).
				Err(err).
				Msgf("default manager load provider failed")
		}
	}
}
