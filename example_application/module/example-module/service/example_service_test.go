package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type contextKey string

type fakeExampleStore struct {
	createFn func(context.Context, *entity.Example) error
	getFn    func(context.Context, string) (*entity.Example, error)
	listFn   func(context.Context, repository.ListOptions) ([]entity.Example, int64, error)
	updateFn func(context.Context, string, *entity.Example) error
	deleteFn func(context.Context, string) error
}

func (f *fakeExampleStore) Create(ctx context.Context, example *entity.Example) error {
	return f.createFn(ctx, example)
}
func (f *fakeExampleStore) Get(ctx context.Context, id string) (*entity.Example, error) {
	return f.getFn(ctx, id)
}
func (f *fakeExampleStore) List(ctx context.Context, opts repository.ListOptions) ([]entity.Example, int64, error) {
	return f.listFn(ctx, opts)
}
func (f *fakeExampleStore) Update(ctx context.Context, id string, example *entity.Example) error {
	return f.updateFn(ctx, id, example)
}
func (f *fakeExampleStore) Delete(ctx context.Context, id string) error {
	return f.deleteFn(ctx, id)
}

func TestListExamplesReqVoNormalizeAndValidationBoundary(t *testing.T) {
	normalized := (requestvo.ListExamplesReqVo{}).Normalize()
	if normalized.Page != 1 || normalized.PageSize != 20 {
		t.Fatalf("Normalize() = page %d size %d, want 1 and 20", normalized.Page, normalized.PageSize)
	}

	validate := validator.New()
	if err := validate.Struct(requestvo.ListExamplesReqVo{Page: 1, PageSize: 100}); err != nil {
		t.Fatalf("page_size=100 should be valid: %v", err)
	}
	if err := validate.Struct(requestvo.ListExamplesReqVo{Page: 1, PageSize: 101}); err == nil {
		t.Fatal("page_size=101 should be invalid")
	}
}

func TestCreateTrimsInputDefaultsStatusMapsResponseAndPreservesContext(t *testing.T) {
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	id := bson.NewObjectID()
	store := &fakeExampleStore{}
	store.createFn = func(gotCtx context.Context, got *entity.Example) error {
		if gotCtx != ctx {
			t.Fatal("Create did not receive caller context")
		}
		want := &entity.Example{
			Name:        "Example",
			Description: "description",
			Status:      entity.ExampleStatusActive,
			Tags:        []string{"go", "mongo"},
			Timestamps:  fields.Timestamps{CreatedAt: now.UTC(), UpdatedAt: now.UTC()},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Create entity = %#v, want %#v", got, want)
		}
		got.ID = id
		return nil
	}
	service := &ExampleService{Store: store, now: func() time.Time { return now }}

	resp, err := service.Create(ctx, requestvo.CreateExampleReqVo{
		Name:        "  Example  ",
		Description: "  description  ",
		Tags:        []string{" go ", "mongo "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != id.Hex() || resp.Name != "Example" || resp.Status != "active" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if !reflect.DeepEqual(resp.Tags, []string{"go", "mongo"}) {
		t.Fatalf("response tags = %#v", resp.Tags)
	}
}

func TestUpdatePreservesUnspecifiedFields(t *testing.T) {
	now := time.Date(2026, 7, 24, 2, 0, 0, 0, time.UTC)
	created := now.Add(-time.Hour)
	name := "  renamed  "
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	existing := &entity.Example{
		ID:          bson.NewObjectID(),
		Name:        "old",
		Description: "keep",
		Status:      entity.ExampleStatusArchived,
		Tags:        []string{"keep"},
		Timestamps:  fields.Timestamps{CreatedAt: created, UpdatedAt: created},
	}
	store := &fakeExampleStore{
		getFn: func(got context.Context, id string) (*entity.Example, error) {
			if got != ctx {
				t.Fatal("Get did not receive caller context")
			}
			copy := *existing
			return &copy, nil
		},
		updateFn: func(got context.Context, id string, example *entity.Example) error {
			if got != ctx {
				t.Fatal("Update did not receive caller context")
			}
			if example.Name != "renamed" || example.Description != "keep" ||
				example.Status != entity.ExampleStatusArchived ||
				!reflect.DeepEqual(example.Tags, []string{"keep"}) {
				t.Fatalf("partial update lost fields: %#v", example)
			}
			if !example.CreatedAt.Equal(created) || !example.UpdatedAt.Equal(now) {
				t.Fatalf("timestamps = %#v", example.Timestamps)
			}
			return nil
		},
	}
	service := &ExampleService{Store: store, now: func() time.Time { return now }}

	resp, err := service.Update(ctx, existing.ID.Hex(), requestvo.UpdateExampleReqVo{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "renamed" || resp.Description != "keep" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestListReturnsEmptyJSONArrayAndPropagatesErrors(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	sentinel := errors.New("store failed")
	store := &fakeExampleStore{
		listFn: func(got context.Context, opts repository.ListOptions) ([]entity.Example, int64, error) {
			if got != ctx {
				t.Fatal("List did not receive caller context")
			}
			if opts.Page != 1 || opts.PageSize != 20 || opts.Status != entity.ExampleStatusActive {
				t.Fatalf("list options = %#v", opts)
			}
			return nil, 0, nil
		},
		getFn: func(context.Context, string) (*entity.Example, error) { return nil, sentinel },
	}
	service := &ExampleService{Store: store}

	resp, err := service.List(ctx, requestvo.ListExamplesReqVo{Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Items == nil || len(resp.Items) != 0 {
		t.Fatalf("Items = %#v, want non-nil empty slice", resp.Items)
	}
	if _, err = service.Get(ctx, "id"); !errors.Is(err, sentinel) {
		t.Fatalf("Get error = %v, want sentinel", err)
	}
}
