package fiberhouse

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

// CtxGinProvider Gin 框架上下文提供者
type CtxGinProvider struct {
	IProvider
}

func NewCtxGinProvider() *CtxGinProvider {
	return &CtxGinProvider{
		IProvider: NewProvider().SetName("CtxGinProvider").SetTarget("gin").SetType(ProviderTypeDefault().GroupCoreContextChoose),
	}
}

func (p *CtxGinProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	if len(initFunc) == 0 {
		return nil, errors.New("no initFunc provided")
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
		return nil, fmt.Errorf("invalid core context type: expected *gin.Context, got %T", ginCtx)
	}

	return providerCtx.WithGinContext(ginCtx), nil
}
