package optioninit

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
	// 设置的创建CoreStarter所需的选项，如有
	coreOpts := []fiberhouse.CoreStarterOption{} // 空的选项
	return coreOpts, nil
}
