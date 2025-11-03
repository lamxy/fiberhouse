package main

import (
	"github.com/lamxy/fiberhouse/example_application/command/application"
	"github.com/lamxy/fiberhouse/frame"
	"github.com/lamxy/fiberhouse/frame/bootstrap"
	"github.com/lamxy/fiberhouse/frame/commandstarter"
	"github.com/lamxy/fiberhouse/frame/option"
)

func main() {
	// bootstrap 初始化启动配置(全局配置、全局日志器)，配置路径为当前工作目录下的"./../config"
	cfg := bootstrap.NewConfigOnce("./../../example_config")

	// 全局日志器，定义日志目录为当前工作目录下的"./logs"
	logger := bootstrap.NewLoggerOnce(cfg, "./logs")

	// 初始化命令全局上下文
	ctx := frame.NewCmdContextOnce(cfg, logger)

	// 初始化应用注册器对象，注入应用启动器
	appRegister := application.NewApplication(ctx)

	// 实例化命令行应用启动器
	cmdlineStarter := &commandstarter.CMDLineApplication{
		// 实例化框架命令启动器对象
		FrameCmdStarter: commandstarter.NewFrameCmdApplication(ctx, option.WithCmdRegister(appRegister)),
		// 实例化核心命令启动器对象
		CoreCmdStarter: commandstarter.NewCoreCmdCli(ctx),
	}

	// 运行命令启动器
	commandstarter.RunCommandStarter(cmdlineStarter)
}
