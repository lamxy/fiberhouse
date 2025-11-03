// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package commandstarter 提供基于 cli.v2 的命令行应用启动器实现，负责命令行应用的完整生命周期管理和启动流程编排。
package commandstarter

import (
	"github.com/lamxy/fiberhouse/frame"
)

// RunCommandStarter 运行命令启动器
func RunCommandStarter(starter frame.CommandStarter) {
	starter.InitCoreApp()
	starter.RegisterGlobalErrHandler(starter.GetFrameCmdApp())
	starter.RegisterCommands(starter.GetFrameCmdApp())
	starter.RegisterCoreGlobalOptional(starter.GetFrameCmdApp())
	starter.RegisterApplicationGlobals()
	_ = starter.AppCoreRun()
}

// FrameCmdApplication 是命令行应用启动器，实现 frame.FrameCmdStarter 接口。
// 负责管理命令行应用的生命周期，包括初始化、注册全局选项和动作、运行应用及错误处理。
type FrameCmdApplication struct {
	Ctx         frame.ContextCommander
	application frame.ApplicationCmdRegister
}

// NewFrameCmdApplication 创建一个命令启动器对象，实现FrameCmdStarter接口
func NewFrameCmdApplication(ctx frame.ContextCommander, opts ...frame.FrameCmdStarterOption) frame.FrameCmdStarter {
	fca := &FrameCmdApplication{
		Ctx: ctx,
	}

	if len(opts) == 0 {
		ctx.GetLogger().FatalWith(ctx.GetConfig().LogOriginFrame()).Msg("No registrar option available for injection into the command starter")
	}

	for _, opt := range opts {
		opt(fca)
	}

	return fca
}

// GetContext 获取全局上下文
func (fca *FrameCmdApplication) GetContext() frame.ContextCommander {
	return fca.Ctx
}

// GetFrameCmdApp 获取框架命令行启动器实例
func (fca *FrameCmdApplication) GetFrameCmdApp() frame.FrameCmdStarter {
	return fca
}

// RegisterApplicationGlobals 注册应用全局对象初始化器和初始化部分必要对象
func (fca *FrameCmdApplication) RegisterApplicationGlobals() {
	// 注册配置文件预定义不同来源(LogOrigin)的子日志器初始化器到容器
	fca.RegisterLoggerWithOriginToContainer()
	// 注册自定义的应用全局初始化器和启动必要的全局单例
	fca.GetApplication().(frame.ApplicationCmdRegister).RegisterApplicationGlobals()

	// 全局对象健康检查和重建
	if fca.GetContext().GetConfig().Bool("application.globalManage.keepAlive") {
		fca.startHealthCheck()
	}
}

// StartHealthCheck 异步检查全局对象是否健康和重建
func (fca *FrameCmdApplication) startHealthCheck() {
	gm, log, cfg := fca.GetContext().GetContainer(), fca.GetContext().GetLogger(), fca.GetContext().GetConfig()
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

// RegisterLoggerWithOriginToContainer 注册配置文件预定义的不同来源(LogOrigin)的子日志器初始化器到容器
func (fca *FrameCmdApplication) RegisterLoggerWithOriginToContainer() {
	logOriginMap := fca.GetContext().GetConfig().GetLogOriginMap()
	gm := fca.GetContext().GetContainer()
	for originKey, logOriginVal := range logOriginMap {
		if originKey != "" {
			gm.Register(logOriginVal.InstanceKey(), func() (interface{}, error) {
				log := fca.GetContext().GetLogger().With().Str("Origin", logOriginVal.String()).Logger()
				return &log, nil
			})
		}
	}
}

// RegisterApplication 注册应用命令注册器到应用启动器
func (fca *FrameCmdApplication) RegisterApplication(application frame.ApplicationCmdRegister) {
	fca.application = application
}

// GetApplication 获取应用接口对象
func (fca *FrameCmdApplication) GetApplication() frame.IApplication {
	return fca.application
}
