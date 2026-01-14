// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"fmt"
	"sync"
)

// ProviderManager 提供者管理器接口的基类实现，通过组合模式支持子类扩展，子类只需重载所需方法即可实现多态行为
// 注意：在调用提供者管理器接口的一些特性方法前，子类实例应通过 MountToParent 方法将子类实例挂载到该基类的 sonManager 字段，以确保多态行为的正确实现
// 如LoadProvider()方法
//
// 注意：提供者管理器基类实现中未使用锁机制保护并发安全，仅在应用启动阶段初始化、写操作，运行时仅允许读取操作；否则子类应自行实现并发安全保护
type ProviderManager struct {
	name       string
	sonManager IProviderManager
	ctx        IContext
	providers  map[string]IProvider
	pType      IProviderType
	pTypeOnce  sync.Once
	location   IProviderLocation
	isUnique   bool // 标识管理器是否处于唯一提供者模式
}

// NewProviderManager 创建一个基类的提供者管理器，用于组合和扩展
func NewProviderManager(ctx IContext) *ProviderManager {
	return &ProviderManager{
		ctx:       ctx,
		pType:     ProviderTypeDefault().ZeroType,         // 默认零值类型
		location:  ProviderLocationDefault().ZeroLocation, // 默认零值位置
		providers: make(map[string]IProvider),
	}
}

// Check 检查提供者类型是否设置，未设置则抛出异常，强制Initialize方法内优先进行检查
func (m *ProviderManager) Check() {
	if m.pType.GetTypeID() == ProviderTypeDefault().ZeroType.GetTypeID() {
		panic(fmt.Errorf("manager '%s' type is not set", m.name))
	}
}

// Name 返回提供者管理器名称
func (m *ProviderManager) Name() string {
	return m.name
}

// SetName 设置提供者管理器名称
func (m *ProviderManager) SetName(name string) IProviderManager {
	m.name = name
	return m
}

// Type 返回提供者类型
func (m *ProviderManager) Type() IProviderType {
	return m.pType
}

// SetType 设置提供者类型，仅允许设置一次
func (m *ProviderManager) SetType(typ IProviderType) IProviderManager {
	m.pTypeOnce.Do(func() {
		m.pType = typ
	})
	return m
}

// Location 返回管理器执行位置点标识
func (m *ProviderManager) Location() IProviderLocation {
	return m.location
}

// SetOrBindToLocation 设置管理器执行位置点标识，如果传入 bind 参数且为 true，则将管理器绑定（添加）到该位置点的管理器列表中
func (m *ProviderManager) SetOrBindToLocation(l IProviderLocation, bind ...bool) IProviderManager {
	m.location = l
	if len(bind) > 0 && bind[0] {
		// 绑定管理器到执行位点对象: 子实例存在绑定子实例，否则绑定父实例
		if m.sonManager != nil {
			_ = l.Bind(m.sonManager)
			return m
		}
		_ = l.Bind(m)
	}
	return m
}

// GetContext 获取应用上下文
func (m *ProviderManager) GetContext() IContext {
	return m.ctx
}

// Register 注册一个 provider
func (m *ProviderManager) Register(provider IProvider) error {
	// 如果管理器处于唯一提供者模式,且已有提供者,则拒绝注册
	if m.isUnique && len(m.providers) > 0 {
		return fmt.Errorf("manager '%s' is in unique provider mode, cannot register another provider", m.name)
	}

	if _, exists := m.providers[provider.Name()]; exists {
		return ErrProviderAlreadyExists
	}

	m.providers[provider.Name()] = provider
	return nil
}

// Unregister 注销一个 provider
func (m *ProviderManager) Unregister(name string) error {
	//m.lock.Lock()
	//defer m.lock.Unlock()
	//
	//if _, exists := m.providers[name]; !exists {
	//	return ErrProviderNotFound
	//}
	//
	//delete(m.providers, name)
	//return nil
	return nil
}

// GetProvider 获取指定名称的 provider
func (m *ProviderManager) GetProvider(name string) (IProvider, error) {
	provider, exists := m.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' does not exist: %s", name, ErrProviderNotFound.Error())
	}

	return provider, nil
}

// List 返回所有已注册的 providers
func (m *ProviderManager) List() []IProvider {
	list := make([]IProvider, 0, len(m.providers))
	for _, provider := range m.providers {
		list = append(list, provider)
	}

	return list
}

// Map 返回所有已注册的 providers 映射
func (m *ProviderManager) Map() map[string]IProvider {
	return m.providers
}

// LoadProvider 加载 providers
func (m *ProviderManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.Check()
	if m.sonManager == nil {
		return nil, fmt.Errorf("sonManager from base class '%s' is not set, need to call the MountToParent method of the subclass instance to attach the subclass instance to the parent class's sonManager field", m.name)
	}
	if m == m.sonManager {
		return nil, fmt.Errorf("sonManager from base class '%s' cannot be the same as the parent manager instance", m.name)
	}
	return m.sonManager.LoadProvider(loadFunc...)
}

// IsUnique 返回管理器是否处于唯一提供者模式
func (m *ProviderManager) IsUnique() bool {
	return m.isUnique
}

// BindToUniqueProvider 绑定唯一的提供者到管理器
// 确保管理器有且仅有一个提供者注册进来
// 如果已存在相同的提供者记录，视为注册成功并设置唯一属性为 true
// 如果已存在多个提供者，则 panic 错误
// 返回管理器自身以支持链式调用
func (m *ProviderManager) BindToUniqueProvider(provider IProvider) IProviderManager {
	providerCount := len(m.providers)

	// 检查是否已存在多个提供者
	if providerCount > 1 {
		panic(fmt.Errorf("manager '%s' already has multiple providers (%d), cannot bind unique provider", m.name, providerCount))
	}

	// 检查是否已存在一个提供者
	if providerCount == 1 {
		// 检查是否是相同的提供者
		if existingProvider, exists := m.providers[provider.Name()]; exists {
			// 相同提供者，视为注册成功
			if existingProvider == provider {
				m.isUnique = true
				return m
			}
		}
		// 已存在不同的提供者，无法绑定
		panic(fmt.Errorf("manager '%s' already has a different provider, cannot bind unique provider '%s'", m.name, provider.Name()))
	}

	// 没有提供者，直接注册
	m.providers[provider.Name()] = provider
	m.isUnique = true
	return m
}

// MountToParent 将子类管理器实例挂载到父类管理器的 sonManager 字段上
func (m *ProviderManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) == 0 {
		panic(fmt.Errorf("MountToParent() from base class '%s' must provide at least one IProviderManager", m.name))
	}
	if m == son[0] {
		panic(fmt.Errorf("MountToParent() form base class '%s', sonManager parameter cannot be the same as the parent manager instance", m.name))
	}
	m.sonManager = son[0]
	return m
}

// DefaultPManager 默认提供者管理器，实现默认的提供者加载逻辑
type DefaultPManager struct {
	IProviderManager
}

// NewDefaultPManager 创建一个默认提供者管理器实例，实现默认的提供者加载逻辑
func NewDefaultPManager(ctx IContext) *DefaultPManager {
	son := &DefaultPManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("DefaultPManager").
			SetType(ProviderTypeDefault().GroupDefaultManagerType).
			SetOrBindToLocation(ProviderLocationDefault().ZeroLocation),
	}
	// 让子管理器挂载到父管理器上，确保多态行为的正确实现
	// 无需重载MountToParent方法，NewDefaultPManager()内已调基类挂载方法进行了挂载
	son.MountToParent(son)
	return son
}

func (m *DefaultPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	if len(loadFunc) > 0 {
		return m.IProviderManager.LoadProvider(loadFunc...)
	}

	// 默认加载逻辑，根据引导配置加载相应的提供者
	bootCfg := m.GetContext().(IApplicationContext).GetBootConfig()

	if len(m.List()) == 0 {
		return nil, ErrProviderNotFound
	}

	var errs []error

	for _, provider := range m.List() {
		if provider.Type().GetTypeID() == ProviderTypeDefault().GroupProviderAutoRun.GetTypeID() {
			// 自动运行类型的提供者，不依赖Target约束可以直接初始化
			_, err := provider.Initialize(m.GetContext())
			errs = append(errs, err)
		} else if provider.Target() == bootCfg.CoreType {
			// 目标类型匹配启动配置的核心类型的提供者，进行初始化
			_, err := provider.Initialize(m.GetContext())
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to load providers: %v", errs)
	}

	return nil, nil
}

// MountToParent 重载挂载到父级提供者管理器
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *DefaultPManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
