package main

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/example_application/providers/middleware"
	"github.com/lamxy/fiberhouse/example_application/providers/module"
	"github.com/lamxy/fiberhouse/example_application/providers/optioninit"
	_ "github.com/lamxy/fiberhouse/example_main/docs" // swagger docs
)

// Version 版本信息，通过编译时 ldflags 注入
// 使用方式: go build -ldflags "-X main.Version=v1.0.0"
var (
	Version string // version
)

// Swagger Annotations

// @title XXX Service APIs
// @version 1.0
// @license.name XXX copyright
// @accept json
// @produce json
// @schemes http https
// @host localhost:8080
// @BasePath /
func main() {
	/*  必要的实现和说明：
	// 初始化应用注册器、模块/子系统注册器和任务注册器对象，注入到框架启动器
	var appCtx fiberhouse.IApplicationContext
	appRegister := example_application.NewApplication(appCtx)
	moduleRegister := module.NewModule(appCtx)
	taskRegister := module.NewTaskAsync(appCtx)

	// 框架启动器初始化函数选项列表，用于启动FrameStarter
	frameOpts := []fiberhouse.FrameStarterOption{
		option.WithAppRegister(appRegister),
		option.WithModuleRegister(moduleRegister),
		option.WithTaskRegister(taskRegister),
	}

	// 创建 FrameStarter 启动器实例
	frameStarter := NewFrameApplication(ctx.(IApplicationContext), frameOpts...)

	// 核心启动器选项和核心启动器的创建同上类似
	coreStarter := NewCoreFiberStarter(ctx.(IApplicationContext), coreOpts...)

	// 最终的应用启动器由框架启动器和核心启动器的实现而实现（应用启动器接口由框架启动器接口和核心启动器接口组合实现），
	//运行应用启动器即启动应用Web服务

	// 以下为FiberHouse实例，封装了上述的基础逻辑，并由提供者和提供者管理器模块化设计和扩展后运行
	*/

	// 创建 FiberHouse 应用运行实例
	fh := fiberhouse.New(&fiberhouse.BootConfig{
		AppName:      "Default FiberHouse Application",          // 应用名称
		Version:      Version,                                   // 应用版本
		FrameType:    constant.FrameTypeWithDefaultFrameStarter, // 默认提供的框架启动器标识: DefaultFrameStarter
		CoreType:     constant.CoreTypeWithFiber,                // fiber | gin | ...
		TrafficCodec: constant.TrafficCodecWithSonic,            // 传输流量的编解码器: sonic_json_codec|std_json_codec|go_json_codec|pb...
		ConfigPath:   "./example_config",                        // 应用全局配置路径
		LogPath:      "./example_main/logs",                     // 日志文件路径
	})

	// 收集提供者和管理器
	providers := fiberhouse.DefaultProviders().AndMore(
		// 框架启动器和核心启动器的选项参数初始化提供者，
		//注意：由于选项初始化管理器New时已唯一绑定对应的提供者，此处提供者可以无需收集
		//见NewFrameOptionInitPManager()函数
		//optioninit.NewFrameOptionInitProvider(),
		//optioninit.NewCoreOptionInitProvider(),

		//中间件注册提供者
		middleware.NewFiberAppMiddlewareProvider(),
		middleware.NewFiberModuleMiddlewareProvider(),
		// 其他可切换的框架相关中间件提供者
		// ...

		// fiber模块路由和swagger注册提供者
		module.NewFiberRouteRegisterProvider(),
		module.NewFiberSwaggerRegisterProvider(),
		// gin模块路由和swagger注册提供者
		// ...
	)
	managers := fiberhouse.DefaultPManagers(fh.AppCtx).AndMore(
		// 框架选项初始化管理器
		optioninit.NewFrameOptionInitPManager(fh.AppCtx),
		// 核心选项初始化管理器
		optioninit.NewCoreOptionInitPManager(fh.AppCtx).MountToParent(),
		// 应用中间件管理器
		middleware.NewAppMiddlewarePManager(fh.AppCtx),
		// 模块路由注册管理器
		module.NewRouteRegisterPManager(fh.AppCtx),
	)

	// 初始化提供者和管理器并运行服务器
	fh.WithProviders(providers...).WithPManagers(managers...).RunServer()
}
