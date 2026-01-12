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
	// 通过路由注册提供者注册各模块中间件和路由处理器
	// 从路由注册器执行位置点获取路由注册管理器列表
	managers := fiberhouse.ProviderLocationDefault().LocationRouteRegisterInit.GetManagers()
	if len(managers) > 0 {
		for _, manager := range managers {
			// 仅加载路由注册管理器类型的提供者
			if manager.Type().GetTypeID() == fiberhouse.ProviderTypeDefault().GroupRouteRegisterType.GetTypeID() {
				_, err := manager.LoadProvider(func(manager fiberhouse.IProviderManager) (any, error) {
					return cs, nil
				})
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

// RegisterSwagger 注册swagger
func (m *Module) RegisterSwagger(cs fiberhouse.CoreStarter) {
	// swagger注册提供者，路由注册管理器已完成全部路由注册
}
