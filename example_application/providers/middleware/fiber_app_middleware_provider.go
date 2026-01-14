package middleware

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
)

// FiberAppMiddlewareProvider 应用级中间件提供者
// 用于注册全局应用中间件，如日志、恢复、跨域等
type FiberAppMiddlewareProvider struct {
	fiberhouse.IProvider
}

// NewFiberAppMiddlewareProvider 创建应用级中间件提供者实例
func NewFiberAppMiddlewareProvider() *FiberAppMiddlewareProvider {
	p := &FiberAppMiddlewareProvider{
		IProvider: fiberhouse.NewProvider().SetName("FiberAppMiddlewareProvider").
			SetTarget(constant.CoreTypeWithFiber).
			SetType(fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType),
	}
	// 挂载子类实例到父类，确保多态行为正确
	p.MountToParent(p)
	return p
}

// Initialize 初始化应用级中间件
func (p *FiberAppMiddlewareProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
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

	// 注册应用全局中间件
	RegisterAppMiddleware(ctx.(fiberhouse.IApplicationContext), cs)

	// 设置提供者状态为已加载
	p.SetStatus(fiberhouse.StateLoaded)

	return nil, nil
}
