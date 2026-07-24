// Package api 是 example 模块的传输层：暴露 HTTP 处理器，负责解析并校验请求、
// 将请求转换为服务调用，并把服务层错误映射为 HTTP 响应。它依赖 service
// （并通过 transport 间接依赖 repository），但不得越过 service 层直接触达
// repository 或 model。
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

// ExampleHandler 是 example 资源的 Fiber 传输层。它只负责解析/校验请求，
// 并将全部业务逻辑委托给 UseCase。
type ExampleHandler struct {
	fiberhouse.ApiLocator
	UseCase service.ExampleUseCase
}

// NewExampleHandler 构建一个绑定到指定应用上下文与用例的 ExampleHandler，
// 并以 GetKeyExampleHandler 作为键进行注册。
func NewExampleHandler(ctx fiberhouse.IApplicationContext, useCase service.ExampleUseCase) *ExampleHandler {
	return &ExampleHandler{
		ApiLocator: fiberhouse.NewApi(ctx).SetName(GetKeyExampleHandler()),
		UseCase:    useCase,
	}
}

// GetKeyExampleHandler 返回用于定位 ExampleHandler 实例的注册键，
// 可通过 ns 追加命名空间。
func GetKeyExampleHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleHandler", fiberhouse.GetNamespace([]string{moduleconstant.NameModuleExample}, ns...)...)
}

// validate 使用请求语言对 value 执行结构体校验，以返回本地化的错误信息。
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

// validateID 按照 requestvo.ObjId 的规则校验路径/查询中的 id 参数。
func (h *ExampleHandler) validateID(id, lang string) error {
	return h.validate(&requestvo.ObjId{ID: id}, lang)
}

// Create 处理创建新 example 资源的 POST 请求。
//
// @Summary      创建 example
// @Description  使用 name、description、status 和 tags 创建一个新的 example 资源。
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        example  body      requestvo.CreateExampleReqVo  true  "待创建的 example"
// @Success      201      {object}  response.RespInfo{data=responsevo.ExampleRespVo}
// @Failure      400      {object}  response.RespInfo  "输入非法 / 校验错误"
// @Failure      500      {object}  response.RespInfo  "内部错误"
// @Router       /examples [post]
// @ID           fiberCreateExample
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

// Get 处理按 id 获取单个 example 资源的 GET 请求。
//
// @Summary      获取 example
// @Description  按 id 获取单个 example 资源。
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "example id"
// @Success      200  {object}  response.RespInfo{data=responsevo.ExampleRespVo}
// @Failure      400  {object}  response.RespInfo  "id 非法"
// @Failure      404  {object}  response.RespInfo  "example 不存在"
// @Failure      500  {object}  response.RespInfo  "内部错误"
// @Router       /examples/{id} [get]
// @ID           fiberGetExample
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

// List 处理对 example 资源进行分页查询的 GET 请求。
//
// @Summary      查询 example 列表
// @Description  分页查询 example 资源，可按 status 过滤。
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "页码（默认 1）"
// @Param        page_size  query     int     false  "每页数量，最大 100（默认 20）"
// @Param        status     query     string  false  "按状态过滤"  Enums(active, archived)
// @Success      200        {object}  response.RespInfo{data=responsevo.ExampleListRespVo}
// @Failure      400        {object}  response.RespInfo  "查询参数非法"
// @Failure      500        {object}  response.RespInfo  "内部错误"
// @Router       /examples [get]
// @ID           fiberListExamples
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

// Update 处理按 id 部分更新 example 资源的 PATCH/PUT 请求。
// 仅更新请求体中出现的字段。
//
// @Summary      更新 example
// @Description  按 id 部分更新 example 资源，仅更新请求体中出现的字段。
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        id       path      string                         true  "example id"
// @Param        example  body      requestvo.UpdateExampleReqVo   true  "待更新的字段"
// @Success      200      {object}  response.RespInfo{data=responsevo.ExampleRespVo}
// @Failure      400      {object}  response.RespInfo  "输入非法 / 校验错误"
// @Failure      404      {object}  response.RespInfo  "example 不存在"
// @Failure      409      {object}  response.RespInfo  "更新冲突或内容未变化"
// @Failure      500      {object}  response.RespInfo  "内部错误"
// @Router       /examples/{id} [put]
// @ID           fiberUpdateExample
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

// Delete 处理按 id 删除 example 资源的 DELETE 请求。
//
// @Summary      删除 example
// @Description  按 id 删除 example 资源。
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "example id"
// @Success      204  "无内容"
// @Failure      400  {object}  response.RespInfo  "id 非法"
// @Failure      404  {object}  response.RespInfo  "example 不存在"
// @Failure      500  {object}  response.RespInfo  "内部错误"
// @Router       /examples/{id} [delete]
// @ID           fiberDeleteExample
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
