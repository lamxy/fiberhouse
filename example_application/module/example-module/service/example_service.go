package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/lamxy/fiberhouse/example_application/apivo/commonvo"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/responsevo"
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	exampletask "github.com/lamxy/fiberhouse/example_application/module/example-module/task"
)

const exampleListCacheTTL = 30 * time.Second

type exampleListLoader func(context.Context) (*responsevo.ExampleListRespVo, error)
type exampleListCache func(context.Context, string, time.Duration, exampleListLoader) (*responsevo.ExampleListRespVo, error)

type exampleTaskDispatcher interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

type ExampleUseCase interface {
	Create(context.Context, requestvo.CreateExampleReqVo) (*responsevo.ExampleRespVo, error)
	Get(context.Context, string) (*responsevo.ExampleRespVo, error)
	List(context.Context, requestvo.ListExamplesReqVo) (*responsevo.ExampleListRespVo, error)
	Update(context.Context, string, requestvo.UpdateExampleReqVo) (*responsevo.ExampleRespVo, error)
	Delete(context.Context, string) error
}

type ExampleService struct {
	fiberhouse.ServiceLocator
	Store repository.ExampleStore
	now   func() time.Time

	listCached        exampleListCache
	getTaskDispatcher func() (exampleTaskDispatcher, error)
}

func NewExampleService(ctx fiberhouse.IApplicationContext, store repository.ExampleStore) *ExampleService {
	service := &ExampleService{
		ServiceLocator: fiberhouse.NewService(ctx).SetName(GetKeyExampleService()),
		Store:          store,
		now:            time.Now,
	}
	service.listCached = service.readThroughList
	service.getTaskDispatcher = func() (exampleTaskDispatcher, error) {
		if ctx == nil || ctx.GetStarterApp() == nil || ctx.GetStarterApp().GetTask() == nil {
			return nil, errors.New("task dispatcher is not configured")
		}
		return ctx.GetStarterApp().GetTask().GetTaskDispatcher()
	}
	return service
}

func GetKeyExampleService(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

func (s *ExampleService) Create(ctx context.Context, req requestvo.CreateExampleReqVo) (*responsevo.ExampleRespVo, error) {
	now := s.currentTime()
	status := entity.ExampleStatus(req.Status)
	if status == "" {
		status = entity.ExampleStatusActive
	}
	example := &entity.Example{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Status:      status,
		Tags:        trimTags(req.Tags),
		Timestamps:  fields.NewTimestamps(now),
	}
	if err := s.Store.Create(ctx, example); err != nil {
		return nil, err
	}
	s.observeDispatchError(s.dispatchExampleChanged(ctx, example.ID.Hex(), "create"))
	resp := toResponse(*example)
	return &resp, nil
}

func (s *ExampleService) Get(ctx context.Context, id string) (*responsevo.ExampleRespVo, error) {
	example, err := s.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toResponse(*example)
	return &resp, nil
}

func (s *ExampleService) List(ctx context.Context, req requestvo.ListExamplesReqVo) (*responsevo.ExampleListRespVo, error) {
	req = req.Normalize()
	req.Status = strings.TrimSpace(req.Status)
	loader := func(loaderCtx context.Context) (*responsevo.ExampleListRespVo, error) {
		return s.listFromStore(loaderCtx, req)
	}
	if s.listCached == nil {
		return loader(ctx)
	}
	return s.listCached(ctx, exampleListCacheKey(req), exampleListCacheTTL, loader)
}

func (s *ExampleService) listFromStore(ctx context.Context, req requestvo.ListExamplesReqVo) (*responsevo.ExampleListRespVo, error) {
	examples, total, err := s.Store.List(ctx, repository.ListOptions{
		Page: req.Page, PageSize: req.PageSize, Status: entity.ExampleStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	items := make([]responsevo.ExampleRespVo, 0, len(examples))
	for _, example := range examples {
		items = append(items, toResponse(example))
	}
	return &responsevo.ExampleListRespVo{
		Items: items, Page: req.Page, PageSize: req.PageSize, Total: total,
	}, nil
}

func (s *ExampleService) Update(ctx context.Context, id string, req requestvo.UpdateExampleReqVo) (*responsevo.ExampleRespVo, error) {
	example, err := s.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		example.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		example.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		example.Status = entity.ExampleStatus(*req.Status)
	}
	if req.Tags != nil {
		example.Tags = trimTags(*req.Tags)
	}
	example.UpdatedAt = s.currentTime()
	if err := s.Store.Update(ctx, id, example); err != nil {
		return nil, err
	}
	s.observeDispatchError(s.dispatchExampleChanged(ctx, id, "update"))
	resp := toResponse(*example)
	return &resp, nil
}

func (s *ExampleService) Delete(ctx context.Context, id string) error {
	if err := s.Store.Delete(ctx, id); err != nil {
		return err
	}
	s.observeDispatchError(s.dispatchExampleChanged(ctx, id, "delete"))
	return nil
}

func exampleListCacheKey(req requestvo.ListExamplesReqVo) string {
	return fmt.Sprintf("example:list:page:%d:size:%d:status:%s", req.Page, req.PageSize, req.Status)
}

func (s *ExampleService) readThroughList(ctx context.Context, key string, ttl time.Duration, loader exampleListLoader) (*responsevo.ExampleListRespVo, error) {
	if s.ServiceLocator == nil {
		return loader(ctx)
	}
	appCtx := s.GetContext()
	if appCtx == nil {
		return loader(ctx)
	}
	option := cache.OptionPoolGet(appCtx)
	defer cache.OptionPoolPut(option)
	option.Level2().
		SetCacheKey(key).
		SetLocalTTL(ttl).
		SetRemoteTTL(ttl).
		SetContextCtx(ctx).
		SetSyncStrategyWriteRemoteOnly().
		EnableSingleFlight()
	return cache.GetCached[*responsevo.ExampleListRespVo](option, loader)
}

func (s *ExampleService) dispatchExampleChanged(ctx context.Context, id, operation string) error {
	if s.getTaskDispatcher == nil {
		return errors.New("task dispatcher is not configured")
	}
	dispatcher, err := s.getTaskDispatcher()
	if err != nil {
		return err
	}
	if dispatcher == nil {
		return errors.New("task dispatcher is nil")
	}
	changedTask, err := exampletask.NewExampleChangedTask(s.applicationContext(), exampletask.ExampleChangedPayload{
		ID: id, Operation: operation,
	})
	if err != nil {
		return err
	}
	_, err = dispatcher.EnqueueContext(ctx, changedTask, asynq.Queue("default"), asynq.MaxRetry(constant.TaskMaxRetryDefault))
	return err
}

func (s *ExampleService) applicationContext() fiberhouse.IContext {
	if s.ServiceLocator == nil {
		return nil
	}
	return s.GetContext()
}

func (s *ExampleService) observeDispatchError(err error) {
	if err == nil || s.ServiceLocator == nil || s.GetContext() == nil {
		return
	}
	s.GetContext().GetLogger().WarnWith(s.GetContext().GetConfig().LogOriginTask()).
		Err(err).Msg("example changed event was not enqueued")
}

func (s *ExampleService) currentTime() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func trimTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		result = append(result, strings.TrimSpace(tag))
	}
	return result
}

func toResponse(example entity.Example) responsevo.ExampleRespVo {
	tags := append([]string(nil), example.Tags...)
	if tags == nil {
		tags = make([]string, 0)
	}
	return responsevo.ExampleRespVo{
		ID:          example.ID.Hex(),
		Name:        example.Name,
		Description: example.Description,
		Status:      string(example.Status),
		Tags:        tags,
		Timestamps: commonvo.Timestamps{
			CreatedAt: example.CreatedAt,
			UpdatedAt: example.UpdatedAt,
		},
	}
}
