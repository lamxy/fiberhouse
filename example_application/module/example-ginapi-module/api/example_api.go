package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/lamxy/fiberhouse"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	moduleconstant "github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/transport"
)

// ExampleHandler is the Gin transport for the example resource. It exposes
// the same CRUD contract as example-module/api.ExampleHandler (Fiber) at the
// identical paths (POST/GET/PUT/DELETE /examples[/{id}]).
//
// Swaggo annotations are intentionally NOT duplicated here: swag init would
// collide on identical @Router paths declared by both adapters. The Fiber
// handlers in example_application/module/example-module/api/example_api.go
// are the single annotated source of truth for the generated OpenAPI spec;
// see example_application/docs/README.md for the documented rationale.
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

func (h *ExampleHandler) language(c *gin.Context) string {
	lang := c.GetHeader(moduleconstant.XLanguageFlag)
	if lang == "" {
		return "en"
	}
	return lang
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

// Create handles POST /examples, mirroring the Fiber Create handler's
// contract (see example-module/api.ExampleHandler.Create for the swaggo spec).
func (h *ExampleHandler) Create(c *gin.Context) {
	var req requestvo.CreateExampleReqVo
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.validate(&req, h.language(c)); err != nil {
		_ = c.Error(err)
		return
	}
	resp, err := h.UseCase.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(transport.MapDomainError(err))
		return
	}
	if err := fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithGinContext(c), http.StatusCreated); err != nil {
		_ = c.Error(err)
	}
}

// Get handles GET /examples/{id}, mirroring the Fiber Get handler's contract
// (see example-module/api.ExampleHandler.Get for the swaggo spec).
func (h *ExampleHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if err := h.validateID(id, h.language(c)); err != nil {
		_ = c.Error(err)
		return
	}
	resp, err := h.UseCase.Get(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(transport.MapDomainError(err))
		return
	}
	if err := fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithGinContext(c), http.StatusOK); err != nil {
		_ = c.Error(err)
	}
}

// List handles GET /examples, mirroring the Fiber List handler's contract
// (see example-module/api.ExampleHandler.List for the swaggo spec).
func (h *ExampleHandler) List(c *gin.Context) {
	var req requestvo.ListExamplesReqVo
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.validate(&req, h.language(c)); err != nil {
		_ = c.Error(err)
		return
	}
	resp, err := h.UseCase.List(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(transport.MapDomainError(err))
		return
	}
	if err := fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithGinContext(c), http.StatusOK); err != nil {
		_ = c.Error(err)
	}
}

// Update handles PUT /examples/{id}, mirroring the Fiber Update handler's
// contract (see example-module/api.ExampleHandler.Update for the swaggo spec).
func (h *ExampleHandler) Update(c *gin.Context) {
	var req requestvo.UpdateExampleReqVo
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}
	id := c.Param("id")
	lang := h.language(c)
	if err := h.validate(&req, lang); err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.validateID(id, lang); err != nil {
		_ = c.Error(err)
		return
	}
	resp, err := h.UseCase.Update(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(transport.MapDomainError(err))
		return
	}
	if err := fiberhouse.Response().
		SuccessWithData(resp).
		JsonWithCtx(adaptorctx.WithGinContext(c), http.StatusOK); err != nil {
		_ = c.Error(err)
	}
}

// Delete handles DELETE /examples/{id}, mirroring the Fiber Delete handler's
// contract (see example-module/api.ExampleHandler.Delete for the swaggo spec).
func (h *ExampleHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.validateID(id, h.language(c)); err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.UseCase.Delete(c.Request.Context(), id); err != nil {
		_ = c.Error(transport.MapDomainError(err))
		return
	}
	c.Status(http.StatusNoContent)
}
