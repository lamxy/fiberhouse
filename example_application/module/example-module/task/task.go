package task

import (
	"errors"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
)

const TypeExampleChanged = "example:changed"

// ExampleChangedPayload is the stable wire contract emitted after a canonical
// example mutation succeeds.
type ExampleChangedPayload struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
}

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
