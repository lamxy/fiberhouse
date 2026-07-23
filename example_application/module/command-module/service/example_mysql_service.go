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

var ErrInvalidInput = errors.New("invalid example input")

type CreateInput struct {
	Name        string
	Description string
	Status      string
}

type ListInput struct {
	Page     int
	PageSize int
	Status   string
}

type UpdateInput struct {
	Name        *string
	Description *string
	Status      *string
}

type ListResult struct {
	Items    []entity.ExampleRecord `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int64                  `json:"total"`
}

type ExampleUseCase interface {
	Migrate(context.Context) error
	Create(context.Context, CreateInput) (*entity.ExampleRecord, error)
	Get(context.Context, uint64) (*entity.ExampleRecord, error)
	List(context.Context, ListInput) (*ListResult, error)
	Update(context.Context, uint64, UpdateInput) (*entity.ExampleRecord, error)
	Delete(context.Context, uint64) error
}

type ExampleMysqlService struct {
	repository repository.ExampleRepository
}

func NewExampleMysqlService(repo repository.ExampleRepository) *ExampleMysqlService {
	return &ExampleMysqlService{repository: repo}
}

func (s *ExampleMysqlService) Migrate(ctx context.Context) error {
	return s.repository.Migrate(ctx)
}

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

func (s *ExampleMysqlService) Get(ctx context.Context, id uint64) (*entity.ExampleRecord, error) {
	if id == 0 {
		return nil, invalid("id must be greater than zero")
	}
	return s.repository.Get(ctx, id)
}

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

func (s *ExampleMysqlService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return invalid("id must be greater than zero")
	}
	return s.repository.Delete(ctx, id)
}

func validStatus(status string) bool {
	return status == "active" || status == "archived"
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
