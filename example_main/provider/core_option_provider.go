package provider

import (
	"github.com/lamxy/fiberhouse"
)

// CoreOptionInitProvider 核心启动器选项初始化提供者
type CoreOptionInitProvider struct {
	fiberhouse.IProvider
}

func NewCoreOptionInitProvider() *CoreOptionInitProvider {
	return &CoreOptionInitProvider{
		IProvider: fiberhouse.NewProvider().SetName("CoreOptionInitProvider").SetTarget("fiber").SetType(fiberhouse.ProviderTypeDefault().GroupCoreStarterOptsInitUnique),
	}
}

func (p *CoreOptionInitProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	// TODO 设置的创建CoreStarter所需的选项
	coreOpts := []fiberhouse.CoreStarterOption{} // 空的选项
	return coreOpts, nil
}
