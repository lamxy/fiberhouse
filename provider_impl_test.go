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
	p.Check()
	p.initializeCalls++
	if len(initFunc) > 0 {
		injected, err := initFunc[0](p)
		p.injected = injected
		if err != nil {
			return nil, err
		}
	}
	return p.result, p.err
}

func TestState_SetAndSetState(t *testing.T) {
	state := new(State)
	assert.Same(t, state, state.Set(7, "custom"))
	assert.Equal(t, uint8(7), state.Id())
	assert.Equal(t, "custom", state.Name())

	other := new(State).Set(8, "other")
	assert.Same(t, state, state.SetState(other))
	assert.Equal(t, uint8(8), state.Id())
	assert.Equal(t, "other", state.Name())
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
	assert.Same(t, StateLoaded, provider.Status())
	assert.NotEmpty(t, (&ProviderError{msg: "problem"}).Error())
}

func TestProvider_CheckAndMountValidation(t *testing.T) {
	provider := NewProvider().SetName("provider").(*Provider)
	assert.Panics(t, provider.Check)
	provider.SetType(&PType{id: CustomTypeStart, name: "custom"})
	assert.NotPanics(t, provider.Check)

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
