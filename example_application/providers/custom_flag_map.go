package providers

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/provider"
)

type CustomFlagMapProvider struct {
	provider.IProvider
	custom map[string]string
}

func NewCustomFlagMapProvider() *CustomFlagMapProvider {
	return &CustomFlagMapProvider{
		IProvider: provider.NewProvider().SetName("custom_flag_map"),
	}
}

func (p *CustomFlagMapProvider) Initialize(ctx fiberhouse.IContext, initFn ...provider.InitFunc) error {
	// TODO
	return nil
}
