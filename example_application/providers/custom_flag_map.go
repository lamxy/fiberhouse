package providers

import (
	"github.com/lamxy/fiberhouse"
)

type CustomFlagMapProvider struct {
	fiberhouse.IProvider
	custom map[string]string
}

func NewCustomFlagMapProvider() *CustomFlagMapProvider {
	return &CustomFlagMapProvider{
		IProvider: fiberhouse.NewProvider().SetName("custom_flag_map"),
	}
}

func (p *CustomFlagMapProvider) Initialize(ctx fiberhouse.IContext, initFn ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFn) > 0 {
		return initFn[0](p)
	}
	return nil, nil
}
