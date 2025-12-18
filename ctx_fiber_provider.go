package fiberhouse

import (
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

type CtxFiberProvider struct {
	IProvider
}

func NewCtxFiberProvider() *CtxFiberProvider {
	return &CtxFiberProvider{
		IProvider: NewProvider().SetName("CtxFiberProvider").SetTarget("fiber").SetType(ProviderTypeDefault().GroupCoreContextType),
	}
}

func (p *CtxFiberProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
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
		fiberCtx *fiber.Ctx
		ok       bool
	)

	if fiberCtx, ok = coreCtx.(*fiber.Ctx); !ok {
		return nil, fmt.Errorf("invalid core context type: expected *fiber.Ctx, got %T", fiberCtx)
	}

	return providerCtx.WithFiberContext(fiberCtx), nil
}
