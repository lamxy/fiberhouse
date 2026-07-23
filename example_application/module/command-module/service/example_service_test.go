package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/repository"
)

type fakeRepository struct {
	ctx        context.Context
	create     *entity.ExampleRecord
	list       repository.ListOptions
	updateID   uint64
	update     repository.UpdateInput
	deleteID   uint64
	result     *entity.ExampleRecord
	listResult []entity.ExampleRecord
	total      int64
	err        error
}

func (f *fakeRepository) Migrate(ctx context.Context) error {
	f.ctx = ctx
	return f.err
}
func (f *fakeRepository) Create(ctx context.Context, record *entity.ExampleRecord) error {
	f.ctx, f.create = ctx, record
	return f.err
}
func (f *fakeRepository) Get(ctx context.Context, id uint64) (*entity.ExampleRecord, error) {
	f.ctx = ctx
	return f.result, f.err
}
func (f *fakeRepository) List(ctx context.Context, input repository.ListOptions) ([]entity.ExampleRecord, int64, error) {
	f.ctx, f.list = ctx, input
	return f.listResult, f.total, f.err
}
func (f *fakeRepository) Update(ctx context.Context, id uint64, input repository.UpdateInput) (*entity.ExampleRecord, error) {
	f.ctx, f.updateID, f.update = ctx, id, input
	return f.result, f.err
}
func (f *fakeRepository) Delete(ctx context.Context, id uint64) error {
	f.ctx, f.deleteID = ctx, id
	return f.err
}

func TestServiceCreateNormalizesInputAndPropagatesContext(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewExampleMysqlService(repo)
	ctx := context.WithValue(context.Background(), "trace", "abc")

	_, err := svc.Create(ctx, CreateInput{Name: "  alpha  ", Description: " first "})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.ctx != ctx {
		t.Fatal("Create() did not propagate context")
	}
	if repo.create.Name != "alpha" || repo.create.Description != "first" || repo.create.Status != "active" {
		t.Fatalf("normalized record = %#v", repo.create)
	}
}

func TestServicePropagatesContextAcrossUseCaseMethods(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *ExampleMysqlService) error
	}{
		{"migrate", func(ctx context.Context, svc *ExampleMysqlService) error {
			return svc.Migrate(ctx)
		}},
		{"get", func(ctx context.Context, svc *ExampleMysqlService) error {
			_, err := svc.Get(ctx, 7)
			return err
		}},
		{"list", func(ctx context.Context, svc *ExampleMysqlService) error {
			_, err := svc.List(ctx, ListInput{})
			return err
		}},
		{"update", func(ctx context.Context, svc *ExampleMysqlService) error {
			name := "updated"
			_, err := svc.Update(ctx, 7, UpdateInput{Name: &name})
			return err
		}},
		{"delete", func(ctx context.Context, svc *ExampleMysqlService) error {
			return svc.Delete(ctx, 7)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{result: &entity.ExampleRecord{ID: 7}}
			svc := NewExampleMysqlService(repo)
			ctx := context.WithValue(context.Background(), "trace", tt.name)

			if err := tt.run(ctx, svc); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			if repo.ctx != ctx {
				t.Fatalf("%s did not propagate the caller context", tt.name)
			}
		})
	}
}

func TestServiceCreateAcceptsMultibyteCharactersUpToMySQLVarcharLimits(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewExampleMysqlService(repo)
	name := strings.Repeat("界", 80)
	description := strings.Repeat("🙂", 500)

	_, err := svc.Create(context.Background(), CreateInput{
		Name:        "  " + name + "  ",
		Description: "  " + description + "  ",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.create.Name != name {
		t.Fatalf("name rune count = %d, want 80", len([]rune(repo.create.Name)))
	}
	if repo.create.Description != description {
		t.Fatalf("description rune count = %d, want 500", len([]rune(repo.create.Description)))
	}
}

func TestServiceListNormalizesPagination(t *testing.T) {
	repo := &fakeRepository{total: 3}
	svc := NewExampleMysqlService(repo)

	result, err := svc.List(context.Background(), ListInput{Page: -3, PageSize: 0})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.list.Page != 1 || repo.list.PageSize != 20 {
		t.Fatalf("repository options = %#v", repo.list)
	}
	if result.Page != 1 || result.PageSize != 20 || result.Total != 3 {
		t.Fatalf("result = %#v", result)
	}
}

func TestServiceRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		run  func(*ExampleMysqlService) error
	}{
		{"empty name", func(s *ExampleMysqlService) error {
			_, err := s.Create(context.Background(), CreateInput{Name: " "})
			return err
		}},
		{"invalid status", func(s *ExampleMysqlService) error {
			_, err := s.Create(context.Background(), CreateInput{Name: "alpha", Status: "pending"})
			return err
		}},
		{"page size too large", func(s *ExampleMysqlService) error {
			_, err := s.List(context.Background(), ListInput{PageSize: 101})
			return err
		}},
		{"empty update", func(s *ExampleMysqlService) error {
			_, err := s.Update(context.Background(), 1, UpdateInput{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(NewExampleMysqlService(&fakeRepository{})); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceUpdateMapsOnlyProvidedFields(t *testing.T) {
	repo := &fakeRepository{result: &entity.ExampleRecord{ID: 4}}
	svc := NewExampleMysqlService(repo)
	empty := ""
	status := "archived"

	_, err := svc.Update(context.Background(), 4, UpdateInput{Description: &empty, Status: &status})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repo.update.Description == nil || *repo.update.Description != "" {
		t.Fatalf("description update = %#v", repo.update.Description)
	}
	if repo.update.Status == nil || *repo.update.Status != "archived" {
		t.Fatalf("status update = %#v", repo.update.Status)
	}
	if repo.update.Name != nil {
		t.Fatalf("unexpected name update = %#v", repo.update.Name)
	}
}

func TestServiceUpdateAcceptsMultibyteCharactersUpToMySQLVarcharLimits(t *testing.T) {
	repo := &fakeRepository{result: &entity.ExampleRecord{ID: 4}}
	svc := NewExampleMysqlService(repo)
	name := "  " + strings.Repeat("界", 80) + "  "
	description := "  " + strings.Repeat("🙂", 500) + "  "

	_, err := svc.Update(context.Background(), 4, UpdateInput{
		Name:        &name,
		Description: &description,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repo.update.Name == nil || len([]rune(*repo.update.Name)) != 80 {
		t.Fatalf("name update = %#v, want 80 characters", repo.update.Name)
	}
	if repo.update.Description == nil || len([]rune(*repo.update.Description)) != 500 {
		t.Fatalf("description update = %#v, want 500 characters", repo.update.Description)
	}
}
