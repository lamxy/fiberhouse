// Package service 是 example 模块的业务逻辑层：负责校验并规范化输入、编排
// repository 调用、为读取应用缓存，并在写操作后派发异步通知。它依赖 repository
// （以及与调用方交换的 entity/apivo 类型），但绝不直接与 model 或驱动层交互。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

const (
	exampleListCacheTTL         = 30 * time.Second
	exampleDetailLocalCacheTTL  = 30 * time.Second
	exampleDetailRemoteCacheTTL = 5 * time.Minute
)

// ErrInvalidInput 在请求数据未通过业务规则校验时返回（区别于 api 层完成的
// 传输级结构体 tag 校验）。用 fmt.Errorf("%w: ...", ErrInvalidInput) 包装它，
// 可在追加细节的同时保持 errors.Is 匹配。
var ErrInvalidInput = errors.New("invalid example input")

type exampleListLoader func(context.Context) (*responsevo.ExampleListRespVo, error)
type exampleListCache func(context.Context, string, time.Duration, exampleListLoader) (*responsevo.ExampleListRespVo, error)

// exampleTaskDispatcher 是 ExampleService 入队通知所需的最小 asynq 接口，
// 刻意保持狭窄，以便在测试中打桩。
type exampleTaskDispatcher interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

// ExampleUseCase 是由传输层消费的 service 层契约。所有方法都接收调用方提供的
// context.Context，必须原样传播到 repository/store（不得替换为 context.Background()）。
// 错误为 repository 包中定义的稳定哨兵值（ErrInvalidID、ErrNotFound、ErrConflict、
// ErrUnchanged），外加校验失败的 ErrInvalidInput；调用方应使用 errors.Is 分支处理，
// 尤其是通过 transport.MapDomainError。
//
// Update 应用部分 patch（仅更改请求中非 nil 的字段），且不是 upsert：以不存在的
// id 调用 Update 会返回 ErrNotFound，而非创建记录。List 以固定、确定性的顺序返回
// 结果（见 ExampleModel.buildFindQuery），使分页在多次调用间保持稳定。
type ExampleUseCase interface {
	Create(context.Context, requestvo.CreateExampleReqVo) (*responsevo.ExampleRespVo, error)
	Get(context.Context, string) (*responsevo.ExampleRespVo, error)
	GetWithCache(context.Context, string) (*responsevo.ExampleRespVo, error)
	List(context.Context, requestvo.ListExamplesReqVo) (*responsevo.ExampleListRespVo, error)
	Update(context.Context, string, requestvo.UpdateExampleReqVo) (*responsevo.ExampleRespVo, error)
	Delete(context.Context, string) error
}

// ExampleService 是默认的 ExampleUseCase 实现。它校验并规范化输入，将持久化
// 委托给 Store，缓存 List 结果，并在写操作成功后尽力派发一条
// 「example changed」异步通知。
type ExampleService struct {
	fiberhouse.ServiceLocator // 继承 service 定位层接口
	Store                     repository.ExampleStore
	now                       func() time.Time
	cacheCoordinator          *cache.Coordinator
	cacheBackend              cache.Cache
	cacheCoordinatorOnce      sync.Once

	listCached        exampleListCache
	getTaskDispatcher func() (exampleTaskDispatcher, error)
}

// NewExampleService 构建一个绑定到指定应用上下文与 repository.ExampleStore 的
// ExampleService，并接入默认的穿透读取列表缓存与 asynq 任务派发器查找。
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

// GetKeyExampleService 返回用于定位 ExampleService 实例的注册键，
// 可通过 ns 追加命名空间。
func GetKeyExampleService(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// Create 校验 req，赋予创建时间戳，并通过 Store 持久化一个新 example。成功时
// 尽力派发一条「create」异步通知（派发失败仅记录日志，不返回给调用方）。
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

// Get 按 id 获取单个 example，视情况（经 Store）返回 ErrInvalidID 或 ErrNotFound。
func (s *ExampleService) Get(ctx context.Context, id string) (*responsevo.ExampleRespVo, error) {
	example, err := s.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toResponse(*example)
	return &resp, nil
}

// GetWithCache 演示缓存组件推荐的读取方式：复用长期存活的 Coordinator，
// 为每次请求创建独立的 CacheOption，并把调用方上下文传给完整的缓存读取、
// 数据加载和回填流程。相同 id 的并发未命中只会执行一次 Store.Get。
func (s *ExampleService) GetWithCache(ctx context.Context, id string) (*responsevo.ExampleRespVo, error) {
	if s.ServiceLocator == nil || s.GetContext() == nil {
		return s.Get(ctx, id)
	}
	appCtx := s.GetContext()
	option := cache.NewCacheOption(appCtx).
		Level2().
		SetDefaultInstanceKey("example-cache-demo").
		SetCacheKey(exampleDetailCacheKey(id)).
		SetContextCtx(ctx).
		SetLocalTTLRandomPercent(exampleDetailLocalCacheTTL, 0.1).
		SetRemoteTTLRandomPercent(exampleDetailRemoteCacheTTL, 0.1).
		SetLoadTimeout(3 * time.Second).
		SetSyncStrategyWriteRemoteOnly()
	loader := func(loaderCtx context.Context) (*responsevo.ExampleRespVo, error) {
		return s.Get(loaderCtx, id)
	}
	if coordinator := s.resolveCacheCoordinator(appCtx); coordinator != nil {
		return cache.GetOrLoad(coordinator, option, loader)
	}
	return loader(ctx)
}

// List 以确定性顺序返回一页 example（排序见 ExampleModel.buildFindQuery）。
// 结果经由一个以规范化请求（exampleListCacheKey）为键的短时 read-through 缓存
// 提供，因此在 exampleListCacheTTL 内重复的相同查询不会命中存储。
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

// listFromStore 是 List 的缓存未命中加载器：直接查询 Store 并把结果映射为
// 响应 DTO。
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

// Update 对已有 example 应用部分 patch：只更改 req 上被设置的字段，其余（包括
// CreatedAt）保持不变。它刻意不是 upsert——若 id 未解析到已有 example，
// 下方的 Get 会返回 ErrNotFound，且不会创建任何记录。
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
	s.invalidateCachedExample(ctx, id)
	s.observeDispatchError(s.dispatchExampleChanged(ctx, id, "update"))
	resp := toResponse(*example)
	return &resp, nil
}

// Delete 按 id 删除 example，成功时尽力派发一条「delete」异步通知。
func (s *ExampleService) Delete(ctx context.Context, id string) error {
	if err := s.Store.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateCachedExample(ctx, id)
	s.observeDispatchError(s.dispatchExampleChanged(ctx, id, "delete"))
	return nil
}

// exampleListCacheKey 从规范化的列表请求派生出稳定的缓存键，使相同查询共享
// 同一缓存条目。
func exampleListCacheKey(req requestvo.ListExamplesReqVo) string {
	return fmt.Sprintf("example:list:page:%d:size:%d:status:%s", req.Page, req.PageSize, req.Status)
}

// exampleDetailCacheKey 生成缓存样例接口使用的稳定详情键。
func exampleDetailCacheKey(id string) string {
	return "example:detail:" + id
}

// readThroughList 是默认的 exampleListCache 实现：用一个两级（本地 + 远端）
// 单飞缓存包裹 loader；当 service 没有应用上下文时（例如轻量单元测试），
// 回退为直接调用 loader。
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
	if coordinator := s.resolveCacheCoordinator(appCtx); coordinator != nil {
		return cache.GetOrLoad(coordinator, option, loader)
	}
	return s.readThroughListLegacy(option, loader)
}

// resolveCacheCoordinator 惰性解析并长期保存可选的二级缓存后端。这样既能让只注册
// 路由的轻量上下文安全完成构造，也能让真实请求共享同一个完整路径单飞协调器。
func (s *ExampleService) resolveCacheCoordinator(appCtx fiberhouse.IContext) *cache.Coordinator {
	s.cacheCoordinatorOnce.Do(func() {
		backend, err := cache.NewFactory(appCtx).Lookup(cache.Level2)
		if err != nil {
			return
		}
		s.cacheBackend = backend
		s.cacheCoordinator, _ = cache.NewCoordinator(backend)
	})
	return s.cacheCoordinator
}

// invalidateCachedExample 在写操作成功后删除详情缓存，避免样例接口返回旧数据。
// 删除失败只记录警告，不回滚已经成功的持久化操作。
func (s *ExampleService) invalidateCachedExample(ctx context.Context, id string) {
	if s.ServiceLocator == nil || s.GetContext() == nil {
		return
	}
	appCtx := s.GetContext()
	if s.resolveCacheCoordinator(appCtx) == nil || s.cacheBackend == nil {
		return
	}
	if err := s.cacheBackend.Delete(ctx, exampleDetailCacheKey(id)); err != nil {
		appCtx.GetLogger().WarnWith(appCtx.GetConfig().LogOriginCache()).
			Err(err).Str("cache_key", exampleDetailCacheKey(id)).
			Msg("example detail cache was not invalidated")
	}
}

// readThroughListLegacy 保留通过 CacheOption 解析后端的兼容路径。新代码应像
// NewExampleService 的主路径一样长期保存并复用 Coordinator。
func (s *ExampleService) readThroughListLegacy(option *cache.CacheOption, loader exampleListLoader) (*responsevo.ExampleListRespVo, error) {
	return cache.GetCached[*responsevo.ExampleListRespVo](option, loader)
}

// dispatchExampleChanged 入队一个 asynq 任务，通知某个 example 被
// 创建/更新/删除。错误返回给调用方（由 observeDispatchError 决定是否
// 记录日志并吞掉）。
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

// applicationContext 返回支撑本 service 的 fiberhouse.IContext；若构造时未提供，
// 则返回 nil。
func (s *ExampleService) applicationContext() fiberhouse.IContext {
	if s.ServiceLocator == nil {
		return nil
	}
	return s.GetContext()
}

// observeDispatchError 将失败的异步派发记为警告日志而不向上传播：通知投递是
// 尽力而为的，绝不能让已经成功的写操作因此失败。
func (s *ExampleService) observeDispatchError(err error) {
	if err == nil || s.ServiceLocator == nil || s.GetContext() == nil {
		return
	}
	s.GetContext().GetLogger().WarnWith(s.GetContext().GetConfig().LogOriginTask()).
		Err(err).Msg("example changed event was not enqueued")
}

// currentTime 通过可注入的 now 函数返回当前 UTC 时间；当其未设置时（例如
// 零值 ExampleService）回退为 time.Now。
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
