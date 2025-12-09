package repository

import "github.com/google/wire"

var (
	ExampleRepoWireSet = wire.NewSet(NewExampleRepository)
)

func GetExampleRepoWireSet() wire.ProviderSet {
	return wire.NewSet(NewExampleRepository)
}
