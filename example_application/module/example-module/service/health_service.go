package service

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
)

type HealthService struct {
	fiberhouse.ServiceLocator
	Resp *repository.HealthRepository
}

func NewHealthService(ctx fiberhouse.IApplicationContext, resp *repository.HealthRepository) *HealthService {
	name := GetKeyHealthService()
	return &HealthService{
		ServiceLocator: fiberhouse.NewService(ctx).SetName(name),
		Resp:           resp,
	}
}

func GetKeyHealthService(ns ...string) string {
	return fiberhouse.RegisterKeyName("HealthService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

func (s *HealthService) GetHealth() string {
	return s.Resp.Status
}
