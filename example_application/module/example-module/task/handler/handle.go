// Package handler 消费由 example 模块 service 层生产的 asynq 任务
// （见 task.NewExampleChangedTask）。就 example 领域而言，处理器是只读/观测性的：
// 它们记录通知日志，绝不能对存储发起第二次写入。
package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
)

// HandleExampleChangedTask 消费写操作通知，不替换调用方的上下文，也不执行第二次
// 写入。它只读取已存放在 ctx 上的 fiberhouse.IApplicationContext（通过
// fiberhouse.ContextKeyAppCtx）——绝不构造或替换自己的上下文，因为 asynq 的
// worker 上下文携带着处理器必须尊重的请求作用域值。
func HandleExampleChangedTask(ctx context.Context, t *asynq.Task) error {
	if t == nil {
		return errors.New("example changed task is required")
	}

	var appCtx fiberhouse.IApplicationContext
	if ctx != nil {
		appCtx, _ = ctx.Value(fiberhouse.ContextKeyAppCtx).(fiberhouse.IApplicationContext)
	}
	var payload task.ExampleChangedPayload
	if err := fiberhouse.NewPayloadBase().GetMustJsonHandler(appCtx).Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}
	payload.ID = strings.TrimSpace(payload.ID)
	payload.Operation = strings.TrimSpace(payload.Operation)
	if payload.ID == "" || payload.Operation == "" {
		return errors.New("invalid example changed payload")
	}
	if appCtx != nil {
		appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginTask()).
			Str("example_id", payload.ID).
			Str("operation", payload.Operation).
			Msg("example change observed")
	}
	return nil
}
