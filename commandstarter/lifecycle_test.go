package commandstarter

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

type commandLifecycleRecorder struct {
	fiberhouse.CommandStarter
	stages []string
}

func (r *commandLifecycleRecorder) InitCoreApp()                               { r.stages = append(r.stages, "InitCoreApp") }
func (r *commandLifecycleRecorder) GetFrameCmdApp() fiberhouse.FrameCmdStarter { return r }
func (r *commandLifecycleRecorder) RegisterGlobalErrHandler(fiberhouse.FrameCmdStarter) {
	r.stages = append(r.stages, "RegisterGlobalErrHandler")
}
func (r *commandLifecycleRecorder) RegisterCommands(fiberhouse.FrameCmdStarter) {
	r.stages = append(r.stages, "RegisterCommands")
}
func (r *commandLifecycleRecorder) RegisterCoreGlobalOptional(fiberhouse.FrameCmdStarter) {
	r.stages = append(r.stages, "RegisterCoreGlobalOptional")
}
func (r *commandLifecycleRecorder) RegisterApplicationGlobals() {
	r.stages = append(r.stages, "RegisterApplicationGlobals")
}
func (r *commandLifecycleRecorder) AppCoreRun() error {
	r.stages = append(r.stages, "AppCoreRun")
	return errors.New("ignored by lifecycle")
}

type commandTestApplication struct {
	fiberhouse.ApplicationCmdRegister
	name                  string
	globalErrHandlerCalls int
	registerCommandsCalls int
	globalOptionalCalls   int
	applicationGlobals    int
	lastCore              interface{}
}

func (a *commandTestApplication) GetName() string     { return a.name }
func (a *commandTestApplication) SetName(name string) { a.name = name }
func (a *commandTestApplication) RegisterGlobalErrHandler(core interface{}) {
	a.globalErrHandlerCalls++
	a.lastCore = core
}
func (a *commandTestApplication) RegisterCommands(core interface{}) {
	a.registerCommandsCalls++
	a.lastCore = core
}
func (a *commandTestApplication) RegisterCoreGlobalOptional(core interface{}) {
	a.globalOptionalCalls++
	a.lastCore = core
}
func (a *commandTestApplication) RegisterApplicationGlobals() { a.applicationGlobals++ }

type failingCommandHealthResource struct {
	rebuildErr error
}

func (r *failingCommandHealthResource) IsHealthy() bool { return false }
func (r *failingCommandHealthResource) Rebuild(...interface{}) (interface{}, error) {
	return nil, r.rebuildErr
}
func (r *failingCommandHealthResource) GetConfPath() string { return "" }

type commandTestContext struct {
	fiberhouse.ICommandContext
	cfg       appconfig.IAppConfig
	logger    bootstrap.LoggerWrapper
	container *globalmanager.GlobalManager
	starter   fiberhouse.CommandStarter
}

func (c *commandTestContext) GetConfig() appconfig.IAppConfig            { return c.cfg }
func (c *commandTestContext) GetLogger() bootstrap.LoggerWrapper         { return c.logger }
func (c *commandTestContext) GetContainer() *globalmanager.GlobalManager { return c.container }
func (c *commandTestContext) GetStarter() fiberhouse.IStarter            { return c.starter }
func (c *commandTestContext) RegisterStarterApp(starter fiberhouse.CommandStarter) {
	c.starter = starter
}
func (c *commandTestContext) GetStarterApp() fiberhouse.CommandStarter { return c.starter }

func newCommandTestContext() fiberhouse.ICommandContext {
	cfg := appconfig.NewAppConfig()
	cfg.LoadDefault(map[string]interface{}{
		"command.name":                       "task7-command",
		"command.usage":                      "exercise lifecycle",
		"command.version":                    "7.0.0",
		"command.sortFlagsByName":            true,
		"command.sortCommandsByName":         true,
		"application.globalManage.keepAlive": false,
	})
	cfg.Initialize()
	logger := zerolog.Nop()
	return &commandTestContext{
		cfg:       cfg,
		logger:    bootstrap.NewLoggerWrap(&logger),
		container: globalmanager.NewGlobalManager(),
	}
}

func TestRunCommandStarter_UsesDocumentedOrder(t *testing.T) {
	recorder := &commandLifecycleRecorder{}

	RunCommandStarter(recorder)

	assert.Equal(t, []string{
		"InitCoreApp",
		"RegisterGlobalErrHandler",
		"RegisterCommands",
		"RegisterCoreGlobalOptional",
		"RegisterApplicationGlobals",
		"AppCoreRun",
	}, recorder.stages)
}

func TestCoreCmdCli_InitializesSortsRegistersAndRunsExplicitArgs(t *testing.T) {
	ctx := newCommandTestContext()
	core := NewCoreCmdCli(ctx, func(fiberhouse.CoreCmdStarter) {}).(*CoreCmdCli)
	existing := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "zulu"},
			&cli.StringFlag{Name: "alpha"},
		},
		Commands: []*cli.Command{{Name: "zulu"}, {Name: "alpha"}},
	}
	core.RegisterCoreApp(existing)
	core.InitCoreApp()
	assert.Same(t, ctx, core.GetAppContext())
	assert.Same(t, core, core.GetCoreCmdApp())
	assert.Equal(t, "alpha", core.coreApp.Flags[0].Names()[0])
	assert.Equal(t, "alpha", core.coreApp.Commands[0].Name)

	core.RegisterCoreApp("wrong type")
	assert.Same(t, existing, core.coreApp)

	application := &commandTestApplication{name: "application"}
	frame := &FrameCmdApplication{Ctx: ctx, application: application}
	core.RegisterGlobalErrHandler(frame)
	core.RegisterCommands(frame)
	core.RegisterCoreGlobalOptional(frame)
	assert.Equal(t, 1, application.globalErrHandlerCalls)
	assert.Equal(t, 1, application.registerCommandsCalls)
	assert.Equal(t, 1, application.globalOptionalCalls)
	assert.Same(t, existing, application.lastCore)

	missing := &FrameCmdApplication{Ctx: ctx}
	assert.Panics(t, func() { core.RegisterGlobalErrHandler(missing) })
	assert.Panics(t, func() { core.RegisterCommands(missing) })
	assert.Panics(t, func() { core.RegisterCoreGlobalOptional(missing) })

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	var received []string
	core.coreApp = &cli.App{
		Name: "task7-command",
		Action: func(c *cli.Context) error {
			received = append([]string(nil), c.Args().Slice()...)
			return nil
		},
	}
	os.Args = []string{"task7-command", "first", "second"}
	require.NoError(t, core.AppCoreRun())
	assert.Equal(t, []string{"first", "second"}, received)
}

func TestCoreCmdCli_ErrorActionUsesRestoredSafeOsExiter(t *testing.T) {
	ctx := newCommandTestContext()
	core := NewCoreCmdCli(ctx, func(fiberhouse.CoreCmdStarter) {}).(*CoreCmdCli)
	core.coreApp = &cli.App{
		Name:   "task7-command",
		Action: func(*cli.Context) error { return cli.Exit("command failed", 7) },
	}
	oldArgs := os.Args
	oldExiter := cli.OsExiter
	t.Cleanup(func() {
		os.Args = oldArgs
		cli.OsExiter = oldExiter
	})
	exitCodes := make([]int, 0, 1)
	cli.OsExiter = func(code int) { exitCodes = append(exitCodes, code) }
	os.Args = []string{"task7-command"}

	err := core.AppCoreRun()

	require.Error(t, err)
	assert.Equal(t, []int{7}, exitCodes)
}

func TestFrameCmdApplication_RegistrationLoggerOriginsAndGlobals(t *testing.T) {
	ctx := newCommandTestContext()
	application := &commandTestApplication{name: "application"}
	frame := NewFrameCmdApplication(ctx, func(starter fiberhouse.FrameCmdStarter) {
		starter.RegisterApplication(application)
	}).(*FrameCmdApplication)

	assert.Same(t, ctx, frame.GetContext())
	assert.Same(t, frame, frame.GetFrameCmdApp())
	assert.Same(t, application, frame.GetApplication())
	for key, origin := range ctx.GetConfig().GetLogOriginMap() {
		if key != "" {
			assert.False(t, ctx.GetContainer().IsRegistered(origin.InstanceKey()), key)
		}
	}
	frame.RegisterApplicationGlobals()
	assert.Equal(t, 1, application.applicationGlobals)
	for key, origin := range ctx.GetConfig().GetLogOriginMap() {
		if key != "" {
			assert.True(t, ctx.GetContainer().IsRegistered(origin.InstanceKey()), key)
		}
	}
}

func TestFrameCmdApplication_StartHealthCheckDoesNotLogSuccessWhenRebuildFails(t *testing.T) {
	const resourceKey = "failing-command-health-resource"
	rebuildErr := errors.New("sentinel rebuild failure")
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	ctx := newCommandTestContext().(*commandTestContext)
	ctx.logger = bootstrap.NewLoggerWrap(&logger)
	resource := &failingCommandHealthResource{rebuildErr: rebuildErr}
	require.True(t, ctx.container.Register(resourceKey, func() (interface{}, error) {
		return resource, nil
	}))
	_, err := ctx.container.Get(resourceKey)
	require.NoError(t, err)

	frame := &FrameCmdApplication{Ctx: ctx}
	frame.startHealthCheck()

	output := logs.String()
	assert.Contains(t, output, rebuildErr.Error())
	assert.Contains(t, output, "global resource '"+resourceKey+"' rebuild failed.")
	assert.NotContains(t, output, "global resource '"+resourceKey+"' rebuild success.")
}
