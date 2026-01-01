package apphook

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
)

type AppCoreHookPManager struct {
	fiberhouse.IProviderManager
}

func NewAppCoreHookPManager(ctx fiberhouse.IApplicationContext) *AppCoreHookPManager {
	son := &AppCoreHookPManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx).
			SetName("AppCoreHookPManager").
			SetType(fiberhouse.ProviderTypeDefault().GroupCoreHookChoose).
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationCoreHookInit, true),
	}
	son.MountToParent(son)
	return son
}

func (m *AppCoreHookPManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
	if len(loadFunc) == 0 {
		return nil, fmt.Errorf("provider loadFunc must not be empty")
	}
	instance, err := m.IProviderManager.LoadProvider(loadFunc[0])
	if err != nil {
		return nil, err
	}
	coreType := m.GetContext().(fiberhouse.IApplicationContext).GetBootConfig().CoreType
	for _, provider := range m.List() {
		if provider.Target() == coreType &&
			provider.Type().GetTypeID() == fiberhouse.ProviderTypeDefault().GroupCoreHookChoose.GetTypeID() {
			_, err := provider.Initialize(m.GetContext(), func(provider fiberhouse.IProvider) (any, error) {
				return instance, nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}
