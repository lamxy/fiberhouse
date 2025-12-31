package fiberhouse

import (
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
	"sync"
)

// WebApplication Web应用启动器，框架和核心启动器组合体，实现了 fiberhouse.FrameStarter 和 fiberhouse.CoreStarter 接口
type WebApplication struct {
	FrameStarter
	CoreStarter
}

// RunApplicationStarter 接受实现了ApplicationStarter接口的实例，执行应用启动流程
func RunApplicationStarter(starter ApplicationStarter, managers ...IProviderManager) {
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
	starter.AppCoreRun(managers...)                                      // 同上
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
	// JsonCodec json编解码器类型标识，由提供者的name属性区分，如"json_codec"、"sonic_json_codec"、"go_json_codec"、其他选择
	JsonCodec string
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
		AppId:      "",
		AppName:    "FiberHouse Application",
		Version:    "1.0.0",
		Date:       "",
		FrameType:  constant.FrameTypeWithDefaultFrameStarter, // TODO 追加默认配置项的常量声明
		CoreType:   "fiber",
		JsonCodec:  "sonic_json_codec",
		ConfigPath: "./config",
		LogPath:    "./logs",
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

// WithJsonCodec 设置JSON编解码器类型
func WithJsonCodec(jsonCodec string) BootConfigOption {
	return func(boot *BootConfig) {
		boot.JsonCodec = jsonCodec
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
// TODO 记录已收集的提供者和已加载和未加载的提供者日志: pending、loaded、skipped、failed
func (fh *FiberHouse) RunServer(manager ...IProviderManager) {
	// 引导配置完成位置点，获取该位点的提供者管理器列表并加载提供者
	ms := ProviderLocationDefault().LocationBootStrapConfig.GetManagers()
	if len(ms) > 0 {
		for _, m := range ms {
			if m.IsUnique() { // 只允许唯一绑定单一提供者的管理器
				_, _ = m.LoadProvider(func(manager IProviderManager) (any, error) {
					return fh, nil
				})
			}
			break
		}
	}

	// 全局应用上下文
	appContext := fh.AppCtx
	cfg := appContext.GetConfig()
	logger := appContext.GetLogger()

	// 收集提供者并注册到同类型组的管理器中
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
					appContext.GetLogger().Error(appContext.GetConfig().LogOriginFrame()).
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
			appContext.GetLogger().Error(appContext.GetConfig().LogOriginFrame()).
				Err(err).
				Msgf("provider %s register to default manager failed", pdr.Type().GetTypeName())
		}
	}

	// 加载所有管理器中的提供者，排除已绑定到特定位置点的管理器，这些管理器将在对应位置点被单独加载
	if len(fh.managers) > 0 {
		for _, mgr := range fh.managers {
			// 排除已设置位点的管理器，未设置位点的管理器直接加载
			if mgr.Location().GetLocationID() == ProviderLocationDefault().ZeroLocation.GetLocationID() {
				_, _ = mgr.LoadProvider()
			}
		}
	}

	// 默认管理器加载
	if len(defaultManager.List()) > 0 {
		_, _ = defaultManager.LoadProvider()
	}

	// 获取创建框架启动器选项参数列表
	frameOptions := fh.frameStarterOpts
	if len(frameOptions) == 0 {
		logger.WarnWith(cfg.LogOriginFrame()).Msg("FiberHouse: frameStarterOpts not set, loading from FrameStarterOptionInit location point")
		// 配置项未设置，从框架启动器选项位置点加载
		ms := ProviderLocationDefault().LocationFrameStarterOptionInit.GetManagers()
		if len(ms) > 0 {
			anyFrameOpts, err := ms[0].LoadProvider()
			if err != nil {
				logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msg("FrameStarterOptionInit provider load failed")
			}
			opts, ok := anyFrameOpts.([]FrameStarterOption)
			if !ok {
				logger.FatalWith(cfg.LogOriginFrame()).Msg("loaded FrameStarterOptionInit provider is not []FrameStarterOption type")
			}
			frameOptions = opts
		}
	}
	// 创建框架启动器位置点加载获取框架启动器对象
	ms = ProviderLocationDefault().LocationFrameStarterCreate.GetManagers()
	if len(ms) == 0 {
		logger.FatalWith(cfg.LogOriginFrame()).Msg("Location point:LocationFrameStarterCreate， no FrameStarterCreate provider manager found")
	}
	// 通过提供者加载回调函数(ProviderLoadFunc)参数注入框架启动器选项
	anyStarter, err := ms[0].LoadProvider(func(manager IProviderManager) (any, error) {
		return frameOptions, nil
	})
	if err != nil {
		logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msg("FrameStarterCreate provider load failed")
	}
	frameStarter, ok := anyStarter.(FrameStarter)
	if !ok {
		logger.FatalWith(cfg.LogOriginFrame()).Msg("loaded FrameStarterCreate provider is not FrameStarter type")
	}

	// 获取创建核心启动器选项参数列表
	coreOptions := fh.coreStarterOpts
	if len(coreOptions) == 0 {
		logger.WarnWith(cfg.LogOriginFrame()).Msg("FiberHouse: coreStarterOpts not set, loading from CoreStarterOptionInit location point")
		// 配置项未设置，从核心启动器选项位置点加载
		ms = ProviderLocationDefault().LocationCoreStarterOptionInit.GetManagers()
		if len(ms) > 0 {
			anyCoreOpts, err := ms[0].LoadProvider()
			if err != nil {
				logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msg("CoreStarterOptionInit provider load failed")
			}
			opts, ok := anyCoreOpts.([]CoreStarterOption)
			if !ok {
				logger.FatalWith(cfg.LogOriginFrame()).Msg("loaded CoreStarterOptionInit provider is not []CoreStarterOption type")
			}
			coreOptions = opts
		}
	}
	// 创建核心启动器位置点
	ms = ProviderLocationDefault().LocationCoreStarterCreate.GetManagers()
	if len(ms) == 0 {
		logger.FatalWith(cfg.LogOriginFrame()).Msg("Location point: LocationCoreStarterCreate, no CoreStarterCreate provider manager found")
	}
	// 通过提供者加载回调函数(ProviderLoadFunc)参数注入核心启动器选项
	anyCoreStarter, err := ms[0].LoadProvider(func(manager IProviderManager) (any, error) {
		return coreOptions, nil
	})
	if err != nil {
		logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msg("CoreStarterCreate provider load failed")
	}
	coreStarter, ok := anyCoreStarter.(CoreStarter)
	if !ok {
		logger.FatalWith(cfg.LogOriginFrame()).Msg("loaded CoreStarterCreate provider is not CoreStarter type")
	}

	// 创建应用启动器
	appStarter := &WebApplication{
		FrameStarter: frameStarter,
		CoreStarter:  coreStarter,
	}

	// 应用启动流程，保持执行顺序
	appStarter.RegisterToCtx(appStarter)
	// 注册全局应用对象位置点
	appStarter.RegisterApplicationGlobals(ProviderLocationDefault().LocationGlobalInit.GetManagers()...)
	// 初始化应用核心位置点
	appStarter.InitCoreApp(appStarter.GetFrameApp(), ProviderLocationDefault().LocationCoreEngineInit.GetManagers()...)
	// 应用钩子函数注册位置点
	appStarter.RegisterAppHooks(appStarter.GetFrameApp(), ProviderLocationDefault().LocationCoreHookInit.GetManagers()...)
	// 应用中间件注册位置点
	appStarter.RegisterAppMiddleware(appStarter.GetFrameApp(), ProviderLocationDefault().LocationAppMiddlewareInit.GetManagers()...)

	// 模块初始化位置点，合并模块中间件初始化和路由注册位置点的管理器列表
	moduleMS := ProviderLocationDefault().LocationModuleMiddlewareInit.GetManagers()
	routeMS := ProviderLocationDefault().LocationRouteRegisterInit.GetManagers()
	ms = make([]IProviderManager, 0, len(moduleMS)+len(routeMS))
	ms = append(ms, moduleMS...)
	ms = append(ms, routeMS...)
	appStarter.RegisterModuleInitialize(appStarter.GetFrameApp(), ms...)

	// Swagger模块初始化位置点
	appStarter.RegisterModuleSwagger(appStarter.GetFrameApp(), ProviderLocationDefault().LocationModuleSwaggerInit.GetManagers()...)
	// 异步任务服务器注册位置点
	appStarter.RegisterTaskServer(ProviderLocationDefault().LocationTaskServerInit.GetManagers()...)
	// 全局对象保活注册位置点
	appStarter.RegisterGlobalsKeepalive(ProviderLocationDefault().LocationGlobalKeepaliveInit.GetManagers()...)

	// 运行前位置点
	beforeRuns := ProviderLocationDefault().LocationServerRunBefore.GetManagers()
	if len(beforeRuns) > 0 {
		for _, m := range beforeRuns {
			if m.IsUnique() { // 只允许唯一绑定单一提供者的管理器
				_, _ = m.LoadProvider(func(manager IProviderManager) (any, error) {
					return appStarter, nil // 向当前管理器加载提供者函数中注入当前执行位点的应用启动器实例
				})
				break
			}
		}
	}

	// 应用核心运行位置点
	runMS := ProviderLocationDefault().LocationServerRun.GetManagers()
	shutdownMS := ProviderLocationDefault().LocationServerShutdown.GetManagers()
	ms = make([]IProviderManager, 0, len(runMS)+len(shutdownMS))
	ms = append(ms, runMS...)
	ms = append(ms, shutdownMS...)
	appStarter.AppCoreRun(ms...)

	// 运行后位置点
	afterRun := ProviderLocationDefault().LocationServerRunAfter.GetManagers()
	if len(afterRun) > 0 {
		for _, m := range afterRun {
			if m.IsUnique() { // 只允许唯一绑定单一提供者的管理器
				_, _ = m.LoadProvider(func(manager IProviderManager) (any, error) {
					return appStarter, nil // 向当前管理器加载提供者函数中注入当前执行位点的应用启动器实例
				})
				break
			}
		}
	}
}
