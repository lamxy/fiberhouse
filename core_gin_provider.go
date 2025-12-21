package fiberhouse

import (
	"fmt"
)

// CoreGinProvider 核心Gin提供者
type CoreGinProvider struct {
	IProvider
}

func NewCoreGinProvider() *CoreGinProvider {
	return &CoreGinProvider{
		IProvider: NewProvider().SetName("CoreGinProvider").SetTarget("gin").SetType(ProviderTypeDefault().GroupCoreStarterChoose),
	}
}

// Initialize 重载初始化核心Gin提供者
func (p *CoreGinProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
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
		return NewCoreWithGin(ctx.(IApplicationContext), coreStarterOptions...), nil
	}

	return anything, err
}
