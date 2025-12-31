package middleware

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
)

// AppMiddlewarePManager 应用中间件提供者管理器
type AppMiddlewarePManager struct {
	fiberhouse.IProviderManager
}

func NewAppMiddlewarePManager(appCtx fiberhouse.IApplicationContext) *AppMiddlewarePManager {
	son := &AppMiddlewarePManager{
		IProviderManager: fiberhouse.NewProviderManager(appCtx).
			SetName("AppMiddlewarePManager").
			SetType(fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType).
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationAppMiddlewareInit, true), // 设置并绑定执行位置点
	}
	// 让子管理器挂载到父管理器上
	// 无需重载MountToParent方法，NewCoreStarterPManager()内已调基类挂载方法进行了挂载
	son.MountToParent(son)
	return son
}

// LoadProvider 重载加载提供者
func (m *AppMiddlewarePManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
	m.Check()
	if len(loadFunc) == 0 {
		return nil, fmt.Errorf("manager %s : load function is required", m.Name())
	}
	anything, err := loadFunc[0](m)
	if err != nil {
		return nil, err
	}

	var (
		cs fiberhouse.CoreStarter
		ok bool
	)

	if cs, ok = anything.(fiberhouse.CoreStarter); !ok {
		return nil, fmt.Errorf("manager %s : load function is required", m.Name())
	}

	bootCfg := m.GetContext().(fiberhouse.IApplicationContext).GetBootConfig()
	if bootCfg.CoreType == "" {
		return nil, fmt.Errorf("manager %s : load function is required", m.Name())
	}

	for _, provider := range m.List() {
		if provider.Target() == bootCfg.CoreType {
			_, err := provider.Initialize(m.GetContext(), func(provider fiberhouse.IProvider) (any, error) {
				return cs, nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

// MountToParent 重载挂载到父级提供者管理器
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *AppMiddlewarePManager) MountToParent(son ...fiberhouse.IProviderManager) fiberhouse.IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
