// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import "context"

// ClosableProvider 标记一个持有资源、需要在关闭链上回收的提供者。
//
// opt-in 接口，不并入 IProvider：绝大多数 provider 只是装配期的一次性工厂，产出
// 实例后自身不持有任何资源，强制它们实现空 Close 只会制造噪音。沿用
// globalmanager.Closable 的同一惯例——由确实持有资源的实现自行声明。
//
// 实现方须保证：
//   - Close 幂等。同一 provider 可能经由多条路径被关闭（管理器关闭链、应用直接
//     调用），第二次调用必须安全返回。
//   - 尊重 ctx。关闭超时靠它传播，长时间阻塞会拖垮整个关闭预算。
//   - 只在 StateLoaded 时被调用。未初始化或初始化失败的 provider 不持有实例，
//     CloseProviders 会跳过它们。
type ClosableProvider interface {
	// Close 释放本提供者持有的资源。
	Close(ctx context.Context) error
}

// ProviderInitFunc 提供者初始化函数类型
type ProviderInitFunc func(IProvider) (any, error)

// ProviderLoadFunc 提供者加载函数类型
type ProviderLoadFunc func(manager IProviderManager) (any, error)

// IProvider 提供者接口
type IProvider interface {
	// Name 返回提供者名称
	Name() string
	// Version 返回提供者版本
	Version() string
	// Initialize 执行提供者初始化操作
	Initialize(IContext, ...ProviderInitFunc) (any, error)
	// RegisterTo 将提供者注册到提供者管理器中
	RegisterTo(manager IProviderManager) error
	// Status 返回提供者当前状态
	Status() State
	// Target 返回提供者的目标框架引擎类型, e.g., "gin", "fiber",...。该字段区分不同框架引擎类型的提供者实现，也可以用区分其他维度
	Target() string
	// Type 返回提供者的类型, e.g., "middleware", "route_register", "sonic_json_codec", "std_json_codec",...
	Type() IProviderType
	// SetName 设置提供者名称
	SetName(string) IProvider
	// SetVersion 设置提供者版本
	SetVersion(string) IProvider
	// SetTarget 设置提供者目标框架
	SetTarget(string) IProvider
	// SetStatus 设置提供者状态
	SetStatus(State) IProvider
	// SetType 设置提供者类型，仅允许设置一次
	SetType(IProviderType) IProvider
	// Check 检查提供者是否设置类型值
	//
	// 类型未设置时 panic；需要处理该错误请改用 CheckWithError。
	Check() bool
	// CheckWithError 与 Check 相同，但类型未设置时返回错误而非 panic
	CheckWithError() (bool, error)
	// BindToUniqueManagerIfSingleton 将提供者绑定到唯一的管理器
	// 注意：传入的管理器对象应当是一个单例实现，以确保全局唯一性
	// 该方法内部调用管理器的 BindToUniqueProvider 方法进行彼此唯一绑定
	// 返回提供者自身以支持链式调用
	// 生效条件：1. 传入的管理器对象是单例实现；2. 子类提供者重载该方法且子类实例本身调用该方法；3. 需要将子类实例反向挂载到父类属性上
	BindToUniqueManagerIfSingleton(IProviderManager) IProvider
	// MountToParent 将当前提供者挂载到父级提供者中
	//
	// 参数不合法时 panic；需要处理该错误请改用 MountToParentWithError。
	MountToParent(son ...IProvider) IProvider
	// MountToParentWithError 与 MountToParent 相同，但参数不合法时返回错误而非 panic
	MountToParentWithError(son ...IProvider) (IProvider, error)
	// ReturnAndSetSuccessInitialize 设置提供者状态并返回成功的初始化结果
	// 用于子类重载 initialize 方法内调用
	ReturnAndSetSuccessInitialize(any, error) (any, error)
	// ReturnAndSetFailInitialize 设置提供者状态并返回失败的初始化结果
	// 用于子类重载 initialize 方法内调用
	ReturnAndSetFailInitialize(any, error) (any, error)
	// ReturnInitialized 依据状态识别直接返回已缓存的初始化结果
	// 用于子类重载 initialize 方法内调用
	ReturnInitialized() (any, error)
}

// IProviderManager 提供者管理器接口
type IProviderManager interface {
	// Name 返回提供者管理器名称
	Name() string
	// SetName 设置提供者管理器名称
	SetName(string) IProviderManager
	// Type 返回提供者类型
	Type() IProviderType
	// SetType 设置提供者类型，仅允许设置一次
	SetType(IProviderType) IProviderManager
	// Location 获取管理器的执行位置点
	Location() IProviderLocation
	// SetOrBindToLocation 设置管理器的执行位置点，仅允许设置一次
	//
	// 绑定失败（manager 为 nil、同一实例重复绑定）时本方法不报错，以保持构造子里的
	// 链式调用；需要观测绑定结果请改用 SetOrBindToLocationWithError。
	SetOrBindToLocation(IProviderLocation, ...bool) IProviderManager
	// SetOrBindToLocationWithError 与 SetOrBindToLocation 相同，但返回绑定错误
	SetOrBindToLocationWithError(IProviderLocation, ...bool) (IProviderManager, error)
	// GetContext 获取管理器关联的上下文对象
	GetContext() IContext
	// Register 注册提供者到管理器中
	Register(provider IProvider) error
	// Unregister 从管理器中注销提供者
	Unregister(name string) error
	// GetProvider 根据名称获取提供者实例
	GetProvider(name string) (IProvider, error)
	// List 列出管理器中所有注册的提供者
	List() []IProvider
	// Map 以名称为键，提供者实例为值，返回管理器中所有注册的提供者映射
	Map() map[string]IProvider
	// LoadProvider 加载提供者
	LoadProvider(loadFunc ...ProviderLoadFunc) (any, error)
	// InitializeProvider 通过管理器统一初始化提供者，并复用已完成的初始化结果
	InitializeProvider(provider IProvider, initFunc ...ProviderInitFunc) (any, error)
	// Check 检查提供者管理器是否设置类型值
	//
	// 类型未设置时 panic；需要处理该错误请改用 CheckWithError。
	Check()
	// CheckWithError 与 Check 相同，但类型未设置时返回错误而非 panic
	CheckWithError() error
	// BindToUniqueProvider 绑定唯一的提供者到管理器
	// 确保管理器有且仅有一个提供者注册进来
	// 如果已存在相同的提供者记录，视为注册成功
	// 如果已存在多个提供者，则 panic 错误
	// 返回管理器自身以支持链式调用
	//
	// 绑定失败时 panic；需要处理该错误请改用 BindToUniqueProviderWithError。
	BindToUniqueProvider(IProvider) IProviderManager
	// BindToUniqueProviderWithError 与 BindToUniqueProvider 相同，但绑定失败时
	// 返回错误而非 panic
	BindToUniqueProviderWithError(IProvider) (IProviderManager, error)
	// IsUnique 返回管理器是否处于唯一提供者模式
	IsUnique() bool
	// MountToParent 将当前管理器挂载到父级管理器中
	//
	// 参数不合法时 panic；需要处理该错误请改用 MountToParentWithError。
	MountToParent(son ...IProviderManager) IProviderManager
	// MountToParentWithError 与 MountToParent 相同，但参数不合法时返回错误而非 panic
	MountToParentWithError(son ...IProviderManager) (IProviderManager, error)
	// CloseProviders 按注册逆序关闭实现了 ClosableProvider 的已初始化提供者
	//
	// 单个提供者关闭失败不中断其余关闭，全部错误经 errors.Join 聚合后返回。
	CloseProviders(ctx context.Context) error
}

// IProviderLocation 提供者位点接口，用于标识提供者管理器的执行位置，应用启动流程或生命周期的相关阶段，以及其他自定义执行点
// 位置点对象可以收集绑定到该位置点的提供者管理器，并按顺序加载和执行这些管理器中的提供者，实现灵活地扩展和定制化行为
type IProviderLocation interface {
	// GetLocationID 获取位点序号
	GetLocationID() uint8
	// GetLocationName 获取位点名称
	GetLocationName() string
	// IsDefaultLocation 是否为默认位点
	IsDefaultLocation() bool
	// Bind 绑定管理器到该位点的管理器列表中
	Bind(manager IProviderManager) error
	// GetManagers 获取已绑定到该位点的管理器列表
	GetManagers() []IProviderManager
}
