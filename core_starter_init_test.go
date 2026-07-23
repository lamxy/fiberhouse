package fiberhouse

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/gofiber/fiber/v2"
	adaptorlogging "github.com/lamxy/fiberhouse/adaptor/logging"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type task4CodecManager struct {
	IProviderManager
	typ       IProviderType
	location  IProviderLocation
	result    any
	err       error
	loadCalls int
}

func (m *task4CodecManager) Type() IProviderType { return m.typ }
func (m *task4CodecManager) Location() IProviderLocation {
	return m.location
}

func (m *task4CodecManager) LoadProvider(...ProviderLoadFunc) (any, error) {
	m.loadCalls++
	return m.result, m.err
}

type task4Frame struct {
	FrameStarter
	application ApplicationRegister
	module      ModuleRegister
}

func (f *task4Frame) GetApplication() IApplication { return f.application }
func (f *task4Frame) GetModule() ModuleRegister    { return f.module }

type task4Application struct {
	ApplicationRegister
	hookCalls  int
	defaultKey string
}

func (a *task4Application) RegisterCoreHook(CoreStarter) { a.hookCalls++ }
func (a *task4Application) GetDefaultTrafficCodecKey() string {
	return a.defaultKey
}

type task4Module struct {
	ModuleRegister
	routeCalls   int
	swaggerCalls int
}

func (m *task4Module) RegisterModuleRouteHandlers(CoreStarter) { m.routeCalls++ }
func (m *task4Module) RegisterSwagger(CoreStarter)             { m.swaggerCalls++ }

type task4LifecycleManager struct {
	IProviderManager
	typ       IProviderType
	location  IProviderLocation
	err       error
	loadCalls int
}

func (m *task4LifecycleManager) Type() IProviderType         { return m.typ }
func (m *task4LifecycleManager) Location() IProviderLocation { return m.location }
func (m *task4LifecycleManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.loadCalls++
	if m.err != nil {
		return nil, m.err
	}
	if len(loadFunc) > 0 {
		return loadFunc[0](m)
	}
	return nil, nil
}

func newTask4InternalAppContext(t *testing.T, values map[string]interface{}) IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	if values != nil {
		cfg.LoadDefault(values)
	}
	cfg.Initialize()
	logger := zerolog.Nop()
	return NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

func newTask4LoggingAppContext(
	t *testing.T,
	values map[string]interface{},
) (IApplicationContext, *bytes.Buffer) {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	if values != nil {
		cfg.LoadDefault(values)
	}
	cfg.Initialize()
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.DebugLevel)
	return NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger)), &output
}

func decodeTask4LogRecords(t *testing.T, output *bytes.Buffer) []map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		records = append(records, record)
	}
	return records
}

func task4LogRecordsWithMessage(
	t *testing.T,
	output *bytes.Buffer,
	message string,
) []map[string]any {
	t.Helper()
	var matched []map[string]any
	for _, record := range decodeTask4LogRecords(t, output) {
		if record["message"] == message {
			matched = append(matched, record)
		}
	}
	return matched
}

func task4GoodCodecManager() *task4CodecManager {
	return &task4CodecManager{
		typ:      ProviderTypeDefault().GroupTrafficCodecChoose,
		location: ProviderLocationDefault().LocationCoreCodecInit,
		result:   jsoncodec.StdJsonDefault(),
	}
}

func task4WrongManager() *task4CodecManager {
	return &task4CodecManager{
		typ:      ProviderTypeDefault().GroupCoreStarterChoose,
		location: ProviderLocationDefault().ZeroLocation,
	}
}

func preserveTask4GinMode(t *testing.T) {
	t.Helper()
	oldMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(oldMode) })
}

func cleanupTask4GinCore(t *testing.T, core *CoreWithGin) {
	t.Helper()
	t.Cleanup(func() {
		replacement := &task4LifecycleManager{
			typ:      ProviderTypeDefault().GroupExtendReplace,
			location: ProviderLocationDefault().LocationServerRun,
		}
		_ = core.AppCoreRun(replacement)
	})
}

func installTask4GinLoggerProbe(t *testing.T) (*adaptorlogging.GinLoggerLease, error) {
	t.Helper()
	logger := zerolog.Nop()
	adapter := adaptorlogging.NewGinLoggerAdapter(
		bootstrap.NewLoggerWrap(&logger),
		appconfig.LogOrigin("Frame"),
	)
	return adaptorlogging.InstallGinLogger(adapter)
}

func requireTask4GinLoggerActive(t *testing.T) {
	t.Helper()
	lease, err := installTask4GinLoggerProbe(t)
	if lease != nil {
		lease.Release()
	}
	require.ErrorIs(t, err, adaptorlogging.ErrGinLoggerAlreadyInstalled)
}

func requireTask4GinLoggerReleased(t *testing.T) {
	t.Helper()
	lease, err := installTask4GinLoggerProbe(t)
	require.NoError(t, err)
	require.NotNil(t, lease)
	lease.Release()
}

func isolateTask4ErrorHandlerSingleton(t *testing.T) {
	t.Helper()
	oldInstance := errorHandlerInstance
	errorHandlerInstance = nil
	errorHandlerOnce = sync.Once{}
	t.Cleanup(func() {
		errorHandlerInstance = oldInstance
		errorHandlerOnce = sync.Once{}
		if oldInstance != nil {
			errorHandlerOnce.Do(func() {})
		}
	})
}

func TestCoreInit_CreatesFiberAndGinAppsUsingSelectedManager(t *testing.T) {
	isolateTask4ErrorHandlerSingleton(t)
	preserveTask4GinMode(t)
	ctx := newTask4InternalAppContext(t, nil)
	frame := &task4Frame{}

	fiberWrong, fiberManager := task4WrongManager(), task4GoodCodecManager()
	fiberCore := NewCoreWithFiber(ctx).(*CoreWithFiber)
	fiberCore.InitCoreApp(frame, fiberWrong, fiberManager)
	assert.IsType(t, &fiber.App{}, fiberCore.GetCoreApp())
	assert.Same(t, fiberManager.result, fiberCore.json)
	assert.Same(t, ctx, errorHandlerInstance.AppCtx)
	assert.Zero(t, fiberWrong.loadCalls)
	assert.Equal(t, 1, fiberManager.loadCalls)

	ginWrong, ginManager := task4WrongManager(), task4GoodCodecManager()
	ginCore := NewCoreWithGin(ctx).(*CoreWithGin)
	ginCore.InitCoreApp(frame, ginWrong, ginManager)
	cleanupTask4GinCore(t, ginCore)
	assert.NotNil(t, ginCore.coreApp)
	assert.Same(t, ginCore.coreApp, ginCore.GetCoreApp())
	assert.Zero(t, ginWrong.loadCalls)
	assert.Equal(t, 1, ginManager.loadCalls)
	assert.NotNil(t, ginCore.httpServer)
}

func TestCoreInit_NoManagerUsesApplicationDefaultCodec(t *testing.T) {
	isolateTask4ErrorHandlerSingleton(t)
	preserveTask4GinMode(t)
	oldGinJSON := ginJson.API
	t.Cleanup(func() { ginJson.API = oldGinJSON })
	ctx := newTask4InternalAppContext(t, nil)
	key := "task4-default-codec"
	defaultCodec := jsoncodec.StdJsonDefault()
	factoryCalls := 0
	manager := ctx.GetContainer()
	manager.Unregister(key)
	t.Cleanup(func() { manager.Unregister(key) })
	require.True(t, manager.Register(key, func() (interface{}, error) {
		factoryCalls++
		return defaultCodec, nil
	}))
	frame := &task4Frame{application: &task4Application{defaultKey: key}}

	fiberCore := NewCoreWithFiber(ctx).(*CoreWithFiber)
	fiberCore.InitCoreApp(frame)
	assert.IsType(t, jsoncodec.StdJsonDefault(), fiberCore.json)
	assert.NotNil(t, fiberCore.coreApp)
	assert.Same(t, ctx, errorHandlerInstance.AppCtx)

	ginCore := NewCoreWithGin(ctx).(*CoreWithGin)
	ginCore.InitCoreApp(frame)
	cleanupTask4GinCore(t, ginCore)
	assert.NotNil(t, ginCore.coreApp)
	assert.NotNil(t, ginCore.httpServer)
	assert.Equal(t, 1, factoryCalls)
	assert.Same(t, defaultCodec, ginJson.API)
}

func TestCoreInit_AppStateStopsInitializationAndRegistration(t *testing.T) {
	ctx := newTask4InternalAppContext(t, map[string]interface{}{"application.swagger.enable": true})
	ctx.RegisterAppState(true)
	manager := task4GoodCodecManager()
	module := &task4Module{}
	application := &task4Application{}
	frame := &task4Frame{application: application, module: module}

	existingFiberApp := fiber.New()
	fiberCore := &CoreWithFiber{ctx: ctx, coreApp: existingFiberApp}
	fiberCore.InitCoreApp(frame, manager)
	fiberCore.RegisterModuleInitialize(frame)
	fiberCore.RegisterModuleSwagger(frame)
	fiberCore.RegisterAppHooks(frame)
	assert.Same(t, existingFiberApp, fiberCore.coreApp)

	ginCore := &CoreWithGin{ctx: ctx}
	ginCore.InitCoreApp(frame, manager)
	ginCore.RegisterModuleInitialize(frame)
	ginCore.RegisterModuleSwagger(frame)
	ginCore.RegisterAppHooks(frame)
	assert.Nil(t, ginCore.coreApp)
	assert.Nil(t, ginCore.httpServer)

	assert.Zero(t, manager.loadCalls)
	assert.Zero(t, module.routeCalls)
	assert.Zero(t, module.swaggerCalls)
	assert.Zero(t, application.hookCalls)
}

func TestCoreRegisterModuleInitialize_LoadsRouteManagerOnlyOnce(t *testing.T) {
	for _, testCase := range []struct {
		name string
		core func(IApplicationContext) CoreStarter
	}{
		{name: "fiber", core: func(ctx IApplicationContext) CoreStarter { return &CoreWithFiber{ctx: ctx} }},
		{name: "gin", core: func(ctx IApplicationContext) CoreStarter { return &CoreWithGin{ctx: ctx} }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := newTask4InternalAppContext(t, nil)
			module := &task4Module{}
			frame := &task4Frame{module: module}
			routeManager := &task4LifecycleManager{
				typ:      ProviderTypeDefault().GroupRouteRegisterType,
				location: ProviderLocationDefault().LocationRouteRegisterInit,
			}

			testCase.core(ctx).RegisterModuleInitialize(frame, routeManager)

			assert.Equal(t, 1, routeManager.loadCalls)
			assert.Zero(t, module.routeCalls)
		})
	}
}

func TestCoreRegisterModuleInitialize_ReplacementIsScopedToItsLocation(t *testing.T) {
	for _, testCase := range []struct {
		name string
		core func(IApplicationContext) CoreStarter
	}{
		{name: "fiber", core: func(ctx IApplicationContext) CoreStarter { return &CoreWithFiber{ctx: ctx} }},
		{name: "gin", core: func(ctx IApplicationContext) CoreStarter { return &CoreWithGin{ctx: ctx} }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := newTask4InternalAppContext(t, nil)
			frame := &task4Frame{module: &task4Module{}}
			moduleManager := &task4LifecycleManager{
				typ:      ProviderTypeDefault().GroupMiddlewareRegisterType,
				location: ProviderLocationDefault().LocationModuleMiddlewareInit,
			}
			routeReplacement := &task4LifecycleManager{
				typ:      ProviderTypeDefault().GroupExtendReplace,
				location: ProviderLocationDefault().LocationRouteRegisterInit,
			}

			testCase.core(ctx).RegisterModuleInitialize(frame, moduleManager, routeReplacement)

			assert.Equal(t, 1, moduleManager.loadCalls)
			assert.Equal(t, 1, routeReplacement.loadCalls)
		})
	}
}

func TestCoreAppCoreRun_SuccessfulReplacementReturnsNil(t *testing.T) {
	for _, testCase := range []struct {
		name string
		core func(IApplicationContext) CoreStarter
	}{
		{name: "fiber", core: func(ctx IApplicationContext) CoreStarter { return &CoreWithFiber{ctx: ctx} }},
		{name: "gin", core: func(ctx IApplicationContext) CoreStarter { return &CoreWithGin{ctx: ctx} }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := newTask4InternalAppContext(t, nil)
			replacement := &task4LifecycleManager{
				typ:      ProviderTypeDefault().GroupExtendReplace,
				location: ProviderLocationDefault().LocationServerRun,
			}

			require.NoError(t, testCase.core(ctx).AppCoreRun(replacement))
			assert.Equal(t, 1, replacement.loadCalls)
		})
	}
}

func TestCoreShutdown_SuccessfulLifecycleManagersDoNotBecomeErrors(t *testing.T) {
	for _, testCase := range []struct {
		name string
		core func(IApplicationContext) CoreStarter
	}{
		{
			name: "fiber",
			core: func(ctx IApplicationContext) CoreStarter {
				return &CoreWithFiber{ctx: ctx, coreApp: fiber.New()}
			},
		},
		{
			name: "gin",
			core: func(ctx IApplicationContext) CoreStarter {
				return &CoreWithGin{ctx: ctx, httpServer: &http.Server{}}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := newTask4InternalAppContext(t, nil)
			before := &task4LifecycleManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationServerShutdownBefore,
			}
			after := &task4LifecycleManager{
				typ:      ProviderTypeDefault().GroupProviderAutoRun,
				location: ProviderLocationDefault().LocationServerShutdownAfter,
			}

			require.NoError(t, testCase.core(ctx).Shutdown(before, after))
			assert.Equal(t, 1, before.loadCalls)
			assert.Equal(t, 1, after.loadCalls)
		})
	}
}

func TestCoreInit_FiberRejectsMissingWrongAndFailedCodecManagers(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	frame := &task4Frame{}

	assert.PanicsWithValue(t,
		"No JSON codec manager found in provided managers, using default JSON codec.",
		func() { NewCoreWithFiber(ctx).InitCoreApp(frame, task4WrongManager()) },
	)

	wrongCodec := task4GoodCodecManager()
	wrongCodec.result = struct{}{}
	assert.PanicsWithValue(t,
		"Loaded JSON codec provider does not implement JsonWrapper interface",
		func() { NewCoreWithFiber(ctx).InitCoreApp(frame, wrongCodec) },
	)

	sentinel := errors.New("codec load failed")
	failed := task4GoodCodecManager()
	failed.err = sentinel
	assert.PanicsWithError(t, sentinel.Error(), func() {
		NewCoreWithFiber(ctx).InitCoreApp(frame, failed)
	})
}

func TestCoreInit_GinRejectsMissingAndFailedCodecManagers(t *testing.T) {
	preserveTask4GinMode(t)
	ctx := newTask4InternalAppContext(t, nil)
	frame := &task4Frame{}

	assert.PanicsWithValue(t,
		"No JSON codec manager provided, using default JSON codec",
		func() { NewCoreWithGin(ctx).InitCoreApp(frame, task4WrongManager()) },
	)

	sentinel := errors.New("codec load failed")
	failed := task4GoodCodecManager()
	failed.err = sentinel
	assert.PanicsWithError(t, sentinel.Error(), func() {
		NewCoreWithGin(ctx).InitCoreApp(frame, failed)
	})
}

func TestCoreInit_GinServerUsesConfiguredAddressAndTimeouts(t *testing.T) {
	preserveTask4GinMode(t)
	ctx := newTask4InternalAppContext(t, map[string]interface{}{
		"application.plugins.engine.servers.gin.host":              "127.0.0.1",
		"application.plugins.engine.servers.gin.port":              "9099",
		"application.plugins.engine.servers.gin.readTimeout":       2,
		"application.plugins.engine.servers.gin.writeTimeout":      3,
		"application.plugins.engine.servers.gin.idleTimeout":       4,
		"application.plugins.engine.servers.gin.readHeaderTimeout": 5,
		"application.plugins.engine.servers.gin.maxHeaderBytes":    8,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)

	require.NotNil(t, core.httpServer)
	assert.Equal(t, "127.0.0.1:9099", core.httpServer.Addr)
	assert.Same(t, core.coreApp, core.httpServer.Handler)
	assert.Equal(t, 2*time.Second, core.httpServer.ReadTimeout)
	assert.Equal(t, 3*time.Second, core.httpServer.WriteTimeout)
	assert.Equal(t, 4*time.Second, core.httpServer.IdleTimeout)
	assert.Equal(t, 5*time.Second, core.httpServer.ReadHeaderTimeout)
	assert.Equal(t, 8*1024, core.httpServer.MaxHeaderBytes)
	assert.NotNil(t, core.httpServer.BaseContext)
	assert.IsType(t, http.Handler(core.coreApp), core.httpServer.Handler)
}

func TestCoreInit_GinModeResolution(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		values map[string]interface{}
		want   string
	}{
		{
			name: "canonical key wins",
			values: map[string]interface{}{
				"application.plugins.engine.servers.gin.mode": gin.TestMode,
				"application.plugins.server.gin.mode":         gin.DebugMode,
			},
			want: gin.TestMode,
		},
		{
			name: "legacy key remains supported",
			values: map[string]interface{}{
				"application.plugins.server.gin.mode": gin.DebugMode,
			},
			want: gin.DebugMode,
		},
		{
			name: "missing keys default to release",
			want: gin.ReleaseMode,
		},
		{
			name: "recovery debug does not override Gin mode",
			values: map[string]interface{}{
				"application.plugins.engine.servers.gin.mode": gin.ReleaseMode,
				"application.recover.debugMode":               true,
			},
			want: gin.ReleaseMode,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			preserveTask4GinMode(t)
			ctx := newTask4InternalAppContext(t, testCase.values)
			core := NewCoreWithGin(ctx).(*CoreWithGin)

			core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
			cleanupTask4GinCore(t, core)

			require.Equal(t, testCase.want, gin.Mode())
		})
	}
}

func TestCoreInit_GinLoggerCapturesStartupAndRouteDiagnostics(t *testing.T) {
	preserveTask4GinMode(t)
	ctx, output := newTask4LoggingAppContext(t, map[string]interface{}{
		"application.plugins.engine.servers.gin.mode": gin.DebugMode,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)

	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)

	var startupRecords []map[string]any
	for _, record := range decodeTask4LogRecords(t, output) {
		if record["level"] == "debug" &&
			record["Component"] == "Gin" &&
			record["Channel"] == "debug" {
			startupRecords = append(startupRecords, record)
		}
	}
	require.NotEmpty(t, startupRecords)

	core.coreApp.GET("/gin-logger", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	routeRecords := task4LogRecordsWithMessage(t, output, "Gin route registered")
	require.Len(t, routeRecords, 1)
	require.Equal(t, "Gin", routeRecords[0]["Component"])
	require.Equal(t, http.MethodGet, routeRecords[0]["method"])
	require.Equal(t, "/gin-logger", routeRecords[0]["path"])
}

func TestCoreInit_GinHTTPErrorLoggerDefaultsAndPreservesCustom(t *testing.T) {
	t.Run("default server logger", func(t *testing.T) {
		preserveTask4GinMode(t)
		ctx, output := newTask4LoggingAppContext(t, nil)
		core := NewCoreWithGin(ctx).(*CoreWithGin)

		core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
		cleanupTask4GinCore(t, core)

		require.NotNil(t, core.httpServer.ErrorLog)
		core.httpServer.ErrorLog.Print("accept failed")
		records := task4LogRecordsWithMessage(t, output, "accept failed")
		require.Len(t, records, 1)
		require.Equal(t, "error", records[0]["level"])
		require.Equal(t, "Gin", records[0]["Component"])
		require.Equal(t, "server", records[0]["Channel"])
	})

	t.Run("custom server logger", func(t *testing.T) {
		preserveTask4GinMode(t)
		ctx := newTask4InternalAppContext(t, nil)
		customOutput := &bytes.Buffer{}
		customLogger := log.New(customOutput, "custom: ", 0)
		core := &CoreWithGin{
			ctx:        ctx,
			httpServer: &http.Server{ErrorLog: customLogger},
		}

		core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
		cleanupTask4GinCore(t, core)

		require.Same(t, customLogger, core.httpServer.ErrorLog)
	})
}

func TestCoreInit_GinLoggerConflictDefersErrorAndGuardsRegistration(t *testing.T) {
	preserveTask4GinMode(t)
	isolateTask4ErrorHandlerSingleton(t)
	externalLease, err := installTask4GinLoggerProbe(t)
	require.NoError(t, err)
	t.Cleanup(externalLease.Release)

	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	frame := &task4Frame{}
	core.InitCoreApp(frame, task4GoodCodecManager())

	assert.Nil(t, core.GetCoreApp())
	if core.httpServer != nil {
		core.httpServer.Addr = "bad address"
	}
	assert.ErrorIs(
		t,
		core.AppCoreRun(),
		adaptorlogging.ErrGinLoggerAlreadyInstalled,
	)
	assert.NotPanics(t, func() {
		core.RegisterAppMiddleware(frame)
		core.RegisterModuleInitialize(frame)
		core.RegisterModuleSwagger(frame)
		core.RegisterAppHooks(frame)
	})

	var shutdownErr error
	require.NotPanics(t, func() {
		shutdownErr = core.Shutdown()
	})
	require.ErrorIs(
		t,
		shutdownErr,
		adaptorlogging.ErrGinLoggerAlreadyInstalled,
	)
	requireTask4GinLoggerActive(t)
}

func TestCoreInit_GinLoggerAccessRecordHasComponentWithoutDuplication(t *testing.T) {
	preserveTask4GinMode(t)
	ctx, output := newTask4LoggingAppContext(t, map[string]interface{}{
		"application.middleware.coreHttp": true,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)

	core.coreApp.Use(core.loggerMiddleware())
	core.coreApp.GET("/access", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	core.coreApp.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/access", nil),
	)

	records := task4LogRecordsWithMessage(t, output, "HTTP Request")
	require.Len(t, records, 1)
	require.Equal(t, "Gin", records[0]["Component"])
	require.Equal(t, http.MethodGet, records[0]["method"])
	require.Equal(t, "/access", records[0]["path"])
}

func TestCoreRun_GinLoggerReleasedOnListenerFailure(t *testing.T) {
	preserveTask4GinMode(t)
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)
	requireTask4GinLoggerActive(t)
	core.httpServer.Addr = "bad address"

	require.Error(t, core.AppCoreRun())

	requireTask4GinLoggerReleased(t)
}

func TestCoreRun_GinLoggerReleasedOnNormalReturn(t *testing.T) {
	preserveTask4GinMode(t)
	ctx := newTask4InternalAppContext(t, nil)
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)
	requireTask4GinLoggerActive(t)
	require.NoError(t, core.httpServer.Close())

	require.NoError(t, core.AppCoreRun())

	requireTask4GinLoggerReleased(t)
}

func TestCoreShutdown_GinLoggerReleasedOnAllPaths(t *testing.T) {
	shutdownProviderErr := errors.New("shutdown provider failed")
	for _, testCase := range []struct {
		name     string
		managers func() []IProviderManager
		wantErr  error
	}{
		{
			name: "successful graceful shutdown",
		},
		{
			name: "shutdown provider error",
			managers: func() []IProviderManager {
				return []IProviderManager{&task4LifecycleManager{
					typ:      ProviderTypeDefault().GroupProviderAutoRun,
					location: ProviderLocationDefault().LocationServerShutdown,
					err:      shutdownProviderErr,
				}}
			},
			wantErr: shutdownProviderErr,
		},
		{
			name: "replacement shutdown provider",
			managers: func() []IProviderManager {
				return []IProviderManager{&task4LifecycleManager{
					typ:      ProviderTypeDefault().GroupExtendReplace,
					location: ProviderLocationDefault().LocationServerShutdown,
				}}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			preserveTask4GinMode(t)
			ctx := newTask4InternalAppContext(t, nil)
			core := NewCoreWithGin(ctx).(*CoreWithGin)
			core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
			cleanupTask4GinCore(t, core)
			requireTask4GinLoggerActive(t)
			var managers []IProviderManager
			if testCase.managers != nil {
				managers = testCase.managers()
			}

			err := core.Shutdown(managers...)
			if testCase.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, testCase.wantErr)
			}

			requireTask4GinLoggerReleased(t)
		})
	}
}

// generateTask4SelfSignedCert 生成一份临时的 ECDSA 自签名证书/私钥文件，
// 供 TLS 相关测试复用。提取自 TestCoreInit_GinTLSLoadsConfiguredCertificate
// 的原始实现，逻辑与产出结果保持不变。
func generateTask4SelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	tempDir := t.TempDir()
	certFile = filepath.Join(tempDir, "cert.pem")
	keyFile = filepath.Join(tempDir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0o600))
	return certFile, keyFile
}

func TestCoreInit_GinTLSLoadsConfiguredCertificate(t *testing.T) {
	preserveTask4GinMode(t)
	certFile, keyFile := generateTask4SelfSignedCert(t)

	ctx := newTask4InternalAppContext(t, map[string]interface{}{
		"application.plugins.engine.servers.gin.tls.enable":   true,
		"application.plugins.engine.servers.gin.tls.certFile": certFile,
		"application.plugins.engine.servers.gin.tls.keyFile":  keyFile,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)

	require.NotPanics(t, func() {
		core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	})
	cleanupTask4GinCore(t, core)
	require.NotNil(t, core.httpServer.TLSConfig)
	require.Len(t, core.httpServer.TLSConfig.Certificates, 1)
}

func TestCoreInit_GinTLSRejectsInvalidConfiguredCertificate(t *testing.T) {
	preserveTask4GinMode(t)
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, []byte("invalid certificate"), 0o600))
	require.NoError(t, os.WriteFile(keyFile, []byte("invalid private key"), 0o600))

	ctx := newTask4InternalAppContext(t, map[string]interface{}{
		"application.plugins.engine.servers.gin.tls.enable":   true,
		"application.plugins.engine.servers.gin.tls.certFile": certFile,
		"application.plugins.engine.servers.gin.tls.keyFile":  keyFile,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)

	require.Panics(t, func() {
		core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	})
}

// TestCoreInit_GinLoopbackTLSHandshake 验证 Gin server 在启用 TLS 后，
// 使用已装配的 TLSConfig 确实可以完成一次真实的 loopback TLS 握手与
// HTTP 请求-响应链路，而不仅仅是静态检查 TLSConfig 字段。
func TestCoreInit_GinLoopbackTLSHandshake(t *testing.T) {
	preserveTask4GinMode(t)
	certFile, keyFile := generateTask4SelfSignedCert(t)

	ctx := newTask4InternalAppContext(t, map[string]interface{}{
		"application.plugins.engine.servers.gin.tls.enable":   true,
		"application.plugins.engine.servers.gin.tls.certFile": certFile,
		"application.plugins.engine.servers.gin.tls.keyFile":  keyFile,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)
	core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	cleanupTask4GinCore(t, core)
	require.NotNil(t, core.httpServer.TLSConfig)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "loopback TCP listener must be available in this environment")

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- core.httpServer.ServeTLS(listener, "", "")
	}()

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	url := fmt.Sprintf("https://%s/", listener.Addr().String())

	resp, err := client.Get(url)
	require.NoError(t, err, "TLS handshake and HTTP request must succeed")
	require.NotNil(t, resp)
	_ = resp.Body.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, core.httpServer.Shutdown(shutdownCtx))

	select {
	case serveErr := <-serveErrCh:
		require.ErrorIs(t, serveErr, http.ErrServerClosed)
	case <-time.After(3 * time.Second):
		t.Fatal("ServeTLS goroutine did not return after Shutdown")
	}
}

func TestCoreInit_CustomCoreCfgStillResolvesJSONCodec(t *testing.T) {
	isolateTask4ErrorHandlerSingleton(t)
	ctx := newTask4InternalAppContext(t, nil)
	frame := &task4Frame{}
	manager := task4GoodCodecManager()
	cfg := &fiber.Config{AppName: "custom-cfg"}
	core := &CoreWithFiber{ctx: ctx, CoreCfg: cfg}

	core.InitCoreApp(frame, manager)

	require.NotNil(t, core.coreApp)
	assert.Equal(t, "custom-cfg", core.coreApp.Config().AppName)
	// 使用自定义 CoreCfg 并继续标准启动链时，cf.json 仍必须被赋值，
	// 否则 RegisterAppMiddleware 中 cf.json.Marshal 会因 nil 接口 panic。
	assert.Same(t, manager.result, core.json)
	assert.Equal(t, 1, manager.loadCalls)

	// RegisterAppMiddleware依赖cf.json.Marshal；直接验证该调用不会因cf.json为nil接口而panic
	// (recover manager 的装配是独立于本修复范围的关注点，此处不涉及)
	require.NotPanics(t, func() {
		_, err := core.json.Marshal(map[string]string{"ok": "true"})
		require.NoError(t, err)
	})
}

func TestCoreInit_CustomCoreCfgWithNilFrameSkipsCodecResolution(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	cfg := &fiber.Config{AppName: "custom-cfg-no-frame"}
	core := &CoreWithFiber{ctx: ctx, CoreCfg: cfg}

	require.NotPanics(t, func() {
		core.InitCoreApp(nil)
	})
	require.NotNil(t, core.coreApp)
	assert.Equal(t, "custom-cfg-no-frame", core.coreApp.Config().AppName)
	assert.Nil(t, core.json)
}

func TestCoreRegistration_ModuleSwaggerAndApplicationHooks(t *testing.T) {
	for _, testCase := range []struct {
		name string
		core func(IApplicationContext) CoreStarter
	}{
		{name: "fiber", core: func(ctx IApplicationContext) CoreStarter {
			return &CoreWithFiber{ctx: ctx, coreApp: fiber.New()}
		}},
		{name: "gin", core: func(ctx IApplicationContext) CoreStarter {
			return &CoreWithGin{ctx: ctx}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := newTask4InternalAppContext(t, map[string]interface{}{"application.swagger.enable": true})
			module := &task4Module{}
			application := &task4Application{}
			frame := &task4Frame{application: application, module: module}
			core := testCase.core(ctx)

			core.RegisterModuleInitialize(frame)
			core.RegisterModuleSwagger(frame)
			core.RegisterAppHooks(frame)
			assert.Equal(t, 1, module.routeCalls)
			assert.Equal(t, 1, module.swaggerCalls)
			assert.Equal(t, 1, application.hookCalls)
		})
	}

	disabledCtx := newTask4InternalAppContext(t, map[string]interface{}{"application.swagger.enable": false})
	disabledGinModule := &task4Module{}
	NewCoreWithGin(disabledCtx).RegisterModuleSwagger(&task4Frame{module: disabledGinModule})
	assert.Zero(t, disabledGinModule.swaggerCalls)

	disabledFiberModule := &task4Module{}
	NewCoreWithFiber(disabledCtx).RegisterModuleSwagger(&task4Frame{module: disabledFiberModule})
	assert.Zero(t, disabledFiberModule.swaggerCalls)
}
