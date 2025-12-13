package provider

import (
	"github.com/lamxy/fiberhouse"
)

type Provider struct {
	name    string
	version string
	target  string
	status  IStater
	typ     string
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
func (p *Provider) Initialize(ctx fiberhouse.IContext, initFn ...InitFunc) error {
	if len(initFn) > 0 {
		return initFn[0](ctx)
	}
	return nil
}

// Status 返回提供者状态
func (p *Provider) Status() IStater {
	return p.status
}

// Type 返回提供者类型
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

// SetType 设置提供者类型
func (p *Provider) SetTarget(t string) IProvider {
	p.target = t
	return p
}

// SetStatus 设置提供者状态
func (p *Provider) SetStatus(status IStater) IProvider {
	p.status = status
	return p
}

// Type 返回提供者类型
func (p *Provider) Type() string {
	return p.typ
}

// SetType 设置提供者类型
func (p *Provider) SetType(typ string) IProvider {
	p.typ = typ
	return p
}

// RegisterToManager 将提供者注册到提供者管理器
func (p *Provider) RegisterToManager(m IManager) error {
	return m.Register(p.Name(), p)
}
