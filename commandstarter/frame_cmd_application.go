// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package commandstarter 提供基于 cli.v2 的命令行应用启动器实现，负责命令行应用的完整生命周期管理和启动流程编排。
package commandstarter

import (
	"errors"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/globalmanager"
)

// RunCommandStarter 运行命令启动器
//
// 命令执行完毕后释放全局对象。此前 CLI 没有任何关闭段：应用注册的 MySQL 连接池、
// Mongo 客户端与 Redis 客户端从未 Close，只能等进程退出由 OS 回收。短命命令影响
// 有限，但常驻型 CLI（daemon、cron worker、批处理）会真实泄漏，且 Mongo/Redis 的
// 后台 goroutine 不会停止。
//
// 不复用 Web 侧的 clearApplicationGlobals：它第一步要停止健康检查，而那依赖
// IApplicationContext.GetStarterApp() 返回的 ApplicationStarter；CLI 的
// ICommandContext.GetStarterApp() 返回的是 CommandStarter，类型不同。CLI 的健康
// 检查本身也是一次性的（注册阶段执行一轮，没有后台 ticker），没有需要停止的东西，
// 因此这里只做「释放资源」与「清空条目」两步。
func RunCommandStarter(starter fiberhouse.CommandStarter) {
	starter.InitCoreApp()
	starter.RegisterGlobalErrHandler(starter.GetFrameCmdApp())
	starter.RegisterCommands(starter.GetFrameCmdApp())
	starter.RegisterCoreGlobalOptional(starter.GetFrameCmdApp())
	starter.RegisterApplicationGlobals()
	defer releaseCommandGlobals(starter)
	_ = starter.AppCoreRun()
}

// releaseCommandGlobals 释放 CLI 注册的全局对象，并按 key 上报关闭失败。
//
// 用 defer 调用：命令自身返回错误、或 urfave/cli 的 action 触发退出流程时，资源
// 同样需要回收，不能只在正常返回时执行。
//
// 关闭失败不中断回收——一个资源关不掉不应阻止其余资源被释放，因此只上报、不返回
// 错误。这与 Web 侧 releaseApplicationGlobals 的取舍一致。
//
// 全程做 nil 防护：starter 的实现可能不持有真实上下文（例如只记录调用顺序的测试
// 替身），此时应安静跳过而不是 panic。
func releaseCommandGlobals(starter fiberhouse.CommandStarter) {
	if starter == nil {
		return
	}
	ctx := starter.GetContext()
	if ctx == nil {
		return
	}
	container := ctx.GetContainer()
	if container == nil {
		return
	}

	logger, cfg := ctx.GetLogger(), ctx.GetConfig()
	container.ReleaseAllWithObserver(func(name globalmanager.KeyName, err error) {
		if logger == nil || cfg == nil {
			return
		}
		logger.Error(cfg.LogOriginCMD()).Err(err).
			Msgf("failed to release global object '%s' during command shutdown", name)
	}, true)
	container.ClearAll(true)
}

// FrameCmdApplication 是命令行应用启动器，实现 fiberhouse.FrameCmdStarter 接口。
// 负责管理命令行应用的生命周期，包括初始化、注册全局选项和动作、运行应用及错误处理。
type FrameCmdApplication struct {
	Ctx         fiberhouse.ICommandContext
	application fiberhouse.ApplicationCmdRegister
}

// ErrNoCmdStarterOption 标记未提供任何命令启动器选项。
//
// 装配期的用法错误：没有选项就没有注册器，启动器无事可做。
var ErrNoCmdStarterOption = errors.New(
	"fiberhouse/commandstarter: no registrar option available for injection into the command starter")

// NewFrameCmdApplication 创建一个命令启动器对象，实现FrameCmdStarter接口
//
// 未提供选项时 panic。需要处理该错误的调用方请改用
// NewFrameCmdApplicationWithError。
func NewFrameCmdApplication(ctx fiberhouse.ICommandContext, opts ...fiberhouse.FrameCmdStarterOption) fiberhouse.FrameCmdStarter {
	starter, err := NewFrameCmdApplicationWithError(ctx, opts...)
	if err != nil {
		panic(err)
	}
	return starter
}

// NewFrameCmdApplicationWithError 与 NewFrameCmdApplication 相同，但未提供选项时
// 返回错误而非 panic。
func NewFrameCmdApplicationWithError(ctx fiberhouse.ICommandContext, opts ...fiberhouse.FrameCmdStarterOption) (fiberhouse.FrameCmdStarter, error) {
	fca := &FrameCmdApplication{
		Ctx: ctx,
	}

	if len(opts) == 0 {
		if ctx != nil && ctx.GetLogger() != nil && ctx.GetConfig() != nil {
			ctx.GetLogger().ErrorWith(ctx.GetConfig().LogOriginFrame()).Msg(ErrNoCmdStarterOption.Error())
		}
		return nil, ErrNoCmdStarterOption
	}

	for _, opt := range opts {
		opt(fca)
	}

	return fca, nil
}

// GetContext 获取全局上下文
func (fca *FrameCmdApplication) GetContext() fiberhouse.ICommandContext {
	return fca.Ctx
}

// GetFrameCmdApp 获取框架命令行启动器实例
func (fca *FrameCmdApplication) GetFrameCmdApp() fiberhouse.FrameCmdStarter {
	return fca
}

// RegisterApplicationGlobals 注册应用全局对象初始化器和初始化部分必要对象
func (fca *FrameCmdApplication) RegisterApplicationGlobals() {
	// 注册配置文件预定义不同来源(LogOrigin)的子日志器初始化器到容器
	fca.RegisterLoggerWithOriginToContainer()
	// 注册自定义的应用全局初始化器和启动必要的全局单例
	fca.GetApplication().(fiberhouse.ApplicationCmdRegister).RegisterApplicationGlobals()

	// 全局对象健康检查（仅观测上报，不做重建）
	if fiberhouse.GlobalsHealthCheckEnabled(fca.GetContext().GetConfig(), fca.GetContext().GetLogger()) {
		fca.startHealthCheck()
	}
}

// startHealthCheck 同步执行一轮全局对象健康检查，仅观测与上报，不做任何重建。
//
// 命令行应用为一次性执行模型，此处不启动后台 ticker，仅在注册阶段检查一次。
// 不做重建的原因参见 FrameApplication.checkGlobalsHealthOnce 的说明。
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
			log.Error(cfg.LogOriginCMD()).Msgf("global resource '%s' is unhealthy, waiting for driver-level recovery", name)
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
func (fca *FrameCmdApplication) RegisterApplication(application fiberhouse.ApplicationCmdRegister) {
	fca.application = application
}

// GetApplication 获取应用接口对象
func (fca *FrameCmdApplication) GetApplication() fiberhouse.IApplication {
	return fca.application
}
