package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type fakeModelStore struct {
	insertFn  func(context.Context, *entity.Example) (bson.ObjectID, error)
	findIDFn  func(context.Context, bson.ObjectID) (*entity.Example, error)
	findFn    func(context.Context, model.ExampleFilter) ([]entity.Example, int64, error)
	replaceFn func(context.Context, bson.ObjectID, *entity.Example) (bool, error)
	deleteFn  func(context.Context, bson.ObjectID) (bool, error)
}

func (*fakeModelStore) EnsureIndexes(context.Context) error { return nil }
func (f *fakeModelStore) Insert(ctx context.Context, e *entity.Example) (bson.ObjectID, error) {
	return f.insertFn(ctx, e)
}
func (f *fakeModelStore) FindByID(ctx context.Context, id bson.ObjectID) (*entity.Example, error) {
	return f.findIDFn(ctx, id)
}
func (f *fakeModelStore) Find(ctx context.Context, filter model.ExampleFilter) ([]entity.Example, int64, error) {
	return f.findFn(ctx, filter)
}
func (f *fakeModelStore) Replace(ctx context.Context, id bson.ObjectID, e *entity.Example) (bool, error) {
	return f.replaceFn(ctx, id, e)
}
func (f *fakeModelStore) Delete(ctx context.Context, id bson.ObjectID) (bool, error) {
	return f.deleteFn(ctx, id)
}

func TestRepositoryValidatesIDsAndTranslatesStableErrors(t *testing.T) {
	repo := &ExampleRepository{Model: &fakeModelStore{}}
	if _, err := repo.Get(context.Background(), "bad"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("Get error = %v, want ErrInvalidID", err)
	}

	repo.Model = &fakeModelStore{findIDFn: func(context.Context, bson.ObjectID) (*entity.Example, error) {
		return nil, mongo.ErrNoDocuments
	}}
	if _, err := repo.Get(context.Background(), bson.NewObjectID().Hex()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
}

func TestRepositoryAssignsInsertedIDAndPassesContext(t *testing.T) {
	type key string
	ctx := context.WithValue(context.Background(), key("request"), "same")
	id := bson.NewObjectID()
	example := &entity.Example{Name: "name"}
	repo := &ExampleRepository{Model: &fakeModelStore{
		insertFn: func(got context.Context, gotExample *entity.Example) (bson.ObjectID, error) {
			if got != ctx || gotExample != example {
				t.Fatal("Create did not preserve context/entity identity")
			}
			return id, nil
		},
	}}
	if err := repo.Create(ctx, example); err != nil {
		t.Fatal(err)
	}
	if example.ID != id {
		t.Fatalf("ID = %s, want %s", example.ID, id)
	}
}

func TestRepositoryListMapsOptions(t *testing.T) {
	ctx := context.Background()
	repo := &ExampleRepository{Model: &fakeModelStore{
		findFn: func(got context.Context, filter model.ExampleFilter) ([]entity.Example, int64, error) {
			if got != ctx {
				t.Fatal("List did not preserve context")
			}
			want := model.ExampleFilter{Page: 2, PageSize: 50, Status: entity.ExampleStatusArchived}
			if filter != want {
				t.Fatalf("filter = %#v, want %#v", filter, want)
			}
			return []entity.Example{}, 7, nil
		},
	}}
	items, total, err := repo.List(ctx, ListOptions{Page: 2, PageSize: 50, Status: entity.ExampleStatusArchived})
	if err != nil || total != 7 || items == nil {
		t.Fatalf("List = %#v, %d, %v", items, total, err)
	}
}

func TestRepositoryTranslatesDuplicateName(t *testing.T) {
	repo := &ExampleRepository{Model: &fakeModelStore{
		insertFn: func(context.Context, *entity.Example) (bson.ObjectID, error) {
			return bson.NilObjectID, mongo.WriteException{WriteErrors: mongo.WriteErrors{
				{Code: 11000, Message: "duplicate key"},
			}}
		},
	}}
	if err := repo.Create(context.Background(), &entity.Example{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("Create error = %v, want ErrConflict", err)
	}
}
