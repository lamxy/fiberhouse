package task

import (
	"testing"
)

func TestNewExampleChangedTaskUsesStableTypeAndPayload(t *testing.T) {
	got, err := NewExampleChangedTask(nil, ExampleChangedPayload{
		ID:        " 507f1f77bcf86cd799439011 ",
		Operation: " update ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type() != TypeExampleChanged {
		t.Fatalf("type = %q, want %q", got.Type(), TypeExampleChanged)
	}
	if string(got.Payload()) != `{"id":"507f1f77bcf86cd799439011","operation":"update"}` {
		t.Fatalf("payload = %s", got.Payload())
	}
}

func TestNewExampleChangedTaskRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		payload ExampleChangedPayload
	}{
		{name: "id", payload: ExampleChangedPayload{Operation: "create"}},
		{name: "operation", payload: ExampleChangedPayload{ID: "507f1f77bcf86cd799439011"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExampleChangedTask(nil, tt.payload)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got != nil {
				t.Fatalf("task = %#v, want nil", got)
			}
		})
	}
}
