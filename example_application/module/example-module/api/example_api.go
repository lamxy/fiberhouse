package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
)

type ExampleHandler struct {
	fiberhouse.ApiLocator
	UseCase service.ExampleUseCase
}

func NewExampleHandler(ctx fiberhouse.IApplicationContext, useCase service.ExampleUseCase) *ExampleHandler {
	return &ExampleHandler{
		ApiLocator: fiberhouse.NewApi(ctx).SetName(GetKeyExampleHandler()),
		UseCase:    useCase,
	}
}

func GetKeyExampleHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleHandler", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

func (h *ExampleHandler) Create(c *fiber.Ctx) error {
	var req requestvo.CreateExampleReqVo
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	resp, err := h.UseCase.Create(c.UserContext(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *ExampleHandler) Get(c *fiber.Ctx) error {
	resp, err := h.UseCase.Get(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *ExampleHandler) List(c *fiber.Ctx) error {
	var req requestvo.ListExamplesReqVo
	if err := c.QueryParser(&req); err != nil {
		return err
	}
	resp, err := h.UseCase.List(c.UserContext(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *ExampleHandler) Update(c *fiber.Ctx) error {
	var req requestvo.UpdateExampleReqVo
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	resp, err := h.UseCase.Update(c.UserContext(), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *ExampleHandler) Delete(c *fiber.Ctx) error {
	if err := h.UseCase.Delete(c.UserContext(), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
