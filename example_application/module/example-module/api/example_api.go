// Package api is the transport layer for the example module: it exposes
// HTTP handlers that parse and validate requests, translate them into
// service calls, and map service errors onto HTTP responses. It depends on
// service (and, through transport, repository) but must not reach past the
// service layer into repository or model directly.
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
	"github.com/lamxy/fiberhouse/example_application/module/example-module/transport"
)

// ExampleHandler is the Fiber transport for the example resource. It only
// parses/validates requests and delegates all business logic to UseCase.
type ExampleHandler struct {
	fiberhouse.ApiLocator
	UseCase service.ExampleUseCase
}

// NewExampleHandler builds an ExampleHandler bound to the given application
// context and use case, registering it under GetKeyExampleHandler.
func NewExampleHandler(ctx fiberhouse.IApplicationContext, useCase service.ExampleUseCase) *ExampleHandler {
	return &ExampleHandler{
		ApiLocator: fiberhouse.NewApi(ctx).SetName(GetKeyExampleHandler()),
		UseCase:    useCase,
	}
}

// GetKeyExampleHandler returns the registry key used to locate the
// ExampleHandler instance, optionally namespaced by ns.
func GetKeyExampleHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleHandler", fiberhouse.GetNamespace([]string{moduleconstant.NameModuleExample}, ns...)...)
}

// validate runs struct validation on value using the request's language for
// localized error messages.
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

// validateID validates a path/query id parameter using the same rules as
// requestvo.ObjId.
func (h *ExampleHandler) validateID(id, lang string) error {
	return h.validate(&requestvo.ObjId{ID: id}, lang)
}

// Create handles POST requests to create a new example resource.
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
		return transport.MapDomainError(err)
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusCreated)
}

// Get handles GET requests to fetch a single example resource by id.
func (h *ExampleHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	lang := c.Get(moduleconstant.XLanguageFlag, "en")
	if err := h.validateID(id, lang); err != nil {
		return err
	}
	resp, err := h.UseCase.Get(c.UserContext(), id)
	if err != nil {
		return transport.MapDomainError(err)
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusOK)
}

// List handles GET requests to paginate example resources.
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
		return transport.MapDomainError(err)
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusOK)
}

// Update handles PATCH/PUT requests that partially update an example
// resource by id. Only fields present in the request body are changed.
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
		return transport.MapDomainError(err)
	}
	return fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithFiberContext(c), fiber.StatusOK)
}

// Delete handles DELETE requests to remove an example resource by id.
func (h *ExampleHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	lang := c.Get(moduleconstant.XLanguageFlag, "en")
	if err := h.validateID(id, lang); err != nil {
		return err
	}
	if err := h.UseCase.Delete(c.UserContext(), id); err != nil {
		return transport.MapDomainError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
