package model

import "github.com/google/wire"

var (
	ExampleModelWireSet = wire.NewSet(NewExampleModel)
)

func GetExampleModelWireSet() wire.ProviderSet {
	return wire.NewSet(NewExampleModel)
}
