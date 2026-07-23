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

var (
	ErrNotFound  = errors.New("example record not found")
	ErrDuplicate = errors.New("example record name already exists")
)

type ListOptions struct {
	Page     int
	PageSize int
	Status   string
}

type UpdateInput struct {
	Name        *string
	Description *string
	Status      *string
}

type ExampleRepository interface {
	Migrate(context.Context) error
	Create(context.Context, *entity.ExampleRecord) error
	Get(context.Context, uint64) (*entity.ExampleRecord, error)
	List(context.Context, ListOptions) ([]entity.ExampleRecord, int64, error)
	Update(context.Context, uint64, UpdateInput) (*entity.ExampleRecord, error)
	Delete(context.Context, uint64) error
}

type exampleRepository struct {
	db *gorm.DB
}

func NewExampleRepository(m *model.ExampleMysqlModel) ExampleRepository {
	return &exampleRepository{db: m.DB()}
}

func (r *exampleRepository) Migrate(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(&entity.ExampleRecord{}); err != nil {
		return fmt.Errorf("migrate example records: %w", err)
	}
	return nil
}

func (r *exampleRepository) Create(ctx context.Context, record *entity.ExampleRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return translateError("create example record", err)
	}
	return nil
}

func (r *exampleRepository) Get(ctx context.Context, id uint64) (*entity.ExampleRecord, error) {
	var record entity.ExampleRecord
	if err := r.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, translateError("get example record", err)
	}
	return &record, nil
}

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

func normalizeListOptions(options ListOptions) ListOptions {
	if options.Page < 1 {
		options.Page = 1
	}
	if options.PageSize < 1 {
		options.PageSize = 20
	}
	return options
}

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
