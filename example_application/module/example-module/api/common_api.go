package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
	"github.com/lamxy/fiberhouse/response"
)

// CommonHandler 示例公共处理器，继承自 fiberhouse.ApiLocator，具备获取上下文、配置、日志、注册实例等功能
type CommonHandler struct {
	fiberhouse.ApiLocator        // 继承fiberhouse.ApiLocator
	KeyTestService        string // 定义依赖组件的全局管理器的实例key。通过key即可由 h.GetInstance(key) 方法获取实例，或由 fiberhouse.GetMustInstance[T](key) 泛型方法获取实例，无需wire或其他依赖注入工具
}

// NewCommonHandler 直接New，无需依赖注入(Wire) TestService对象，内部依赖走全局管理器延迟获取依赖组件
func NewCommonHandler(ctx fiberhouse.IApplicationContext) *CommonHandler {
	return &CommonHandler{
		ApiLocator:     fiberhouse.NewApi(ctx).SetName(GetKeyCommonHandler()),
		KeyTestService: service.RegisterKeyTestService(ctx), // 注册依赖的TestService实例初始化器并返回注册实例key，通过 h.GetInstance(key) 方法获取TestService实例
	}
}

// GetKeyCommonHandler 获取 CommonHandler 注册到全局管理器的实例key
func GetKeyCommonHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("CommonHandler", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// TestGetInstance 测试获取注册实例，通过 h.GetInstance(key) 方法获取TestService注册实例，无需编译阶段的wire依赖注入
func (h *CommonHandler) TestGetInstance(c *fiber.Ctx) error {
	t := c.Query("t", "test")

	// 通过 h.GetInstance(key) 方法获取注册实例
	testService, err := h.GetInstance(h.KeyTestService)
	if err != nil {
		return err
	}

	if ts, ok := testService.(*service.TestService); ok {
		return response.RespSuccess(t + ":" + ts.HelloWorld()).JsonWithCtx(providerctx.WithFiberContext(c))
	}

	return response.RespSuccess(t).JsonWithCtx(providerctx.WithFiberContext(c))
}

// TestGetMustInstance 测试获取注册实例，通过 fiberhouse.GetMustInstance[T](key) 泛型方法获取注册实例，无需编译阶段的wire依赖注入
func (h *CommonHandler) TestGetMustInstance(c *fiber.Ctx) error {
	t := c.Query("t", "test")

	// 通过fiberhouse.GetMustInstance[T](key) 泛型方法获取注册实例
	testService := fiberhouse.GetMustInstance[*service.TestService](h.KeyTestService)

	return response.RespSuccess(t + testService.HelloWorld()).JsonWithCtx(providerctx.WithFiberContext(c))
}

// TestGetMustInstanceFailed 测试获取注册实例失败，通过 fiberhouse.GetMustInstance[T](key) 泛型方法获取注册实例，无需编译阶段的wire依赖注入
func (h *CommonHandler) TestGetMustInstanceFailed(c *fiber.Ctx) error {
	t := c.Query("t", "test")

	// 通过fiberhouse.GetMustInstance[T](key) 泛型方法获取注册实例
	testService := fiberhouse.GetMustInstance[service.TestService](h.KeyTestService)

	return response.RespSuccess(t + testService.HelloWorld()).JsonWithCtx(providerctx.WithFiberContext(c))
}
