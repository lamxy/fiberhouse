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

// 状态变量 pending、loaded、skipped、failed
var (
	StatePending = new(State).Set(0, "pending")
	StateLoaded  = new(State).Set(1, "loaded")
	StateSkipped = new(State).Set(1, "skipped")
	StateFailed  = new(State).Set(1, "failed")
)

// State 提供者的状态结构体，实现状态器接口
type State struct {
	id   uint8
	name string
	lock sync.RWMutex
}

// Id 状态Id
func (s *State) Id() uint8 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.id
}

// Name 状态名称
func (s *State) Name() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.name
}

// Set 设置状态Id和名称
func (s *State) Set(id uint8, name string) IState {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.id = id
	s.name = name
	return s
}

// SetState 从另一个状态器设置状态
func (s *State) SetState(state IState) IState {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.id = state.Id()
	s.name = state.Name()
	return s
}

// 定义提供者错误类型
var (
	ErrProviderAlreadyExists = &ProviderError{msg: "provider already exists"}
	ErrProviderNotFound      = &ProviderError{msg: "provider not found"}
)

type ProviderError struct {
	msg string
}

func (e *ProviderError) Error() string {
	return e.msg
}

// Provider 提供者接口的基类实现，通过组合模式支持子类扩展，子类只需重载所需方法即可实现多态行为
// 注意：在调用提供者接口的一些特性方法前，子类实例应通过 MountToParent 方法将子类实例挂载到该基类的 sonProvider 字段，以确保多态行为的正确实现
// 如Initialize、RegisterTo、BindToUniqueManagerIfSingleton
//
// 注意：提供者基类实现中未使用锁机制保护并发安全，仅在应用启动阶段初始化、写操作，运行时仅允许读取操作；否则子类应自行实现并发安全保护
type Provider struct {
	sonProvider IProvider // 允许子类继承该接口以实现多态
	name        string
	version     string
	target      string
	status      IState
	statOnce    sync.Once
	pType       IProviderType
	pTypeOnce   sync.Once
}

// NewProvider 创建一个基础提供者
func NewProvider() *Provider {
	return &Provider{
		status: StatePending,                   // 默认状态为待定
		pType:  ProviderTypeDefault().ZeroType, // 默认零值类型
	}
}

// Check 检查提供者类型是否设置，未设置则抛出异常，强制Initialize方法内优先进行检查
func (p *Provider) Check() {
	if p.pType.GetTypeID() == ProviderTypeDefault().ZeroType.GetTypeID() {
		panic(fmt.Errorf("provider '%s' type is not set", p.name))
	}
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return p.name
}

// Version 返回提供者版本
func (p *Provider) Version() string {
	return p.version
}

// Initialize 初始化提供者
func (p *Provider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	// 检查sonProvider字段是否存在
	err := p.checkSonProvider()
	if err != nil {
		return nil, err
	}
	return p.sonProvider.Initialize(ctx, initFunc...)
}

// checkSonProvider 检查子类提供者是否设置
func (p *Provider) checkSonProvider() error {
	if p.sonProvider == nil {
		return fmt.Errorf("sonManager from base class '%s' is not set, need to call the MountToParent method of the subclass instance to attach the subclass instance to the parent class's sonManager field", p.name)
	}
	if p.sonProvider == p {
		return fmt.Errorf("sonManager from base class '%s' cannot be the same instance as the parent manager, please check the MountToParent method parameter", p.name)
	}
	return nil
}

// Status 返回提供者状态
func (p *Provider) Status() IState {
	return p.status
}

// Target 返回提供者目标标识
func (p *Provider) Target() string {
	return p.target
}

// SetName 设置提供者名称
func (p *Provider) SetName(name string) IProvider {
	p.name = name
	return p
}

// SetVersion 设置提供者版本
func (p *Provider) SetVersion(version string) IProvider {
	p.version = version
	return p
}

// SetTarget 设置提供者目标标识
func (p *Provider) SetTarget(t string) IProvider {
	p.target = t
	return p
}

// SetStatus 设置提供者状态(仅允许设置一次)
func (p *Provider) SetStatus(status IState) IProvider {
	p.statOnce.Do(func() {
		p.status = status
	})
	return p
}

// Type 返回提供者类型
func (p *Provider) Type() IProviderType {
	return p.pType
}

// SetType 设置提供者类型
func (p *Provider) SetType(typ IProviderType) IProvider {
	p.pTypeOnce.Do(func() {
		p.pType = typ
	})
	return p
}

// RegisterTo 将提供者注册到管理器
// 注意：此方法会注册 sonProvider 字段指向的实例，子类型应通过 MountToParent 设置该字段，否则避免使用该基类方法
func (p *Provider) RegisterTo(m IProviderManager) error {
	if err := p.checkSonProvider(); err != nil {
		return err
	}
	return m.Register(p.sonProvider)
}

// BindToUniqueManagerIfSingleton 将提供者绑定到唯一的管理器
// 注意：传入的管理器对象应当是一个单例实现，以确保全局唯一性
// 该方法内部调用管理器的 BindToUniqueProvider 方法进行彼此唯一绑定
// 返回提供者自身以支持链式调用
func (p *Provider) BindToUniqueManagerIfSingleton(m IProviderManager) IProvider {
	m.BindToUniqueProvider(p)
	return p
}

// MountToParent 将当前提供者挂载到父级提供者 sonManager 字段上
func (p *Provider) MountToParent(son ...IProvider) IProvider {
	if len(son) == 0 {
		panic(fmt.Errorf("MountToParent() from base class '%s' must provide at least one IProviderManager", p.name))
	}
	if p == son[0] {
		panic(fmt.Errorf("MountToParent() form base class '%s', sonManager parameter cannot be the same as the parent manager instance", p.name))
	}
	p.sonProvider = son[0]
	return p
}
