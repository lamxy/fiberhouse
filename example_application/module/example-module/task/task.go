// Package task 定义 service 在 example 写操作成功后派发的异步通知契约，并构造
// 承载它的 asynq 任务。配套的消费方位于 task/handler。
package task

import (
	"errors"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
)

// TypeExampleChanged 是 example 写操作通知的 asynq 任务类型名，由
// NewExampleChangedTask（生产方）与 handler.HandleExampleChangedTask（消费方）共享。
const TypeExampleChanged = "example:changed"

// ExampleChangedPayload 是规范的 example 写操作成功后发出的稳定传输契约。
type ExampleChangedPayload struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
}

// NewExampleChangedTask 校验 payload（ID 与 Operation 去除首尾空白后必须非空），
// 并使用 ctx 配置的 JSON 处理器将其编码为一个 TypeExampleChanged 类型的
// asynq.Task。
func NewExampleChangedTask(ctx fiberhouse.IContext, payload ExampleChangedPayload) (*asynq.Task, error) {
	payload.ID = strings.TrimSpace(payload.ID)
	payload.Operation = strings.TrimSpace(payload.Operation)
	if payload.ID == "" {
		return nil, errors.New("example changed payload id is required")
	}
	if payload.Operation == "" {
		return nil, errors.New("example changed payload operation is required")
	}

	codec := fiberhouse.NewPayloadBase().GetMustJsonHandler(ctx)
	encoded, err := codec.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExampleChanged, encoded), nil
}
