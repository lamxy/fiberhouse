package fiberhouse

import (
	"errors"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/lamxy/fiberhouse/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameDefaultProvider_ValidatesCallbackAndBuildsFrame(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	provider := NewFrameDefaultProvider()
	assert.Equal(t, "FrameDefaultProvider", provider.Name())
	assert.Equal(t, constant.FrameTypeWithDefaultFrameStarter, provider.Target())

	_, err := provider.Initialize(ctx)
	assert.ErrorContains(t, err, "no initFunc")

	sentinel := errors.New("frame options failed")
	_, err = provider.Initialize(ctx, func(IProvider) (any, error) { return nil, sentinel })
	assert.ErrorIs(t, err, sentinel)

	_, err = provider.Initialize(ctx, func(IProvider) (any, error) { return "wrong", nil })
	assert.ErrorContains(t, err, "[]FrameStarterOption expected")

	result, err := provider.Initialize(ctx, func(got IProvider) (any, error) {
		assert.Same(t, provider, got)
		return []FrameStarterOption{func(FrameStarter) {}}, nil
	})
	require.NoError(t, err)
	assert.IsType(t, &FrameApplication{}, result)
	assert.Same(t, provider, provider.MountToParent())
}

func TestStarterManagers_ValidatePayloadSelectTargetAndReportMissing(t *testing.T) {
	t.Run("frame manager", func(t *testing.T) {
		ctx, _ := newFrameTestContext(t, nil)
		ctx.GetBootConfig().FrameType = "custom-frame"
		manager := NewFrameDefaultPManager(ctx)
		_, err := manager.LoadProvider()
		assert.ErrorContains(t, err, "load function is required")
		_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return "wrong", nil })
		assert.ErrorContains(t, err, "[]FrameStarterOption")
		sentinel := errors.New("frame load failed")
		_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return nil, sentinel })
		assert.ErrorIs(t, err, sentinel)

		_, err = manager.LoadProvider(func(IProviderManager) (any, error) {
			return []FrameStarterOption{func(FrameStarter) {}}, nil
		})
		assert.ErrorContains(t, err, "custom-frame")

		provider := newTask2Provider("custom-frame-provider", "custom-frame", ProviderTypeDefault().GroupFrameStarterChoose)
		provider.result = "frame-result"
		require.NoError(t, manager.Register(provider))
		result, err := manager.LoadProvider(func(got IProviderManager) (any, error) {
			assert.Same(t, manager, got)
			return []FrameStarterOption{func(FrameStarter) {}}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "frame-result", result)
		assert.Equal(t, 1, provider.initializeCalls)
		assert.IsType(t, []FrameStarterOption{}, provider.injected)
		assert.Same(t, manager, manager.MountToParent())
	})

	t.Run("core manager", func(t *testing.T) {
		ctx, _ := newFrameTestContext(t, nil)
		manager := NewCoreStarterPManager(ctx)
		_, err := manager.LoadProvider()
		assert.ErrorContains(t, err, "load function is required")
		_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return "wrong", nil })
		assert.ErrorContains(t, err, "[]CoreStarterOption")
		sentinel := errors.New("core load failed")
		_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return nil, sentinel })
		assert.ErrorIs(t, err, sentinel)

		_, err = manager.LoadProvider(func(IProviderManager) (any, error) {
			return []CoreStarterOption{}, nil
		})
		assert.ErrorContains(t, err, "no matching core starter")

		provider := newTask2Provider("fiber-provider", "fiber", ProviderTypeDefault().GroupCoreStarterChoose)
		provider.result = "core-result"
		require.NoError(t, manager.Register(provider))
		result, err := manager.LoadProvider(func(got IProviderManager) (any, error) {
			assert.Same(t, manager, got)
			return []CoreStarterOption{}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "core-result", result)
		assert.IsType(t, []CoreStarterOption{}, provider.injected)
		assert.Same(t, manager, manager.MountToParent())
	})
}

func TestResponseProvidersAndManager_SelectByContentType(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	protobufProvider := NewRespInfoProtobufProvider()
	msgpackProvider := NewRespInfoMsgpackProvider()
	assert.Equal(t, "application/x-protobuf", protobufProvider.Name())
	assert.Equal(t, "application/msgpack", msgpackProvider.Name())

	protobufValue, err := protobufProvider.Initialize(ctx)
	require.NoError(t, err)
	protobufResponse, ok := protobufValue.(response.IResponse)
	require.True(t, ok)
	protobufResponse.Release()
	msgpackValue, err := msgpackProvider.Initialize(ctx)
	require.NoError(t, err)
	msgpackResponse, ok := msgpackValue.(response.IResponse)
	require.True(t, ok)
	msgpackResponse.Release()

	manager := NewRespInfoPManager(ctx)
	_, err = manager.LoadProvider()
	assert.ErrorContains(t, err, "no load function")
	sentinel := errors.New("content type failed")
	_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return nil, sentinel })
	assert.ErrorIs(t, err, sentinel)
	_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return 123, nil })
	assert.ErrorContains(t, err, "expected string")
	_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return "missing", nil })
	assert.ErrorContains(t, err, ErrProviderNotFound.Error())
	require.NoError(t, manager.Register(protobufProvider))
	selected, err := manager.LoadProvider(func(got IProviderManager) (any, error) {
		assert.Same(t, manager, got)
		return "application/x-protobuf", nil
	})
	require.NoError(t, err)
	assert.Same(t, protobufProvider, selected)
}

func TestJsonCodecManager_CallbackAndConfiguredSelection(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	manager := NewJsonCodecPManager(ctx)
	_, err := manager.LoadProvider()
	assert.ErrorContains(t, err, "no json codec provider")

	token := &struct{ name string }{"callback"}
	result, err := manager.LoadProvider(func(got IProviderManager) (any, error) {
		assert.Same(t, manager, got)
		return token, nil
	})
	require.NoError(t, err)
	assert.Same(t, token, result)
	sentinel := errors.New("codec callback failed")
	_, err = manager.LoadProvider(func(IProviderManager) (any, error) { return nil, sentinel })
	assert.ErrorIs(t, err, sentinel)

	mismatch := newTask2Provider("mismatch", "gin", ProviderTypeDefault().GroupTrafficCodecChoose)
	mismatch.SetVersion("test")
	require.NoError(t, manager.Register(mismatch))
	_, err = manager.LoadProvider()
	assert.ErrorContains(t, err, "no matching json codec")

	match := newTask2Provider("match", "fiber", ProviderTypeDefault().GroupTrafficCodecChoose)
	match.SetVersion("test")
	match.result = token
	require.NoError(t, manager.Register(match))
	result, err = manager.LoadProvider()
	require.NoError(t, err)
	assert.Same(t, token, result)
	assert.Equal(t, 1, match.initializeCalls)
	assert.Same(t, manager, manager.MountToParent())
}

func TestLayerLocators_FluentNamesContextAndContainerLookup(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	key := globalmanager.KeyName(fmt.Sprintf("task7-layer-%s", t.Name()))
	ctx.GetContainer().Unregister(key)
	t.Cleanup(func() { ctx.GetContainer().Unregister(key) })
	value := &struct{ name string }{"dependency"}
	require.True(t, ctx.GetContainer().Register(key, func() (interface{}, error) { return value, nil }))

	locators := []Locator{NewApi(ctx), NewService(ctx), NewRepository(ctx)}
	for _, locator := range locators {
		assert.Same(t, locator, locator.SetName("named"))
		assert.Equal(t, "named", locator.GetName())
		assert.Same(t, ctx, locator.GetContext())
		got, err := locator.GetInstance(key)
		require.NoError(t, err)
		assert.Same(t, value, got)
		_, err = locator.GetInstance("missing-task7-layer")
		assert.ErrorContains(t, err, "not found")
	}

	var _ ApiLocator = NewApi(ctx)
	var _ ServiceLocator = NewService(ctx)
	var _ RepositoryLocator = NewRepository(ctx)
}

func TestGlobalUtilities_NamespacesInstancesRecoveryAndMongoErrors(t *testing.T) {
	assert.Equal(t, "name", RegisterKeyName("name"))
	assert.Equal(t, "module.layer.name", RegisterKeyName("name", "module", "layer"))
	overrides := []string{"default", "namespace"}
	assert.Equal(t, overrides, GetNamespace(overrides))
	assert.Equal(t, []string{"explicit"}, GetNamespace(overrides, "explicit"))
	assert.Empty(t, GetNamespace(nil))

	key := fmt.Sprintf("task7-global-%s", t.Name())
	manager := globalmanager.NewGlobalManagerOnce()
	manager.Unregister(key)
	t.Cleanup(func() { manager.Unregister(key) })
	assert.Empty(t, RegisterKeyInitializerFunc("", func() (interface{}, error) { return nil, nil }))
	assert.Equal(t, key, RegisterKeyInitializerFunc(key, func() (interface{}, error) { return 42, nil }))
	value, err := GetInstance[int](key)
	require.NoError(t, err)
	assert.Equal(t, 42, value)
	assert.Equal(t, 42, GetMustInstance[int](key))
	_, err = GetInstance[string](key)
	assert.ErrorContains(t, err, "assertion failure")
	assert.Panics(t, func() { GetMustInstance[string](key) })
	_, err = GetInstance[int]("task7-global-missing")
	assert.ErrorContains(t, err, "not found")

	fn := func() string { return "ok" }
	recovered, err := RecoverMiddleware[func() string](fn)
	require.NoError(t, err)
	assert.Equal(t, "ok", recovered())
	assert.Equal(t, fmt.Sprintf("%p", fn), fmt.Sprintf("%p", MustRecoverMiddleware[func() string](fn)))
	_, err = RecoverMiddleware[func() string](nil)
	assert.ErrorContains(t, err, "cannot be nil")
	_, err = RecoverMiddleware[func() string](123)
	assert.ErrorContains(t, err, "assertion failure")
	assert.Panics(t, func() { MustRecoverMiddleware[func() string](123) })

	notFoundKey := constant.RegisterKeyPrefix + "exceptions"
	wasRegistered := manager.IsRegistered(notFoundKey)
	var prior interface{}
	if wasRegistered {
		prior, _ = manager.Get(notFoundKey)
	}
	manager.Unregister(notFoundKey)
	require.True(t, manager.Register(notFoundKey, func() (interface{}, error) {
		return exception.ExceptionMap{"NotFoundDocument": {Code: 4040, Msg: "not found"}}, nil
	}))
	t.Cleanup(func() {
		manager.Unregister(notFoundKey)
		if wasRegistered {
			manager.Register(notFoundKey, func() (interface{}, error) { return prior, nil })
		}
	})
	zero, mapped := GetNoDocumentsError[string](mongo.ErrNoDocuments)
	assert.Empty(t, zero)
	var mappedException *exception.Exception
	require.ErrorAs(t, mapped, &mappedException)
	assert.Equal(t, 4040, mappedException.Code)
	mappedException.Release()
	sentinel := errors.New("database failed")
	zero, mapped = GetNoDocumentsError[string](sentinel)
	assert.Empty(t, zero)
	assert.ErrorIs(t, mapped, sentinel)
	mapped = GetErrOrNoDocuments(mongo.ErrNoDocuments)
	require.ErrorAs(t, mapped, &mappedException)
	mappedException.Release()
	assert.ErrorIs(t, GetErrOrNoDocuments(sentinel), sentinel)
}

func TestDefaultCollections_ReturnCopiesFilterAndAppend(t *testing.T) {
	first := newTask2Provider("first", "fiber", ProviderTypeDefault().GroupCoreStarterChoose)
	second := newTask2Provider("second", "gin", ProviderTypeDefault().GroupCoreStarterChoose)
	providers := &DefaultProviderCollection{providers: []IProvider{first}}
	copyOfProviders := providers.List()
	copyOfProviders[0] = second
	assert.Same(t, first, providers.List()[0])
	assert.Same(t, providers, providers.Add(second))
	assert.Equal(t, []IProvider{first, second}, providers.AndMore())
	providers.Except("first")
	assert.Equal(t, []IProvider{second}, providers.List())
	assert.Same(t, providers, providers.Except())
	assert.Nil(t, (&DefaultProviderCollection{}).List())
	assert.Nil(t, (&DefaultProviderCollection{}).AndMore())

	firstManager := NewProviderManager(nil).SetName("first")
	secondManager := NewProviderManager(nil).SetName("second")
	managers := &DefaultPManagerCollection{managers: []IProviderManager{firstManager}}
	copyOfManagers := managers.List()
	copyOfManagers[0] = secondManager
	assert.Same(t, firstManager, managers.List()[0])
	assert.Same(t, managers, managers.Add(secondManager))
	assert.Equal(t, []IProviderManager{firstManager, secondManager}, managers.AndMore())
	managers.Except("first")
	assert.Equal(t, []IProviderManager{secondManager}, managers.List())
	assert.Same(t, managers, managers.Except())
	assert.Nil(t, (&DefaultPManagerCollection{}).List())
	assert.Nil(t, (&DefaultPManagerCollection{}).AndMore())

	ctx, _ := newFrameTestContext(t, nil)
	assert.GreaterOrEqual(t, len(DefaultProviders().List()), 11)
	assert.GreaterOrEqual(t, len(DefaultPManagers(ctx).List()), 6)
}
