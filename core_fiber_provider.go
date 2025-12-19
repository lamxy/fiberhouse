package fiberhouse

import (
	"fmt"
)

type CoreFiberProvider struct {
	IProvider
}

func NewCoreFiberProvider() *CoreFiberProvider {
	return &CoreFiberProvider{
		IProvider: NewProvider().SetName("CoreFiberProvider").SetTarget("fiber").SetType(ProviderTypeDefault().GroupCoreStarterChoose),
	}
}

func (p *CoreFiberProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	if len(initFunc) == 0 {
		return NewCoreWithFiber(ctx.(IApplicationContext)), nil
	}

	anything, err := initFunc[0](p) // 匿名函数参数获取核心启动器初始化的选项参数切片
	if err != nil {
		return nil, fmt.Errorf("CoreFiberProvider initialize failed: %w", err)
	}

	var (
		coreStarterOptions []CoreStarterOption
		ok                 bool
	)

	if coreStarterOptions, ok = anything.([]CoreStarterOption); ok {
		return NewCoreWithFiber(ctx.(IApplicationContext), coreStarterOptions...), nil
	}

	return anything, err
}
