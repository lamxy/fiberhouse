package handler

import (
	"testing"

	"github.com/hibiken/asynq"
	exampletask "github.com/lamxy/fiberhouse/example_application/module/example-module/task"
)

func TestHandleExampleChangedTaskAcceptsPayloadWithoutApplicationContext(t *testing.T) {
	changed, err := exampletask.NewExampleChangedTask(nil, exampletask.ExampleChangedPayload{
		ID: "507f1f77bcf86cd799439011", Operation: "update",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleExampleChangedTask(nil, changed); err != nil {
		t.Fatal(err)
	}
}

func TestHandleExampleChangedTaskRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "malformed", payload: `{`},
		{name: "empty id", payload: `{"id":"","operation":"update"}`},
		{name: "empty operation", payload: `{"id":"507f1f77bcf86cd799439011","operation":""}`},
		{name: "whitespace id", payload: `{"id":" \t ","operation":"update"}`},
		{name: "whitespace operation", payload: `{"id":"507f1f77bcf86cd799439011","operation":" \n "}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := asynq.NewTask(exampletask.TypeExampleChanged, []byte(tt.payload))
			if err := HandleExampleChangedTask(nil, changed); err == nil {
				t.Fatal("HandleExampleChangedTask() error = nil, want invalid payload error")
			}
		})
	}
}
