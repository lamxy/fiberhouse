package fiberhouse

import (
	"fmt"
	"sync"
)

type Provider struct {
	name      string
	version   string
	target    string
	status    IState
	statOnce  sync.Once
	pType     IProviderType
	pTypeOnce sync.Once
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
	if len(initFunc) > 0 {
		return initFunc[0](p)
	}
	return nil, fmt.Errorf("no initialize function provided for provider '%s'", p.name)
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

// RegisterToManager 将提供者注册到提供者管理器 // TODO 继承的子类提供者未重载该方法，将会注册父类的提供者实例到管理器中，需确认是否符合预期
func (p *Provider) RegisterTo(m IProviderManager) error {
	return m.Register(p)
}

// BindToUniqueManagerIfSingleton 将提供者绑定到唯一的管理器  // TODO 继承的子类提供者未重载该方法，将会绑定父类的提供者实例到管理器中，需确认是否符合预期
// 注意：传入的管理器对象应当是一个单例实现，以确保全局唯一性
// 该方法内部调用管理器的 BindToUniqueProvider 方法进行彼此唯一绑定
// 返回提供者自身以支持链式调用
func (p *Provider) BindToUniqueManagerIfSingleton(m IProviderManager) IProvider {
	m.BindToUniqueProvider(p)
	return p
}
