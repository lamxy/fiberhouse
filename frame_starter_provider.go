package fiberhouse

import (
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse/constant"
)

// FrameDefaultProvider 框架默认(框架启动器)提供者
type FrameDefaultProvider struct {
	IProvider
}

func NewFrameDefaultProvider() *FrameDefaultProvider {
	son := &FrameDefaultProvider{
		IProvider: NewProvider().
			SetName("FrameDefaultProvider").
			SetTarget(constant.FrameTypeWithDefaultFrameStarter).
			SetType(ProviderTypeDefault().GroupFrameStarterChoose),
	}
	// 将子提供者挂载到父提供者上
	son.MountToParent(son)
	return son
}

// Initialize 重载初始化框架默认提供者
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

// MountToParent 重载挂载到父级提供者
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (p *FrameDefaultProvider) MountToParent(son ...IProvider) IProvider {
	if len(son) > 0 {
		p.IProvider.MountToParent(son[0])
		return p
	}
	p.IProvider.MountToParent(p)
	return p
}
