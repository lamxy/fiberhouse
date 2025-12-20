package fiberhouse

// TODO 提供一种方式，收集默认提供者、提供者管理器列表，同时支持追加更多的自定义的提供者、提供者管理器的列表，组合起来方便使用.
//  参考的调用形式为：fiberhouse.DefaultProviders().AndMore([]IProvider{}...)、fiberhouse.DefaultPManagers().AndMore([]IProviderManager{}...)
//  DefaultProviders()返回一个对象，对象内的默认列表可以手动列出，该对象有一个方法AndMore([]IProvider{})，该方法将默认提供者和传入的自定义提供者合并返回一个新的提供者切片列表。
//  DefaultPManagers()同理。

import (
	"sync"
)

// DefaultProviderCollection 默认提供者集合
type DefaultProviderCollection struct {
	providers []IProvider
	mu        sync.RWMutex
}

var (
	defaultProvidersInstance *DefaultProviderCollection
	defaultProvidersOnce     sync.Once
)

// DefaultProviders 获取默认提供者集合（单例）
func DefaultProviders() *DefaultProviderCollection {
	defaultProvidersOnce.Do(func() {
		defaultProvidersInstance = &DefaultProviderCollection{
			providers: make([]IProvider, 0),
		}
		// 在此处添加默认的提供者
		defaultProvidersInstance.providers = append(defaultProvidersInstance.providers,
			NewFrameDefaultProvider(),
			NewCoreFiberProvider(),
			NewCoreGinProvider(),
			NewJsonJCodecFiberProvider(),
			NewJsonJCodecGinProvider(),
			NewSonicJCodecFiberProvider(),
			NewSonicJCodecGinProvider(),
			// TODO more...
		)
	})
	return defaultProvidersInstance
}

// List 获取默认提供者列表（返回副本）
func (c *DefaultProviderCollection) List() []IProvider {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.providers) == 0 {
		return nil
	}

	result := make([]IProvider, len(c.providers))
	copy(result, c.providers)
	return result
}

// Add 添加默认提供者到集合
func (c *DefaultProviderCollection) Add(providers ...IProvider) *DefaultProviderCollection {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.providers = append(c.providers, providers...)
	return c
}

// AndMore 将默认提供者和传入的自定义提供者合并返回一个新的提供者切片列表
func (c *DefaultProviderCollection) AndMore(customProviders ...IProvider) []IProvider {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 计算总容量
	totalLen := len(c.providers) + len(customProviders)
	if totalLen == 0 {
		return nil
	}

	// 创建新切片并合并
	result := make([]IProvider, 0, totalLen)
	result = append(result, c.providers...)
	result = append(result, customProviders...)
	return result
}

// DefaultPManagerCollection 默认提供者管理器集合
type DefaultPManagerCollection struct {
	managers []IProviderManager
	mu       sync.RWMutex
}

var (
	defaultPManagersInstance *DefaultPManagerCollection
	defaultPManagersOnce     sync.Once
)

// DefaultPManagers 获取默认提供者管理器集合（单例）
func DefaultPManagers(ctx IApplicationContext) *DefaultPManagerCollection {
	defaultPManagersOnce.Do(func() {
		defaultPManagersInstance = &DefaultPManagerCollection{
			managers: make([]IProviderManager, 0),
		}
		// 在此处添加默认的提供者管理器
		defaultPManagersInstance.managers = append(defaultPManagersInstance.managers,
			NewDefaultPManager(ctx),
			NewFrameDefaultPManager(ctx),
			NewCoreStarterPManager(ctx),
			NewJsonCodecPManager(ctx),
			// TODO more...
		)
	})
	return defaultPManagersInstance
}

// List 获取默认提供者管理器列表（返回副本）
func (c *DefaultPManagerCollection) List() []IProviderManager {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.managers) == 0 {
		return nil
	}

	result := make([]IProviderManager, len(c.managers))
	copy(result, c.managers)
	return result
}

// Add 添加默认提供者管理器到集合
func (c *DefaultPManagerCollection) Add(managers ...IProviderManager) *DefaultPManagerCollection {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.managers = append(c.managers, managers...)
	return c
}

// AndMore 将默认提供者管理器和传入的自定义提供者管理器合并返回一个新的管理器切片列表
func (c *DefaultPManagerCollection) AndMore(customManagers ...IProviderManager) []IProviderManager {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 计算总容量
	totalLen := len(c.managers) + len(customManagers)
	if totalLen == 0 {
		return nil
	}

	// 创建新切片并合并
	result := make([]IProviderManager, 0, totalLen)
	result = append(result, c.managers...)
	result = append(result, customManagers...)
	return result
}
