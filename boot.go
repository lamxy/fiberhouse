package fiberhouse

import (
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
	"sync"
)

// RunApplicationStarter 接受实现了ApplicationStarter接口的实例，执行应用启动流程
func RunApplicationStarter(starter ApplicationStarter, manager ...IProviderManager) {
	// 应用启动流程，保持执行顺序
	starter.RegisterToCtx(starter)
	starter.RegisterApplicationGlobals()
	starter.InitCoreApp(starter.GetFrameApp(), manager...)
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
	container *globalmanager.GlobalManager
	bootCfg   *BootConfig
	opts      []FrameStarterOption
	providers []IProvider
	managers  []IProviderManager
}

// New 创建FiberHouse实例
func New(cfg *BootConfig) *FiberHouse {
	fh := &FiberHouse{
		container: globalmanager.NewGlobalManagerOnce(),
		opts:      make([]FrameStarterOption, 0, 3),
		providers: make([]IProvider, 0),
		managers:  make([]IProviderManager, 0),
	}
	fh.bootCfg = cfg
	return fh
}

// Default 创建默认的FiberHouse实例
func Default(opts ...BootConfigOption) *FiberHouse {
	return nil
}

func (fh *FiberHouse) WithOptions(opts ...FrameStarterOption) *FiberHouse {
	fh.opts = append(fh.opts, opts...)
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
	if len(fh.opts) == 0 {
		panic("BootConfig opts is nil")
	}

	// bootstrap 初始化启动配置(全局配置、全局日志器)，配置目录默认为当前工作目录"."下的`example_config/`
	cfg := bootstrap.NewConfigOnce(fh.bootCfg.ConfigPath)
	// 日志目录默认为当前工作目录"."下的`example_main/logs`
	logger := bootstrap.NewLoggerOnce(cfg, fh.bootCfg.LogPath)

	// 初始化全局应用上下文
	appContext := NewAppContextOnce(cfg, logger)

	// 注册全局应用上下文到全局管容器
	fh.container.Register(constant.GlobalAppIContext, func() (interface{}, error) {
		return appContext, nil
	})

	defaultManager := NewDefaultManager(appContext)
	if len(manager) == 0 {
		// 使用默认提供者管理器
		fh.managers = append(fh.managers, defaultManager)
	} else {
		fh.managers = append(fh.managers, manager[0])
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
						Msgf("provider %s register failed", provider.Type().GetTypeID())
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
				Msgf("provider %s register to default manager failed", provider.Type().GetTypeID())
		}
	}

	for _, m := range fh.managers {
		if m.Type().GetTypeID() != ProviderTypeDefault().GroupDefaultPManager.GetTypeID() {
			_, _ = m.LoadProvider()
		}
	}

	//  启动服务 TODO  New FrameStart & GetCoreStarter from defaultPManager.LoadProvider
	_, err := defaultManager.LoadProvider()
	appContext.GetLogger().Error(appContext.GetConfig().LogOriginFrame()).Err(err).Msg("run server failed")
}
