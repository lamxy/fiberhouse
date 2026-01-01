package apphook

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
)

type FiberAppHookProvider struct {
	fiberhouse.IProvider
}

func NewFiberAppHookProvider() *FiberAppHookProvider {
	son := &FiberAppHookProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("FiberAppHookProvider").
			SetTarget(constant.CoreTypeWithFiber).
			SetType(fiberhouse.ProviderTypeDefault().GroupCoreHookChoose),
	}
	son.MountToParent(son)
	return son
}

func (p *FiberAppHookProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': initFunc must not be empty", p.Name())
	}

	// 接受传入核心启动器参数
	instance, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}
	var (
		cs fiberhouse.CoreStarter
		ok bool
	)

	if cs, ok = instance.(fiberhouse.CoreStarter); !ok {
		return nil, fmt.Errorf("provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name())
	}

	RegisterFiberAppCoreHook(ctx.(fiberhouse.IApplicationContext), cs)

	p.SetStatus(fiberhouse.StateLoaded)

	return nil, nil
}
