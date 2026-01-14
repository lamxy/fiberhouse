package module

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
)

type RouteRegisterPManager struct {
	fiberhouse.IProviderManager
}

func NewRouteRegisterPManager(ctx fiberhouse.IContext) *RouteRegisterPManager {
	son := &RouteRegisterPManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx).
			SetName("FiberRouteRegisterProvider").
			SetType(fiberhouse.ProviderTypeDefault().GroupRouteRegisterType).
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationRouteRegisterInit, true),
	}
	son.MountToParent(son)
	return son
}

func (m *RouteRegisterPManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
	if len(loadFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': no provider loadFunc", m.Name())
	}

	// 获取注入的核心启动器实例
	instance, err := loadFunc[0](m)
	if err != nil {
		return nil, err
	}

	if len(m.List()) == 0 {
		return nil, fmt.Errorf("provider '%s': no provider list", m.Name())
	}

	// 从启动配置获取核心类型
	coreType := m.GetContext().(fiberhouse.IApplicationContext).GetBootConfig().CoreType
	if coreType == "" {
		return nil, fmt.Errorf("provider '%s': core type is empty", m.Name())
	}

	// 遍历提供者列表，找到匹配的核心类型、且属于GroupRouteRegisterType的提供者并初始化
	for _, provider := range m.List() {
		if provider.Target() == coreType &&
			provider.Type() == fiberhouse.ProviderTypeDefault().GroupRouteRegisterType {
			_, err := provider.Initialize(m.GetContext(), func(provider fiberhouse.IProvider) (any, error) {
				return instance, nil // 注入核心启动器实例
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return nil, nil
}
