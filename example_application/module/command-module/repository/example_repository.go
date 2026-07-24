// Package repository is the persistence layer for the command-module's
// MySQL-backed example CLI: it wraps GORM calls, normalizes pagination
// defaults, and translates driver/GORM errors into the stable sentinels
// defined below. It depends on model (for the *gorm.DB handle) and entity,
// but exposes no GORM types to service.
package repository

import (
	"context"
	"errors"
	"fmt"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/model"
	"gorm.io/gorm"
)

// Stable domain errors returned by ExampleRepository implementations.
// Callers (service) must use errors.Is against these sentinels rather than
// matching on error strings.
var (
	// ErrNotFound is returned when a lookup, update, or delete targets an id
	// that does not exist.
	ErrNotFound = errors.New("example record not found")
	// ErrDuplicate is returned when a create/update violates the unique
	// index on name.
	ErrDuplicate = errors.New("example record name already exists")
)

// ListOptions carries pagination and status-filter parameters for
// ExampleRepository.List.
type ListOptions struct {
	Page     int
	PageSize int
	Status   string
}

// UpdateInput is the partial patch for ExampleRepository.Update. A nil
// field is left unchanged; a non-nil field replaces the current value.
type UpdateInput struct {
	Name        *string
	Description *string
	Status      *string
}

// ExampleRepository is the persistence contract consumed by service. All
// methods accept the caller's context.Context and propagate it unchanged to
// GORM via WithContext. Errors are the stable sentinels above (ErrNotFound,
// ErrDuplicate); callers should use errors.Is.
//
// Update applies a partial patch (only non-nil UpdateInput fields change)
// and is not an upsert: an id that does not resolve to an existing record
// returns ErrNotFound. It never sets created_at, so the original creation
// timestamp is preserved across updates. List returns items and the total
// matching count in deterministic order (created_at desc, id desc).
type ExampleRepository interface {
	Migrate(context.Context) error
	Create(context.Context, *entity.ExampleRecord) error
	Get(context.Context, uint64) (*entity.ExampleRecord, error)
	List(context.Context, ListOptions) ([]entity.ExampleRecord, int64, error)
	Update(context.Context, uint64, UpdateInput) (*entity.ExampleRecord, error)
	Delete(context.Context, uint64) error
}

// exampleRepository is the default ExampleRepository implementation, backed
// directly by a *gorm.DB.
type exampleRepository struct {
	db *gorm.DB
}

// NewExampleRepository builds an ExampleRepository using the *gorm.DB owned
// by m.
func NewExampleRepository(m *model.ExampleMysqlModel) ExampleRepository {
	return &exampleRepository{db: m.DB()}
}

// Migrate runs GORM's AutoMigrate for entity.ExampleRecord, creating or
// updating the example_records table schema.
func (r *exampleRepository) Migrate(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(&entity.ExampleRecord{}); err != nil {
		return fmt.Errorf("migrate example records: %w", err)
	}
	return nil
}

// Create inserts record, returning ErrDuplicate if its name collides with
// the unique index.
func (r *exampleRepository) Create(ctx context.Context, record *entity.ExampleRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return translateError("create example record", err)
	}
	return nil
}

// Get fetches a single record by id, returning ErrNotFound if none exists.
func (r *exampleRepository) Get(ctx context.Context, id uint64) (*entity.ExampleRecord, error) {
	var record entity.ExampleRecord
	if err := r.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, translateError("get example record", err)
	}
	return &record, nil
}

// List returns a page of records matching options.Status (or all statuses
// if empty) plus the total matching count, ordered deterministically by
// created_at desc then id desc.
func (r *exampleRepository) List(ctx context.Context, options ListOptions) ([]entity.ExampleRecord, int64, error) {
	options = normalizeListOptions(options)
	query := func() *gorm.DB {
		result := r.db.WithContext(ctx).Model(&entity.ExampleRecord{})
		if options.Status != "" {
			result = result.Where("status = ?", options.Status)
		}
		return result
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count example records: %w", err)
	}

	var records []entity.ExampleRecord
	offset := (options.Page - 1) * options.PageSize
	if err := query().
		Order("created_at DESC, id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list example records: %w", err)
	}
	return records, total, nil
}

// Update applies a partial patch to the record identified by id (only
// non-nil input fields change) and returns the record's current state
// afterward. This is not an upsert: if id does not exist, the trailing Get
// returns ErrNotFound. Unlike the example-module's Mongo repository, a
// no-op write (RowsAffected == 0 on an existing row) is not treated as an
// error here — it simply falls through to re-reading the unchanged record.
func (r *exampleRepository) Update(ctx context.Context, id uint64, input UpdateInput) (*entity.ExampleRecord, error) {
	fields := make(map[string]any, 3)
	if input.Name != nil {
		fields["name"] = *input.Name
	}
	if input.Description != nil {
		fields["description"] = *input.Description
	}
	if input.Status != nil {
		fields["status"] = *input.Status
	}

	result := r.db.WithContext(ctx).
		Model(&entity.ExampleRecord{}).
		Where("id = ?", id).
		Updates(fields)
	if result.Error != nil {
		return nil, translateError("update example record", result.Error)
	}
	if result.RowsAffected == 0 {
		return r.Get(ctx, id)
	}
	return r.Get(ctx, id)
}

// Delete hard-deletes the record identified by id (bypassing any soft-delete
// hook via Unscoped), returning ErrNotFound if no row was deleted.
func (r *exampleRepository) Delete(ctx context.Context, id uint64) error {
	result := r.db.WithContext(ctx).Unscoped().Delete(&entity.ExampleRecord{}, id)
	if result.Error != nil {
		return translateError("delete example record", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// normalizeListOptions defaults Page to 1 and PageSize to 20 when unset.
func normalizeListOptions(options ListOptions) ListOptions {
	if options.Page < 1 {
		options.Page = 1
	}
	if options.PageSize < 1 {
		options.PageSize = 20
	}
	return options
}

// translateError maps GORM/MySQL-native errors onto the stable sentinels
// (ErrNotFound, ErrDuplicate), wrapping operation and the original error for
// context; anything else is wrapped but left otherwise untranslated.
func translateError(operation string, err error) error {
	var mysqlError *gomysql.MySQLError
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("%s: %w: %w", operation, ErrNotFound, err)
	case errors.Is(err, gorm.ErrDuplicatedKey),
		errors.As(err, &mysqlError) && mysqlError.Number == 1062:
		return fmt.Errorf("%s: %w: %w", operation, ErrDuplicate, err)
	default:
		return fmt.Errorf("%s: %w", operation, err)
	}
}
