package fiberhouse

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/gofiber/fiber/v2"
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
	result    any
	err       error
	loadCalls int
}

func (m *task4CodecManager) Type() IProviderType { return m.typ }

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

func task4GoodCodecManager() *task4CodecManager {
	return &task4CodecManager{
		typ:    ProviderTypeDefault().GroupTrafficCodecChoose,
		result: jsoncodec.StdJsonDefault(),
	}
}

func task4WrongManager() *task4CodecManager {
	return &task4CodecManager{typ: ProviderTypeDefault().GroupCoreStarterChoose}
}

func preserveTask4GinMode(t *testing.T) {
	t.Helper()
	oldMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(oldMode) })
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

func TestCoreInit_GinTLSLoadsConfiguredCertificate(t *testing.T) {
	preserveTask4GinMode(t)
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
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0o600))

	ctx := newTask4InternalAppContext(t, map[string]interface{}{
		"application.plugins.engine.servers.gin.tls.enable":   true,
		"application.plugins.engine.servers.gin.tls.certFile": certFile,
		"application.plugins.engine.servers.gin.tls.keyFile":  keyFile,
	})
	core := NewCoreWithGin(ctx).(*CoreWithGin)

	require.NotPanics(t, func() {
		core.InitCoreApp(&task4Frame{}, task4GoodCodecManager())
	})
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
