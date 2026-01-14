package repository

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
)

type HealthRepository struct {
	fiberhouse.RepositoryLocator
	Status string
}

func NewHealthRepository(ctx fiberhouse.IApplicationContext) *HealthRepository {
	return &HealthRepository{
		RepositoryLocator: fiberhouse.NewRepository(ctx).SetName(GetKeyHealthRepository()),
		Status:            "Health is OK",
	}
}

func GetKeyHealthRepository(ns ...string) string {
	return fiberhouse.RegisterKeyName("HealthRepository", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

func RegisterKeyHealthRepository(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyHealthRepository(ns...), func() (interface{}, error) {
		return NewHealthRepository(ctx), nil
	})
}

func (h *HealthRepository) Test() error {
	return nil
}
