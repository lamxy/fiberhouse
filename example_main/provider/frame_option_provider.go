package provider

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application"
	"github.com/lamxy/fiberhouse/example_application/module"
	"github.com/lamxy/fiberhouse/option"
)

type FrameOptionInitProvider struct {
	fiberhouse.IProvider
}

func NewFrameOptionInitProvider() *FrameOptionInitProvider {
	return &FrameOptionInitProvider{
		IProvider: fiberhouse.NewProvider().SetName("RegisterInitProvider").SetTarget("fiber").SetType(fiberhouse.ProviderTypeDefault().GroupProviderAutoRun),
	}
}

func (p *FrameOptionInitProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	// 初始化应用注册器、模块/子系统注册器和任务注册器对象，注入到框架启动器
	appCtx := ctx.(fiberhouse.IApplicationContext)
	appRegister := example_application.NewApplication(appCtx)
	moduleRegister := module.NewModule(appCtx)
	taskRegister := module.NewTaskAsync(appCtx)

	// 返回框架选项初始化函数
	frameOpts := []fiberhouse.FrameStarterOption{
		option.WithAppRegister(appRegister),
		option.WithModuleRegister(moduleRegister),
		option.WithTaskRegister(taskRegister),
	}
	return frameOpts, nil
}
