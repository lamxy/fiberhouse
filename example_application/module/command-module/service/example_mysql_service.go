// Package service 是 command-module 中基于 MySQL 的 example CLI 的业务逻辑层：
// 负责校验并规范化 CLI 输入，并编排 repository 调用。它依赖 repository（以及与
// 调用方交换的 entity 类型），但绝不直接与 model/gorm 层交互。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/repository"
)

// ErrInvalidInput 在 CLI 输入未通过业务规则校验时返回。通过 invalid() 用
// fmt.Errorf 包装它，可在追加细节的同时保持 errors.Is 匹配。
var ErrInvalidInput = errors.New("invalid example input")

// CreateInput 是 ExampleUseCase.Create 经校验后的输入。
type CreateInput struct {
	Name        string
	Description string
	Status      string
}

// ListInput 承载 ExampleUseCase.List 的分页与状态过滤参数。
type ListInput struct {
	Page     int
	PageSize int
	Status   string
}

// UpdateInput 是 ExampleUseCase.Update 的部分 patch。nil 字段保持不变；
// 非 nil 字段替换当前值。
type UpdateInput struct {
	Name        *string
	Description *string
	Status      *string
}

// ListResult 是一页 example 记录以及匹配总数。
type ListResult struct {
	Items    []entity.ExampleRecord `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int64                  `json:"total"`
}

// ExampleUseCase 是由 CLI 命令层消费的 service 层契约。所有方法都接收调用方的
// context.Context 并原样传播到 repository。错误为校验失败的 ErrInvalidInput，
// 或持久化失败时来自 repository 的稳定哨兵值（ErrNotFound、ErrDuplicate）；
// 调用方应使用 errors.Is。
//
// Update 应用部分 patch（仅更改非 nil 的 UpdateInput 字段），且不是 upsert：
// 不存在的 id 返回 ErrNotFound。List 以固定、确定性的顺序（created_at 降序、
// id 降序）返回结果，使分页在多次调用间保持稳定。
type ExampleUseCase interface {
	Migrate(context.Context) error
	Create(context.Context, CreateInput) (*entity.ExampleRecord, error)
	Get(context.Context, uint64) (*entity.ExampleRecord, error)
	List(context.Context, ListInput) (*ListResult, error)
	Update(context.Context, uint64, UpdateInput) (*entity.ExampleRecord, error)
	Delete(context.Context, uint64) error
}

// ExampleMysqlService 是默认的 ExampleUseCase 实现，由
// repository.ExampleRepository 支撑。
type ExampleMysqlService struct {
	repository repository.ExampleRepository
}

// NewExampleMysqlService 构建一个绑定到指定 repository.ExampleRepository 的
// ExampleMysqlService。
func NewExampleMysqlService(repo repository.ExampleRepository) *ExampleMysqlService {
	return &ExampleMysqlService{repository: repo}
}

// Migrate 通过 repository 应用 example_records 的表结构迁移。
func (s *ExampleMysqlService) Migrate(ctx context.Context) error {
	return s.repository.Migrate(ctx)
}

// Create 校验 input，Status 为空时默认为「active」，并通过 repository 持久化
// 一条新的 example 记录。
func (s *ExampleMysqlService) Create(ctx context.Context, input CreateInput) (*entity.ExampleRecord, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.TrimSpace(input.Status)
	if input.Name == "" {
		return nil, invalid("name is required")
	}
	if utf8.RuneCountInString(input.Name) > 80 {
		return nil, invalid("name must be at most 80 characters")
	}
	if utf8.RuneCountInString(input.Description) > 500 {
		return nil, invalid("description must be at most 500 characters")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if !validStatus(input.Status) {
		return nil, invalid("status must be active or archived")
	}

	record := &entity.ExampleRecord{
		Name:        input.Name,
		Description: input.Description,
		Status:      input.Status,
	}
	if err := s.repository.Create(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

// Get 按 id 获取单条 example 记录；id == 0 时返回 ErrInvalidInput，
// 记录不存在时返回 repository.ErrNotFound。
func (s *ExampleMysqlService) Get(ctx context.Context, id uint64) (*entity.ExampleRecord, error) {
	if id == 0 {
		return nil, invalid("id must be greater than zero")
	}
	return s.repository.Get(ctx, id)
}

// List 以确定性顺序（created_at 降序、id 降序）返回一页 example 记录；未设置时
// 将 Page 默认为 1、PageSize 默认为 20。
func (s *ExampleMysqlService) List(ctx context.Context, input ListInput) (*ListResult, error) {
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		return nil, invalid("page-size must be at most 100")
	}
	input.Status = strings.TrimSpace(input.Status)
	if input.Status != "" && !validStatus(input.Status) {
		return nil, invalid("status must be active or archived")
	}

	items, total, err := s.repository.List(ctx, repository.ListOptions{
		Page: input.Page, PageSize: input.PageSize, Status: input.Status,
	})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = make([]entity.ExampleRecord, 0)
	}
	return &ListResult{Items: items, Page: input.Page, PageSize: input.PageSize, Total: total}, nil
}

// Update 对已有 example 记录应用部分 patch：只更改 input 上非 nil 的字段。
// 它不是 upsert——不存在的 id 返回 repository.ErrNotFound，而非创建记录。
func (s *ExampleMysqlService) Update(ctx context.Context, id uint64, input UpdateInput) (*entity.ExampleRecord, error) {
	if id == 0 {
		return nil, invalid("id must be greater than zero")
	}
	if input.Name == nil && input.Description == nil && input.Status == nil {
		return nil, invalid("at least one update field is required")
	}

	repoInput := repository.UpdateInput{}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" {
			return nil, invalid("name must not be empty")
		}
		if utf8.RuneCountInString(value) > 80 {
			return nil, invalid("name must be at most 80 characters")
		}
		repoInput.Name = &value
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if utf8.RuneCountInString(value) > 500 {
			return nil, invalid("description must be at most 500 characters")
		}
		repoInput.Description = &value
	}
	if input.Status != nil {
		value := strings.TrimSpace(*input.Status)
		if !validStatus(value) {
			return nil, invalid("status must be active or archived")
		}
		repoInput.Status = &value
	}
	return s.repository.Update(ctx, id, repoInput)
}

// Delete 按 id 删除一条 example 记录。
func (s *ExampleMysqlService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return invalid("id must be greater than zero")
	}
	return s.repository.Delete(ctx, id)
}

// validStatus 报告 status 是否为两个允许值之一。
func validStatus(status string) bool {
	return status == "active" || status == "archived"
}

// invalid 将 message 包装为 ErrInvalidInput，保持 errors.Is 匹配。
func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
