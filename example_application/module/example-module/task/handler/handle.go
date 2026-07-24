// Package handler consumes asynq tasks produced by the example module's
// service layer (see task.NewExampleChangedTask). Handlers are
// read-only/observational with respect to the example domain: they log the
// notification and must not perform a second write against the store.
package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
)

// HandleExampleChangedTask consumes the mutation notification without
// replacing the caller's context or performing a second write. It only
// reads the fiberhouse.IApplicationContext already stored on ctx (via
// fiberhouse.ContextKeyAppCtx) — it never constructs or substitutes its own
// context, since asynq's worker context carries request-scoped values the
// handler must respect.
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
