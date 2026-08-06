package api

import (
	ctxpkg "context"
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/go-playground/validator/v10"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	moduleconstant "github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/transport"
)

// ExampleHandler 是 example 資源的 hertz 傳輸層。它在完全相同的路徑
// （POST/GET/PUT/DELETE /examples[/{id}]）上，暴露與 Fiber/Gin 適配器一致的 CRUD 契約。
//
// 此處刻意不重複編寫 swaggo 註解：多個適配器宣告相同的 @Router 路徑會導致
// swag init 衝突。example-module/api 中的 Fiber 處理器是生成 OpenAPI 規範時
// 唯一帶註解的權威來源。
type ExampleHandler struct {
	fiberhouse.ApiLocator
	UseCase service.ExampleUseCase
}

// NewExampleHandler 建立 hertz 的 example 處理器
func NewExampleHandler(ctx fiberhouse.IApplicationContext, useCase service.ExampleUseCase) *ExampleHandler {
	return &ExampleHandler{
		ApiLocator: fiberhouse.NewApi(ctx).SetName(GetKeyExampleHandler()),
		UseCase:    useCase,
	}
}

// GetKeyExampleHandler 取得 ExampleHandler 註冊到全域管理器的實例 key
func GetKeyExampleHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleHandler",
		fiberhouse.GetNamespace([]string{moduleconstant.NameModuleExample}, ns...)...)
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

// Create 處理 POST /examples
func (h *ExampleHandler) Create(c ctxpkg.Context, reqCtx *app.RequestContext) {
	var req requestvo.CreateExampleReqVo
	if err := reqCtx.BindJSON(&req); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	if err := h.validate(&req, language(reqCtx)); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	resp, err := h.UseCase.Create(requestContext(c), req)
	if err != nil {
		respondError(h.GetContext(), reqCtx,transport.MapDomainError(err))
		return
	}
	respondData(reqCtx, http.StatusCreated, resp)
}

// Get 處理 GET /examples/{id}
func (h *ExampleHandler) Get(c ctxpkg.Context, reqCtx *app.RequestContext) {
	id := reqCtx.Param("id")
	if err := h.validateID(id, language(reqCtx)); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	resp, err := h.UseCase.Get(requestContext(c), id)
	if err != nil {
		respondError(h.GetContext(), reqCtx,transport.MapDomainError(err))
		return
	}
	respondData(reqCtx, http.StatusOK, resp)
}

// List 處理 GET /examples
func (h *ExampleHandler) List(c ctxpkg.Context, reqCtx *app.RequestContext) {
	var req requestvo.ListExamplesReqVo
	if err := reqCtx.BindQuery(&req); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	if err := h.validate(&req, language(reqCtx)); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	resp, err := h.UseCase.List(requestContext(c), req)
	if err != nil {
		respondError(h.GetContext(), reqCtx,transport.MapDomainError(err))
		return
	}
	respondData(reqCtx, http.StatusOK, resp)
}

// Update 處理 PUT /examples/{id}
func (h *ExampleHandler) Update(c ctxpkg.Context, reqCtx *app.RequestContext) {
	var req requestvo.UpdateExampleReqVo
	if err := reqCtx.BindJSON(&req); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	id := reqCtx.Param("id")
	lang := language(reqCtx)
	if err := h.validate(&req, lang); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	if err := h.validateID(id, lang); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	resp, err := h.UseCase.Update(requestContext(c), id, req)
	if err != nil {
		respondError(h.GetContext(), reqCtx,transport.MapDomainError(err))
		return
	}
	respondData(reqCtx, http.StatusOK, resp)
}

// Delete 處理 DELETE /examples/{id}
func (h *ExampleHandler) Delete(c ctxpkg.Context, reqCtx *app.RequestContext) {
	id := reqCtx.Param("id")
	if err := h.validateID(id, language(reqCtx)); err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}
	if err := h.UseCase.Delete(requestContext(c), id); err != nil {
		respondError(h.GetContext(), reqCtx,transport.MapDomainError(err))
		return
	}
	reqCtx.SetStatusCode(http.StatusNoContent)
}
