package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/service"
	"github.com/urfave/cli/v2"
)

type fakeUseCase struct {
	called string
	ctx    context.Context
	create service.CreateInput
	list   service.ListInput
	update service.UpdateInput
	id     uint64
	record *entity.ExampleRecord
	result *service.ListResult
	err    error
}

func (f *fakeUseCase) Migrate(ctx context.Context) error {
	f.called, f.ctx = "migrate", ctx
	return f.err
}
func (f *fakeUseCase) Create(ctx context.Context, input service.CreateInput) (*entity.ExampleRecord, error) {
	f.called, f.ctx, f.create = "create", ctx, input
	return f.record, f.err
}
func (f *fakeUseCase) Get(ctx context.Context, id uint64) (*entity.ExampleRecord, error) {
	f.called, f.ctx, f.id = "get", ctx, id
	return f.record, f.err
}
func (f *fakeUseCase) List(ctx context.Context, input service.ListInput) (*service.ListResult, error) {
	f.called, f.ctx, f.list = "list", ctx, input
	return f.result, f.err
}
func (f *fakeUseCase) Update(ctx context.Context, id uint64, input service.UpdateInput) (*entity.ExampleRecord, error) {
	f.called, f.ctx, f.id, f.update = "update", ctx, id, input
	return f.record, f.err
}
func (f *fakeUseCase) Delete(ctx context.Context, id uint64) error {
	f.called, f.ctx, f.id = "delete", ctx, id
	return f.err
}

func runExampleCommand(t *testing.T, useCase service.ExampleUseCase, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	app := cli.NewApp()
	app.Writer = &out
	app.ErrWriter = &out
	app.Commands = []*cli.Command{newExampleCommand(useCase).GetCommand().(*cli.Command)}
	err := app.RunContext(context.WithValue(context.Background(), "trace", "cli-17"), append([]string{"test"}, args...))
	return out.String(), err
}

func TestExampleCommandSubcommandsAndJSON(t *testing.T) {
	when := time.Date(2026, 7, 24, 1, 2, 3, 0, time.UTC)
	record := &entity.ExampleRecord{ID: 7, Name: "alpha", Description: "first", Status: "active", CreatedAt: when, UpdatedAt: when}
	tests := []struct {
		name   string
		args   []string
		called string
		output any
	}{
		{"migrate", []string{"example", "migrate"}, "migrate", map[string]string{"status": "ok"}},
		{"create", []string{"example", "create", "--name", "alpha"}, "create", record},
		{"get", []string{"example", "get", "--id", "7"}, "get", record},
		{"list", []string{"example", "list"}, "list", &service.ListResult{Items: []entity.ExampleRecord{*record}, Page: 1, PageSize: 20, Total: 1}},
		{"update", []string{"example", "update", "--id", "7", "--description", ""}, "update", record},
		{"delete", []string{"example", "delete", "--id", "7"}, "delete", map[string]string{"status": "ok"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeUseCase{record: record, result: &service.ListResult{Items: []entity.ExampleRecord{*record}, Page: 1, PageSize: 20, Total: 1}}
			got, err := runExampleCommand(t, fake, tt.args...)
			if err != nil {
				t.Fatalf("run command: %v", err)
			}
			wantJSON, err := json.Marshal(tt.output)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(wantJSON)+"\n" {
				t.Fatalf("output = %q, want %q", got, string(wantJSON)+"\n")
			}
			if fake.called != tt.called {
				t.Fatalf("called = %q, want %q", fake.called, tt.called)
			}
			if fake.ctx.Value("trace") != "cli-17" {
				t.Fatal("CLI context was not propagated")
			}
		})
	}
}

func TestExampleCommandDefaultsAndExplicitEmptyUpdate(t *testing.T) {
	fake := &fakeUseCase{result: &service.ListResult{Items: []entity.ExampleRecord{}, Page: 1, PageSize: 20}}
	if _, err := runExampleCommand(t, fake, "example", "list"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if fake.list.Page != 1 || fake.list.PageSize != 20 {
		t.Fatalf("list defaults = %#v", fake.list)
	}

	fake.record = &entity.ExampleRecord{ID: 3}
	if _, err := runExampleCommand(t, fake, "example", "update", "--id", "3", "--description", ""); err != nil {
		t.Fatalf("update: %v", err)
	}
	if fake.update.Description == nil || *fake.update.Description != "" {
		t.Fatalf("explicit empty description lost: %#v", fake.update.Description)
	}
}

func TestExampleCommandRejectsInvalidArgumentsBeforeCallingService(t *testing.T) {
	tests := [][]string{
		{"example", "create"},
		{"example", "create", "--name", "alpha", "--status", "pending"},
		{"example", "get", "--id", "0"},
		{"example", "list", "--status", "pending"},
		{"example", "list", "--page-size", "101"},
		{"example", "update", "--id", "2"},
		{"example", "delete", "--id", "not-a-number"},
	}
	for _, args := range tests {
		fake := &fakeUseCase{}
		if _, err := runExampleCommand(t, fake, args...); err == nil {
			t.Fatalf("%v returned nil error", args)
		}
		if fake.called != "" {
			t.Fatalf("%v called service method %q", args, fake.called)
		}
	}
}

func TestExampleCommandReturnsUseCaseErrorsWithoutSuccessOutput(t *testing.T) {
	fake := &fakeUseCase{err: errors.New("database unavailable")}
	output, err := runExampleCommand(t, fake, "example", "migrate")
	if err == nil || err.Error() != "database unavailable" {
		t.Fatalf("error = %v", err)
	}
	if output != "" {
		t.Fatalf("output = %q, want empty", output)
	}
}
