package fiberhouse

import (
	"errors"
	"fmt"
)

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
	son.MountToParent(son)
	return son
}

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
