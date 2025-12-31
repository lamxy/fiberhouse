package middleware

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
)

// FiberModuleMiddlewareProvider 应用级中间件提供者
// 用于注册全局应用中间件，如日志、恢复、跨域等
type FiberModuleMiddlewareProvider struct {
	fiberhouse.IProvider
}

// NewFiberModuleMiddlewareProvider 创建应用级中间件提供者实例
func NewFiberModuleMiddlewareProvider() *FiberModuleMiddlewareProvider {
	p := &FiberModuleMiddlewareProvider{
		IProvider: fiberhouse.NewProvider().SetName("FiberModuleMiddlewareProvider").
			SetTarget(constant.CoreTypeWithFiber).
			SetType(fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType),
	}
	// 挂载子类实例到父类，确保多态行为正确
	p.MountToParent(p)
	return p
}

// Initialize 初始化应用级中间件
func (p *FiberModuleMiddlewareProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	p.Check()

	if len(initFunc) == 0 {
		return nil, fmt.Errorf("Provider '%s': initFunc must not be empty", p.Name())
	}

	instance, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}
	var (
		cs fiberhouse.CoreStarter
		ok bool
	)

	if cs, ok = instance.(fiberhouse.CoreStarter); !ok {
		return nil, fmt.Errorf("Provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name())
	}

	// 注册模块级别的中间件
	RegisterModuleMiddleware(cs)

	// 设置提供者状态为已加载
	p.SetStatus(fiberhouse.StateLoaded)

	return nil, nil
}
