package main

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/example_application/providers/middleware"
	_ "github.com/lamxy/fiberhouse/example_main/docs" // swagger docs
	"github.com/lamxy/fiberhouse/example_main/provider"
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
	/*
		// 初始化应用注册器、模块/子系统注册器和任务注册器对象，注入到框架启动器
		appCtx := ctx.(fiberhouse.IApplicationContext)
		appRegister := example_application.NewApplication(appCtx)
		moduleRegister := module.NewModule(appCtx)
		taskRegister := module.NewTaskAsync(appCtx)

		// 框架启动器初始化函数选项列表，用于启动FrameStarter
		frameOpts := []fiberhouse.FrameStarterOption{
			option.WithAppRegister(appRegister),
			option.WithModuleRegister(moduleRegister),
			option.WithTaskRegister(taskRegister),
		}
	*/

	// 创建 FiberHouse 应用运行实例
	fh := fiberhouse.New(&fiberhouse.BootConfig{
		AppName:    "Default FiberHouse Application",          // 应用名称
		Version:    Version,                                   // 应用版本
		FrameType:  constant.FrameTypeWithDefaultFrameStarter, // DefaultFrameStarter
		CoreType:   "fiber",                                   // fiber | gin | ...
		JsonCodec:  "sonic_json_codec",                        // sonic_json_codec|std_json_codec|go_json_codec|...
		ConfigPath: "./example_config",                        // 应用全局配置路径
		LogPath:    "./example_main/logs",                     // 日志文件路径
	})

	// 收集提供者和管理器
	providers := fiberhouse.DefaultProviders().AndMore(
		middleware.NewFiberAppMiddlewareProvider(),
		middleware.NewFiberModuleMiddlewareProvider(),
		// 其他可切换的框架相关中间件提供者
	)
	managers := fiberhouse.DefaultPManagers(fh.AppCtx).AndMore(
		// 管理器继承了基类父类实例，并绑定了管理器执行位置点
		//'New实例化方法中'挂载当前实例到父类属性上，
		//以便调用父类实例的初始化方法Initialize()时内部转调用子类的初始化方法
		//确保多态性行为正确执行子类重载的方法
		provider.NewFrameOptionInitPManager(fh.AppCtx),
		// '实例化后'即时挂载当前实例到父类属性上，子类重载了MountToParent方法，
		//直接调用子类自己重载方法，作用同上
		provider.NewCoreOptionInitPManager(fh.AppCtx).MountToParent(),
		// 应用中间件管理器
		middleware.NewAppMiddlewarePManager(fh.AppCtx),
	)

	// 初始化提供者和管理器并运行服务器
	fh.WithProviders(providers...).WithPManagers(managers...).RunServer()
}
