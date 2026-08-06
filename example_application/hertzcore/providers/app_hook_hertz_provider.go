package providers

import (
	ctxpkg "context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/lamxy/fiberhouse"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
)

// HertzAppHookProvider 基於 hertz 的生命週期鉤子提供者
type HertzAppHookProvider struct {
	fiberhouse.IProvider
}

// NewHertzAppHookProvider 建立 hertz 生命週期鉤子提供者
func NewHertzAppHookProvider() *HertzAppHookProvider {
	son := &HertzAppHookProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("HertzAppHookProvider").
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupCoreHookChoose),
	}
	son.MountToParent(son)
	return son
}

// Initialize 註冊 hertz 生命週期鉤子
func (p *HertzAppHookProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': initFunc must not be empty", p.Name())
	}

	instance, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}

	cs, ok := instance.(fiberhouse.CoreStarter)
	if !ok {
		return nil, fmt.Errorf("provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name())
	}

	RegisterHertzAppCoreHook(ctx.(fiberhouse.IApplicationContext), cs)
	return nil, nil
}

// RegisterHertzAppCoreHook 註冊應用自定義的 hertz 生命週期鉤子
func RegisterHertzAppCoreHook(appCtx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	h, ok := cs.GetCoreApp().(*server.Hertz)
	if !ok {
		return
	}

	h.OnRun = append(h.OnRun, func(c ctxpkg.Context) error {
		appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).
			Str("ApplicationRegister", "Application").
			Msg("ApplicationRegister OnRun...")
		return nil
	})

	h.OnShutdown = append(h.OnShutdown, func(c ctxpkg.Context) {
		appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).
			Str("ApplicationRegister", "Application").
			Msg("ApplicationRegister OnShutdown...")
	})
}
