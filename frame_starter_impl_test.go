package fiberhouse

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

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

func (r *frameTestValidateRegister) RegisterToWrap(*validate.Wrap) { (*r.called)++ }

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
		frame.RegisterGlobalsKeepalive()

		assert.Nil(t, ctx.GetStarterApp())
		assert.Zero(t, task.serverRegistrations)
		assert.Zero(t, task.dispatcherRegistrations)
	})

	t.Run("disabled and nil task paths do not start servers", func(t *testing.T) {
		disabledCtx, _ := newFrameTestContext(t, map[string]interface{}{
			"application.task.enableServer":      false,
			"application.globalManage.keepAlive": false,
		})
		disabled := &FrameApplication{Ctx: disabledCtx, task: &frameTestTask{}}
		disabled.RegisterTaskServer()
		disabled.RegisterGlobalsKeepalive()

		enabledCtx, _ := newFrameTestContext(t, map[string]interface{}{"application.task.enableServer": true})
		enabled := &FrameApplication{Ctx: enabledCtx}
		enabled.RegisterTaskServer()
	})
}
