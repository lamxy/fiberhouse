package jsoncodec

import (
	"errors"
	"github.com/lamxy/fiberhouse"
)

// JsonCodecPManager JSON 编解码提供者管理器
type JsonCodecPManager struct {
	fiberhouse.IProviderManager
}

// NewJsonCodecPManager 创建一个新的 JSON 编解码管理器
func NewJsonCodecPManager(ctx fiberhouse.IApplicationContext) *JsonCodecPManager {
	return &JsonCodecPManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx).
			SetName("JsonCodecPManager").
			SetType(fiberhouse.ProviderTypeDefault().GroupJsonCodecChoose).
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationCoreEngineInit, true), // 设置并绑定到核心引擎初始化位置点
	}
}

func (m *JsonCodecPManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
	m.Check()
	if len(loadFunc) > 0 {
		return loadFunc[0](m)
	}
	if len(m.List()) == 0 {
		return nil, errors.New("no json codec provider found")
	}
	var (
		finalProvider fiberhouse.IProvider
		bootCfg       = m.GetContext().(fiberhouse.IApplicationContext).GetBootConfig()
	)

	for _, provider := range m.List() {
		if provider.Type().GetTypeID() == fiberhouse.ProviderTypeDefault().GroupJsonCodecChoose.GetTypeID() &&
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
