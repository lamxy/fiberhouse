package fiberhouse

import (
	"errors"
	"testing"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type task2Manager struct {
	*ProviderManager
	loadCalls int
	result    any
	err       error
}

func newTask2Manager(ctx IContext) *task2Manager {
	base := NewProviderManager(ctx)
	base.SetName("child").SetType(&PType{id: CustomTypeStart, name: "custom"})
	manager := &task2Manager{ProviderManager: base}
	base.MountToParent(manager)
	return manager
}

func (m *task2Manager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.loadCalls++
	if len(loadFunc) > 0 {
		return loadFunc[0](m)
	}
	return m.result, m.err
}

func newTask2AppContext(t *testing.T, coreType string) IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	logger := zerolog.Nop()
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&BootConfig{CoreType: coreType})
	return ctx
}

func TestProviderManagerUnregister_RemovesProvider(t *testing.T) {
	manager := NewProviderManager(nil)
	provider := NewProvider().SetName("provider")
	require.NoError(t, manager.Register(provider))

	require.NoError(t, manager.Unregister("provider"))
	_, err := manager.GetProvider("provider")
	assert.ErrorContains(t, err, ErrProviderNotFound.Error())
	assert.ErrorIs(t, manager.Unregister("provider"), ErrProviderNotFound)
}

func TestProviderManager_RegisterLookupAndDuplicate(t *testing.T) {
	manager := NewProviderManager(nil)
	provider := NewProvider().SetName("provider")
	require.NoError(t, manager.Register(provider))
	assert.ErrorIs(t, manager.Register(provider), ErrProviderAlreadyExists)

	got, err := manager.GetProvider("provider")
	require.NoError(t, err)
	assert.Same(t, provider, got)
	assert.Len(t, manager.List(), 1)
	assert.Same(t, provider, manager.Map()["provider"])

	_, err = manager.GetProvider("missing")
	assert.ErrorContains(t, err, ErrProviderNotFound.Error())
}

func TestProviderManager_FluentValuesAndOneTimeType(t *testing.T) {
	ctx := newTask2AppContext(t, "fiber")
	firstType := &PType{id: CustomTypeStart, name: "first"}
	secondType := &PType{id: CustomTypeStart + 1, name: "second"}
	location := &PLocation{id: CustomLocationStart, name: "custom"}
	manager := NewProviderManager(ctx)

	returned := manager.SetName("manager").SetType(firstType).SetType(secondType).SetOrBindToLocation(location, true)
	assert.Same(t, manager, returned)
	assert.Equal(t, "manager", manager.Name())
	assert.Same(t, firstType, manager.Type())
	assert.Same(t, location, manager.Location())
	assert.Same(t, ctx, manager.GetContext())
	assert.Equal(t, []IProviderManager{manager}, location.GetManagers())
	assert.NotPanics(t, manager.Check)
}

func TestProviderManager_LoadProviderValidatesAndDelegates(t *testing.T) {
	manager := NewProviderManager(nil).SetName("base").(*ProviderManager)
	assert.Panics(t, func() { _, _ = manager.LoadProvider() })
	manager.SetType(&PType{id: CustomTypeStart, name: "custom"})
	_, err := manager.LoadProvider()
	assert.ErrorContains(t, err, "sonManager")
	assert.Panics(t, func() { manager.MountToParent() })
	assert.Panics(t, func() { manager.MountToParent(manager) })
	manager.sonManager = manager
	_, err = manager.LoadProvider()
	assert.ErrorContains(t, err, "cannot be the same")

	child := newTask2Manager(nil)
	location := &PLocation{id: CustomLocationStart, name: "child-location"}
	child.SetOrBindToLocation(location, true)
	assert.Equal(t, []IProviderManager{child}, location.GetManagers())
	result, err := child.ProviderManager.LoadProvider(func(got IProviderManager) (any, error) {
		assert.Same(t, child, got)
		return "loaded", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "loaded", result)
	assert.Equal(t, 1, child.loadCalls)
}

func TestProviderManager_UniqueBindingContracts(t *testing.T) {
	first := NewProvider().SetName("first")
	manager := NewProviderManager(nil).SetName("manager")
	assert.Same(t, manager, manager.BindToUniqueProvider(first))
	assert.True(t, manager.IsUnique())
	assert.Same(t, manager, manager.BindToUniqueProvider(first))
	assert.Error(t, manager.Register(NewProvider().SetName("second")))

	different := NewProviderManager(nil).SetName("different")
	require.NoError(t, different.Register(first))
	assert.Panics(t, func() { different.BindToUniqueProvider(NewProvider().SetName("other")) })

	multiple := NewProviderManager(nil).SetName("multiple")
	require.NoError(t, multiple.Register(first))
	require.NoError(t, multiple.Register(NewProvider().SetName("second")))
	assert.Panics(t, func() { multiple.BindToUniqueProvider(first) })
}

func TestDefaultPManagerLoadProvider_SuccessDoesNotAggregateNil(t *testing.T) {
	ctx := newTask2AppContext(t, "fiber")
	manager := NewDefaultPManager(ctx)
	matched := newTask2Provider("matched", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	require.NoError(t, manager.Register(matched))

	result, err := manager.LoadProvider()
	assert.Nil(t, result)
	require.NoError(t, err)
	assert.Equal(t, 1, matched.initializeCalls)
}

func TestDefaultPManagerLoadProvider_AggregatesOnlyRealErrors(t *testing.T) {
	ctx := newTask2AppContext(t, "fiber")
	manager := NewDefaultPManager(ctx)
	success := newTask2Provider("success", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	firstFailure := newTask2Provider("first-failure", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	firstFailure.err = errors.New("first failure")
	secondFailure := newTask2Provider("second-failure", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	secondFailure.err = errors.New("second failure")
	for _, provider := range []IProvider{success, firstFailure, secondFailure} {
		require.NoError(t, manager.Register(provider))
	}

	_, err := manager.LoadProvider()
	require.Error(t, err)
	assert.ErrorContains(t, err, "first failure")
	assert.ErrorContains(t, err, "second failure")
	assert.NotContains(t, err.Error(), "<nil>")
}

func TestDefaultPManagerLoadProvider_TargetFilteringAutoRunAndInjection(t *testing.T) {
	ctx := newTask2AppContext(t, "gin")
	manager := NewDefaultPManager(ctx)
	matched := newTask2Provider("matched", "gin", &PType{id: CustomTypeStart, name: "custom"})
	unmatched := newTask2Provider("unmatched", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	autoRun := newTask2Provider("auto", "different", ProviderTypeDefault().GroupProviderAutoRun)
	for _, provider := range []IProvider{matched, unmatched, autoRun} {
		require.NoError(t, manager.Register(provider))
	}

	token := &struct{ name string }{name: "dependency"}
	result, err := manager.LoadProvider(func(got IProviderManager) (any, error) {
		assert.Same(t, manager, got)
		return token, nil
	})
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, matched.initializeCalls)
	assert.Same(t, token, matched.injected)
	assert.Zero(t, unmatched.initializeCalls)
	assert.Equal(t, 1, autoRun.initializeCalls)
	assert.Same(t, token, autoRun.injected)
}

func TestDefaultPManagerLoadProvider_CallbackAndMissingProviderErrors(t *testing.T) {
	ctx := newTask2AppContext(t, "fiber")
	manager := NewDefaultPManager(ctx)

	_, err := manager.LoadProvider()
	assert.ErrorIs(t, err, ErrProviderNotFound)

	provider := newTask2Provider("provider", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	require.NoError(t, manager.Register(provider))
	callbackErr := errors.New("callback failed")
	_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return nil, callbackErr })
	assert.ErrorIs(t, err, callbackErr)
	assert.Zero(t, provider.initializeCalls)
}
