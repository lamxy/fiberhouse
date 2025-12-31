package module

import (
	"github.com/lamxy/fiberhouse"
)

// Module struct
type Module struct {
	name string // for marking & container key
	Ctx  fiberhouse.IApplicationContext
}

func NewModule(ctx fiberhouse.IApplicationContext) fiberhouse.ModuleRegister {
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
func (m *Module) GetContext() fiberhouse.IApplicationContext {
	return m.Ctx
}

// RegisterModuleRouteHandlers 注册模块(子系统)级路由处理器
func (m *Module) RegisterModuleRouteHandlers(cs fiberhouse.CoreStarter) {
	// 注册各模块中间件和路由处理器 // TODO 路由注册提供者
	RegisterRouteHandlers(m.Ctx, cs)
}

// RegisterSwagger 注册swagger
func (m *Module) RegisterSwagger(cs fiberhouse.CoreStarter) {
	// 注册swagger  // TODO swagger注册提供者
	RegisterSwagger(m.Ctx, cs)
}
