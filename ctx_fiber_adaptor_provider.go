// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
)

// CoreCtxFiberProvider Fiber 框架核心上下文提供者
type CoreCtxFiberProvider struct {
	IProvider
}

func NewCoreCtxFiberProvider() *CoreCtxFiberProvider {
	son := &CoreCtxFiberProvider{
		IProvider: NewProvider().SetName("CtxFiberProvider").SetTarget("fiber").SetType(ProviderTypeDefault().GroupCoreContextChoose),
	}
	son.MountToParent(son)
	return son
}

// Initialize 初始化 Fiber 框架核心上下文提供者
func (p *CoreCtxFiberProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s' Initialize: no initFunc provided", p.Name())
	}

	// 通过 initFunc 获取外部的 core context
	coreCtx, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}

	var (
		fiberCtx *fiber.Ctx
		ok       bool
	)

	if fiberCtx, ok = coreCtx.(*fiber.Ctx); !ok {
		return nil, fmt.Errorf("provider '%s' Initialize: invalid core context type: expected *fiber.Ctx, got %T", p.Name(), fiberCtx)
	}

	return providerctx.WithFiberContext(fiberCtx), nil
}
