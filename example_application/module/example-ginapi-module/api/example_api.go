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

// ExampleHandler 是 example 资源的 Gin 传输层。它在完全相同的路径
// （POST/GET/PUT/DELETE /examples[/{id}]）上，暴露与 example-module/api.ExampleHandler
// （Fiber）一致的 CRUD 契约。
//
// 此处刻意不重复编写 swaggo 注解：两个适配器声明相同的 @Router 路径会导致
// swag init 冲突。example_application/module/example-module/api/example_api.go
// 中的 Fiber 处理器是生成 OpenAPI 规范时唯一带注解的权威来源；
// 具体原因见 example_application/docs/README.md。
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

// Create 处理 POST /examples，与 Fiber 的 Create 处理器契约一致
// （swaggo 规范见 example-module/api.ExampleHandler.Create）。
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

// Get 处理 GET /examples/{id}，与 Fiber 的 Get 处理器契约一致
// （swaggo 规范见 example-module/api.ExampleHandler.Get）。
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

// List 处理 GET /examples，与 Fiber 的 List 处理器契约一致
// （swaggo 规范见 example-module/api.ExampleHandler.List）。
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

// Update 处理 PUT /examples/{id}，与 Fiber 的 Update 处理器契约一致
// （swaggo 规范见 example-module/api.ExampleHandler.Update）。
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

// Delete 处理 DELETE /examples/{id}，与 Fiber 的 Delete 处理器契约一致
// （swaggo 规范见 example-module/api.ExampleHandler.Delete）。
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
