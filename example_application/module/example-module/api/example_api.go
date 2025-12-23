package api

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/api-vo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/api-vo/example/responsevo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
	"github.com/lamxy/fiberhouse/response"
	"net/http"
	"strconv"
)

// ExampleHandler 示例处理器，继承自 fiberhouse.ApiLocator，具备获取上下文、配置、日志、注册实例等功能
type ExampleHandler struct {
	fiberhouse.ApiLocator
	Service        *service.ExampleService // 直接定义依赖的组件指针类型成员变量，由wire注入，无需注册到全局管理器
	KeyTestService string                  // 定义依赖组件的全局管理器的实例key。通过key即可由 h.GetInstance(key) 方法获取实例，或由 fiberhouse.GetMustInstance[T](key) 泛型方法获取实例，无需wire或其他依赖注入工具
}

func NewExampleHandler(ctx fiberhouse.IApplicationContext, es *service.ExampleService) *ExampleHandler {
	return &ExampleHandler{
		ApiLocator:     fiberhouse.NewApi(ctx).SetName(GetKeyExampleHandler()),
		Service:        es,
		KeyTestService: service.RegisterKeyTestService(ctx), // 注册依赖的测试服务实例初始化器并返回注册实例key，通过 h.GetInstance(key) 方法获取实例
	}
}

/**
// namespace前缀规则:
// 1. 命名空间作为注册实例key前缀的一部分，起于模块（子系统）名字路径，表明对象属于其所在的模块（子系统），如"common-module."，表示common-module的模块名（子系统名），模块内地目录继续用"."拼接;
// 2. 比如：模块内model目录下的ExampleModel对象要注册进全局管理器，最终组合的key名称为: example-module.model.ExampleModel;

// 注意：
// 1. 框架提供的 fiberhouse.RegisterKeyName() 方法会自动帮你组合命名空间前缀和组件名称，生成完整的注册key名称；其中fiberhouse.GetNamespace()方法会帮你组合命名空间前缀部分，接受一个名字空间的切片，内部自动
// 按"."拼接名字空间后作为默认值，但当参数"ns"存在值时，由ns作为的命名空间前缀。
// 2. 组件注册到全局管理器的key名称，应符合标识符命名规范，注册名需要保证唯一，否则会注册失败。
*/

// GetKeyExampleHandler 定义和获取 ExampleHandler 注册到全局管理器的实例key
func GetKeyExampleHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleHandler", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// HelloWorld 测试接口，通过 h.GetInstance(key) 方法获取TestService注册实例，无需编译阶段的wire依赖注入
func (h *ExampleHandler) HelloWorld(c *fiber.Ctx) error {
	// 通过Key即时获取注册在全局管理器的TestService实例单例
	ts, err := h.GetInstance(h.KeyTestService)

	if err != nil {
		return err
	}

	// 获取TestService服务实例
	if tss, ok := ts.(*service.TestService); ok {
		// 成功的响应
		return response.RespSuccess(tss.HelloWorld()).JsonWithCtx(fiberhouse.CoreContext(c), http.StatusOK)
	}

	// 类型断言失败响应
	return fmt.Errorf("type assert failed for TestService: %v", ts)
}

/**
CURD test Api & Dispatcher Task
*/

// GetExample godoc
//
//	@Summary		get A Example
//	@Description	get Example Object by ID
//	@Tags			example
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Example ID"
//	@Success		200	{object}	response.RespInfo
//	@Failure		400	{object}	response.RespInfo
//	@Failure		404	{object}	response.RespInfo
//	@Failure		500	{object}	response.RespInfo
//	@Router			/example/get/{id} [get]
func (h *ExampleHandler) GetExample(c *fiber.Ctx) error {
	// 获取语言
	var lang = c.Get(constant.XLanguageFlag, "en")

	id := c.Params("id")

	// 构造需要验证的结构体
	var objId = &requestvo.ObjId{
		ID: id,
	}
	// 获取验证包装器对象
	vw := h.GetContext().GetValidateWrap()

	// 获取指定语言的验证器，并对结构体进行验证
	if errVw := vw.GetValidate(lang).Struct(objId); errVw != nil {
		var errs validator.ValidationErrors
		if errors.As(errVw, &errs) {
			return vw.Errors(errs, lang, true)
		}
	}

	// 从服务层获取数据
	resp, err := h.Service.GetExample(id)
	if err != nil {
		return err
	}

	// 返回成功响应
	return response.RespSuccess(resp).JsonWithCtx(providerctx.WithFiberContext(c))
}

// GetExampleWithTaskDispatcher godoc
//
//	@Summary		get A Example with Dispatcher Task
//	@Description	get Example Object by ID, on Dispatcher a task to do something
//	@Tags			example
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Example ID"
//	@Success		200	{object}	response.RespInfo
//	@Failure		400	{object}	response.RespInfo
//	@Failure		404	{object}	response.RespInfo
//	@Failure		500	{object}	response.RespInfo
//	@Router			/example/on-async-task/get/{id} [get]
func (h *ExampleHandler) GetExampleWithTaskDispatcher(c *fiber.Ctx) error {
	// 获取语言
	var lang = c.Get(constant.XLanguageFlag, "en")

	id := c.Params("id")

	// 构造需要验证的结构体
	var objId = &requestvo.ObjId{
		ID: id,
	}
	// 获取验证包装器对象
	vw := h.GetContext().GetValidateWrap()

	// 获取指定语言的验证器，并对结构体进行验证
	if errVw := vw.GetValidate(lang).Struct(objId); errVw != nil {
		var errs validator.ValidationErrors
		if errors.As(errVw, &errs) {
			return vw.Errors(errs, lang, true)
		}
	}

	// 从服务层获取数据
	resp, err := h.Service.GetExampleWithTaskDispatcher(id)
	if err != nil {
		return err
	}

	// 返回成功响应
	return response.RespSuccess(resp).JsonWithCtx(providerctx.WithFiberContext(c))
}

// CreateExample godoc
//
//	@Summary		create A Example
//	@Description	create a new Example Object
//	@Tags			example
//	@Accept			json
//	@Produce		json
//	@Param			example	body		requestvo.ExampleReqVo	true	"Example Request"
//	@Success		200		{object}	response.RespInfo
//	@Failure		400		{object}	response.RespInfo
//	@Failure		422		{object}	response.RespInfo
//	@Failure		500		{object}	response.RespInfo
//	@Router			/create [post]
func (h *ExampleHandler) CreateExample(c *fiber.Ctx) error {
	var req requestvo.ExampleReqVo
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	// 获取验证包装器对象
	vw := h.GetContext().GetValidateWrap()

	// 获取指定语言的验证器，并对结构体进行验证
	var lang = c.Get(constant.XLanguageFlag, "en")

	if errVw := vw.GetValidate(lang).Struct(&req); errVw != nil {
		var errs validator.ValidationErrors
		if errors.As(errVw, &errs) {
			return vw.Errors(errs, lang, true)
		}
	}

	// 从服务层获取数据
	id, err := h.Service.CreateExample(&req)
	if err != nil {
		return err
	}

	resp := &responsevo.ExampleIdRespVo{
		ID: id,
	}

	// 返回成功响应
	return response.RespSuccess(resp).JsonWithCtx(providerctx.WithFiberContext(c))
}

// GetExamples godoc
//
//	@Summary		get Examples with pagination
//	@Description	get paginated list of Example objects
//	@Tags			example
//	@Accept			json
//	@Produce		json
//	@Param			p	query		int	false	"Page number"	default(1)	minimum(1)
//	@Param			s	query		int	false	"Page size"		default(10)	minimum(1)	maximum(20)
//	@Success		200	{object}	response.RespInfo
//	@Failure		400	{object}	response.RespInfo
//	@Failure		422	{object}	response.RespInfo
//	@Failure		500	{object}	response.RespInfo
//	@Router			/list [get]
func (h *ExampleHandler) GetExamples(c *fiber.Ctx) error {
	p, s := c.Query("p", "1"), c.Query("s", "10")

	page, _ := strconv.Atoi(p)
	size, _ := strconv.Atoi(s)

	// 获取验证包装器对象
	var lang = c.Get(constant.XLanguageFlag, "en")
	vw := h.GetContext().GetValidateWrap()

	// 动态组合要验证的参数
	vaMap := fiber.Map{
		"Page": page,
		"Size": size,
	}
	// 动态组合验证规则
	vaRule := fiber.Map{
		"Page": "required,min=1",
		"Size": "required,min=1,max=20",
	}

	// 获取指定语言的验证器，验证map参数的动态规则
	if errsMap := vw.GetValidate(lang).ValidateMap(vaMap, vaRule); len(errsMap) > 0 {
		return vw.ErrorsMap(errsMap, lang, true)
	}

	// 从服务层获取数据
	resp, err := h.Service.GetExamples(page, size)
	if err != nil {
		return err
	}

	// 返回成功响应
	return response.RespSuccess(resp).JsonWithCtx(providerctx.WithFiberContext(c))
}
