package providers

import (
	"github.com/lamxy/fiberhouse"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
	hertzrecovery "github.com/lamxy/fiberhouse/example_application/hertzcore/recovery"
)

// HertzRecoveryProvider hertz 恢復中間件提供者，
// 由框架的 RecoveryPManager 依 CoreType 選中
type HertzRecoveryProvider struct {
	fiberhouse.IProvider
}

// NewHertzRecoveryProvider 建立 hertz 恢復提供者
func NewHertzRecoveryProvider() *HertzRecoveryProvider {
	p := &HertzRecoveryProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("HertzRecoveryProvider").
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupRecoverMiddlewareChoose),
	}
	p.MountToParent(p)
	return p
}

// Initialize 建立 hertz 恢復實例
func (p *HertzRecoveryProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	return hertzrecovery.New(ctx.(fiberhouse.IApplicationContext)), nil
}
