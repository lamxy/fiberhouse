package module

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
)

type GinSwaggerRegisterProvider struct {
	fiberhouse.IProvider
}

func NewGinSwaggerRegisterProvider() *GinSwaggerRegisterProvider {
	son := &GinSwaggerRegisterProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("GinSwaggerRegisterProvider").
			SetTarget(constant.CoreTypeWithGin).
			SetType(fiberhouse.ProviderTypeDefault().GroupRouteRegisterType), // 跟路由注册器类型一起完成注册
	}
	son.MountToParent(son)
	return son
}

func (p *GinSwaggerRegisterProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': no initFunc provided", p.Name())
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

	// 注册 Swagger 路由
	RegisterGinSwagger(ctx.(fiberhouse.IApplicationContext), cs)

	p.SetStatus(fiberhouse.StateLoaded)

	return nil, nil
}
