package fiberhouse

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type task6Application struct{ fastKey globalmanager.KeyName }

func (a *task6Application) GetDBKey() globalmanager.KeyName                  { return "task6-db" }
func (a *task6Application) GetCacheKey() globalmanager.KeyName               { return "task6-cache" }
func (a *task6Application) GetDBMongoKey() globalmanager.KeyName             { return "task6-mongo" }
func (a *task6Application) GetDBMysqlKey() globalmanager.KeyName             { return "task6-mysql" }
func (a *task6Application) GetRedisKey() globalmanager.KeyName               { return "task6-redis" }
func (a *task6Application) GetFastTrafficCodecKey() globalmanager.KeyName    { return a.fastKey }
func (a *task6Application) GetDefaultTrafficCodecKey() globalmanager.KeyName { return "task6-default" }
func (a *task6Application) GetLocalCacheKey() globalmanager.KeyName          { return "task6-local" }
func (a *task6Application) GetRemoteCacheKey() globalmanager.KeyName         { return "task6-remote" }
func (a *task6Application) GetLevel2CacheKey() globalmanager.KeyName         { return "task6-level2" }
func (a *task6Application) GetTaskDispatcherKey() globalmanager.KeyName      { return "task6-dispatcher" }
func (a *task6Application) GetTaskServerKey() globalmanager.KeyName          { return "task6-server" }
func (a *task6Application) GetKey(flag InstanceKeyFlag) (InstanceKey, error) {
	return InstanceKey(flag), nil
}
func (a *task6Application) GetMustKey(flag InstanceKeyFlag) InstanceKey { return InstanceKey(flag) }

type task6Starter struct{ application IApplication }

func (s *task6Starter) GetApplication() IApplication { return s.application }

type task6Context struct {
	IApplicationContext
	starter IStarter
}

func (c *task6Context) GetStarter() IStarter { return c.starter }

func newTask6Context() *task6Context {
	logger := zerolog.Nop()
	base := NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
	application := &task6Application{fastKey: "task6-codec"}
	return &task6Context{IApplicationContext: base, starter: &task6Starter{application: application}}
}

func newTask6RedisClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

func TestTaskWorker_MuxInjectsAppContextAndPropagatesHandlerError(t *testing.T) {
	appCtx := newTask6Context()
	worker := NewTaskWorker(appCtx, newTask6RedisClient(t), asynq.Config{Concurrency: 1})
	require.Same(t, appCtx, worker.GetContext())
	require.NotNil(t, worker.GetServer())
	require.NotNil(t, worker.GetMux())

	sentinel := errors.New("handler failed")
	worker.HandleFunc("task6:inject", func(ctx context.Context, task *asynq.Task) error {
		assert.Same(t, appCtx, ctx.Value(ContextKeyAppCtx))
		assert.Equal(t, "task6:inject", task.Type())
		return sentinel
	})
	err := worker.GetMux().ProcessTask(context.Background(), asynq.NewTask("task6:inject", nil))
	assert.ErrorIs(t, err, sentinel)
}

func TestTaskWorker_HandleAndRegisterHandlersProcessWithoutRedis(t *testing.T) {
	appCtx := newTask6Context()
	worker := NewTaskWorker(appCtx, newTask6RedisClient(t), asynq.Config{Concurrency: 1})
	processed := make(map[string]bool)
	worker.Handle("task6:handler", asynq.HandlerFunc(func(context.Context, *asynq.Task) error {
		processed["handler"] = true
		return nil
	}))
	worker.RegisterHandlers(TaskHandlerMap{
		"task6:map": func(context.Context, *asynq.Task) error {
			processed["map"] = true
			return nil
		},
	})
	for _, taskType := range []string{"task6:handler", "task6:map"} {
		require.NoError(t, worker.GetMux().ProcessTask(context.Background(), asynq.NewTask(taskType, nil)))
	}
	assert.Equal(t, map[string]bool{"handler": true, "map": true}, processed)
}

func TestTaskDispatcher_ConstructsClientWithoutNetworkOperation(t *testing.T) {
	dispatcher := NewTaskDispatcher(newTask6RedisClient(t))
	assert.NotNil(t, dispatcher)
	assert.NotNil(t, dispatcher.Client)
}

func TestPayload_NilFallbackContainerHitMissingAndWrongType(t *testing.T) {
	payload := NewPayloadBase()
	assert.IsType(t, &jsoncodec.SonicJSON{}, payload.GetDefault(nil))
	codec, err := payload.GetJsonHandler(nil)
	require.NoError(t, err)
	assert.IsType(t, &jsoncodec.SonicJSON{}, codec)
	assert.IsType(t, &jsoncodec.SonicJSON{}, payload.GetMustJsonHandler(nil))

	ctx := newTask6Context()
	app := ctx.starter.GetApplication().(*task6Application)
	manager := ctx.GetContainer()
	keys := []globalmanager.KeyName{"task6-codec-hit", "task6-codec-missing", "task6-codec-wrong"}
	for _, key := range keys {
		manager.Clear(key)
		key := key
		t.Cleanup(func() { manager.Clear(key) })
	}

	app.fastKey = keys[0]
	want := jsoncodec.StdJsonDefault()
	require.True(t, manager.Register(app.fastKey, func() (interface{}, error) { return want, nil }))
	got, err := payload.GetJsonHandler(ctx)
	require.NoError(t, err)
	assert.Same(t, want, got)
	assert.Same(t, want, payload.GetMustJsonHandler(ctx))

	app.fastKey = keys[1]
	_, err = payload.GetJsonHandler(ctx)
	assert.ErrorContains(t, err, "not found")
	assert.IsType(t, &jsoncodec.SonicJSON{}, payload.GetMustJsonHandler(ctx))

	app.fastKey = keys[2]
	require.True(t, manager.Register(app.fastKey, func() (interface{}, error) { return "not a codec", nil }))
	_, err = payload.GetJsonHandler(ctx)
	assert.ErrorContains(t, err, "assertion failure")
	assert.IsType(t, &jsoncodec.SonicJSON{}, payload.GetMustJsonHandler(ctx))
}
