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
func RunApplicationStarter(starter ApplicationStarter, jsonCodecManagerOrMore ...IProviderManager) {
	// 应用启动流程，保持执行顺序
	starter.RegisterToCtx(starter)
	starter.RegisterApplicationGlobals()
	starter.InitCoreApp(starter.GetFrameApp(), jsonCodecManagerOrMore...)
	starter.RegisterAppHooks(starter.GetFrameApp())
	starter.RegisterAppMiddleware(starter.GetFrameApp())
	starter.RegisterModuleInitialize(starter.GetFrameApp())
	starter.RegisterModuleSwagger(starter.GetFrameApp())
	starter.RegisterTaskServer()
	starter.RegisterGlobalsKeepalive()
	starter.AppCoreRun()
}

// BootConfig 启动配置
type BootConfig struct {
	FrameType  string
	CoreType   string
	JsonCodec  string
	ConfigPath string
	LogPath    string
	kvStorage  map[string]any // once初始化一次
	kvOnce     sync.Once
}

// InitKVS 初始化键值存储
func (bc *BootConfig) InitKVS(fn func(cfg *BootConfig)) *BootConfig {
	bc.kvOnce.Do(func() {
		bc.kvStorage = make(map[string]any)
		fn(bc)
	})
	return bc
}

// GetKVStorage 获取键值存储映射
func (bc *BootConfig) GetKVStorage() map[string]any {
	return bc.kvStorage
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

// FiberHouse
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

	// 注册全局应用上下文到全局管容器
	fh.container.Register(constant.GlobalAppIContext, func() (interface{}, error) {
		return appContext, nil
	})

	// 注册启动配置到全局应用上下文
	appContext.RegisterBootConfig(cfg)
	fh.AppCtx = appContext

	return fh
}

// Default 创建默认的FiberHouse实例
func Default(opts ...BootConfigOption) *FiberHouse {
	return nil
}

// WithFrameOptions 添加框架启动器选项: 用于fiberhouse.NewFrameApplication(appContext, opts...)创建框架启动器时传入的选项
func (fh *FiberHouse) WithFrameStarterOptions(opts ...FrameStarterOption) *FiberHouse {
	fh.frameStarterOpts = append(fh.frameStarterOpts, opts...)
	return fh
}

// WithCoreOptions 添加核心启动器选项: 用于fiberhouse.NewCoreWithFiber(appContext, opts...)创建核心启动器时传入的选项
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

func (fh *FiberHouse) RunServer(manager ...IProviderManager) {
	// 引导配置完成位置点，获取该位点的提供者管理器列表并加载提供者
	ms := ProviderLocationDefault().LocationBootStrapConfig.GetManagers()
	if len(ms) > 0 {
		for _, m := range ms {
			_, _ = m.LoadProvider()
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
				//err := pdr.RegisterTo(mgr)  // TODO 提供者调用父类的注册方法，将提供者的父类实例注册到了管理器，管理器调用提供者初始化时调用的为父类提供者的初始化方法
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
			// 未设置位点的管理器直接加载
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
		ms = ProviderLocationDefault().LocationFrameStarterOptionInit.GetManagers()
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
		logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msgf("FrameStarterCreate provider load failed")
	}
	frameStarter, ok := anyStarter.(FrameStarter)
	if !ok {
		logger.FatalWith(cfg.LogOriginFrame()).Msgf("loaded FrameStarterCreate provider is not FrameStarter type")
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
				logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msgf("CoreStarterOptionInit provider load failed")
			}
			opts, ok := anyCoreOpts.([]CoreStarterOption)
			if !ok {
				logger.FatalWith(cfg.LogOriginFrame()).Msgf("loaded CoreStarterOptionInit provider is not []CoreStarterOption type")
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
		logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msgf("CoreStarterCreate provider load failed")
	}
	coreStarter, ok := anyCoreStarter.(CoreStarter)
	if !ok {
		logger.FatalWith(cfg.LogOriginFrame()).Msgf("loaded CoreStarterCreate provider is not CoreStarter type")
	}

	// 创建应用启动器
	appStarter := &WebApplication{
		FrameStarter: frameStarter,
		CoreStarter:  coreStarter,
	}

	// TODO 核心启动器接口改造增加传入响应的提供者管理器（如果有需要） 此处可以添加更多的位置点管理器

	// 应用启动
	RunApplicationStarter(appStarter, ProviderLocationDefault().LocationCoreEngineInit.GetManagers()...)
}
