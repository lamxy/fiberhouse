// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package tasklog

import (
	"github.com/lamxy/fiberhouse"
)

// TaskLoggerAdapter 任务日志器适配器，实现 asynq.Logger 接口，用于将任务日志接入全局日志系统
type TaskLoggerAdapter struct {
	Ctx fiberhouse.IContext
}

func NewTaskLoggerAdapter(ctx fiberhouse.IContext) *TaskLoggerAdapter {
	return &TaskLoggerAdapter{
		Ctx: ctx,
	}
}

func (tLog *TaskLoggerAdapter) Debug(args ...interface{}) {
	var msg string
	if len(args) > 0 {
		if v, ok := args[0].(string); ok {
			msg = v
		}
	}
	tLog.Ctx.GetLogger().Debug(tLog.Ctx.GetConfig().LogOriginTask()).Str("Component", "Asynq").Msg(msg)
}

func (tLog *TaskLoggerAdapter) Info(args ...interface{}) {
	var msg string
	if len(args) > 0 {
		if v, ok := args[0].(string); ok {
			msg = v
		}
	}
	tLog.Ctx.GetLogger().Info(tLog.Ctx.GetConfig().LogOriginTask()).Str("Component", "Asynq").Msg(msg)
}

func (tLog *TaskLoggerAdapter) Warn(args ...interface{}) {
	var msg string
	if len(args) > 0 {
		if v, ok := args[0].(string); ok {
			msg = v
		}
	}
	tLog.Ctx.GetLogger().Warn(tLog.Ctx.GetConfig().LogOriginTask()).Str("Component", "Asynq").Msg(msg)
}

func (tLog *TaskLoggerAdapter) Error(args ...interface{}) {
	var msg string
	if len(args) > 0 {
		if v, ok := args[0].(string); ok {
			msg = v
		}
	}
	tLog.Ctx.GetLogger().Error(tLog.Ctx.GetConfig().LogOriginTask()).Str("Component", "Asynq").Msg(msg)
}

func (tLog *TaskLoggerAdapter) Fatal(args ...interface{}) {
	var msg string
	if len(args) > 0 {
		if v, ok := args[0].(string); ok {
			msg = v
		}
	}
	tLog.Ctx.GetLogger().Fatal(tLog.Ctx.GetConfig().LogOriginTask()).Str("Component", "Asynq").Msg(msg)
}
