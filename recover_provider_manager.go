package fiberhouse

import (
	"fmt"
	"github.com/lamxy/fiberhouse/constant"
	"sync"
)

// FiberRecoveryProvider Fiber 恢复提供者
type FiberRecoveryProvider struct {
	IProvider
}

// NewFiberRecoveryProvider 创建 Fiber 恢复提供者
func NewFiberRecoveryProvider() *FiberRecoveryProvider {
	p := &FiberRecoveryProvider{
		IProvider: NewProvider().
			SetName("FiberRecoveryProvider").
			SetTarget(constant.CoreTypeWithFiber).
			SetType(ProviderTypeDefault().GroupRecoverMiddlewareChoose),
	}
	p.MountToParent(p)
	return p
}

// Initialize 重载fiber提供者初始化方法
func (p *FiberRecoveryProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	p.SetStatus(StateLoaded)
	return NewFiberRecovery(ctx.(IApplicationContext)), nil
}

// GinRecoveryProvider Gin 恢复提供者
type GinRecoveryProvider struct {
	IProvider
}

// NewGinRecoveryProvider 创建 Gin 恢复提供者
func NewGinRecoveryProvider() *GinRecoveryProvider {
	p := &GinRecoveryProvider{
		IProvider: NewProvider().
			SetName("GinRecoveryProvider").
			SetTarget(constant.CoreTypeWithGin).
			SetType(ProviderTypeDefault().GroupRecoverMiddlewareChoose),
	}
	p.MountToParent(p)
	return p
}

// Initialize 重载gin提供者初始化方法
func (p *GinRecoveryProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	p.SetStatus(StateLoaded)
	return NewGinRecovery(ctx.(IApplicationContext)), nil
}

//-----------------------------------------------------------------------------------

// RecoveryManager 恢复惊慌管理器
type RecoveryManager struct {
	IProviderManager
}

var (
	recoveryManagerInstance *RecoveryManager
	recoveryManagerOnce     sync.Once
)

// NewRecoveryManager 创建恢复惊慌管理器
func NewRecoveryManager(ctx IContext) *RecoveryManager {
	m := &RecoveryManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("RecoveryManager").
			SetType(ProviderTypeDefault().GroupRecoverMiddlewareChoose).
			SetOrBindToLocation(ProviderLocationDefault().LocationAppMiddlewareInit, true),
	}
	m.MountToParent(m)
	return m
}

// NewRecoveryManagerOnce 获取恢复惊慌管理器单例
func NewRecoveryManagerOnce(ctx IContext) *RecoveryManager {
	recoveryManagerOnce.Do(func() {
		recoveryManagerInstance = NewRecoveryManager(ctx)
	})
	return recoveryManagerInstance
}

// LoadProvider 根据 CoreType 加载对应的恢复惊慌提供者
func (m *RecoveryManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	appCtx, ok := m.GetContext().(IApplicationContext)
	if !ok {
		return nil, fmt.Errorf("context is not IApplicationContext")
	}

	bootCfg := appCtx.GetBootConfig()
	coreType := bootCfg.CoreType

	for _, provider := range m.List() {
		if provider.Target() == coreType {
			result, err := provider.Initialize(m.GetContext())
			if err != nil {
				return nil, err
			}
			if recovery, ok := result.(IRecover); ok {
				return recovery, nil
			}
		}
	}

	return nil, fmt.Errorf("no matching recovery provider found for core type: %s", coreType)
}

// MountToParent 挂载到父级管理器
func (m *RecoveryManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
