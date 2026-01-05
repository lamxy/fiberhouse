package module

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
)

type FiberRouteRegisterProvider struct {
	fiberhouse.IProvider
}

func NewFiberRouteRegisterProvider() *FiberRouteRegisterProvider {
	son := &FiberRouteRegisterProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("FiberRouteRegisterProvider").
			SetTarget(constant.CoreTypeWithFiber).
			SetType(fiberhouse.ProviderTypeDefault().GroupRouteRegisterType),
	}
	son.MountToParent(son)
	return son
}

func (p *FiberRouteRegisterProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': no initFunc provided", p.Name())
	}

	// 注入核心启动器实例
	instance, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}
	cs, ok := instance.(fiberhouse.CoreStarter)
	if !ok {
		return nil, fmt.Errorf("provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name())
	}

	// 注册路由
	RegisterFiberRouteHandlers(ctx.(fiberhouse.IApplicationContext), cs)

	// 注册 Swagger 路由
	RegisterFiberSwagger(ctx.(fiberhouse.IApplicationContext), cs)

	// 设置提供者状态为已加载
	p.SetStatus(fiberhouse.StateLoaded)

	return nil, nil
}
