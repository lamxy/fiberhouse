// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package commandstarter 提供基于 cli.v2 的命令行应用启动器实现，负责命令行应用的完整生命周期管理和启动流程编排。
package commandstarter

import (
	"errors"
	"github.com/lamxy/fiberhouse/frame"
	"github.com/urfave/cli/v2"
	"os"
	"sort"
	"time"
)

// CmdApplication 是命令行应用启动器，实现 frame.CommandStarter 接口。
// 负责管理命令行应用的生命周期，包括初始化、注册全局选项和动作、运行应用及错误处理。
type CmdApplication struct {
	Ctx         frame.ContextCommander
	coreApp     *cli.App
	application frame.ApplicationCmdRegister
}

// RunCommandStarter 运行命令启动器
func RunCommandStarter(cmdStarter frame.CommandStarter) {
	cmdStarter.InitCoreApp()
	cmdStarter.RegisterGlobalErrHandler()
	cmdStarter.RegisterCommands()
	cmdStarter.RegisterCoreGlobalOptional()
	cmdStarter.RegisterApplicationGlobals()
	_ = cmdStarter.AppCoreRun()
}

// NewCmdApplication 创建一个命令启动器对象，实现CommandStarter接口
func NewCmdApplication(ctx frame.ContextCommander, opts ...frame.CommandStarterOption) frame.CommandStarter {
	cApp := &CmdApplication{
		Ctx: ctx,
	}

	if len(opts) == 0 {
		ctx.GetLogger().FatalWith(ctx.GetConfig().LogOriginFrame()).Msg("No registrar option available for injection into the command starter")
	}

	for _, opt := range opts {
		opt(cApp)
	}

	return cApp
}

// GetContext 获取全局上下文
func (cApp *CmdApplication) GetContext() frame.ContextCommander {
	return cApp.Ctx
}

// InitCoreApp 初始化包装的底层核心应用
func (cApp *CmdApplication) InitCoreApp() {
	cfg := cApp.GetContext().GetConfig()

	if cApp.coreApp == nil {
		// 初始化核心应用
		cApp.coreApp = &cli.App{
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
		sort.Sort(cli.FlagsByName(cApp.coreApp.Flags))
	}
	if cfg.Bool("command.sortCommandsByName") {
		sort.Sort(cli.CommandsByName(cApp.coreApp.Commands))
	}
}

// RegisterCoreApp 注册底层核心应用对象
func (cApp *CmdApplication) RegisterCoreApp(core interface{}) {
	if coreApp, ok := core.(*cli.App); ok {
		cApp.coreApp = coreApp
	} else {
		cApp.GetContext().GetLogger().WarnWith(cApp.GetContext().GetConfig().LogOriginCMD()).Msg("RegisterCoreApp received invalid core application type, expected *cli.App")
	}
}

// RegisterApplicationGlobals 注册应用全局对象初始化器和初始化部分必要对象
func (cApp *CmdApplication) RegisterApplicationGlobals() {
	// 注册配置文件预定义不同来源(LogOrigin)的子日志器初始化器到容器
	cApp.RegisterLoggerWithOriginToContainer()
	// 注册自定义的应用全局初始化器和启动必要的全局单例
	cApp.GetApplication().(frame.ApplicationCmdRegister).RegisterApplicationGlobals()

	// 全局对象健康检查和重建
	if cApp.GetContext().GetConfig().Bool("application.globalManage.keepAlive") {
		cApp.startHealthCheck()
	}
}

// StartHealthCheck 异步检查全局对象是否健康和重建
func (cApp *CmdApplication) startHealthCheck() {
	gm, log, cfg := cApp.GetContext().GetContainer(), cApp.GetContext().GetLogger(), cApp.GetContext().GetConfig()
	defer func() {
		if r := recover(); r != nil {
			switch re := r.(type) {
			case error:
				log.Error(cfg.LogOriginCMD()).Err(re).Str("from", "global manager").Msg("StartHealthCheck recover Error")
			default:
				log.Error(cfg.LogOriginCMD()).Str("from", "global manager").Msgf("StartHealthCheck recover Error: %v", re)
			}
		}
	}()
	gm.Range(func(key, value interface{}) bool {
		name := key.(string)
		ret, err := gm.CheckHealth(name)
		if err != nil {
			log.Error(cfg.LogOriginCMD()).Err(err).Msgf("global object from key: '%s', health check failure", name) // return false to stop iteration
			return true
		}
		if !ret {
			log.Error(cfg.LogOriginCMD()).Msgf("global resource '%s' is unhealthy, rebuilding...", name)
			err = gm.Rebuild(name)
			if err != nil {
				log.Error(cfg.LogOriginCMD()).Err(err).Msgf("global resource '%s' rebuild failed.", name)
			}
			log.Info(cfg.LogOriginCMD()).Err(err).Msgf("global resource '%s' rebuild success.", name)
		}
		return true
	})
}

// AppCoreRun 运行核心应用
func (cApp *CmdApplication) AppCoreRun() error {
	if err := cApp.coreApp.Run(os.Args); err != nil {
		cApp.GetContext().GetLogger().Error(cApp.GetContext().GetConfig().LogOriginCMD()).Err(err).Str("Name", cApp.coreApp.Name).
			Str("Version", cApp.coreApp.Version).Strs("Args", os.Args).Msg("CMD Run Error!")
		return err
	}
	return nil
}

// RegisterGlobalErrHandler 核心应用全局错误处理器
func (cApp *CmdApplication) RegisterGlobalErrHandler() {
	if cApp.GetApplication() == nil {
		panic(errors.New("application of ApplicationCmdRegister is nil, please RegisterApplication first"))
	}
	// 注册全局错误处理器
	cApp.GetApplication().(frame.ApplicationCmdRegister).RegisterGlobalErrHandler(cApp.coreApp)
}

// RegisterCommands 注册命令列表到核心应用
func (cApp *CmdApplication) RegisterCommands() {
	if cApp.GetApplication() == nil {
		panic(errors.New("application of ApplicationCmdRegister is nil, please RegisterApplication first"))
	}
	cApp.GetApplication().(frame.ApplicationCmdRegister).RegisterCommands(cApp.coreApp)
}

// RegisterCoreGlobalOptional 注册应用核心的全局可选项
func (cApp *CmdApplication) RegisterCoreGlobalOptional() {
	if cApp.GetApplication() == nil {
		panic(errors.New("application of ApplicationCmdRegister is nil, please RegisterApplication first"))
	}
	// 注册应用核心的全局可选项
	cApp.GetApplication().(frame.ApplicationCmdRegister).RegisterCoreGlobalOptional(cApp.coreApp)
}

// RegisterLoggerWithOriginToContainer 注册配置文件预定义的不同来源(LogOrigin)的子日志器初始化器到容器
func (cApp *CmdApplication) RegisterLoggerWithOriginToContainer() {
	logOriginMap := cApp.GetContext().GetConfig().GetLogOriginMap()
	gm := cApp.GetContext().GetContainer()
	for originKey, logOriginVal := range logOriginMap {
		if originKey != "" {
			gm.Register(logOriginVal.InstanceKey(), func() (interface{}, error) {
				log := cApp.GetContext().GetLogger().With().Str("Origin", logOriginVal.String()).Logger()
				return &log, nil
			})
		}
	}
}

// RegisterApplication 注册应用命令注册器到应用启动器
func (cApp *CmdApplication) RegisterApplication(application frame.ApplicationCmdRegister) {
	cApp.application = application
}

// GetApplication 获取应用接口对象
func (cApp *CmdApplication) GetApplication() frame.IApplication {
	return cApp.application
}
