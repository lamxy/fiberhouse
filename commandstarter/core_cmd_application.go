// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package commandstarter

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/urfave/cli/v2"
)

// CoreCmdCli 核心命令行应用cli启动器
type CoreCmdCli struct {
	Ctx     fiberhouse.ICommandContext
	coreApp *cli.App
}

// NewCoreCmdCli 实例化核心命令行应用cli启动器
func NewCoreCmdCli(ctx fiberhouse.ICommandContext, opts ...fiberhouse.CoreCmdStarterOption) fiberhouse.CoreCmdStarter {
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
func (ccc *CoreCmdCli) GetAppContext() fiberhouse.ICommandContext {
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

// ErrCmdApplicationNotRegistered 标记命令行应用注册器尚未注册。
//
// 装配期的顺序错误：三个 Register* 方法都要求先 RegisterApplication。
var ErrCmdApplicationNotRegistered = errors.New(
	"fiberhouse/commandstarter: application of ApplicationCmdRegister is nil, please RegisterApplication first")

// RegisterGlobalErrHandler 核心应用全局错误处理器
//
// 未注册应用时 panic。需要处理该错误的调用方请改用
// RegisterGlobalErrHandlerWithError。
func (ccc *CoreCmdCli) RegisterGlobalErrHandler(fca fiberhouse.FrameCmdStarter) {
	if err := ccc.RegisterGlobalErrHandlerWithError(fca); err != nil {
		panic(err)
	}
}

// RegisterGlobalErrHandlerWithError 与 RegisterGlobalErrHandler 相同，但未注册
// 应用时返回错误而非 panic。
func (ccc *CoreCmdCli) RegisterGlobalErrHandlerWithError(fca fiberhouse.FrameCmdStarter) error {
	register, err := resolveCmdRegister(fca)
	if err != nil {
		return err
	}
	// 注册全局错误处理器
	register.RegisterGlobalErrHandler(ccc.coreApp)
	return nil
}

// RegisterCommands 注册命令列表到核心应用
//
// 未注册应用时 panic。需要处理该错误的调用方请改用 RegisterCommandsWithError。
func (ccc *CoreCmdCli) RegisterCommands(fca fiberhouse.FrameCmdStarter) {
	if err := ccc.RegisterCommandsWithError(fca); err != nil {
		panic(err)
	}
}

// RegisterCommandsWithError 与 RegisterCommands 相同，但未注册应用时返回错误。
func (ccc *CoreCmdCli) RegisterCommandsWithError(fca fiberhouse.FrameCmdStarter) error {
	register, err := resolveCmdRegister(fca)
	if err != nil {
		return err
	}
	register.RegisterCommands(ccc.coreApp)
	return nil
}

// RegisterCoreGlobalOptional 注册应用核心的全局可选项
//
// 未注册应用时 panic。需要处理该错误的调用方请改用
// RegisterCoreGlobalOptionalWithError。
func (ccc *CoreCmdCli) RegisterCoreGlobalOptional(fca fiberhouse.FrameCmdStarter) {
	if err := ccc.RegisterCoreGlobalOptionalWithError(fca); err != nil {
		panic(err)
	}
}

// RegisterCoreGlobalOptionalWithError 与 RegisterCoreGlobalOptional 相同，但未
// 注册应用时返回错误。
func (ccc *CoreCmdCli) RegisterCoreGlobalOptionalWithError(fca fiberhouse.FrameCmdStarter) error {
	register, err := resolveCmdRegister(fca)
	if err != nil {
		return err
	}
	// 注册应用核心的全局可选项
	register.RegisterCoreGlobalOptional(ccc.coreApp)
	return nil
}

// resolveCmdRegister 取出命令行应用注册器，未注册或类型不符时返回错误。
//
// 三个 Register* 方法共用同一段前置校验；抽出来保证它们的错误完全一致，
// 也顺带补上了原实现缺少的类型断言检查——原代码直接断言，类型不符时
// panic 信息不含任何上下文。
func resolveCmdRegister(fca fiberhouse.FrameCmdStarter) (fiberhouse.ApplicationCmdRegister, error) {
	if fca == nil || fca.GetApplication() == nil {
		return nil, ErrCmdApplicationNotRegistered
	}
	register, ok := fca.GetApplication().(fiberhouse.ApplicationCmdRegister)
	if !ok {
		return nil, fmt.Errorf(
			"%w: registered application is %T, not an ApplicationCmdRegister",
			ErrCmdApplicationNotRegistered, fca.GetApplication())
	}
	return register, nil
}
