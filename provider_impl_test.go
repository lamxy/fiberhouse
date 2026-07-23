package fiberhouse

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type task2Provider struct {
	*Provider
	initializeCalls int
	injected        any
	result          any
	err             error
}

func newTask2Provider(name, target string, typ IProviderType) *task2Provider {
	base := NewProvider()
	base.SetName(name).SetTarget(target).SetType(typ)
	provider := &task2Provider{Provider: base}
	base.MountToParent(provider)
	return provider
}

func (p *task2Provider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	if !p.Check() {
		return p.ReturnDirectly()
	}
	p.initializeCalls++
	if len(initFunc) > 0 {
		injected, err := initFunc[0](p)
		p.injected = injected
		if err != nil {
			return p.SetAndReturnFailedInitialized(nil, err)
		}
	}
	return p.SetAndReturnSucceededInitialized(p.result, p.err)
}

type legacyTask2Provider struct {
	*Provider
	initializeCalls int
	result          any
	err             error
}

func newLegacyTask2Provider() *legacyTask2Provider {
	return &legacyTask2Provider{
		Provider: NewProvider().SetName("legacy").SetType(
			&PType{id: CustomTypeStart, name: "custom"},
		).(*Provider),
	}
}

func (p *legacyTask2Provider) Initialize(IContext, ...ProviderInitFunc) (any, error) {
	p.initializeCalls++
	return p.result, p.err
}

type skippedDuringInitializeTask2Provider struct {
	*Provider
	initializeCalls int
}

func newSkippedDuringInitializeTask2Provider() *skippedDuringInitializeTask2Provider {
	return &skippedDuringInitializeTask2Provider{
		Provider: NewProvider().SetName("skipped-during-initialize").SetType(
			&PType{id: CustomTypeStart, name: "custom"},
		).(*Provider),
	}
}

func (p *skippedDuringInitializeTask2Provider) Initialize(IContext, ...ProviderInitFunc) (any, error) {
	p.initializeCalls++
	p.SetStatus(StateSkipped)
	return nil, nil
}

func TestState_ValueContract(t *testing.T) {
	tests := []struct {
		state State
		id    uint8
		name  string
	}{
		{state: StatePending, id: 0, name: "pending"},
		{state: StateLoaded, id: 1, name: "loaded"},
		{state: StateSkipped, id: 2, name: "skipped"},
		{state: StateFailed, id: 3, name: "failed"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.id, tt.state.Id())
		assert.Equal(t, tt.name, tt.state.Name())
	}
}

func TestState_ZeroAndUnknownValues(t *testing.T) {
	var zero State
	assert.Equal(t, StatePending, zero)

	unknown := State(255)
	assert.Equal(t, uint8(255), unknown.Id())
	assert.Equal(t, "unknown", unknown.Name())
}

func TestProvider_UnknownStateRemainsPendingForInitialization(t *testing.T) {
	provider := NewProvider().
		SetName("unknown-state").
		SetType(&PType{id: CustomTypeStart, name: "custom"}).
		SetStatus(State(255))

	assert.True(t, provider.Check())
	result, err := provider.ReturnDirectly()
	assert.Nil(t, result)
	assert.EqualError(t, err, "provider 'unknown-state' is in an unknown state")
}

func TestProvider_StateIsIsolatedBetweenInstances(t *testing.T) {
	typ := &PType{id: CustomTypeStart, name: "custom"}
	first := NewProvider().SetName("first").SetType(typ)
	second := NewProvider().SetName("second").SetType(typ)

	_, err := first.SetAndReturnSucceededInitialized("first-result", nil)
	require.NoError(t, err)

	assert.Equal(t, StateLoaded, first.Status())
	assert.Equal(t, StatePending, second.Status())
}

func TestProvider_LifecycleStatusBypassesPublicSetterOnce(t *testing.T) {
	p := NewProvider()

	assert.Same(t, p, p.SetStatus(StateSkipped))
	p.setLifecycleStatus(StateFailed)
	p.SetStatus(StateLoaded)

	assert.Equal(t, StateFailed, p.Status())
}

func TestProvider_InitializeReturnsCachedResultWithoutCallingChildAgain(t *testing.T) {
	child := newTask2Provider("child", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	child.result = "initialized"

	first, err := child.Initialize(nil)
	require.NoError(t, err)
	second, err := child.Initialize(nil)
	require.NoError(t, err)

	assert.Equal(t, "initialized", first)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, child.initializeCalls)
}

func TestProviderManager_InitializeProviderCachesLegacyProviderResult(t *testing.T) {
	manager := NewProviderManager(nil)
	initializer, ok := any(manager).(interface {
		InitializeProvider(IProvider, ...ProviderInitFunc) (any, error)
	})
	require.True(t, ok, "ProviderManager must own the guarded provider initialization path")

	child := newLegacyTask2Provider()
	child.result = "initialized"

	first, err := initializer.InitializeProvider(child)
	require.NoError(t, err)
	second, err := initializer.InitializeProvider(child)
	require.NoError(t, err)

	assert.Equal(t, "initialized", first)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, child.initializeCalls)
	assert.Equal(t, StateLoaded.Id(), child.Status().Id())
}

func TestProviderManager_InitializeProviderCachesLegacyProviderError(t *testing.T) {
	manager := NewProviderManager(nil)
	initializer, ok := any(manager).(interface {
		InitializeProvider(IProvider, ...ProviderInitFunc) (any, error)
	})
	require.True(t, ok, "ProviderManager must own the guarded provider initialization path")

	child := newLegacyTask2Provider()
	child.err = errors.New("initialize failed")

	first, firstErr := initializer.InitializeProvider(child)
	second, secondErr := initializer.InitializeProvider(child)

	assert.Nil(t, first)
	assert.Nil(t, second)
	assert.EqualError(t, firstErr, "initialize failed")
	assert.EqualError(t, secondErr, "initialize failed")
	assert.Equal(t, 1, child.initializeCalls)
	assert.Equal(t, StateFailed.Id(), child.Status().Id())
}

func TestProviderManager_InitializeProviderCachesSkippedState(t *testing.T) {
	manager := NewProviderManager(nil)
	provider := newSkippedDuringInitializeTask2Provider()

	first, firstErr := manager.InitializeProvider(provider)
	second, secondErr := manager.InitializeProvider(provider)

	assert.Nil(t, first)
	assert.Nil(t, second)
	assert.EqualError(t, firstErr, "provider 'skipped-during-initialize' is skipped, cannot return initialized result")
	assert.EqualError(t, secondErr, "provider 'skipped-during-initialize' is skipped, cannot return initialized result")
	assert.Equal(t, 1, provider.initializeCalls)
	assert.Equal(t, StateSkipped, provider.Status())
}

func TestProvider_FluentValuesAndOneTimeState(t *testing.T) {
	firstType := &PType{id: CustomTypeStart, name: "first"}
	secondType := &PType{id: CustomTypeStart + 1, name: "second"}
	provider := NewProvider()

	returned := provider.SetName("provider").SetVersion("v1").SetTarget("fiber")
	assert.Same(t, provider, returned)
	provider.SetType(firstType).SetType(secondType)
	provider.SetStatus(StateLoaded).SetStatus(StateFailed)

	assert.Equal(t, "provider", provider.Name())
	assert.Equal(t, "v1", provider.Version())
	assert.Equal(t, "fiber", provider.Target())
	assert.Same(t, firstType, provider.Type())
	assert.Equal(t, StateLoaded, provider.Status())
	assert.NotEmpty(t, (&ProviderError{msg: "problem"}).Error())
}

func TestProvider_CheckAndMountValidation(t *testing.T) {
	provider := NewProvider().SetName("provider").(*Provider)
	assert.Panics(t, func() { provider.Check() })
	provider.SetType(&PType{id: CustomTypeStart, name: "custom"})
	assert.NotPanics(t, func() { provider.Check() })

	assert.Panics(t, func() { provider.MountToParent() })
	assert.Panics(t, func() { provider.MountToParent(provider) })
	_, err := provider.Initialize(nil)
	assert.ErrorContains(t, err, "MountToParent")
	assert.ErrorContains(t, provider.RegisterTo(NewProviderManager(nil)), "MountToParent")
}

func TestProvider_InitializeAndRegistrationDelegateToMountedChild(t *testing.T) {
	typ := &PType{id: CustomTypeStart, name: "custom"}
	child := newTask2Provider("child", "fiber", typ)
	child.result = "initialized"

	result, err := child.Provider.Initialize(nil, func(provider IProvider) (any, error) {
		assert.Same(t, child, provider)
		return "injected", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "initialized", result)
	assert.Equal(t, 1, child.initializeCalls)
	assert.Equal(t, "injected", child.injected)

	manager := NewProviderManager(nil)
	require.NoError(t, child.RegisterTo(manager))
	registered, err := manager.GetProvider("child")
	require.NoError(t, err)
	assert.Same(t, child, registered)
}

func TestProvider_InitializePropagatesChildErrors(t *testing.T) {
	child := newTask2Provider("child", "fiber", &PType{id: CustomTypeStart, name: "custom"})
	child.err = errors.New("initialize failed")

	result, err := child.Provider.Initialize(nil)
	assert.Nil(t, result)
	assert.EqualError(t, err, "initialize failed")
}

func TestProvider_BindToUniqueManager(t *testing.T) {
	provider := NewProvider().SetName("provider").SetType(&PType{id: CustomTypeStart, name: "custom"})
	manager := NewProviderManager(nil).SetName("manager")

	assert.Same(t, provider, provider.BindToUniqueManagerIfSingleton(manager))
	assert.True(t, manager.IsUnique())
	got, err := manager.GetProvider("provider")
	require.NoError(t, err)
	assert.Same(t, provider, got)
}
