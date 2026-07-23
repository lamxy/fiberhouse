package module

import (
	"fmt"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
)

type GinRouteRegisterProvider struct {
	fiberhouse.IProvider
}

func NewGinRouteRegisterProvider() *GinRouteRegisterProvider {
	son := &GinRouteRegisterProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("GinRouteRegisterProvider").
			SetTarget(constant.CoreTypeWithGin).
			SetType(fiberhouse.ProviderTypeDefault().GroupRouteRegisterType),
	}
	son.MountToParent(son)
	return son
}

func (p *GinRouteRegisterProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if !p.Check() {
		return p.ReturnDirectly()
	}

	if len(initFunc) == 0 {
		return p.SetAndReturnFailedInitialized(nil, fmt.Errorf("provider '%s': no initFunc provided", p.Name()))
		//return nil, fmt.Errorf("provider '%s': no initFunc provided", p.Name())
	}

	// 注入核心启动器实例
	instance, err := initFunc[0](p)
	if err != nil {
		return p.SetAndReturnFailedInitialized(nil, err)
		//return nil, err
	}
	cs, ok := instance.(fiberhouse.CoreStarter)
	if !ok {
		return p.SetAndReturnFailedInitialized(nil, fmt.Errorf("provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name()))
	}

	// 注册路由
	RegisterGinRouteHandlers(ctx.(fiberhouse.IApplicationContext), cs)

	// 注册 Swagger 路由
	RegisterGinSwagger(ctx.(fiberhouse.IApplicationContext), cs)

	return p.SetAndReturnSucceededInitialized(nil, nil)
}
