// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

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
	defer p.SetStatus(StateLoaded)
	return NewGinRecovery(ctx.(IApplicationContext)), nil
}

//-----------------------------------------------------------------------------------

// RecoveryPManager 恢复惊慌管理器
type RecoveryPManager struct {
	IProviderManager
}

var (
	recoveryManagerInstance *RecoveryPManager
	recoveryManagerOnce     sync.Once
)

// NewRecoveryPManager 创建恢复惊慌管理器
func NewRecoveryPManager(ctx IContext) *RecoveryPManager {
	m := &RecoveryPManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("RecoveryPManager").
			SetType(ProviderTypeDefault().GroupRecoverMiddlewareChoose).
			SetOrBindToLocation(ProviderLocationDefault().LocationAppMiddlewareInit, true),
	}
	m.MountToParent(m)
	return m
}

// NewRecoveryPManagerOnce 获取恢复惊慌管理器单例
func NewRecoveryPManagerOnce(ctx IContext) *RecoveryPManager {
	recoveryManagerOnce.Do(func() {
		recoveryManagerInstance = NewRecoveryPManager(ctx)
	})
	return recoveryManagerInstance
}

// LoadProvider 根据 CoreType 加载对应的恢复惊慌提供者
func (m *RecoveryPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
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
			return nil, fmt.Errorf("provider %s is not recoverable", provider.Name())
		}
	}

	return nil, fmt.Errorf("no matching recovery provider found for core type: %s", coreType)
}

// MountToParent 挂载到父级管理器
func (m *RecoveryPManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
