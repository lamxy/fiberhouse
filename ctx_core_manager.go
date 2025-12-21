package fiberhouse

import (
	"errors"
	"sync"
)

// CoreCtxPManager 核心上下文提供者管理器
type CoreCtxPManager struct {
	IProviderManager
}

var (
	coreCtxPManager *CoreCtxPManager
	coreCtxOnce     sync.Once
)

// NewCoreCtxPManager 创建核心上下文提供者管理器
func NewCoreCtxPManager(appCtx IApplicationContext) *CoreCtxPManager {
	return &CoreCtxPManager{
		IProviderManager: NewProviderManager(appCtx).SetType(ProviderTypeDefault().GroupCoreContextChoose),
	}
}

// NewCoreCtxPManagerOnce 单例模式创建核心上下文提供者管理器
func NewCoreCtxPManagerOnce(ctx IApplicationContext) *CoreCtxPManager {
	coreCtxOnce.Do(func() {
		coreCtxPManager = NewCoreCtxPManager(ctx)
	})
	return coreCtxPManager
}

// LoadProvider 重载加载提供者
func (m *CoreCtxPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.Check()
	if len(loadFunc) == 0 {
		return nil, errors.New("load function is required")
	}
	coreCtx, err := loadFunc[0](m)
	if err != nil {
		return nil, err
	}
	bootCfg := m.GetContext().(IApplicationContext).GetBootConfig()
	for _, provider := range m.List() {
		if provider.Target() == bootCfg.CoreType {
			return provider.Initialize(m.GetContext(), func(provider IProvider) (any, error) {
				return coreCtx, nil
			})
		}
	}
	return nil, errors.New("no core context provider found")
}

// MountToParent 重载挂载到父级提供者管理器
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *CoreCtxPManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
