// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

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
	Status() IState
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
	SetStatus(IState) IProvider
	// SetType 设置提供者类型，仅允许设置一次
	SetType(IProviderType) IProvider
	// Check 检查提供者是否设置类型值
	Check()
	// BindToUniqueManagerIfSingleton 将提供者绑定到唯一的管理器
	// 注意：传入的管理器对象应当是一个单例实现，以确保全局唯一性
	// 该方法内部调用管理器的 BindToUniqueProvider 方法进行彼此唯一绑定
	// 返回提供者自身以支持链式调用
	// 生效条件：1. 传入的管理器对象是单例实现；2. 子类提供者重载该方法且子类实例本身调用该方法；3. 需要将子类实例反向挂载到父类属性上
	BindToUniqueManagerIfSingleton(IProviderManager) IProvider
	// MountToParent 将当前提供者挂载到父级提供者中
	MountToParent(son ...IProvider) IProvider
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
	SetOrBindToLocation(IProviderLocation, ...bool) IProviderManager
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
	// Check 检查提供者管理器是否设置类型值
	Check()
	// BindToUniqueProvider 绑定唯一的提供者到管理器
	// 确保管理器有且仅有一个提供者注册进来
	// 如果已存在相同的提供者记录，视为注册成功
	// 如果已存在多个提供者，则 panic 错误
	// 返回管理器自身以支持链式调用
	BindToUniqueProvider(IProvider) IProviderManager
	// IsUnique 返回管理器是否处于唯一提供者模式
	IsUnique() bool
	// MountToParent 将当前管理器挂载到父级管理器中
	MountToParent(son ...IProviderManager) IProviderManager
}

// IState 提供者状态接口
type IState interface {
	// Id 返回状态标识符
	Id() uint8
	// Name 返回状态名称
	Name() string
	// Set 设置状态标识符和名称
	Set(uint8, string) IState
	// SetState 用另一个状态对象的值设置当前状态对象
	SetState(IState) IState
}
