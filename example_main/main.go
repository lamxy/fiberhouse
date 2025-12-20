package main

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
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
		// bootstrap 初始化启动配置(全局配置、全局日志器)，配置目录默认为当前工作目录"."下的`example_config/`
		cfg := bootstrap.NewConfigOnce("./example_config")
		// 日志目录默认为当前工作目录"."下的`example_main/logs`
		logger := bootstrap.NewLoggerOnce(cfg, "./example_main/logs")

		// 初始化全局应用上下文
		appContext := fiberhouse.NewAppContextOnce(cfg, logger)
		// 设置版本信息
		appContext.GetConfig().SetVersion(Version)

		// 初始化应用注册器、模块/子系统注册器和任务注册器对象，注入到框架启动器
		appRegister := example_application.NewApplication(appContext)
		moduleRegister := module.NewModule(appContext)
		taskRegister := module.NewTaskAsync(appContext)

		// 实例化 Web 应用启动器
		web := &fiberhouse.WebApplication{
			// 实例化框架启动器
			FrameStarter: fiberhouse.NewFrameApplication(appContext,
				option.WithAppRegister(appRegister),
				option.WithModuleRegister(moduleRegister),
				option.WithTaskRegister(taskRegister),
			),
			// 实例化核心应用启动器
			CoreStarter: fiberhouse.NewCoreWithFiber(appContext),
		}

		// 运行应用启动器
		fiberhouse.RunApplicationStarter(web)
	*/

	fh := fiberhouse.New(&fiberhouse.BootConfig{
		FrameType:  "default",
		CoreType:   "fiber",
		JsonCodec:  "sonic_json_codec", // sonic_json_codec|json_codec|go_json_codec|...
		ConfigPath: "./example_config",
		LogPath:    "./example_main/logs",
	})
	providers := fiberhouse.DefaultProviders().List()
	managers := fiberhouse.DefaultPManagers(fh.AppCtx).AndMore(provider.NewFrameOptionInitPManager(fh.AppCtx), provider.NewCoreOptionInitPManager(fh.AppCtx).MountToParent())

	for _, manager := range managers {
		fmt.Printf("Provider Manager: Name=%s, Type=%s, Location=%s\n", manager.Name(), manager.Type().GetTypeName(), manager.Location().GetLocationName())
	}

	for _, p := range providers {
		fmt.Printf("Provider: Name=%s, Type=%s, Target=%s\n", p.Name(), p.Type().GetTypeName(), p.Target())
	}

	fh.WithProviders(providers...).WithPManagers(managers...).RunServer()
}
