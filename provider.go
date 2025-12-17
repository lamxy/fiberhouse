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

// RegisterToManager 将提供者注册到提供者管理器
func (p *Provider) RegisterTo(m IProviderManager) error {
	return m.Register(p.Name(), p)
}

type DefaultServerProvider struct {
	IProvider
}

// NewDefaultServerProvider 创建一个默认服务器提供者
func NewDefaultServerProvider() *DefaultServerProvider {
	return &DefaultServerProvider{
		IProvider: NewProvider().SetName("default_server_provider").SetType(ProviderTypeDefault().WebRunServer),
	}
}
