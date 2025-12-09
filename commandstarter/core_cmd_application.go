// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package commandstarter

import (
	"errors"
	"github.com/lamxy/fiberhouse"
	"github.com/urfave/cli/v2"
	"os"
	"sort"
	"time"
)

// CoreCmdCli 核心命令行应用cli启动器
type CoreCmdCli struct {
	Ctx     fiberhouse.ContextCommander
	coreApp *cli.App
}

// NewCoreCmdCli 实例化核心命令行应用cli启动器
func NewCoreCmdCli(ctx fiberhouse.ContextCommander, opts ...fiberhouse.CoreCmdStarterOption) fiberhouse.CoreCmdStarter {
	cca := &CoreCmdCli{
		Ctx: ctx,
	}

	if len(opts) == 0 {
		ctx.GetLogger().Warn(ctx.GetConfig().LogOriginFrame()).Msg("No registrar option available for injection into the core command starter")
	}

	for _, opt := range opts {
		opt(cca)
	}

	return cca
}

// GetAppContext 获取全局上下文
func (ccc *CoreCmdCli) GetAppContext() fiberhouse.ContextCommander {
	return ccc.Ctx
}

// GetCoreCmdApp 获取核心命令行启动器实例
func (ccc *CoreCmdCli) GetCoreCmdApp() fiberhouse.CoreCmdStarter {
	return ccc
}

// InitCoreApp 初始化包装的底层核心应用
func (ccc *CoreCmdCli) InitCoreApp() {
	cfg := ccc.GetAppContext().GetConfig()

	if ccc.coreApp == nil {
		// 初始化核心应用
		ccc.coreApp = &cli.App{
			// 应用基本配置
			Name:     cfg.String("command.name", ""),
			Usage:    cfg.String("command.usage", ""),
			Version:  cfg.String("command.version", ""),
			Suggest:  true,
			Compiled: time.Now(),

			//Flags: []cli.Flag{},  // 全局选项 Options
			//Action: func(context *cli.Context) error {}, // 全局动作 actions

			// 命令列表选项
			EnableBashCompletion:   true,
			UseShortOptionHandling: true,

			//Commands:               []*cli.Command{},  // 命令列表 []*cli.Command
		}
	}

	if cfg.Bool("command.sortFlagsByName") {
		sort.Sort(cli.FlagsByName(ccc.coreApp.Flags))
	}
	if cfg.Bool("command.sortCommandsByName") {
		sort.Sort(cli.CommandsByName(ccc.coreApp.Commands))
	}
}

// RegisterCoreApp 注册底层核心应用对象
func (ccc *CoreCmdCli) RegisterCoreApp(core interface{}) {
	if coreApp, ok := core.(*cli.App); ok {
		ccc.coreApp = coreApp
	} else {
		ccc.GetAppContext().GetLogger().WarnWith(ccc.GetAppContext().GetConfig().LogOriginCMD()).Msg("RegisterCoreApp received invalid core application type, expected *cli.App")
	}
}

// AppCoreRun 运行核心应用
func (ccc *CoreCmdCli) AppCoreRun() error {
	if err := ccc.coreApp.Run(os.Args); err != nil {
		ccc.GetAppContext().GetLogger().Error(ccc.GetAppContext().GetConfig().LogOriginCMD()).Err(err).Str("Name", ccc.coreApp.Name).
			Str("Version", ccc.coreApp.Version).Strs("Args", os.Args).Msg("CMD Run Error!")
		return err
	}
	return nil
}

// RegisterGlobalErrHandler 核心应用全局错误处理器
func (ccc *CoreCmdCli) RegisterGlobalErrHandler(fca fiberhouse.FrameCmdStarter) {
	if fca.GetApplication() == nil {
		panic(errors.New("application of ApplicationCmdRegister is nil, please RegisterApplication first"))
	}
	// 注册全局错误处理器
	fca.GetApplication().(fiberhouse.ApplicationCmdRegister).RegisterGlobalErrHandler(ccc.coreApp)
}

// RegisterCommands 注册命令列表到核心应用
func (ccc *CoreCmdCli) RegisterCommands(fca fiberhouse.FrameCmdStarter) {
	if fca.GetApplication() == nil {
		panic(errors.New("application of ApplicationCmdRegister is nil, please RegisterApplication first"))
	}
	fca.GetApplication().(fiberhouse.ApplicationCmdRegister).RegisterCommands(ccc.coreApp)
}

// RegisterCoreGlobalOptional 注册应用核心的全局可选项
func (ccc *CoreCmdCli) RegisterCoreGlobalOptional(fca fiberhouse.FrameCmdStarter) {
	if fca.GetApplication() == nil {
		panic(errors.New("application of ApplicationCmdRegister is nil, please RegisterApplication first"))
	}
	// 注册应用核心的全局可选项
	fca.GetApplication().(fiberhouse.ApplicationCmdRegister).RegisterCoreGlobalOptional(ccc.coreApp)
}
