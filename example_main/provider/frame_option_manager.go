package provider

import "github.com/lamxy/fiberhouse"

type FrameOptionInitManager struct {
	fiberhouse.IProviderManager
}

func NewFrameOptionInitManager(ctx fiberhouse.IContext) *FrameOptionInitManager {
	return &FrameOptionInitManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx).
			SetName("FrameOptionManager").
			SetType(fiberhouse.ProviderTypeDefault().GroupProviderAutoRun).
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationFrameStarterOptionInit, true),
	}
}

func (m *FrameOptionInitManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
	m.Check()
	return m.List()[0].Initialize(m.GetContext().(fiberhouse.IApplicationContext))
}
