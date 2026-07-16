package fiberhouse_test

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/option"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTask4ExternalAppContext(t *testing.T) fiberhouse.IApplicationContext {
	t.Helper()
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
}

func TestCoreStarterProviders_NoCallbackSelectsRequestedCore(t *testing.T) {
	ctx := newTask4ExternalAppContext(t)

	fiberCore, err := fiberhouse.NewCoreStarterFiberProvider().Initialize(ctx)
	require.NoError(t, err)
	assert.IsType(t, &fiberhouse.CoreWithFiber{}, fiberCore)

	ginCore, err := fiberhouse.NewCoreStarterGinProvider().Initialize(ctx)
	require.NoError(t, err)
	assert.IsType(t, &fiberhouse.CoreWithGin{}, ginCore)
}

func TestCoreStarterProviders_EmptyOptionsConstructRequestedCore(t *testing.T) {
	ctx := newTask4ExternalAppContext(t)
	emptyOptions := func(fiberhouse.IProvider) (any, error) {
		return []fiberhouse.CoreStarterOption{}, nil
	}

	fiberCore, err := fiberhouse.NewCoreStarterFiberProvider().Initialize(ctx, emptyOptions)
	require.NoError(t, err)
	assert.IsType(t, &fiberhouse.CoreWithFiber{}, fiberCore)

	ginCore, err := fiberhouse.NewCoreStarterGinProvider().Initialize(ctx, emptyOptions)
	require.NoError(t, err)
	assert.IsType(t, &fiberhouse.CoreWithGin{}, ginCore)
}

func TestCoreStarterProviders_CallbackErrorsIdentifyProvider(t *testing.T) {
	ctx := newTask4ExternalAppContext(t)
	sentinel := errors.New("options failed")
	failingOptions := func(fiberhouse.IProvider) (any, error) { return nil, sentinel }

	_, err := fiberhouse.NewCoreStarterFiberProvider().Initialize(ctx, failingOptions)
	require.ErrorIs(t, err, sentinel)
	assert.ErrorContains(t, err, "CoreFiberProvider")

	_, err = fiberhouse.NewCoreStarterGinProvider().Initialize(ctx, failingOptions)
	require.ErrorIs(t, err, sentinel)
	assert.ErrorContains(t, err, "CoreGinProvider")
}

func TestCoreStarterProviders_WrongPayloadIsReturnedToCaller(t *testing.T) {
	ctx := newTask4ExternalAppContext(t)
	payload := &struct{ value string }{value: "not options"}
	wrongPayload := func(fiberhouse.IProvider) (any, error) { return payload, nil }

	fiberResult, err := fiberhouse.NewCoreStarterFiberProvider().Initialize(ctx, wrongPayload)
	require.NoError(t, err)
	assert.Same(t, payload, fiberResult)

	ginResult, err := fiberhouse.NewCoreStarterGinProvider().Initialize(ctx, wrongPayload)
	require.NoError(t, err)
	assert.Same(t, payload, ginResult)
}

func TestCoreWithFiber_WithCoreCfgControlsCreatedApp(t *testing.T) {
	ctx := newTask4ExternalAppContext(t)
	configured := &fiber.Config{
		AppName:       "configured-fiber",
		CaseSensitive: true,
		StrictRouting: true,
	}
	core := fiberhouse.NewCoreWithFiber(ctx, option.WithCoreCfg(configured))

	core.InitCoreApp(nil)
	app, ok := core.GetCoreApp().(*fiber.App)
	require.True(t, ok)
	assert.Equal(t, configured.AppName, app.Config().AppName)
	assert.Equal(t, configured.CaseSensitive, app.Config().CaseSensitive)
	assert.Equal(t, configured.StrictRouting, app.Config().StrictRouting)
}
