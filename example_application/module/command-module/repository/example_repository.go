// Package repository 是 command-module 中基于 MySQL 的 example CLI 的持久化层：
// 负责包裹 GORM 调用、规范化分页默认值，并把驱动/GORM 错误翻译为下方定义的
// 稳定哨兵错误。它依赖 model（获取 *gorm.DB 句柄）与 entity，但不向 service
// 暴露任何 GORM 类型。
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

// ExampleRepository 实现返回的稳定领域错误。调用方（service）必须用 errors.Is
// 与这些哨兵值比较，而非匹配错误字符串。
var (
	// ErrNotFound 在查询、更新或删除针对一个不存在的 id 时返回。
	ErrNotFound = errors.New("example record not found")
	// ErrDuplicate 在创建/更新违反 name 上的唯一索引时返回。
	ErrDuplicate = errors.New("example record name already exists")
)

// ListOptions 承载 ExampleRepository.List 的分页与状态过滤参数。
type ListOptions struct {
	Page     int
	PageSize int
	Status   string
}

// UpdateInput 是 ExampleRepository.Update 的部分 patch。nil 字段保持不变；
// 非 nil 字段替换当前值。
type UpdateInput struct {
	Name        *string
	Description *string
	Status      *string
}

// ExampleRepository 是由 service 消费的持久化契约。所有方法都接收调用方的
// context.Context，并经 WithContext 原样传播到 GORM。错误为上文的稳定哨兵值
// （ErrNotFound、ErrDuplicate）；调用方应使用 errors.Is。
//
// Update 应用部分 patch（仅更改非 nil 的 UpdateInput 字段），且不是 upsert：
// 未解析到已有记录的 id 返回 ErrNotFound。它绝不设置 created_at，因此原始创建
// 时间在更新间得以保留。List 以确定性顺序（created_at 降序、id 降序）返回条目
// 与匹配总数。
type ExampleRepository interface {
	Migrate(context.Context) error
	Create(context.Context, *entity.ExampleRecord) error
	Get(context.Context, uint64) (*entity.ExampleRecord, error)
	List(context.Context, ListOptions) ([]entity.ExampleRecord, int64, error)
	Update(context.Context, uint64, UpdateInput) (*entity.ExampleRecord, error)
	Delete(context.Context, uint64) error
}

// exampleRepository 是默认的 ExampleRepository 实现，直接由 *gorm.DB 支撑。
type exampleRepository struct {
	db *gorm.DB
}

// NewExampleRepository 使用 m 持有的 *gorm.DB 构建一个 ExampleRepository。
func NewExampleRepository(m *model.ExampleMysqlModel) ExampleRepository {
	return &exampleRepository{db: m.DB()}
}

// Migrate 对 entity.ExampleRecord 运行 GORM 的 AutoMigrate，创建或更新
// example_records 表结构。
func (r *exampleRepository) Migrate(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(&entity.ExampleRecord{}); err != nil {
		return fmt.Errorf("migrate example records: %w", err)
	}
	return nil
}

// Create 插入 record；若其 name 与唯一索引冲突，返回 ErrDuplicate。
func (r *exampleRepository) Create(ctx context.Context, record *entity.ExampleRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return translateError("create example record", err)
	}
	return nil
}

// Get 按 id 获取单条记录；若不存在，返回 ErrNotFound。
func (r *exampleRepository) Get(ctx context.Context, id uint64) (*entity.ExampleRecord, error) {
	var record entity.ExampleRecord
	if err := r.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, translateError("get example record", err)
	}
	return &record, nil
}

// List 以确定性顺序（created_at 降序、再按 id 降序）返回匹配 options.Status
// （为空则匹配所有状态）的一页记录以及匹配总数。
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

// Update 对由 id 标识的记录应用部分 patch（仅更改非 nil 的 input 字段），并在
// 之后返回该记录的当前状态。这不是 upsert：若 id 不存在，末尾的 Get 会返回
// ErrNotFound。与 example-module 的 Mongo repository 不同，这里空操作写入
// （已有行上 RowsAffected == 0）不被视为错误——它只是继续去重新读取未变化的记录。
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

// Delete 硬删除由 id 标识的记录（通过 Unscoped 绕过任何软删除钩子）；
// 若未删除任何行，返回 ErrNotFound。
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

// normalizeListOptions 在未设置时将 Page 默认为 1、PageSize 默认为 20。
func normalizeListOptions(options ListOptions) ListOptions {
	if options.Page < 1 {
		options.Page = 1
	}
	if options.PageSize < 1 {
		options.PageSize = 20
	}
	return options
}

// translateError 将 GORM/MySQL 原生错误映射为稳定的哨兵错误（ErrNotFound、
// ErrDuplicate），并包装 operation 与原始错误以提供上下文；其余错误仅作包装、
// 不做翻译。
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
