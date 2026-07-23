package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/requestvo"
	"github.com/lamxy/fiberhouse/example_application/apivo/example/responsevo"
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/entity"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeExampleTaskDispatcher struct {
	enqueueFn func(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

func (f fakeExampleTaskDispatcher) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return f.enqueueFn(ctx, task, opts...)
}

type contextKey string

type listTestApplication struct {
	fiberhouse.IApplication
	level2Key globalmanager.KeyName
	codecKey  globalmanager.KeyName
}

func (a *listTestApplication) GetLevel2CacheKey() globalmanager.KeyName { return a.level2Key }
func (a *listTestApplication) GetDefaultTrafficCodecKey() globalmanager.KeyName {
	return a.codecKey
}

type listTestStarter struct {
	fiberhouse.IStarter
	application fiberhouse.IApplication
}

func (s *listTestStarter) GetApplication() fiberhouse.IApplication { return s.application }

type listTestContext struct {
	fiberhouse.IApplicationContext
	starter fiberhouse.IStarter
}

func (c *listTestContext) GetStarter() fiberhouse.IStarter { return c.starter }

type listMemoryCache struct {
	values map[string]string
}

func (c *listMemoryCache) Get(_ context.Context, key string, _ *cache.CacheOption) (string, error) {
	value, ok := c.values[key]
	if !ok {
		return "", cache.ErrKeyNotFound
	}
	return value, nil
}

func (c *listMemoryCache) Set(_ context.Context, key string, value interface{}, _ *cache.CacheOption) error {
	c.values[key] = value.(string)
	return nil
}

func (*listMemoryCache) Delete(context.Context, ...string) error { return nil }
func (*listMemoryCache) Close() error                            { return nil }
func (*listMemoryCache) Wait() error                             { return nil }
func (*listMemoryCache) GetLevel() cache.Level                   { return cache.Level2 }

type fakeExampleStore struct {
	createFn func(context.Context, *entity.Example) error
	getFn    func(context.Context, string) (*entity.Example, error)
	listFn   func(context.Context, repository.ListOptions) ([]entity.Example, int64, error)
	updateFn func(context.Context, string, *entity.Example) error
	deleteFn func(context.Context, string) error
}

func (f *fakeExampleStore) Create(ctx context.Context, example *entity.Example) error {
	return f.createFn(ctx, example)
}
func (f *fakeExampleStore) Get(ctx context.Context, id string) (*entity.Example, error) {
	return f.getFn(ctx, id)
}
func (f *fakeExampleStore) List(ctx context.Context, opts repository.ListOptions) ([]entity.Example, int64, error) {
	return f.listFn(ctx, opts)
}
func (f *fakeExampleStore) Update(ctx context.Context, id string, example *entity.Example) error {
	return f.updateFn(ctx, id, example)
}
func (f *fakeExampleStore) Delete(ctx context.Context, id string) error {
	return f.deleteFn(ctx, id)
}

func TestListExamplesReqVoNormalizeAndValidationBoundary(t *testing.T) {
	normalized := (requestvo.ListExamplesReqVo{}).Normalize()
	if normalized.Page != 1 || normalized.PageSize != 20 {
		t.Fatalf("Normalize() = page %d size %d, want 1 and 20", normalized.Page, normalized.PageSize)
	}

	validate := validator.New()
	if err := validate.Struct(requestvo.ListExamplesReqVo{Page: 1, PageSize: 100}); err != nil {
		t.Fatalf("page_size=100 should be valid: %v", err)
	}
	if err := validate.Struct(requestvo.ListExamplesReqVo{Page: 1, PageSize: 101}); err == nil {
		t.Fatal("page_size=101 should be invalid")
	}
}

func TestCreateTrimsInputDefaultsStatusMapsResponseAndPreservesContext(t *testing.T) {
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	id := bson.NewObjectID()
	store := &fakeExampleStore{}
	store.createFn = func(gotCtx context.Context, got *entity.Example) error {
		if gotCtx != ctx {
			t.Fatal("Create did not receive caller context")
		}
		want := &entity.Example{
			Name:        "Example",
			Description: "description",
			Status:      entity.ExampleStatusActive,
			Tags:        []string{"go", "mongo"},
			Timestamps:  fields.Timestamps{CreatedAt: now.UTC(), UpdatedAt: now.UTC()},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Create entity = %#v, want %#v", got, want)
		}
		got.ID = id
		return nil
	}
	service := &ExampleService{Store: store, now: func() time.Time { return now }}

	resp, err := service.Create(ctx, requestvo.CreateExampleReqVo{
		Name:        "  Example  ",
		Description: "  description  ",
		Tags:        []string{" go ", "mongo "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != id.Hex() || resp.Name != "Example" || resp.Status != "active" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if !reflect.DeepEqual(resp.Tags, []string{"go", "mongo"}) {
		t.Fatalf("response tags = %#v", resp.Tags)
	}
}

func TestCreateRejectsInvalidCanonicalValuesAfterNormalization(t *testing.T) {
	tests := []struct {
		name string
		req  requestvo.CreateExampleReqVo
	}{
		{name: "blank name", req: requestvo.CreateExampleReqVo{Name: "  "}},
		{name: "short trimmed name", req: requestvo.CreateExampleReqVo{Name: " a "}},
		{name: "long rune name", req: requestvo.CreateExampleReqVo{Name: strings.Repeat("界", 81)}},
		{name: "long rune description", req: requestvo.CreateExampleReqVo{Name: "ok", Description: strings.Repeat("界", 501)}},
		{name: "unsupported status", req: requestvo.CreateExampleReqVo{Name: "ok", Status: "pending"}},
		{name: "too many tags", req: requestvo.CreateExampleReqVo{Name: "ok", Tags: make([]string, 11)}},
		{name: "blank tag", req: requestvo.CreateExampleReqVo{Name: "ok", Tags: []string{" "}}},
		{name: "long rune tag", req: requestvo.CreateExampleReqVo{Name: "ok", Tags: []string{strings.Repeat("界", 31)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			service := &ExampleService{Store: &fakeExampleStore{createFn: func(context.Context, *entity.Example) error {
				called = true
				return nil
			}}}
			if _, err := service.Create(context.Background(), tt.req); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
			}
			if called {
				t.Fatal("invalid canonical input reached the store")
			}
		})
	}
}

func TestUpdatePreservesUnspecifiedFields(t *testing.T) {
	now := time.Date(2026, 7, 24, 2, 0, 0, 0, time.UTC)
	created := now.Add(-time.Hour)
	name := "  renamed  "
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	existing := &entity.Example{
		ID:          bson.NewObjectID(),
		Name:        "old",
		Description: "keep",
		Status:      entity.ExampleStatusArchived,
		Tags:        []string{"keep"},
		Timestamps:  fields.Timestamps{CreatedAt: created, UpdatedAt: created},
	}
	store := &fakeExampleStore{
		getFn: func(got context.Context, id string) (*entity.Example, error) {
			if got != ctx {
				t.Fatal("Get did not receive caller context")
			}
			copy := *existing
			return &copy, nil
		},
		updateFn: func(got context.Context, id string, example *entity.Example) error {
			if got != ctx {
				t.Fatal("Update did not receive caller context")
			}
			if example.Name != "renamed" || example.Description != "keep" ||
				example.Status != entity.ExampleStatusArchived ||
				!reflect.DeepEqual(example.Tags, []string{"keep"}) {
				t.Fatalf("partial update lost fields: %#v", example)
			}
			if !example.CreatedAt.Equal(created) || !example.UpdatedAt.Equal(now) {
				t.Fatalf("timestamps = %#v", example.Timestamps)
			}
			return nil
		},
	}
	service := &ExampleService{Store: store, now: func() time.Time { return now }}

	resp, err := service.Update(ctx, existing.ID.Hex(), requestvo.UpdateExampleReqVo{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "renamed" || resp.Description != "keep" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestUpdateRejectsInvalidProvidedCanonicalValuesBeforeLoading(t *testing.T) {
	blank := " "
	short := " a "
	longDescription := strings.Repeat("界", 501)
	unsupportedStatus := "pending"
	blankTags := []string{" "}
	tests := []requestvo.UpdateExampleReqVo{
		{Name: &blank},
		{Name: &short},
		{Description: &longDescription},
		{Status: &unsupportedStatus},
		{Tags: &blankTags},
	}
	for i, req := range tests {
		called := false
		service := &ExampleService{Store: &fakeExampleStore{getFn: func(context.Context, string) (*entity.Example, error) {
			called = true
			return nil, nil
		}}}
		if _, err := service.Update(context.Background(), "id", req); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("case %d Update() error = %v, want ErrInvalidInput", i, err)
		}
		if called {
			t.Fatalf("case %d invalid canonical input reached the store", i)
		}
	}
}

func TestListReturnsEmptyJSONArrayAndPropagatesErrors(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	sentinel := errors.New("store failed")
	store := &fakeExampleStore{
		listFn: func(got context.Context, opts repository.ListOptions) ([]entity.Example, int64, error) {
			if got != ctx {
				t.Fatal("List did not receive caller context")
			}
			if opts.Page != 1 || opts.PageSize != 20 || opts.Status != entity.ExampleStatusActive {
				t.Fatalf("list options = %#v", opts)
			}
			return nil, 0, nil
		},
		getFn: func(context.Context, string) (*entity.Example, error) { return nil, sentinel },
	}
	service := &ExampleService{Store: store}

	resp, err := service.List(ctx, requestvo.ListExamplesReqVo{Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Items == nil || len(resp.Items) != 0 {
		t.Fatalf("Items = %#v, want non-nil empty slice", resp.Items)
	}
	if _, err = service.Get(ctx, "id"); !errors.Is(err, sentinel) {
		t.Fatalf("Get error = %v, want sentinel", err)
	}
}

func TestListCacheUsesNormalizedKeyAndCallerContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	store := &fakeExampleStore{
		listFn: func(got context.Context, opts repository.ListOptions) ([]entity.Example, int64, error) {
			if got != ctx {
				t.Fatal("cache loader replaced caller context")
			}
			if opts.Page != 1 || opts.PageSize != 20 {
				t.Fatalf("options = %#v", opts)
			}
			return nil, 0, nil
		},
	}
	service := &ExampleService{
		Store: store,
		listCached: func(got context.Context, key string, ttl time.Duration, loader exampleListLoader) (*responsevo.ExampleListRespVo, error) {
			if got != ctx {
				t.Fatal("cache did not receive caller context")
			}
			if key != "example:list:page:1:size:20:status:active" {
				t.Fatalf("cache key = %q", key)
			}
			if ttl != exampleListCacheTTL {
				t.Fatalf("ttl = %s", ttl)
			}
			return loader(got)
		},
	}

	if _, err := service.List(ctx, requestvo.ListExamplesReqVo{Status: " active "}); err != nil {
		t.Fatal(err)
	}
}

func TestListProductionCacheOptionsLoadStoreOnceAndPreserveCallerContext(t *testing.T) {
	logger := zerolog.Nop()
	base := fiberhouse.NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
	key := globalmanager.KeyName("example-list-cache-test")
	codecKey := globalmanager.KeyName("example-list-codec-test")
	app := &listTestApplication{level2Key: key, codecKey: codecKey}
	ctx := &listTestContext{
		IApplicationContext: base,
		starter:             &listTestStarter{application: app},
	}
	memory := &listMemoryCache{values: make(map[string]string)}
	ctx.GetContainer().Clear(key)
	ctx.GetContainer().Clear(codecKey)
	if !ctx.GetContainer().Register(key, func() (interface{}, error) { return memory, nil }) {
		t.Fatal("register list cache")
	}
	if !ctx.GetContainer().Register(codecKey, func() (interface{}, error) { return jsoncodec.StdJsonDefault(), nil }) {
		t.Fatal("register list codec")
	}
	t.Cleanup(func() {
		ctx.GetContainer().Clear(key)
		ctx.GetContainer().Clear(codecKey)
	})

	caller := context.WithValue(context.Background(), contextKey("request"), "same")
	loads := 0
	store := &fakeExampleStore{listFn: func(got context.Context, _ repository.ListOptions) ([]entity.Example, int64, error) {
		loads++
		if got != caller {
			t.Fatal("cache loader replaced caller context")
		}
		return nil, 0, nil
	}}
	service := NewExampleService(ctx, store)

	for i := 0; i < 2; i++ {
		if _, err := service.List(caller, requestvo.ListExamplesReqVo{Status: "active"}); err != nil {
			t.Fatalf("List() call %d error = %v", i+1, err)
		}
	}
	if loads != 1 {
		t.Fatalf("store loader calls = %d, want 1", loads)
	}
}

func TestDispatchExampleChangedUsesCallerContextAndStableOptions(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKey("request"), "same")
	service := &ExampleService{
		getTaskDispatcher: func() (exampleTaskDispatcher, error) {
			return fakeExampleTaskDispatcher{enqueueFn: func(got context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
				if got != ctx {
					t.Fatal("dispatcher did not receive caller context")
				}
				if task.Type() != "example:changed" {
					t.Fatalf("task type = %q", task.Type())
				}
				if len(opts) != 2 || opts[0].Type() != asynq.QueueOpt || opts[0].Value() != "default" ||
					opts[1].Type() != asynq.MaxRetryOpt || opts[1].Value() != 3 {
					t.Fatalf("enqueue options = %#v", opts)
				}
				return &asynq.TaskInfo{ID: "task-id"}, nil
			}}, nil
		},
	}

	if err := service.dispatchExampleChanged(ctx, "507f1f77bcf86cd799439011", "update"); err != nil {
		t.Fatal(err)
	}
}

func TestDispatchExampleChangedReturnsConstructionAndEnqueueErrorsWithoutNilDereference(t *testing.T) {
	constructionErr := errors.New("dispatcher unavailable")
	service := &ExampleService{
		getTaskDispatcher: func() (exampleTaskDispatcher, error) {
			return nil, constructionErr
		},
	}
	if err := service.dispatchExampleChanged(context.Background(), "507f1f77bcf86cd799439011", "create"); !errors.Is(err, constructionErr) {
		t.Fatalf("construction error = %v", err)
	}

	service.getTaskDispatcher = func() (exampleTaskDispatcher, error) { return nil, nil }
	if err := service.dispatchExampleChanged(context.Background(), "507f1f77bcf86cd799439011", "create"); err == nil {
		t.Fatal("nil dispatcher should return an error")
	}

	enqueueErr := errors.New("enqueue failed")
	service.getTaskDispatcher = func() (exampleTaskDispatcher, error) {
		return fakeExampleTaskDispatcher{enqueueFn: func(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
			return nil, enqueueErr
		}}, nil
	}
	if err := service.dispatchExampleChanged(context.Background(), "507f1f77bcf86cd799439011", "create"); !errors.Is(err, enqueueErr) {
		t.Fatalf("enqueue error = %v", err)
	}
}

func TestCreateKeepsCanonicalCRUDSuccessWhenEventDispatchFails(t *testing.T) {
	id := bson.NewObjectID()
	store := &fakeExampleStore{createFn: func(_ context.Context, example *entity.Example) error {
		example.ID = id
		return nil
	}}
	service := &ExampleService{
		Store: store,
		getTaskDispatcher: func() (exampleTaskDispatcher, error) {
			return nil, errors.New("redis unavailable")
		},
	}

	got, err := service.Create(context.Background(), requestvo.CreateExampleReqVo{Name: "example"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id.Hex() {
		t.Fatalf("id = %q", got.ID)
	}
}
