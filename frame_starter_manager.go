package fiberhouse

import (
	"errors"
	"fmt"
)

// FrameDefaultPManager 框架默认(框架启动器)提供者管理器
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

// LoadProvider 重载加载提供者
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
		defaultFrame = "defaultFrame"
	}

	for _, provider := range m.List() {
		if provider.Target() == defaultFrame {
			return provider.Initialize(m.GetContext(), func(provider IProvider) (any, error) {
				return frameStarterOpts, nil
			})
		}
	}
	return nil, fmt.Errorf("no matching frame starter's target '%s' provider found", defaultFrame)
}

// MountToParent 重载挂载到父级提供者管理器
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *FrameDefaultPManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
