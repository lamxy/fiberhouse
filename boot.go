package fiberhouse

import (
	"errors"
	"fmt"
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

// InitKV 初始化键值存储
func (bc *BootConfig) InitKVS(fn func(cfg *BootConfig)) *BootConfig {
	bc.kvOnce.Do(func() {
		bc.kvStorage = make(map[string]any)
		fn(bc)
	})
	return bc
}

// GetKVMap 获取键值存储映射
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
}

// New 创建FiberHouse实例
func New(cfg *BootConfig) *FiberHouse {
	fh := &FiberHouse{
		container: globalmanager.NewGlobalManagerOnce(),
		opts:      make([]FrameStarterOption, 0, 3),
		providers: make([]IProvider, 0),
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

func (fh *FiberHouse) RunServer(manager ...IProviderManager) {
	// TODO 检查MustWith的配置项是否正确、记录WithProviders的提供者和管理器
	// bootstrap基础初始化并实例化初始的AppContext, 并即时注册进全局管理器（双向互引用）
	// TODO 允许提供者相关的管理器初始化

	if len(manager) == 0 {
		// 使用默认提供者管理器
	}

	//  启动服务
}
