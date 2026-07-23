package api

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	moduleconstant "github.com/lamxy/fiberhouse/example_application/module/constant"
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
	return fiberhouse.RegisterKeyName("ExampleHandler", fiberhouse.GetNamespace([]string{moduleconstant.NameModuleExample}, ns...)...)
}

func (h *ExampleHandler) validate(value interface{}, lang string) error {
	vw := h.GetContext().GetValidateWrap()
	if err := vw.GetValidate(lang).Struct(value); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			return vw.Errors(validationErrors, lang, true)
		}
		return err
	}
	return nil
}

func (h *ExampleHandler) validateID(id, lang string) error {
	return h.validate(&requestvo.ObjId{ID: id}, lang)
}

func (h *ExampleHandler) Create(c *fiber.Ctx) error {
	var req requestvo.CreateExampleReqVo
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	lang := c.Get(moduleconstant.XLanguageFlag, "en")
	if err := h.validate(&req, lang); err != nil {
		return err
	}
	resp, err := h.UseCase.Create(c.UserContext(), req)
	if err != nil {
		return err
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusCreated)
}

func (h *ExampleHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	lang := c.Get(moduleconstant.XLanguageFlag, "en")
	if err := h.validateID(id, lang); err != nil {
		return err
	}
	resp, err := h.UseCase.Get(c.UserContext(), id)
	if err != nil {
		return err
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusOK)
}

func (h *ExampleHandler) List(c *fiber.Ctx) error {
	var req requestvo.ListExamplesReqVo
	if err := c.QueryParser(&req); err != nil {
		return err
	}
	lang := c.Get(moduleconstant.XLanguageFlag, "en")
	if err := h.validate(&req, lang); err != nil {
		return err
	}
	resp, err := h.UseCase.List(c.UserContext(), req)
	if err != nil {
		return err
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusOK)
}

func (h *ExampleHandler) Update(c *fiber.Ctx) error {
	var req requestvo.UpdateExampleReqVo
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	id := c.Params("id")
	lang := c.Get(moduleconstant.XLanguageFlag, "en")
	if err := h.validate(&req, lang); err != nil {
		return err
	}
	if err := h.validateID(id, lang); err != nil {
		return err
	}
	resp, err := h.UseCase.Update(c.UserContext(), id, req)
	if err != nil {
		return err
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusOK)
}

func (h *ExampleHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	lang := c.Get(moduleconstant.XLanguageFlag, "en")
	if err := h.validateID(id, lang); err != nil {
		return err
	}
	if err := h.UseCase.Delete(c.UserContext(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
