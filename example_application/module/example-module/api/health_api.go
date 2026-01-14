package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/lamxy/fiberhouse/response"
)

type HealthHandler struct {
	fiberhouse.ApiLocator
	Service *service.HealthService
}

func NewHealthHandler(ctx fiberhouse.IApplicationContext, serv *service.HealthService) *HealthHandler {
	name := GetKeyHealthHandler()
	return &HealthHandler{
		ApiLocator: fiberhouse.NewApi(ctx).SetName(name),
		Service:    serv,
	}
}

func GetKeyHealthHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("HealthHandler", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

func (ha *HealthHandler) Liveness(c *fiber.Ctx) error {
	result := ha.Service.GetHealth()
	return c.Status(fiber.StatusOK).JSON(response.RespSuccess(result))
}
