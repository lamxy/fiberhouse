package fiberhouse

import (
	"errors"
)

// JsonCodecPManager JSON 编解码提供者管理器
type JsonCodecPManager struct {
	IProviderManager
}

// NewJsonCodecPManager 创建一个新的 JSON 编解码管理器
func NewJsonCodecPManager(ctx IApplicationContext) *JsonCodecPManager {
	son := &JsonCodecPManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("JsonCodecPManager").
			SetType(ProviderTypeDefault().GroupJsonCodecChoose).
			SetOrBindToLocation(ProviderLocationDefault().LocationCoreEngineInit, true), // 设置并绑定到核心引擎初始化位置点
	}
	// 将子管理器挂载到父管理器
	son.MountToParent(son)
	return son
}

// LoadProvider 重载加载提供者
func (m *JsonCodecPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.Check()
	if len(loadFunc) > 0 {
		return loadFunc[0](m)
	}
	if len(m.List()) == 0 {
		return nil, errors.New("no json codec provider found")
	}
	var (
		finalProvider IProvider
		bootCfg       = m.GetContext().(IApplicationContext).GetBootConfig()
	)

	for _, provider := range m.List() {
		if provider.Type().GetTypeID() == ProviderTypeDefault().GroupJsonCodecChoose.GetTypeID() &&
			provider.Name() == bootCfg.JsonCodec &&
			provider.Target() == bootCfg.CoreType {
			finalProvider = provider
			break
		}
	}
	if finalProvider == nil {
		return nil, errors.New("no matching json codec provider found")
	}
	return finalProvider.Initialize(m.GetContext())
}

// MountToParent 重载挂载到父级提供者管理器
// 注意: 该方法的重载实现不是必须的，当NewXXX()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *JsonCodecPManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
