package handler

import (
	"testing"

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
