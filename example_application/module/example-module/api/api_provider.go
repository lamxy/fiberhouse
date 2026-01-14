//go:build wireinject
// +build wireinject

package api

import (
	"github.com/google/wire"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/model"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
)

// xxxWireSet 表示当前层级的NewXxx构造器集合，需要跟下级层级的依赖组合
// xxxProvide表示为当前层级包含当前以下层级依赖组合的完整provide，用来跟当前层级及上级层级组合依赖

func InjectExampleApi(ctx fiberhouse.IApplicationContext) (*ExampleHandler, error) {
	wire.Build(NewExampleHandler, service.ExampleServiceWireSet, repository.ExampleRepoWireSet, model.ExampleModelWireSet)
	return nil, nil
}

func InjectHealthApi(ctx fiberhouse.IApplicationContext) (*HealthHandler, error) {
	wire.Build(NewHealthHandler, service.HealthServiceProvide)
	return nil, nil
}
