package fiberhouse

import (
	"errors"
	"fmt"
)

type FrameDefaultProvider struct {
	IProvider
}

func NewFrameDefaultProvider() *FrameDefaultProvider {
	return &FrameDefaultProvider{
		IProvider: NewProvider().SetName("FrameDefaultProvider").SetTarget("default").SetType(ProviderTypeDefault().GroupFrameStarterChoose),
	}
}

func (p *FrameDefaultProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	p.Check()
	if len(initFunc) == 0 {
		return nil, errors.New("no initFunc provided")
	}

	// 通过 initFunc 获取外部传递的 FrameStarterOption列表
	anything, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}

	var (
		opts []FrameStarterOption
		ok   bool
	)

	if opts, ok = anything.([]FrameStarterOption); !ok {
		return nil, fmt.Errorf("invalid type %T, []FrameStarterOption expected", anything)
	}

	// 创建 FrameApplication 实例
	return NewFrameApplication(ctx.(IApplicationContext), opts...), nil
}
