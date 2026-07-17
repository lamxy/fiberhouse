package fiberhouse

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/validate"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type frameTestApplication struct {
	ApplicationRegister
	name             string
	initializers     globalmanager.InitializerMap
	required         []globalmanager.KeyName
	validateInits    []validate.ValidateInitializer
	tagRegistrations []validate.RegisterValidatorTagFunc
}

func (a *frameTestApplication) GetName() string     { return a.name }
func (a *frameTestApplication) SetName(name string) { a.name = name }
func (a *frameTestApplication) ConfigGlobalInitializers() globalmanager.InitializerMap {
	return a.initializers
}
func (a *frameTestApplication) ConfigRequiredGlobalKeys() []globalmanager.KeyName {
	return a.required
}
func (a *frameTestApplication) ConfigCustomValidateInitializers() []validate.ValidateInitializer {
	return a.validateInits
}
func (a *frameTestApplication) ConfigValidatorCustomTags() []validate.RegisterValidatorTagFunc {
	return a.tagRegistrations
}

type frameTestModule struct{ ModuleRegister }

type frameTestTask struct {
	TaskRegister
	serverRegistrations     int
	dispatcherRegistrations int
}

func (t *frameTestTask) RegisterTaskServerToContainer()     { t.serverRegistrations++ }
func (t *frameTestTask) RegisterTaskDispatcherToContainer() { t.dispatcherRegistrations++ }

type frameTestValidateRegister struct {
	called *int
}

type frameTestZeroIntervalConfig struct{ appconfig.IAppConfig }

func (frameTestZeroIntervalConfig) Duration(string, ...time.Duration) time.Duration { return 0 }

func forceFrameTestZeroInterval(t *testing.T, ctx IApplicationContext) {
	t.Helper()
	appCtx, ok := ctx.(*AppContext)
	require.True(t, ok)
	appCtx.cfg = frameTestZeroIntervalConfig{IAppConfig: appCtx.cfg}
}

func (r *frameTestValidateRegister) RegisterToWrap(*validate.Wrap) { (*r.called)++ }

type frameBlockingHealthChecker struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (h *frameBlockingHealthChecker) IsHealthy() bool {
	if h.calls.Add(1) == 1 {
		close(h.entered)
	}
	<-h.release
	return true
}

type frameFailingHealthRebuilder struct{}

func (*frameFailingHealthRebuilder) IsHealthy() bool { return false }

func (*frameFailingHealthRebuilder) Rebuild(...interface{}) (interface{}, error) {
	return nil, errors.New("frame rebuild failed")
}

func (*frameFailingHealthRebuilder) GetConfPath() string { return "frame-test" }

type frameTestSavedEntry struct {
	key        string
	registered bool
	value      interface{}
}

func isolateFrameTestEntries(t *testing.T, manager *globalmanager.GlobalManager, keys []string) {
	t.Helper()
	saved := make([]frameTestSavedEntry, 0, len(keys))
	for _, key := range keys {
		entry := frameTestSavedEntry{key: key, registered: manager.IsRegistered(key)}
		if entry.registered {
			value, err := manager.Get(key)
			require.NoError(t, err, key)
			entry.value = value
		}
		manager.Unregister(key)
		saved = append(saved, entry)
	}
	t.Cleanup(func() {
		for i := len(saved) - 1; i >= 0; i-- {
			entry := saved[i]
			manager.Unregister(entry.key)
			if entry.registered {
				require.True(t, manager.Register(entry.key, func() (interface{}, error) {
					return entry.value, nil
				}), entry.key)
				restored, err := manager.Get(entry.key)
				require.NoError(t, err, entry.key)
				assert.Same(t, entry.value, restored, entry.key)
			}
		}
	})
}

func newFrameTestContext(t *testing.T, values map[string]interface{}) (IApplicationContext, *bytes.Buffer) {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	if values != nil {
		cfg.LoadDefault(values)
	}
	require.NoError(t, cfg.RegisterLogOrigin("task7-"+t.Name(), appconfig.LogOrigin("Task7-"+t.Name())))
	cfg.Initialize()
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&BootConfig{CoreType: "fiber", TrafficCodec: "test"})
	return ctx, &logs
}

func isolateFrameHealthManager(t *testing.T, ctx IApplicationContext) *globalmanager.GlobalManager {
	t.Helper()
	appCtx := ctx.(*AppContext)
	manager := globalmanager.NewGlobalManager()
	appCtx.container = manager
	return manager
}

func TestFrameApplication_RegistrationGettersAndContextMount(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	application := &frameTestApplication{name: "application"}
	module := &frameTestModule{}
	task := &frameTestTask{}
	frame := &FrameApplication{Ctx: ctx}

	frame.RegisterApplication(application)
	frame.RegisterModule(module)
	frame.RegisterTask(task)

	assert.Same(t, ctx, frame.GetContext())
	assert.Same(t, frame, frame.GetFrameApp())
	assert.Same(t, application, frame.GetApplication())
	assert.Same(t, module, frame.GetModule())
	assert.Same(t, task, frame.GetTask())

	starter := &WebApplication{FrameStarter: frame, CoreStarter: &lifecycleRecordingStarter{
		managerCalls: make(map[string][]IProviderManager),
	}}
	frame.RegisterToCtx(starter)
	assert.Same(t, starter, ctx.GetStarterApp())
}

func TestFrameApplication_ApplicationGlobalsInitializesObservableContracts(t *testing.T) {
	ctx, logs := newFrameTestContext(t, nil)
	key := globalmanager.KeyName(fmt.Sprintf("task7-required-%s", t.Name()))
	missing := globalmanager.KeyName(fmt.Sprintf("task7-missing-%s", t.Name()))
	ctx.GetContainer().Unregister(key)
	t.Cleanup(func() {
		ctx.GetContainer().Unregister(key)
	})
	initializerCalls := 0
	validateInitCalls := 0
	tagCalls := 0
	task := &frameTestTask{}
	application := &frameTestApplication{
		initializers: globalmanager.InitializerMap{
			key: func() (interface{}, error) {
				initializerCalls++
				return "ready", nil
			},
		},
		required: []globalmanager.KeyName{key, missing},
		validateInits: []validate.ValidateInitializer{func() validate.ValidateRegister {
			return &frameTestValidateRegister{called: &validateInitCalls}
		}},
		tagRegistrations: []validate.RegisterValidatorTagFunc{
			func(*validate.Wrap) error { tagCalls++; return nil },
			func(*validate.Wrap) error { tagCalls++; return errors.New("tag registration failed") },
		},
	}
	frame := &FrameApplication{Ctx: ctx, application: application, task: task}
	originKeys := make([]string, 0, len(ctx.GetConfig().GetLogOriginMap()))
	for originKey, origin := range ctx.GetConfig().GetLogOriginMap() {
		if originKey != "" {
			originKeys = append(originKeys, origin.InstanceKey())
		}
	}
	isolateFrameTestEntries(t, ctx.GetContainer(), originKeys)
	for originKey, origin := range ctx.GetConfig().GetLogOriginMap() {
		if originKey != "" {
			assert.False(t, ctx.GetContainer().IsRegistered(origin.InstanceKey()), originKey)
		}
	}

	frame.RegisterApplicationGlobals()

	assert.Equal(t, 1, initializerCalls)
	assert.Equal(t, 1, validateInitCalls)
	assert.Equal(t, 2, tagCalls)
	assert.Equal(t, 1, task.serverRegistrations)
	assert.Equal(t, 1, task.dispatcherRegistrations)
	assert.True(t, ctx.GetContainer().IsRegistered(key))
	assert.Contains(t, logs.String(), string(missing))
	assert.Contains(t, logs.String(), "tag registration failed")
	for originKey, origin := range ctx.GetConfig().GetLogOriginMap() {
		if originKey != "" {
			assert.True(t, ctx.GetContainer().IsRegistered(origin.InstanceKey()), originKey)
		}
	}
}

func TestFrameApplication_GuardsAvoidStartupSideEffects(t *testing.T) {
	t.Run("missing application panics", func(t *testing.T) {
		ctx, _ := newFrameTestContext(t, nil)
		frame := &FrameApplication{Ctx: ctx}
		assert.PanicsWithError(t,
			"application that implements the ApplicationRegister interface is nil. Please RegisterApplication first",
			frame.RegisterGlobalInitializers,
		)
	})

	t.Run("started application returns early", func(t *testing.T) {
		ctx, _ := newFrameTestContext(t, map[string]interface{}{
			"application.task.enableServer":      true,
			"application.globalManage.keepAlive": true,
		})
		ctx.RegisterAppState(true)
		forceFrameTestZeroInterval(t, ctx)
		application := &frameTestApplication{}
		task := &frameTestTask{}
		frame := &FrameApplication{Ctx: ctx, application: application, task: task}

		frame.RegisterToCtx(&WebApplication{FrameStarter: frame})
		frame.RegisterApplicationGlobals()
		frame.RegisterGlobalInitializers()
		frame.InitializeGlobalRequired()
		frame.InitializeCustomValidateInitializers()
		frame.RegisterValidatorCustomTags()
		frame.RegisterLoggerWithOriginToContainer()
		frame.RegisterTaskServer()
		assert.NotPanics(t, func() { frame.RegisterGlobalsKeepalive() })

		assert.Nil(t, ctx.GetStarterApp())
		assert.Zero(t, task.serverRegistrations)
		assert.Zero(t, task.dispatcherRegistrations)
	})

	t.Run("disabled and nil task paths do not start servers", func(t *testing.T) {
		disabledCtx, _ := newFrameTestContext(t, map[string]interface{}{
			"application.task.enableServer":      false,
			"application.globalManage.keepAlive": false,
		})
		forceFrameTestZeroInterval(t, disabledCtx)
		disabled := &FrameApplication{Ctx: disabledCtx, task: &frameTestTask{}}
		disabled.RegisterTaskServer()
		assert.NotPanics(t, func() { disabled.RegisterGlobalsKeepalive() })

		enabledCtx, _ := newFrameTestContext(t, map[string]interface{}{"application.task.enableServer": true})
		enabled := &FrameApplication{Ctx: enabledCtx}
		enabled.RegisterTaskServer()
	})
}

func TestFrameApplication_StopHealthCheckWaitsAndPreventsRestart(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	manager := isolateFrameHealthManager(t, ctx)
	checker := &frameBlockingHealthChecker{entered: make(chan struct{}), release: make(chan struct{})}
	manager.Register("health", func() (interface{}, error) { return checker, nil })
	_, _ = manager.Get("health")
	frame := &FrameApplication{Ctx: ctx}

	frame.startHealthCheck(time.Millisecond)
	select {
	case <-checker.entered:
	case <-time.After(time.Second):
		t.Fatal("health check did not start")
	}
	stopped := make(chan struct{})
	go func() {
		frame.stopHealthCheck()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("stopHealthCheck returned before active check completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(checker.release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stopHealthCheck did not return")
	}

	before := checker.calls.Load()
	frame.startHealthCheck(time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	if got := checker.calls.Load(); got != before {
		t.Fatalf("health check restarted: calls=%d, want %d", got, before)
	}
}

func TestFrameApplication_StopHealthCheckIsConcurrentAndIdempotent(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	isolateFrameHealthManager(t, ctx)
	frame := &FrameApplication{Ctx: ctx}
	frame.startHealthCheck(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			frame.stopHealthCheck()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("concurrent stopHealthCheck calls did not return")
	}
}

func TestFrameApplication_StartHealthCheckRejectsInvalidInterval(t *testing.T) {
	ctx, logs := newFrameTestContext(t, nil)
	isolateFrameHealthManager(t, ctx)
	frame := &FrameApplication{Ctx: ctx}
	frame.startHealthCheck(0)
	if frame.healthCancel != nil {
		t.Fatal("invalid interval started health check")
	}
	assert.Contains(t, logs.String(), "health check interval must be positive")
}

func TestFrameApplication_CheckGlobalsHealthOnceDoesNotLogSuccessAfterRebuildFailure(t *testing.T) {
	ctx, logs := newFrameTestContext(t, nil)
	manager := isolateFrameHealthManager(t, ctx)
	manager.Register("unhealthy", func() (interface{}, error) {
		return &frameFailingHealthRebuilder{}, nil
	})
	_, err := manager.Get("unhealthy")
	require.NoError(t, err)
	frame := &FrameApplication{Ctx: ctx}

	frame.checkGlobalsHealthOnce()

	assert.Contains(t, logs.String(), "rebuild failed")
	assert.NotContains(t, logs.String(), "rebuild success")
}
