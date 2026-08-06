// Package providers 提供 hertz 核心引擎所需的各類提供者。
//
// 這些提供者全部宣告 Target() == "hertz"，由框架既有的管理器依
// BootConfig.CoreType 自動選中，無需修改框架任何程式碼。
package providers

import (
	"fmt"

	"github.com/lamxy/fiberhouse"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
	hertzstarter "github.com/lamxy/fiberhouse/example_application/hertzcore/starter"
)

// CoreStarterHertzProvider 核心 hertz 啟動器提供者，
// 由框架的 CoreStarterPManager 依 CoreType 選中
type CoreStarterHertzProvider struct {
	fiberhouse.IProvider
}

// NewCoreStarterHertzProvider 建立核心 hertz 啟動器提供者
func NewCoreStarterHertzProvider() *CoreStarterHertzProvider {
	return &CoreStarterHertzProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("CoreHertzProvider").
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupCoreStarterChoose),
	}
}

// Initialize 建立 hertz 核心啟動器實例
//
// 行為對齊框架的 CoreStarterFiberProvider：無 initFunc 時以預設選項建立，
// 有 initFunc 時取出 []CoreStarterOption 傳入。
func (p *CoreStarterHertzProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	appCtx := ctx.(fiberhouse.IApplicationContext)
	if len(initFunc) == 0 {
		return hertzstarter.NewCoreWithHertz(appCtx), nil
	}

	anything, err := initFunc[0](p)
	if err != nil {
		return nil, fmt.Errorf("CoreHertzProvider initialize failed: %w", err)
	}

	if opts, ok := anything.([]fiberhouse.CoreStarterOption); ok {
		return hertzstarter.NewCoreWithHertz(appCtx, opts...), nil
	}
	return anything, nil
}
