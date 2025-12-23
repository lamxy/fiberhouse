package fiberhouse

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

// CtxFiberProvider Fiber 框架核心上下文提供者
type CtxFiberProvider struct {
	IProvider
}

func NewCtxFiberProvider() *CtxFiberProvider {
	son := &CtxFiberProvider{
		IProvider: NewProvider().SetName("CtxFiberProvider").SetTarget("fiber").SetType(ProviderTypeDefault().GroupCoreContextChoose),
	}
	son.MountToParent(son)
	return son
}

// Initialize 初始化 Fiber 框架核心上下文提供者
func (p *CtxFiberProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
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

	return providerCtx.WithFiberContext(fiberCtx), nil
}
