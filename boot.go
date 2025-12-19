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
	AppCtx        IApplicationContext
	container     *globalmanager.GlobalManager
	bootCfg       *BootConfig
	optsWithFrame []FrameStarterOption
	optsWithCore  []CoreStarterOption
	providers     []IProvider
	managers      []IProviderManager
}

// New 创建FiberHouse实例
func New(cfg *BootConfig) *FiberHouse {
	fh := &FiberHouse{
		container:     globalmanager.NewGlobalManagerOnce(),
		optsWithFrame: make([]FrameStarterOption, 0, 3),
		optsWithCore:  make([]CoreStarterOption, 0),
		providers:     make([]IProvider, 0),
		managers:      make([]IProviderManager, 0),
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

	fh.AppCtx = appContext

	return fh
}

// Default 创建默认的FiberHouse实例
func Default(opts ...BootConfigOption) *FiberHouse {
	return nil
}

func (fh *FiberHouse) WithFrameOptions(opts ...FrameStarterOption) *FiberHouse {
	fh.optsWithFrame = append(fh.optsWithFrame, opts...)
	return fh
}

func (fh *FiberHouse) WithCoreOptions(opts ...CoreStarterOption) *FiberHouse {
	fh.optsWithCore = append(fh.optsWithCore, opts...)
	return fh
}

// WithProviders 添加服务提供者，启动时初始化的全局服务提供者: 框架默认的提供者、用户自定义的提供者
func (fh *FiberHouse) WithProviders(providers ...IProvider) *FiberHouse {
	fh.providers = append(fh.providers, providers...)
	return fh
}

// WithManagers 添加服务提供者管理器，启动时初始化的全局服务提供者管理器: 框架默认的提供者管理器、用户自定义的提供者管理器
func (fh *FiberHouse) WithManagers(managers ...IProviderManager) *FiberHouse {
	fh.managers = append(fh.managers, managers...)
	return fh
}

func (fh *FiberHouse) RunServer(manager ...IProviderManager) {
	if len(fh.optsWithFrame) == 0 {
		//panic("FrameStarter options is empty")
	}

	// 引导配置完成位置点
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
	defaultManager := NewDefaultManager(appContext)
	if len(manager) == 0 {
		// 使用默认提供者管理器
		fh.managers = append(fh.managers, defaultManager)
	} else {
		defaultManager := manager[0]
		fh.managers = append(fh.managers, defaultManager)
	}
	var leftProviders = make([]IProvider, 0)
	for _, provider := range fh.providers {
		matched := false
		for _, mgr := range fh.managers {
			if provider.Type().GetTypeID() == mgr.Type().GetTypeID() {
				matched = true
				err := provider.RegisterTo(mgr)
				if err != nil {
					// 注册失败（如已注册同名提供者）记录日志即可，不影响匹配状态
					appContext.GetLogger().Error(appContext.GetConfig().LogOriginFrame()).
						Err(err).
						Msgf("provider %s register failed", provider.Type().GetTypeName())
				}
				break
			}
		}
		// 未找到匹配类型的管理器，收集到leftProviders中
		if !matched {
			leftProviders = append(leftProviders, provider)
		}
	}

	// 将未匹配的提供者注册到默认管理器
	for _, provider := range leftProviders {
		err := provider.RegisterTo(defaultManager)
		if err != nil {
			appContext.GetLogger().Error(appContext.GetConfig().LogOriginFrame()).
				Err(err).
				Msgf("provider %s register to default manager failed", provider.Type().GetTypeName())
		}
	}

	if len(fh.managers) > 0 {
		for _, manager := range fh.managers {
			if manager.Location().GetLocationID() == ProviderLocationDefault().ZeroLocation.GetLocationID() { // 未设置位点的管理器直接加载
				_, _ = manager.LoadProvider()
			}
		}
	}

	// 默认管理器
	if len(fh.providers) > 0 {
		_, _ = defaultManager.LoadProvider()
	}

	// 创建框架启动器位置点
	ms = ProviderLocationDefault().LocationFrameStarterCreate.GetManagers()
	if len(ms) == 0 {
		logger.FatalWith(cfg.LogOriginFrame()).Msgf("no FrameStarterCreate provider manager found")
	}
	anyStarter, err := ms[0].LoadProvider() // TODO 依赖框架启动器选项参数注入
	if err != nil {
		logger.FatalWith(cfg.LogOriginFrame()).Err(err).Msgf("FrameStarterCreate provider load failed")
	}
	frameStarter, ok := anyStarter.(FrameStarter)
	if !ok {
		logger.FatalWith(cfg.LogOriginFrame()).Msgf("loaded FrameStarterCreate provider is not FrameStarter type")
	}

	// 创建核心启动器位置点
	ms = ProviderLocationDefault().LocationCoreStarterCreate.GetManagers()
	if len(ms) == 0 {
		logger.FatalWith(cfg.LogOriginFrame()).Msgf("no CoreStarterCreate provider manager found")
	}
	anyCoreStarter, err := ms[0].LoadProvider() // TODO 依赖核心启动器选项参数注入
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
