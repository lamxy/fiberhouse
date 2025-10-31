package main

import (
	"github.com/lamxy/fiberhouse/example_application"
	"github.com/lamxy/fiberhouse/example_application/module"
	_ "github.com/lamxy/fiberhouse/example_main/docs" // swagger docs
	"github.com/lamxy/fiberhouse/frame"
	"github.com/lamxy/fiberhouse/frame/applicationstarter"
	"github.com/lamxy/fiberhouse/frame/bootstrap"
	"github.com/lamxy/fiberhouse/frame/option"
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
	// bootstrap 初始化启动配置(全局配置、全局日志器)，配置目录默认为当前工作目录"."下的`example_config/`
	cfg := bootstrap.NewConfigOnce("./example_config")
	// 日志目录默认为当前工作目录"."下的`example_main/logs`
	logger := bootstrap.NewLoggerOnce(cfg, "./example_main/logs")

	// 初始化全局应用上下文
	appContext := frame.NewAppContextOnce(cfg, logger)
	// 设置版本信息
	appContext.GetConfig().SetVersion(Version)

	// 初始化应用注册器、模块/子系统注册器和任务注册器对象，注入到应用启动器
	appRegister := example_application.NewApplication(appContext)
	moduleRegister := module.NewModule(appContext)
	taskRegister := module.NewTaskAsync(appContext)

	// 实例化应用启动器
	starterApp := applicationstarter.NewFrameApplication(appContext,
		option.WithAppRegister(appRegister),
		option.WithModuleRegister(moduleRegister),
		option.WithTaskRegister(taskRegister),
	)

	// 运行应用启动器
	applicationstarter.RunApplicationStarter(starterApp)
}
