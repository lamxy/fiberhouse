// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"fmt"
	"sync"
)

// CoreCtxPManager 核心上下文提供者管理器
type CoreCtxPManager struct {
	IProviderManager
}

var (
	coreCtxPManager       *CoreCtxPManager
	coreCtxOnce           sync.Once
	coreCtxPManagerParent *ProviderManager
	coreCtxParentOnce     sync.Once
)

// NewCoreCtxPManager 创建核心上下文提供者管理器
func NewCoreCtxPManager(appCtx IApplicationContext) *CoreCtxPManager {
	son := &CoreCtxPManager{
		IProviderManager: NewProviderManager(appCtx).
			SetName("CoreCtxPManager").
			SetType(ProviderTypeDefault().GroupCoreContextChoose).
			SetOrBindToLocation(ProviderLocationDefault().LocationAdaptCoreCtxChoose, true), // 绑定到核心适配器核心上下文选择位置点
	}
	// 挂载子实例到父实例的sonManager字段
	son.MountToParent(son)
	return son
}

// NewCoreCtxPManagerOnce 单例模式创建核心上下文提供者管理器
func NewCoreCtxPManagerOnce(appCtx IContext) *CoreCtxPManager {
	coreCtxOnce.Do(func() {
		coreCtxPManager = NewCoreCtxPManager(appCtx.(IApplicationContext))
	})
	return coreCtxPManager
}

// NewCoreCtxPManagerParentOnce 单例模式获取核心上下文提供者管理器的父级管理器
// 注意: 该父级管理器应已通过位置点注册了核心上下文提供者管理器，否则将抛出异常
func NewCoreCtxPManagerParentOnce() *ProviderManager {
	coreCtxParentOnce.Do(func() {
		// 从核心适配器核心上下文选择位置点获取相应的管理器
		managers := ProviderLocationDefault().LocationAdaptCoreCtxChoose.GetManagers()
		if len(managers) == 0 {
			panic(errors.New("no core context provider manager found in location '" + ProviderLocationDefault().LocationAdaptCoreCtxChoose.GetLocationName() + "'"))
		}
		coreCtxPManagerParent = managers[0].(*ProviderManager)
	})
	return coreCtxPManagerParent
}

// LoadProvider 重载加载提供者
func (m *CoreCtxPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.Check()
	if len(loadFunc) == 0 {
		return nil, fmt.Errorf("manager '%s' LoadProvider: load function is required", m.Name())
	}
	coreCtx, err := loadFunc[0](m)
	if err != nil {
		return nil, err
	}
	bootCfg := m.GetContext().(IApplicationContext).GetBootConfig()
	for _, provider := range m.List() {
		if provider.Target() == bootCfg.CoreType {
			return provider.Initialize(m.GetContext(), func(provider IProvider) (any, error) {
				return coreCtx, nil
			})
		}
	}
	return nil, fmt.Errorf("manager '%s' LoadProvider: no core context provider found", m.Name())
}

// MountToParent 重载挂载到父级提供者管理器
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *CoreCtxPManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
