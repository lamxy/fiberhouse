package module

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache/cacheremote"
	"github.com/rs/zerolog"
)

type taskAsyncApplication struct {
	fiberhouse.IApplication
	redisKey      string
	dispatcherKey string
}

func (a *taskAsyncApplication) GetRedisKey() string          { return a.redisKey }
func (a *taskAsyncApplication) GetTaskDispatcherKey() string { return a.dispatcherKey }

type taskAsyncStarter struct {
	fiberhouse.ApplicationStarter
	application fiberhouse.IApplication
}

func (s *taskAsyncStarter) GetApplication() fiberhouse.IApplication { return s.application }

func newTaskAsyncForTest(t *testing.T) (*TaskAsync, string, string) {
	t.Helper()
	suffix := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	suffix = fmt.Sprintf("%s-%d", suffix, time.Now().UnixNano())
	redisKey := "task-async-redis-" + suffix
	dispatcherKey := "task-async-dispatcher-" + suffix

	cfg := appconfig.NewAppConfig()
	cfg.LoadDefault(map[string]interface{}{
		"application": map[string]interface{}{
			"task": map[string]interface{}{"enableServer": true},
		},
	})
	cfg.Initialize()
	logger := zerolog.Nop()
	ctx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterStarterApp(&taskAsyncStarter{application: &taskAsyncApplication{
		redisKey: redisKey, dispatcherKey: dispatcherKey,
	}})
	manager := ctx.GetContainer()
	manager.Unregister(redisKey)
	manager.Unregister(dispatcherKey)
	t.Cleanup(func() {
		manager.Unregister(redisKey)
		manager.Unregister(dispatcherKey)
	})
	return NewTaskAsync(ctx).(*TaskAsync), redisKey, dispatcherKey
}

func TestTaskAsyncDispatcherInitializerRejectsInvalidRedisInstances(t *testing.T) {
	tests := []struct {
		name     string
		register func(*testing.T, *TaskAsync, string)
		wantErr  string
	}{
		{
			name:     "lookup failure",
			register: func(*testing.T, *TaskAsync, string) {},
			wantErr:  "get redis instance",
		},
		{
			name: "wrong type",
			register: func(t *testing.T, taskAsync *TaskAsync, redisKey string) {
				t.Helper()
				if !taskAsync.Ctx.GetContainer().Register(redisKey, func() (interface{}, error) {
					return "not a redis client", nil
				}) {
					t.Fatalf("register redis initializer %q", redisKey)
				}
			},
			wantErr: "invalid redis instance",
		},
		{
			name: "typed nil",
			register: func(t *testing.T, taskAsync *TaskAsync, redisKey string) {
				t.Helper()
				var client *cacheremote.RedisDb
				if !taskAsync.Ctx.GetContainer().Register(redisKey, func() (interface{}, error) {
					return client, nil
				}) {
					t.Fatalf("register redis initializer %q", redisKey)
				}
			},
			wantErr: "invalid redis instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskAsync, redisKey, _ := newTaskAsyncForTest(t)
			tt.register(t, taskAsync, redisKey)
			taskAsync.RegisterTaskDispatcherToContainer()

			dispatcher, err := taskAsync.GetTaskDispatcher()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("GetTaskDispatcher() = %#v, %v, want error containing %q", dispatcher, err, tt.wantErr)
			}
			if dispatcher != nil {
				t.Fatalf("GetTaskDispatcher() dispatcher = %#v, want nil", dispatcher)
			}
		})
	}
}

func TestTaskAsyncGetTaskDispatcherRejectsTypedNil(t *testing.T) {
	taskAsync, _, dispatcherKey := newTaskAsyncForTest(t)
	var dispatcher *fiberhouse.TaskDispatcher
	if !taskAsync.Ctx.GetContainer().Register(dispatcherKey, func() (interface{}, error) {
		return dispatcher, nil
	}) {
		t.Fatalf("register dispatcher initializer %q", dispatcherKey)
	}

	got, err := taskAsync.GetTaskDispatcher()
	if err == nil || !strings.Contains(err.Error(), "assertion failure") {
		t.Fatalf("GetTaskDispatcher() = %#v, %v, want assertion failure", got, err)
	}
	if got != nil {
		t.Fatalf("GetTaskDispatcher() = %#v, want nil", got)
	}
}
