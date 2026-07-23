// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"fmt"
)

// CoreStarterFiberProvider 核心Fiber提供者
type CoreStarterFiberProvider struct {
	IProvider
}

func NewCoreStarterFiberProvider() *CoreStarterFiberProvider {
	return &CoreStarterFiberProvider{
		IProvider: NewProvider().SetName("CoreFiberProvider").SetTarget("fiber").SetType(ProviderTypeDefault().GroupCoreStarterChoose),
	}
}

// Initialize 重载初始化核心Fiber提供者
func (p *CoreStarterFiberProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	if !p.Check() {
		return p.ReturnDirectly()
	}
	if len(initFunc) == 0 {
		return p.SetAndReturnSucceededInitialized(NewCoreWithFiber(ctx.(IApplicationContext)), nil)
	}

	anything, err := initFunc[0](p) // 匿名函数参数获取核心启动器初始化的选项参数切片
	if err != nil {
		return p.SetAndReturnFailedInitialized(nil, fmt.Errorf("CoreFiberProvider initialize failed: %w", err))
	}

	var (
		coreStarterOptions []CoreStarterOption
		ok                 bool
	)

	if coreStarterOptions, ok = anything.([]CoreStarterOption); ok {
		return p.SetAndReturnSucceededInitialized(NewCoreWithFiber(ctx.(IApplicationContext), coreStarterOptions...), nil)
	}

	return anything, err
}
