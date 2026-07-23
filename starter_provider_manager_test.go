package fiberhouse

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/lamxy/fiberhouse/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type task7LocationSnapshot struct {
	location *PLocation
	managers []IProviderManager
}

func task7DefaultLocations(t *testing.T) []*PLocation {
	t.Helper()
	locations := ProviderLocationDefault()
	interfaces := []IProviderLocation{
		locations.ZeroLocation,
		locations.LocationAdaptCoreCtxChoose,
		locations.LocationBootStrapConfig,
		locations.LocationFrameStarterOptionInit,
		locations.LocationCoreStarterOptionInit,
		locations.LocationFrameStarterCreate,
		locations.LocationCoreStarterCreate,
		locations.LocationGlobalInit,
		locations.LocationGlobalKeepaliveInit,
		locations.LocationCoreEngineInit,
		locations.LocationCoreHookInit,
		locations.LocationAppMiddlewareInit,
		locations.LocationModuleMiddlewareInit,
		locations.LocationRouteRegisterInit,
		locations.LocationTaskServerInit,
		locations.LocationModuleSwaggerInit,
		locations.LocationServerRunBefore,
		locations.LocationServerRun,
		locations.LocationServerRunAfter,
		locations.LocationServerShutdownBefore,
		locations.LocationServerShutdown,
		locations.LocationServerShutdownAfter,
		locations.LocationResponseInfoInit,
	}
	result := make([]*PLocation, 0, len(interfaces))
	for _, location := range interfaces {
		concrete, ok := location.(*PLocation)
		require.True(t, ok, location.GetLocationName())
		result = append(result, concrete)
	}
	return result
}

func isolateTask7ProviderGlobals(t *testing.T) {
	t.Helper()
	locationSnapshots := make([]task7LocationSnapshot, 0)
	for _, location := range task7DefaultLocations(t) {
		location.mu.RLock()
		managers := append([]IProviderManager(nil), location.managers...)
		location.mu.RUnlock()
		locationSnapshots = append(locationSnapshots, task7LocationSnapshot{location: location, managers: managers})
	}
	previousDefaultProviders := defaultProvidersInstance
	previousDefaultManagers := defaultPManagersInstance
	previousRecoveryManager := recoveryManagerInstance

	t.Cleanup(func() {
		for _, snapshot := range locationSnapshots {
			snapshot.location.mu.Lock()
			snapshot.location.managers = append([]IProviderManager(nil), snapshot.managers...)
			snapshot.location.mu.Unlock()
		}

		defaultProvidersInstance = previousDefaultProviders
		defaultProvidersOnce = sync.Once{}
		if previousDefaultProviders != nil {
			defaultProvidersOnce.Do(func() {})
		}
		defaultPManagersInstance = previousDefaultManagers
		defaultPManagersOnce = sync.Once{}
		if previousDefaultManagers != nil {
			defaultPManagersOnce.Do(func() {})
		}
		recoveryManagerInstance = previousRecoveryManager
		recoveryManagerOnce = sync.Once{}
		if previousRecoveryManager != nil {
			recoveryManagerOnce.Do(func() {})
		}
	})
}

func assertTask7ManagersSame(t *testing.T, expected, actual []IProviderManager) {
	t.Helper()
	require.Len(t, actual, len(expected))
	for index := range expected {
		assert.Same(t, expected[index], actual[index], index)
	}
}

func TestTask7ProviderGlobalFixtures_RestoreLocationsAndSingletons(t *testing.T) {
	locations := task7DefaultLocations(t)
	beforeManagers := make(map[*PLocation][]IProviderManager, len(locations))
	for _, location := range locations {
		beforeManagers[location] = location.GetManagers()
	}
	beforeProviders := defaultProvidersInstance
	beforeManagersSingleton := defaultPManagersInstance
	beforeRecovery := recoveryManagerInstance

	t.Run("isolated constructors", func(t *testing.T) {
		isolateTask7ProviderGlobals(t)
		ctx, _ := newFrameTestContext(t, nil)
		_ = NewFrameDefaultPManager(ctx)
		_ = NewCoreStarterPManager(ctx)
		_ = NewJsonCodecPManager(ctx)
		_ = NewRespInfoPManager(ctx)
		_ = NewRecoveryPManagerOnce(ctx)
		_ = DefaultProviders()
		_ = DefaultPManagers(ctx)
	})

	for _, location := range locations {
		assertTask7ManagersSame(t, beforeManagers[location], location.GetManagers())
	}
	if beforeProviders == nil {
		assert.Nil(t, defaultProvidersInstance)
	} else {
		assert.Same(t, beforeProviders, defaultProvidersInstance)
	}
	if beforeManagersSingleton == nil {
		assert.Nil(t, defaultPManagersInstance)
	} else {
		assert.Same(t, beforeManagersSingleton, defaultPManagersInstance)
	}
	if beforeRecovery == nil {
		assert.Nil(t, recoveryManagerInstance)
	} else {
		assert.Same(t, beforeRecovery, recoveryManagerInstance)
	}
}

func TestFrameDefaultProvider_ValidatesCallbackAndBuildsFrame(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	provider := NewFrameDefaultProvider()
	assert.Equal(t, "FrameDefaultProvider", provider.Name())
	assert.Equal(t, constant.FrameTypeWithDefaultFrameStarter, provider.Target())

	_, err := provider.Initialize(ctx)
	assert.ErrorContains(t, err, "no initFunc")

	provider = NewFrameDefaultProvider()
	sentinel := errors.New("frame options failed")
	_, err = provider.Initialize(ctx, func(IProvider) (any, error) { return nil, sentinel })
	assert.ErrorIs(t, err, sentinel)

	provider = NewFrameDefaultProvider()
	_, err = provider.Initialize(ctx, func(IProvider) (any, error) { return "wrong", nil })
	assert.ErrorContains(t, err, "[]FrameStarterOption expected")

	provider = NewFrameDefaultProvider()
	result, err := provider.Initialize(ctx, func(got IProvider) (any, error) {
		assert.Same(t, provider, got)
		return []FrameStarterOption{func(FrameStarter) {}}, nil
	})
	require.NoError(t, err)
	assert.IsType(t, &FrameApplication{}, result)
	assert.Same(t, provider, provider.MountToParent())
}

func TestStarterManagers_ValidatePayloadSelectTargetAndReportMissing(t *testing.T) {
	isolateTask7ProviderGlobals(t)
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
	isolateTask7ProviderGlobals(t)
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
	isolateTask7ProviderGlobals(t)
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

type layerTestModel struct {
	*Service
	dbName string
	table  string
}

func (m *layerTestModel) SetName(name string) Locator {
	m.Service.SetName(name)
	return m
}
func (m *layerTestModel) GetDbName() string { return m.dbName }
func (m *layerTestModel) SetDbName(name string) Modeler {
	m.dbName = name
	return m
}
func (m *layerTestModel) GetTable() string { return m.table }
func (m *layerTestModel) SetTable(table string, namespace ...string) Modeler {
	m.table = RegisterKeyName(table, namespace...)
	return m
}

var _ Modeler = (*layerTestModel)(nil)

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

	model := &layerTestModel{Service: NewService(ctx)}
	assert.Same(t, model, model.SetName("model"))
	assert.Equal(t, "model", model.GetName())
	assert.Same(t, ctx, model.GetContext())
	assert.Same(t, model, model.SetDbName("application"))
	assert.Equal(t, "application", model.GetDbName())
	assert.Same(t, model, model.SetTable("users", "tenant", "model"))
	assert.Equal(t, "tenant.model.users", model.GetTable())
	got, err := model.GetInstance(key)
	require.NoError(t, err)
	assert.Same(t, value, got)
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
		var err error
		prior, err = manager.Get(notFoundKey)
		require.NoError(t, err)
	}
	manager.Unregister(notFoundKey)
	require.True(t, manager.Register(notFoundKey, func() (interface{}, error) {
		return exception.ExceptionMap{"NotFoundDocument": {Code: 4040, Msg: "not found"}}, nil
	}))
	t.Cleanup(func() {
		manager.Unregister(notFoundKey)
		if wasRegistered {
			require.True(t, manager.Register(notFoundKey, func() (interface{}, error) { return prior, nil }))
			restored, err := manager.Get(notFoundKey)
			require.NoError(t, err)
			assert.Equal(t, prior, restored)
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
	isolateTask7ProviderGlobals(t)
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
