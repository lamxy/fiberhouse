// Package task defines the async notification contract dispatched by
// service after a successful example mutation, and constructs the asynq
// task carrying it. The paired consumer lives in task/handler.
package task

import (
	"errors"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
)

// TypeExampleChanged is the asynq task type name for example mutation
// notifications, shared by NewExampleChangedTask (producer) and
// handler.HandleExampleChangedTask (consumer).
const TypeExampleChanged = "example:changed"

// ExampleChangedPayload is the stable wire contract emitted after a canonical
// example mutation succeeds.
type ExampleChangedPayload struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
}

// NewExampleChangedTask validates payload (ID and Operation must be
// non-blank after trimming) and encodes it into an asynq.Task of type
// TypeExampleChanged, using ctx's configured JSON handler.
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
