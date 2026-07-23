package repository

import "github.com/google/wire"
import "github.com/lamxy/fiberhouse/example_application/module/example-module/model"

var (
	ExampleRepoWireSet = wire.NewSet(
		NewExampleRepository,
		wire.Bind(new(ExampleModelStore), new(*model.ExampleModel)),
	)
)

func GetExampleRepoWireSet() wire.ProviderSet {
	return ExampleRepoWireSet
}
