package fiberhouse

import (
	"fmt"
	"sync"
)

// Provider 提供者接口的基类实现，通过组合模式支持子类扩展，子类只需重载所需方法即可实现多态行为
// 注意：在调用提供者接口的一些特性方法前，子类实例应通过 MountToParent 方法将子类实例挂载到该基类的 sonProvider 字段，以确保多态行为的正确实现
// 如Initialize、RegisterTo、BindToUniqueManagerIfSingleton
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
		status: StateUnload,
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
// 注意：此方法会注册 sonProvider 字段指向的实例，子类型应通过 MountToParent 设置该字段，否则避免该基类方法
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
