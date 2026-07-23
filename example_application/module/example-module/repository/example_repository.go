package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrInvalidID = errors.New("invalid example id")
	ErrNotFound  = errors.New("example not found")
	ErrConflict  = errors.New("example already exists")
	ErrUnchanged = errors.New("example unchanged")
)

type ExampleModelStore interface {
	EnsureIndexes(context.Context) error
	Insert(context.Context, *entity.Example) (bson.ObjectID, error)
	FindByID(context.Context, bson.ObjectID) (*entity.Example, error)
	Find(context.Context, model.ExampleFilter) ([]entity.Example, int64, error)
	Replace(context.Context, bson.ObjectID, *entity.Example) (bool, error)
	Delete(context.Context, bson.ObjectID) (bool, error)
}

type ListOptions struct {
	Page     int
	PageSize int
	Status   entity.ExampleStatus
}

type ExampleStore interface {
	Create(context.Context, *entity.Example) error
	Get(context.Context, string) (*entity.Example, error)
	List(context.Context, ListOptions) ([]entity.Example, int64, error)
	Update(context.Context, string, *entity.Example) error
	Delete(context.Context, string) error
}

type ExampleRepository struct {
	fiberhouse.RepositoryLocator
	Model ExampleModelStore

	readyOnce sync.Once
	readyErr  error
}

func NewExampleRepository(ctx fiberhouse.IApplicationContext, store ExampleModelStore) *ExampleRepository {
	return &ExampleRepository{
		RepositoryLocator: fiberhouse.NewRepository(ctx).SetName(GetKeyExampleRepository()),
		Model:             store,
	}
}

func GetKeyExampleRepository(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleRepository", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

func RegisterKeyExampleRepository(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleRepository(ns...), func() (interface{}, error) {
		return NewExampleRepository(ctx, model.NewExampleModel(ctx)), nil
	})
}

func (r *ExampleRepository) Create(ctx context.Context, example *entity.Example) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	id, err := r.Model.Insert(ctx, example)
	if err != nil {
		return translateModelError(err)
	}
	example.ID = id
	return nil
}

func (r *ExampleRepository) Get(ctx context.Context, rawID string) (*entity.Example, error) {
	if err := r.ready(ctx); err != nil {
		return nil, err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return nil, ErrInvalidID
	}
	example, err := r.Model.FindByID(ctx, id)
	if err != nil {
		return nil, translateModelError(err)
	}
	return example, nil
}

func (r *ExampleRepository) List(ctx context.Context, opts ListOptions) ([]entity.Example, int64, error) {
	if err := r.ready(ctx); err != nil {
		return nil, 0, err
	}
	examples, total, err := r.Model.Find(ctx, model.ExampleFilter{
		Page: opts.Page, PageSize: opts.PageSize, Status: opts.Status,
	})
	if err != nil {
		return nil, 0, translateModelError(err)
	}
	if examples == nil {
		examples = make([]entity.Example, 0)
	}
	return examples, total, nil
}

func (r *ExampleRepository) Update(ctx context.Context, rawID string, example *entity.Example) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ErrInvalidID
	}
	changed, err := r.Model.Replace(ctx, id, example)
	if err != nil {
		return translateModelError(err)
	}
	if !changed {
		return ErrUnchanged
	}
	return nil
}

func (r *ExampleRepository) Delete(ctx context.Context, rawID string) error {
	if err := r.ready(ctx); err != nil {
		return err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ErrInvalidID
	}
	deleted, err := r.Model.Delete(ctx, id)
	if err != nil {
		return translateModelError(err)
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func (r *ExampleRepository) ready(ctx context.Context) error {
	r.readyOnce.Do(func() {
		r.readyErr = r.Model.EnsureIndexes(ctx)
	})
	return r.readyErr
}

func translateModelError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return ErrNotFound
	case mongo.IsDuplicateKeyError(err):
		return ErrConflict
	default:
		return err
	}
}

// CreateExample temporarily supports the old demonstration service while the
// transport layer is migrated in the next slice.
func (r *ExampleRepository) CreateExample(ctx context.Context, req *requestvo.ExampleReqVo) (string, error) {
	example := &entity.Example{Name: req.ExamName, Status: entity.ExampleStatusActive}
	if err := r.Create(ctx, example); err != nil {
		return "", err
	}
	return example.ID.Hex(), nil
}
