package fiberhouse

import (
	"errors"
	"sync"
)

type CoreCtxPManager struct {
	IProviderManager
}

var (
	coreCtxPManager *CoreCtxPManager
	coreCtxOnce     sync.Once
)

func NewCoreCtxPManager(appCtx IApplicationContext) *CoreCtxPManager {
	return &CoreCtxPManager{
		IProviderManager: NewProviderManager(appCtx).SetType(ProviderTypeDefault().GroupCoreContextType),
	}
}

func NewCoreCtxPManagerOnce(ctx IApplicationContext) *CoreCtxPManager {
	coreCtxOnce.Do(func() {
		coreCtxPManager = NewCoreCtxPManager(ctx)
	})
	return coreCtxPManager
}

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
