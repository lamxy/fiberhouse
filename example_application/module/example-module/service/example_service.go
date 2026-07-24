// Package service is the business-logic layer for the example module: it
// validates and normalizes input, orchestrates repository calls, applies
// caching for reads, and dispatches async notifications for mutations. It
// depends on repository (and the entity/apivo types it exchanges with
// callers) but never talks to the model or driver layers directly.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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

// ErrInvalidInput is returned when request data fails business-rule
// validation (as opposed to the transport-level struct-tag validation done
// in the api layer). Wrap it with fmt.Errorf("%w: ...", ErrInvalidInput) to
// add detail while preserving errors.Is matching.
var ErrInvalidInput = errors.New("invalid example input")

type exampleListLoader func(context.Context) (*responsevo.ExampleListRespVo, error)
type exampleListCache func(context.Context, string, time.Duration, exampleListLoader) (*responsevo.ExampleListRespVo, error)

// exampleTaskDispatcher is the minimal asynq surface ExampleService needs to
// enqueue notifications, kept narrow so it can be stubbed in tests.
type exampleTaskDispatcher interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

// ExampleUseCase is the service-layer contract consumed by the transport
// layer. All methods take a caller-supplied context.Context that must be
// propagated to the repository/store unchanged (no context.Background()
// substitution). Errors are the stable sentinels defined in the repository
// package (ErrInvalidID, ErrNotFound, ErrConflict, ErrUnchanged) plus
// ErrInvalidInput for validation failures; callers should use errors.Is to
// branch on them, notably via transport.MapDomainError.
//
// Update applies a partial patch (only non-nil fields on the request are
// changed) and is not an upsert: calling Update with an id that does not
// exist returns ErrNotFound rather than creating a record. List returns
// results in a fixed, deterministic order (see ExampleModel.buildFindQuery)
// so that pagination is stable across calls.
type ExampleUseCase interface {
	Create(context.Context, requestvo.CreateExampleReqVo) (*responsevo.ExampleRespVo, error)
	Get(context.Context, string) (*responsevo.ExampleRespVo, error)
	List(context.Context, requestvo.ListExamplesReqVo) (*responsevo.ExampleListRespVo, error)
	Update(context.Context, string, requestvo.UpdateExampleReqVo) (*responsevo.ExampleRespVo, error)
	Delete(context.Context, string) error
}

// ExampleService is the default ExampleUseCase implementation. It validates
// and normalizes input, delegates persistence to Store, caches List results,
// and best-effort dispatches an async "example changed" notification after
// successful mutations.
type ExampleService struct {
	fiberhouse.ServiceLocator
	Store repository.ExampleStore
	now   func() time.Time

	listCached        exampleListCache
	getTaskDispatcher func() (exampleTaskDispatcher, error)
}

// NewExampleService builds an ExampleService bound to the given application
// context and repository.ExampleStore, wiring the default read-through list
// cache and asynq task dispatcher lookup.
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

// GetKeyExampleService returns the registry key used to locate the
// ExampleService instance, optionally namespaced by ns.
func GetKeyExampleService(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// Create validates req, assigns creation timestamps, and persists a new
// example via Store. On success it best-effort dispatches an async "create"
// notification (dispatch failures are logged, not returned to the caller).
func (s *ExampleService) Create(ctx context.Context, req requestvo.CreateExampleReqVo) (*responsevo.ExampleRespVo, error) {
	now := s.currentTime()
	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	status := entity.ExampleStatus(strings.TrimSpace(req.Status))
	if status == "" {
		status = entity.ExampleStatusActive
	}
	tags := trimTags(req.Tags)
	if err := validateCanonicalExample(name, description, status, tags); err != nil {
		return nil, err
	}
	example := &entity.Example{
		Name:        name,
		Description: description,
		Status:      status,
		Tags:        tags,
		Timestamps:  fields.NewTimestamps(now),
	}
	if err := s.Store.Create(ctx, example); err != nil {
		return nil, err
	}
	s.observeDispatchError(s.dispatchExampleChanged(ctx, example.ID.Hex(), "create"))
	resp := toResponse(*example)
	return &resp, nil
}

// Get fetches a single example by id, returning ErrInvalidID or ErrNotFound
// (via the Store) as appropriate.
func (s *ExampleService) Get(ctx context.Context, id string) (*responsevo.ExampleRespVo, error) {
	example, err := s.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toResponse(*example)
	return &resp, nil
}

// List returns a page of examples in deterministic order (see
// ExampleModel.buildFindQuery for the sort). Results are served through a
// short-lived read-through cache keyed by the normalized request
// (exampleListCacheKey), so repeated identical queries within
// exampleListCacheTTL avoid hitting the store.
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

// listFromStore is the cache-miss loader for List: it queries Store directly
// and maps the results to response DTOs.
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

// Update applies a partial patch to an existing example: only fields set on
// req are changed, all others (including CreatedAt) are left untouched. This
// is intentionally not an upsert — if id does not resolve to an existing
// example, Get below returns ErrNotFound and no record is created.
func (s *ExampleService) Update(ctx context.Context, id string, req requestvo.UpdateExampleReqVo) (*responsevo.ExampleRespVo, error) {
	if err := normalizeAndValidateUpdate(&req); err != nil {
		return nil, err
	}
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

// Delete removes an example by id and, on success, best-effort dispatches an
// async "delete" notification.
func (s *ExampleService) Delete(ctx context.Context, id string) error {
	if err := s.Store.Delete(ctx, id); err != nil {
		return err
	}
	s.observeDispatchError(s.dispatchExampleChanged(ctx, id, "delete"))
	return nil
}

// exampleListCacheKey derives a stable cache key from the normalized list
// request so identical queries share a cache entry.
func exampleListCacheKey(req requestvo.ListExamplesReqVo) string {
	return fmt.Sprintf("example:list:page:%d:size:%d:status:%s", req.Page, req.PageSize, req.Status)
}

// readThroughList is the default exampleListCache implementation: it wraps
// loader with a two-level (local + remote) single-flight cache, falling back
// to calling loader directly if the service has no application context
// (e.g. in lightweight unit tests).
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
		EnableCache().
		SetCacheKey(key).
		SetLocalTTL(ttl).
		SetRemoteTTL(ttl).
		SetContextCtx(ctx).
		SetSyncStrategyWriteRemoteOnly().
		EnableSingleFlight()
	return cache.GetCached[*responsevo.ExampleListRespVo](option, loader)
}

// dispatchExampleChanged enqueues an asynq task notifying that an example
// was created/updated/deleted. Errors are returned to the caller
// (observeDispatchError decides whether to log-and-swallow them).
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

// applicationContext returns the fiberhouse.IContext backing this service,
// or nil if the service was constructed without one.
func (s *ExampleService) applicationContext() fiberhouse.IContext {
	if s.ServiceLocator == nil {
		return nil
	}
	return s.GetContext()
}

// observeDispatchError logs a failed async dispatch as a warning without
// propagating it: notification delivery is best-effort and must not fail
// the mutation that already succeeded.
func (s *ExampleService) observeDispatchError(err error) {
	if err == nil || s.ServiceLocator == nil || s.GetContext() == nil {
		return
	}
	s.GetContext().GetLogger().WarnWith(s.GetContext().GetConfig().LogOriginTask()).
		Err(err).Msg("example changed event was not enqueued")
}

// currentTime returns the current UTC time via the injectable now func,
// falling back to time.Now when unset (e.g. zero-value ExampleService).
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

func validateCanonicalExample(name, description string, status entity.ExampleStatus, tags []string) error {
	if count := utf8.RuneCountInString(name); count < 2 || count > 80 {
		return fmt.Errorf("%w: name must contain 2 to 80 nonblank characters", ErrInvalidInput)
	}
	if utf8.RuneCountInString(description) > 500 {
		return fmt.Errorf("%w: description must contain at most 500 characters", ErrInvalidInput)
	}
	if status != entity.ExampleStatusActive && status != entity.ExampleStatusArchived {
		return fmt.Errorf("%w: status must be active or archived", ErrInvalidInput)
	}
	if len(tags) > 10 {
		return fmt.Errorf("%w: at most 10 tags are allowed", ErrInvalidInput)
	}
	for _, tag := range tags {
		if count := utf8.RuneCountInString(tag); count < 1 || count > 30 {
			return fmt.Errorf("%w: tags must contain 1 to 30 nonblank characters", ErrInvalidInput)
		}
	}
	return nil
}

func normalizeAndValidateUpdate(req *requestvo.UpdateExampleReqVo) error {
	if req.Name != nil {
		normalized := strings.TrimSpace(*req.Name)
		req.Name = &normalized
		if count := utf8.RuneCountInString(normalized); count < 2 || count > 80 {
			return fmt.Errorf("%w: name must contain 2 to 80 nonblank characters", ErrInvalidInput)
		}
	}
	if req.Description != nil {
		normalized := strings.TrimSpace(*req.Description)
		req.Description = &normalized
		if utf8.RuneCountInString(normalized) > 500 {
			return fmt.Errorf("%w: description must contain at most 500 characters", ErrInvalidInput)
		}
	}
	if req.Status != nil {
		normalized := strings.TrimSpace(*req.Status)
		req.Status = &normalized
		status := entity.ExampleStatus(normalized)
		if status != entity.ExampleStatusActive && status != entity.ExampleStatusArchived {
			return fmt.Errorf("%w: status must be active or archived", ErrInvalidInput)
		}
	}
	if req.Tags != nil {
		normalized := trimTags(*req.Tags)
		req.Tags = &normalized
		if len(normalized) > 10 {
			return fmt.Errorf("%w: at most 10 tags are allowed", ErrInvalidInput)
		}
		for _, tag := range normalized {
			if count := utf8.RuneCountInString(tag); count < 1 || count > 30 {
				return fmt.Errorf("%w: tags must contain 1 to 30 nonblank characters", ErrInvalidInput)
			}
		}
	}
	return nil
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
