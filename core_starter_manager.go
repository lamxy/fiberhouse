package fiberhouse

import (
	"errors"
	"fmt"
)

// CoreStarterPManager 核心启动器提供者管理器
type CoreStarterPManager struct {
	IProviderManager
}

func NewCoreStarterPManager(appCtx IApplicationContext) *CoreStarterPManager {
	son := &CoreStarterPManager{
		IProviderManager: NewProviderManager(appCtx).
			SetType(ProviderTypeDefault().GroupCoreStarterChoose).
			SetName("CoreStarterPManager").
			SetOrBindToLocation(ProviderLocationDefault().LocationCoreStarterCreate, true), // 设置并绑定执行位置点
	}
	// 让子管理器挂载到父管理器上
	// 无需重载MountToParent方法，NewCoreStarterPManager()内已调基类挂载方法进行了挂载
	son.MountToParent(son)
	return son
}

// LoadProvider 重载加载提供者
func (m *CoreStarterPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.Check()
	if len(loadFunc) == 0 {
		return nil, fmt.Errorf("manager %s : load function is required", m.Name())
	}
	anything, err := loadFunc[0](m)
	if err != nil {
		return nil, err
	}

	var (
		coreStarterOpts []CoreStarterOption
		ok              bool
	)

	if coreStarterOpts, ok = anything.([]CoreStarterOption); !ok {
		return nil, errors.New("load function must return []CoreStarterOption")
	}

	bootCfg := m.GetContext().(IApplicationContext).GetBootConfig()
	defaultCore := bootCfg.CoreType
	if defaultCore == "" {
		defaultCore = "fiber"
	}

	for _, provider := range m.List() {
		if provider.Target() == defaultCore {
			return provider.Initialize(m.GetContext(), func(provider IProvider) (any, error) {
				return coreStarterOpts, nil
			})
		}
	}
	return nil, errors.New("no matching core starter provider found")
}

// MountToParent 重载挂载到父级提供者管理器
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *CoreStarterPManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
