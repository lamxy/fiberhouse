// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package fiberhouse 包提供了一个基于 Fiber/gin 构建的 Web 应用程序框架，
// 结合了框架层与核心层的启动器(可切換)，旨在简化应用程序的引导过程。
package fiberhouse

import (
	"errors"
	"fmt"
	"io"
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
	starter.RegisterGlobalsHealthCheck(managers...)                      // 同上
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
	// LogWriters 附加的日志输出目标（如错误追踪平台的 zerolog writer），追加在 console、file 之后；
	// 实现 io.Closer 的由全局日志器关闭时一并关闭，见 bootstrap.NewLoggerOnceWithWriters
	LogWriters []io.Writer
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

// ErrBootAssemblyFailed 标记启动装配失败。
//
// 位点未配置管理器、提供者加载失败、加载结果类型不符——三者都是启动期不可恢复
// 的装配错误。本哨兵使它们可被 errors.Is 分类，调用方据此与运行期错误区分。
var ErrBootAssemblyFailed = errors.New("fiberhouse: boot assembly failed")

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
	logger := bootstrap.NewLoggerOnceWithWriters(appCfg, fh.bootCfg.LogWriters, fh.bootCfg.LogPath)

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

// WithLogWriters 追加日志输出目标，可多次调用累加
func WithLogWriters(writers ...io.Writer) BootConfigOption {
	return func(boot *BootConfig) {
		boot.LogWriters = append(boot.LogWriters, writers...)
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
// 装配失败时 panic。需要处理该错误的调用方请改用 RunServerWithError。
//
// 保留 panic 是刻意的：装配失败意味着应用没有起来，静默返回会让调用方带着
// 半装配的应用继续运行。与 GetValue / GetMustValue 同一惯例。
func (fh *FiberHouse) RunServer(manager ...IProviderManager) {
	if err := fh.RunServerWithError(manager...); err != nil {
		panic(err)
	}
}

// RunServerWithError 与 RunServer 相同，但装配失败时返回错误而非 panic。
//
// 仅装配阶段的失败经由返回值传出：四个 resolve* 中任意一个失败都会中止启动并
// 返回错误，此时尚未监听套接字，也未注册信号处理。进入监听之后的行为与
// RunServer 完全一致——运行与关闭错误仍按原有方式打印，因为那时进程已经在
// 服务，调用方拿不到控制权。
func (fh *FiberHouse) RunServerWithError(manager ...IProviderManager) error {
	// 引导配置完成位置点，获取该位点的提供者管理器列表并加载提供者
	ms := ProviderLocationDefault().LocationBootStrapConfig.GetManagers()
	if len(ms) > 0 {
		for _, m := range ms {
			if m.IsUnique() { // 只允许唯一绑定单一提供者的管理器
				_, _ = m.LoadProvider(func(manager IProviderManager) (any, error) {
					return fh, nil // 向当前管理器加载提供者函数中注入当前执行位点的FiberHouse实例
				})
				break
			}
		}
	}

	// 收集提供者并注册到同类型组的管理器中，管理器加載提供者并執行提供者的初始化逻辑
	fh.resolveManagerWithProviders(manager...)

	// 框架启动器选项初始化位置点，获取创建框架启动器所需的选项参数列表
	fso, err := fh.resolveFrameStarterOptsWithError()
	if err != nil {
		return err
	}

	// 核心启动器选项初始化位置点，获取创建核心启动器所需的选项参数列表
	cso, err := fh.resolveCoreStarterOptsWithError()
	if err != nil {
		return err
	}

	// 框架启动器创建位置点，加载并获取框架启动器对象
	frameStarter, err := fh.resolveAndReturnFrameStarterWithError(fso)
	if err != nil {
		return err
	}

	// 核心启动器创建位置点，加载并获取核心启动器对象
	coreStarter, err := fh.resolveAndReturnCoreStarterWithError(cso)
	if err != nil {
		return err
	}

	// 创建应用启动器
	appStarter := &WebApplication{
		FrameStarter: frameStarter,
		CoreStarter:  coreStarter,
	}

	// ======== 应用启动流程，保持执行顺序 =========

	// 进入装配阶段。此后的注册器与位点都属于「装配中」，PhaseServing 要等到内核
	// 真正建立监听时才由 OnListen hook（Fiber）或 net.Listen 之后（Gin）置位。
	_ = fh.AppCtx.TransitionTo(PhaseInitializing)

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

	// 全局对象健康检查注册执行位置点，完成全局对象周期性健康观测（不做重建）
	appStarter.RegisterGlobalsHealthCheck(ProviderLocationDefault().LocationGlobalHealthCheckInit.GetManagers()...)

	// 运行前执行位置点，完成核心应用服务器监听前的必要逻辑（如有）
	// 该位点允许注册多个管理器并全部执行，不再限制为唯一绑定模式；
	// 复用统一的位点分派函数以获得与其他位点一致的 GroupExtendReplace 替代语义。
	if _, _, err := LoadProviderManagersAtLocation(
		ProviderLocationDefault().LocationServerRunBefore.GetManagers(),
		ProviderLocationDefault().LocationServerRunBefore,
		appStarter, // 向当前管理器加载提供者函数中注入当前执行位点的应用启动器实例
	); err != nil {
		_, runBeforeCfg, runBeforeLogger := fh.resolveGlobalTools()
		runBeforeLogger.ErrorWith(runBeforeCfg.LogOriginFrame()).Err(err).Msg("ServerRunBefore providers failed")
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
		// 运行分支返回代表核心从未进入信号协调的关闭流程：监听绑定失败，或运行
		// goroutine panic 被 coordinateServerRun 收敛成错误。这两条路径都不会调用
		// Shutdown，因此全局对象此前从未被回收——MySQL 连接池、Mongo 与 Redis
		// 客户端一直保持到进程退出。
		//
		// 这里补上回收。核心自身的回收入口带 once 守卫，重复调用是空操作，因此即使
		// 未来这条路径也走到 Shutdown，也不会重复释放。
		releaseGlobalsAfterRunFailure(appStarter)
	}

	if shutdownRequested {
		fmt.Printf("Application shutdown gracefully: %v\n", shutdownErr)
	}

	fmt.Println("Application RunServer exited")

	// 运行与关闭错误不经由返回值传出：走到这里意味着装配已经成功、服务已经跑过，
	// 进程正在退出。这两类错误已按原有方式打印，且 runErr 分支已完成资源回收。
	// 本方法的返回值只表达「装配是否成功」这一件事。
	return nil
}

// releaseGlobalsAfterRunFailure 在运行分支失败后回收全局对象。
//
// coordinateServerRun 的 runErr 分支直接返回、从不调用 Shutdown，覆盖两种情况：
// 监听绑定失败（AppCoreRun 返回错误），以及运行 goroutine panic 被收敛成错误。
// 这两条路径此前没有任何回收，注册的 MySQL 连接池、Mongo 与 Redis 客户端会一直
// 保持到进程退出。
//
// 这里直接调用通用回收函数，不尝试走核心自带的 once 守卫入口：守卫方法未导出，
// 而 WebApplication 以接口字段嵌入 CoreStarter，跨接口断言取不到具体类型的未导出
// 方法（已实测确认）。走通用路径本身是安全的——这条分支意味着 Shutdown 从未执行，
// 不存在重复回收；Fiber 的守卫也会让后续可能的回收成为空操作。
//
// 全程 nil 防护：启动流程可能在核心装配完成前就失败。
func releaseGlobalsAfterRunFailure(starter ApplicationStarter) {
	if starter == nil {
		return
	}
	ctx := starter.GetAppContext()
	if ctx == nil || ctx.GetContainer() == nil {
		return
	}
	clearApplicationGlobals(ctx)
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
// 装配失败时 panic。需要处理该错误的调用方请改用 resolveFrameStarterOptsWithError。
func (fh *FiberHouse) resolveFrameStarterOpts() []FrameStarterOption {
	opts, err := fh.resolveFrameStarterOptsWithError()
	if err != nil {
		panic(err)
	}
	return opts
}

// resolveFrameStarterOptsWithError 与 resolveFrameStarterOpts 相同，但装配失败时
// 返回错误而非 panic。
func (fh *FiberHouse) resolveFrameStarterOptsWithError() ([]FrameStarterOption, error) {
	_, cfg, logger := fh.resolveGlobalTools()
	if len(fh.frameStarterOpts) == 0 {
		logger.WarnWith(cfg.LogOriginFrame()).Msg("FiberHouse: frameStarterOpts not set, loading from FrameStarterOptionInit location point")
		// 配置项未设置，从框架启动器选项位置点加载
		ms := ProviderLocationDefault().LocationFrameStarterOptionInit.GetManagers()
		if len(ms) > 0 {
			anyFrameOpts, err := ms[0].LoadProvider()
			if err != nil {
				logger.ErrorWith(cfg.LogOriginFrame()).Err(err).Msg("FrameStarterOptionInit provider load failed")
				return nil, fmt.Errorf("%w: FrameStarterOptionInit provider load failed: %w",
					ErrBootAssemblyFailed, err)
			}
			opts, ok := anyFrameOpts.([]FrameStarterOption)
			if !ok {
				msg := "loaded FrameStarterOptionInit provider is not []FrameStarterOption type"
				logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
				return nil, fmt.Errorf("%w: %s (got %T)", ErrBootAssemblyFailed, msg, anyFrameOpts)
			}
			fh.frameStarterOpts = opts
		}
	}
	return fh.frameStarterOpts, nil
}

// resolveCoreStarterOpts 创建并返回核心启动器选项参数
// 装配失败时 panic。需要处理该错误的调用方请改用 resolveCoreStarterOptsWithError。
func (fh *FiberHouse) resolveCoreStarterOpts() []CoreStarterOption {
	opts, err := fh.resolveCoreStarterOptsWithError()
	if err != nil {
		panic(err)
	}
	return opts
}

// resolveCoreStarterOptsWithError 与 resolveCoreStarterOpts 相同，但装配失败时
// 返回错误而非 panic。
func (fh *FiberHouse) resolveCoreStarterOptsWithError() ([]CoreStarterOption, error) {
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
				return nil, fmt.Errorf("%w: %s: %w", ErrBootAssemblyFailed, msg, err)
			}
			opts, ok := anyCoreOpts.([]CoreStarterOption)
			if !ok {
				msg := "loaded CoreStarterOptionInit provider is not []CoreStarterOption type"
				logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
				return nil, fmt.Errorf("%w: %s (got %T)", ErrBootAssemblyFailed, msg, anyCoreOpts)
			}
			fh.coreStarterOpts = opts
		}
	}
	return fh.coreStarterOpts, nil
}

// resolveAndReturnFrameStarter 创建并返回框架启动器
// 装配失败时 panic。需要处理该错误的调用方请改用
// resolveAndReturnFrameStarterWithError。
func (fh *FiberHouse) resolveAndReturnFrameStarter(fs []FrameStarterOption) FrameStarter {
	starter, err := fh.resolveAndReturnFrameStarterWithError(fs)
	if err != nil {
		panic(err)
	}
	return starter
}

// resolveAndReturnFrameStarterWithError 与 resolveAndReturnFrameStarter 相同，
// 但装配失败时返回错误而非 panic。
//
// 「位点无管理器」是最常见的装配遗漏：应用忘记把 FrameStarterCreate 管理器
// 装配进默认集合时就会命中，旧实现在此直接 panic。
func (fh *FiberHouse) resolveAndReturnFrameStarterWithError(fs []FrameStarterOption) (FrameStarter, error) {
	_, cfg, logger := fh.resolveGlobalTools()

	// 通过框架启动器创建位置点加载获取框架启动器对象
	ms := ProviderLocationDefault().LocationFrameStarterCreate.GetManagers()
	if len(ms) == 0 {
		msg := "Location point: LocationFrameStarterCreate, no FrameStarterCreate provider manager found"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		return nil, fmt.Errorf("%w: %s", ErrBootAssemblyFailed, msg)
	}
	// 通过提供者加载回调函数(ProviderLoadFunc)参数注入框架启动器选项
	anyStarter, err := ms[0].LoadProvider(func(manager IProviderManager) (any, error) {
		return fs, nil
	})
	if err != nil {
		msg := "FrameStarterCreate provider load failed"
		logger.ErrorWith(cfg.LogOriginFrame()).Err(err).Msg(msg)
		return nil, fmt.Errorf("%w: %s: %w", ErrBootAssemblyFailed, msg, err)
	}
	// 初始化框架启动器
	frameStarter, ok := anyStarter.(FrameStarter)
	if !ok {
		msg := "loaded FrameStarterCreate provider is not FrameStarter type"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		return nil, fmt.Errorf("%w: %s (got %T)", ErrBootAssemblyFailed, msg, anyStarter)
	}

	return frameStarter, nil
}

// resolveAndReturnCoreStarter 创建并返回核心启动器
// 装配失败时 panic。需要处理该错误的调用方请改用
// resolveAndReturnCoreStarterWithError。
func (fh *FiberHouse) resolveAndReturnCoreStarter(cs []CoreStarterOption) CoreStarter {
	starter, err := fh.resolveAndReturnCoreStarterWithError(cs)
	if err != nil {
		panic(err)
	}
	return starter
}

// resolveAndReturnCoreStarterWithError 与 resolveAndReturnCoreStarter 相同，
// 但装配失败时返回错误而非 panic。
func (fh *FiberHouse) resolveAndReturnCoreStarterWithError(cs []CoreStarterOption) (CoreStarter, error) {
	_, cfg, logger := fh.resolveGlobalTools()

	// 通过核心启动器创建位置点加载获取核心启动器对象
	ms := ProviderLocationDefault().LocationCoreStarterCreate.GetManagers()
	if len(ms) == 0 {
		msg := "Location point: LocationCoreStarterCreate, no CoreStarterCreate provider manager found"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		return nil, fmt.Errorf("%w: %s", ErrBootAssemblyFailed, msg)
	}
	// 通过提供者加载回调函数(ProviderLoadFunc)参数注入核心启动器选项
	anyCoreStarter, err := ms[0].LoadProvider(func(manager IProviderManager) (any, error) {
		return cs, nil
	})
	if err != nil {
		logger.ErrorWith(cfg.LogOriginFrame()).Err(err).Msg("CoreStarterCreate provider load failed")
		return nil, fmt.Errorf("%w: CoreStarterCreate provider load failed: %w",
			ErrBootAssemblyFailed, err)
	}
	// 初始化核心启动器
	coreStarter, ok := anyCoreStarter.(CoreStarter)
	if !ok {
		msg := "loaded CoreStarterCreate provider is not CoreStarter type"
		logger.ErrorWith(cfg.LogOriginFrame()).Msg(msg)
		return nil, fmt.Errorf("%w: %s (got %T)", ErrBootAssemblyFailed, msg, anyCoreStarter)
	}
	return coreStarter, nil
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
