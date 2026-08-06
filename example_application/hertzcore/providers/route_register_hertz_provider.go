package providers

import (
	"fmt"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/lamxy/fiberhouse"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
	exampleHertzApi "github.com/lamxy/fiberhouse/example_application/module/example-hertzapi-module/api"
)

// HertzRouteRegisterProvider 基於 hertz 的模組路由與 swagger 註冊提供者
type HertzRouteRegisterProvider struct {
	fiberhouse.IProvider
}

// NewHertzRouteRegisterProvider 建立 hertz 路由註冊提供者
func NewHertzRouteRegisterProvider() *HertzRouteRegisterProvider {
	son := &HertzRouteRegisterProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("HertzRouteRegisterProvider").
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupRouteRegisterType),
	}
	son.MountToParent(son)
	return son
}

// Initialize 註冊 hertz 路由處理器
func (p *HertzRouteRegisterProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': no initFunc provided", p.Name())
	}

	instance, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}

	cs, ok := instance.(fiberhouse.CoreStarter)
	if !ok {
		return nil, fmt.Errorf("provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name())
	}

	RegisterHertzRouteHandlers(ctx.(fiberhouse.IApplicationContext), cs)
	return nil, nil
}

// RegisterHertzRouteHandlers 註冊各業務模組的 hertz 路由處理器
func RegisterHertzRouteHandlers(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	h, ok := cs.GetCoreApp().(*server.Hertz)
	if !ok {
		return
	}

	// 註冊 example 模組的路由處理器
	exampleHertzApi.RegisterRouteHandlers(ctx, h)

	// TODO 註冊更多業務模組路由處理器 ...
}
