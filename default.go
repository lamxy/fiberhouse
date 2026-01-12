// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

// 提问AI: 提供一种方式，收集默认提供者、提供者管理器列表，同时支持追加更多的自定义的提供者、提供者管理器的列表，组合起来方便使用.
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
		// 在此处添加默认的提供者，实例化后挂载到基类字段
		defaultProvidersInstance.providers = append(defaultProvidersInstance.providers,
			NewFrameDefaultProvider(),     // 默认框架启动器提供者
			NewCoreStarterFiberProvider(), // 核心启动器Fiber提供者
			NewCoreStarterGinProvider(),   // 核心启动器Gin提供者
			NewJsonJCodecFiberProvider(),  // JSON编解码Fiber提供者
			NewJsonJCodecGinProvider(),    // JSON编解码Gin提供者
			NewSonicJCodecFiberProvider(), // Sonic编解码Fiber提供者
			NewSonicJCodecGinProvider(),   // Sonic编解码Gin提供者
			NewCoreCtxFiberProvider(),     // 核心上下文Fiber适配器提供者
			NewCoreCtxGinProvider(),       // 核心上下文Gin适配器提供者
			NewFiberRecoveryProvider(),    // Fiber恢复提供者（框架默认提供）
			NewGinRecoveryProvider(),      // Gin恢复提供者（框架默认提供）、及其他更多的基于自定义框架的恢复提供者
			NewRespInfoProtobufProvider(), // Protobuf响应编解码提供者
			NewRespInfoMsgpackProvider(),  // Msgpack响应编解码提供者
			// more...
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

// Except 排除指定名称的提供者，返回自身以支持链式调用
func (c *DefaultProviderCollection) Except(names ...string) *DefaultProviderCollection {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(names) == 0 {
		return c
	}

	// 构建排除名称集合
	excludeSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		excludeSet[name] = struct{}{}
	}

	// 过滤提供者
	filtered := make([]IProvider, 0, len(c.providers))
	for _, provider := range c.providers {
		if _, excluded := excludeSet[provider.Name()]; !excluded {
			filtered = append(filtered, provider)
		}
	}

	c.providers = filtered
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
		// 在此处添加默认的提供者管理器，实例化后挂载到基类字段
		defaultPManagersInstance.managers = append(defaultPManagersInstance.managers,
			// 默认提供者管理器(加载提供者的默认处理逻辑)
			//初始化方法NewDefaultPManager内调用基类挂载方法已挂载子类实例，该管理器无需重载MountToParent并进行调用
			NewDefaultPManager(ctx),
			// 默认框架启动器管理器
			//对于未在初始化方法内调用基类挂载方法的子类管理器，则需要在此处调用子类重载MountToParent进行挂载
			NewFrameDefaultPManager(ctx).MountToParent(),
			// 核心启动器管理器
			NewCoreStarterPManager(ctx).MountToParent(),
			// JSON编解码管理器
			NewJsonCodecPManager(ctx).MountToParent(),
			// 核心上下文适配器管理器
			NewCoreCtxPManager(ctx),
			// 恢复惊慌管理器单例（性能需要）
			NewRecoveryPManagerOnce(ctx),
			// 响应编解码管理器
			NewRespInfoPManager(ctx),
			// more...
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

// Add 添加默认提供者管理器到集合， 返回自身以支持链式调用
func (c *DefaultPManagerCollection) Add(managers ...IProviderManager) *DefaultPManagerCollection {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.managers = append(c.managers, managers...)
	return c
}

// Except 排除指定名称的提供者管理器，返回自身以支持链式调用
func (c *DefaultPManagerCollection) Except(names ...string) *DefaultPManagerCollection {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(names) == 0 {
		return c
	}

	// 构建排除名称集合
	excludeSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		excludeSet[name] = struct{}{}
	}

	// 过滤管理器
	filtered := make([]IProviderManager, 0, len(c.managers))
	for _, manager := range c.managers {
		if _, excluded := excludeSet[manager.Name()]; !excluded {
			filtered = append(filtered, manager)
		}
	}

	c.managers = filtered
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
