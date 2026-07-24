// Package repository is the persistence-orchestration layer for the example
// module: it translates between service-facing string/domain types and the
// model layer's storage-facing types (e.g. hex id <-> bson.ObjectID),
// normalizes model errors into the stable sentinels defined below, and lazily
// ensures indexes exist. It depends on model (and, transitively, the
// database driver) but exposes no MongoDB-specific types to service.
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

// Stable domain errors returned by ExampleStore implementations. Callers
// (service, transport) must use errors.Is against these sentinels rather
// than matching on error strings.
var (
	// ErrInvalidID is returned when the supplied id is not a well-formed
	// identifier (e.g. not a valid hex ObjectID) before any store access.
	ErrInvalidID = errors.New("invalid example id")
	// ErrNotFound is returned when a lookup, update, or delete targets an id
	// that does not exist.
	ErrNotFound = errors.New("example not found")
	// ErrConflict is returned when a create violates a uniqueness constraint.
	ErrConflict = errors.New("example already exists")
	// ErrUnchanged is returned by Update when the target exists but the
	// write modified no documents (e.g. the patch is a no-op).
	ErrUnchanged = errors.New("example unchanged")
)

// ExampleModelStore is the storage-facing contract ExampleRepository depends
// on. Implementations operate on bson.ObjectID and raw entity.Example values
// and return driver-native errors (e.g. mongo.ErrNoDocuments); translation
// into the stable ErrInvalidID/ErrNotFound/ErrConflict/ErrUnchanged sentinels
// happens in this package via translateModelError, not in the model layer.
type ExampleModelStore interface {
	EnsureIndexes(context.Context) error
	Insert(context.Context, *entity.Example) (bson.ObjectID, error)
	FindByID(context.Context, bson.ObjectID) (*entity.Example, error)
	Find(context.Context, model.ExampleFilter) ([]entity.Example, int64, error)
	Replace(context.Context, bson.ObjectID, *entity.Example) (bool, error)
	Delete(context.Context, bson.ObjectID) (bool, error)
}

// ListOptions carries pagination and filtering parameters for ExampleStore.List.
type ListOptions struct {
	Page     int
	PageSize int
	Status   entity.ExampleStatus
}

// ExampleStore is the repository-layer contract consumed by service. All
// methods accept the caller's context.Context and propagate it unchanged to
// the model layer. Errors are the stable sentinels above (ErrInvalidID,
// ErrNotFound, ErrConflict, ErrUnchanged); callers should use errors.Is.
//
// Update replaces the mutable fields of an existing record and is not an
// upsert: if id does not resolve to an existing example it returns
// ErrNotFound, and if the replace is a true no-op it returns ErrUnchanged.
// List returns items and the total matching count for the given page,
// ordered deterministically (delegated to the model layer).
type ExampleStore interface {
	Create(context.Context, *entity.Example) error
	Get(context.Context, string) (*entity.Example, error)
	List(context.Context, ListOptions) ([]entity.Example, int64, error)
	Update(context.Context, string, *entity.Example) error
	Delete(context.Context, string) error
}

// ExampleRepository is the default ExampleStore implementation backed by an
// ExampleModelStore. It lazily ensures indexes on first use and translates
// model-layer errors into the stable ExampleStore sentinels.
type ExampleRepository struct {
	fiberhouse.RepositoryLocator
	Model ExampleModelStore

	readyMu     sync.Mutex
	initialized bool
}

// NewExampleRepository builds an ExampleRepository bound to the given
// application context and ExampleModelStore.
func NewExampleRepository(ctx fiberhouse.IApplicationContext, store ExampleModelStore) *ExampleRepository {
	return &ExampleRepository{
		RepositoryLocator: fiberhouse.NewRepository(ctx).SetName(GetKeyExampleRepository()),
		Model:             store,
	}
}

// GetKeyExampleRepository returns the registry key used to locate the
// ExampleRepository instance, optionally namespaced by ns.
func GetKeyExampleRepository(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleRepository", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleRepository registers a lazy initializer that builds an
// ExampleRepository backed by model.NewExampleModel, and returns its
// registry key.
func RegisterKeyExampleRepository(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleRepository(ns...), func() (interface{}, error) {
		return NewExampleRepository(ctx, model.NewExampleModel(ctx)), nil
	})
}

// Create inserts example and sets its generated ID on success.
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

// Get fetches an example by its hex-encoded id, returning ErrInvalidID if
// rawID is not a valid ObjectID hex string, or ErrNotFound if no such
// example exists.
func (r *ExampleRepository) Get(ctx context.Context, rawID string) (*entity.Example, error) {
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return nil, ErrInvalidID
	}
	if err := r.ready(ctx); err != nil {
		return nil, err
	}
	example, err := r.Model.FindByID(ctx, id)
	if err != nil {
		return nil, translateModelError(err)
	}
	return example, nil
}

// List returns a page of examples matching opts.Status (or all statuses if
// empty) plus the total matching count, in deterministic order. The
// returned slice is never nil, even when there are no matches.
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

// Update replaces the mutable fields of the example identified by rawID.
// This is not an upsert: it returns ErrInvalidID for a malformed id,
// ErrUnchanged if the target exists but the replace modified nothing (the
// caller's patch was a no-op), and otherwise the model layer's error
// translated by translateModelError. It does not create a new record.
func (r *ExampleRepository) Update(ctx context.Context, rawID string, example *entity.Example) error {
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ErrInvalidID
	}
	if err := r.ready(ctx); err != nil {
		return err
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

// Delete removes the example identified by rawID, returning ErrInvalidID
// for a malformed id or ErrNotFound if no document was deleted.
func (r *ExampleRepository) Delete(ctx context.Context, rawID string) error {
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ErrInvalidID
	}
	if err := r.ready(ctx); err != nil {
		return err
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

// ready lazily ensures indexes exist exactly once, guarded by readyMu so
// concurrent first calls don't race on EnsureIndexes.
func (r *ExampleRepository) ready(ctx context.Context) error {
	r.readyMu.Lock()
	defer r.readyMu.Unlock()

	if r.initialized {
		return nil
	}
	if err := r.Model.EnsureIndexes(ctx); err != nil {
		return err
	}
	r.initialized = true
	return nil
}

// translateModelError maps driver-native errors from the model layer onto
// the stable ExampleStore sentinels (ErrNotFound, ErrConflict), passing
// through anything else unchanged.
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
