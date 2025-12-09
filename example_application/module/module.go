package module

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	moduleApi "github.com/lamxy/fiberhouse/example_application/module/api"
)

// Module struct
type Module struct {
	name string // for marking & container key
	Ctx  fiberhouse.ContextFramer
}

func NewModule(ctx fiberhouse.ContextFramer) fiberhouse.ModuleRegister {
	return &Module{
		name: "module",
		Ctx:  ctx,
	}
}

// GetName get module name
func (m *Module) GetName() string {
	return m.name
}

// SetName set module name
func (m *Module) SetName(name string) {
	m.name = name
}

// GetContext get module context
func (m *Module) GetContext() fiberhouse.ContextFramer {
	return m.Ctx
}

// RegisterModuleMiddleware 注册模块(子系统)级中间件
func (m *Module) RegisterModuleMiddleware(core interface{}) {
	// 注册模块(子系统)级中间件
	moduleApi.RegisterMiddleware(core.(*fiber.App))
}

// RegisterModuleRouteHandlers 注册模块(子系统)级路由处理器
func (m *Module) RegisterModuleRouteHandlers(core interface{}) {
	// 注册各模块中间件和路由处理器
	RegisterRouteHandlers(m.Ctx, core.(*fiber.App))
}

// RegisterSwagger 注册swagger
func (m *Module) RegisterSwagger(core interface{}) {
	// 注册swagger
	RegisterSwagger(m.Ctx, core.(*fiber.App))
}
