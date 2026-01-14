// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"fmt"
)

// CoreStarterGinProvider 核心Gin提供者
type CoreStarterGinProvider struct {
	IProvider
}

func NewCoreStarterGinProvider() *CoreStarterGinProvider {
	return &CoreStarterGinProvider{
		IProvider: NewProvider().SetName("CoreGinProvider").SetTarget("gin").SetType(ProviderTypeDefault().GroupCoreStarterChoose),
	}
}

// Initialize 重载初始化核心Gin提供者
func (p *CoreStarterGinProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	if len(initFunc) == 0 {
		return NewCoreWithFiber(ctx.(IApplicationContext)), nil
	}

	anything, err := initFunc[0](p) // 匿名函数参数获取核心启动器初始化的选项参数切片
	if err != nil {
		return nil, fmt.Errorf("CoreFiberProvider initialize failed: %w", err)
	}

	var (
		coreStarterOptions []CoreStarterOption
		ok                 bool
	)

	if coreStarterOptions, ok = anything.([]CoreStarterOption); ok {
		return NewCoreWithGin(ctx.(IApplicationContext), coreStarterOptions...), nil
	}

	return anything, err
}
