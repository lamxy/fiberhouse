package fiberhouse

import (
	"errors"
)

type FrameDefaultPManager struct {
	IProviderManager
}

func NewFrameDefaultPManager(appCtx IApplicationContext) *FrameDefaultPManager {
	son := &FrameDefaultPManager{
		IProviderManager: NewProviderManager(appCtx).
			SetName("FrameDefaultPManager").
			SetType(ProviderTypeDefault().GroupFrameStarterChoose).
			SetOrBindToLocation(ProviderLocationDefault().LocationFrameStarterCreate, true), // 设置并绑定到 FrameStarterCreate 创建位置点
	}
	// 挂载子实例到父实例的sonManager字段，用上述继承的父类SetOrBindToLocation绑定子类实例到执行位置点
	son.MountToParent(son)
	return son
}

func (m *FrameDefaultPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.Check()
	if len(loadFunc) == 0 {
		return nil, errors.New("load function is required")
	}
	anything, err := loadFunc[0](m)
	if err != nil {
		return nil, err
	}

	var (
		frameStarterOpts []FrameStarterOption
		ok               bool
	)

	if frameStarterOpts, ok = anything.([]FrameStarterOption); !ok {
		return nil, errors.New("load function must return []FrameStarterOption")
	}

	bootCfg := m.GetContext().(IApplicationContext).GetBootConfig()
	defaultFrame := bootCfg.FrameType
	if defaultFrame == "" {
		defaultFrame = "default"
	}

	for _, provider := range m.List() {
		if provider.Target() == defaultFrame {
			return provider.Initialize(m.GetContext(), func(provider IProvider) (any, error) {
				return frameStarterOpts, nil
			})
		}
	}
	return nil, errors.New("no matching frame starter provider found")
}
