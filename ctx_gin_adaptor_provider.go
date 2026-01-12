// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"fmt"
	"github.com/gin-gonic/gin"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
)

// CoreCtxGinProvider Gin 框架上下文提供者
type CoreCtxGinProvider struct {
	IProvider
}

func NewCoreCtxGinProvider() *CoreCtxGinProvider {
	return &CoreCtxGinProvider{
		IProvider: NewProvider().SetName("CtxGinProvider").SetTarget("gin").SetType(ProviderTypeDefault().GroupCoreContextChoose),
	}
}

func (p *CoreCtxGinProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
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
		ginCtx *gin.Context
		ok     bool
	)

	if ginCtx, ok = coreCtx.(*gin.Context); !ok {
		return nil, fmt.Errorf("provider '%s' Initialize: invalid core context type: expected *gin.Context, got %T", p.Name(), ginCtx)
	}

	return providerctx.WithGinContext(ginCtx), nil
}
