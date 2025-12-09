package service

import (
	"github.com/google/wire"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
)

// wire: a call to wire.Build indicates that this function is an injector, but injectors must consist of only the wire.Build call and an optional return
var (
	ExampleServiceWireSet = wire.NewSet(NewExampleService)
	HealthServiceProvide  = wire.NewSet(NewHealthService, repository.NewHealthRepository)
)
