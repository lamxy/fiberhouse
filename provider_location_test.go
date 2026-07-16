package fiberhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProviderLocationRegistryForTest() *ProviderLocationRegistry {
	return &ProviderLocationRegistry{
		defaultLocations: make(map[string]IProviderLocation),
		customLocations:  make(map[string]IProviderLocation),
		nextDefaultID:    DefaultLocationStart,
		nextCustomID:     CustomLocationStart,
	}
}

func TestProviderLocationRegistry_DefaultAndCustomNamespaces(t *testing.T) {
	registry := newProviderLocationRegistryForTest()

	defaultLocation, err := registry.Default("default")
	require.NoError(t, err)
	assert.Equal(t, DefaultLocationStart, defaultLocation.GetLocationID())
	assert.Equal(t, "default", defaultLocation.GetLocationName())
	assert.True(t, defaultLocation.IsDefaultLocation())

	customLocation, err := registry.Custom("custom")
	require.NoError(t, err)
	assert.Equal(t, CustomLocationStart, customLocation.GetLocationID())
	assert.False(t, customLocation.IsDefaultLocation())

	got, err := registry.Location("default")
	require.NoError(t, err)
	assert.Same(t, defaultLocation, got)
	assert.Same(t, customLocation, registry.MustLocation("custom"))
	_, err = registry.Location("missing")
	assert.ErrorContains(t, err, "not found")
	assert.Panics(t, func() { registry.MustLocation("missing") })
}

func TestProviderLocationRegistry_RejectsDuplicateNamesAcrossNamespaces(t *testing.T) {
	registry := newProviderLocationRegistryForTest()
	require.NotNil(t, registry.MustDefault("shared"))

	_, err := registry.Default("shared")
	assert.ErrorContains(t, err, "already registered")
	_, err = registry.Custom("shared")
	assert.ErrorContains(t, err, "default location")
	assert.Panics(t, func() { registry.MustDefault("shared") })

	require.NotNil(t, registry.MustCustom("custom"))
	_, err = registry.Custom("custom")
	assert.ErrorContains(t, err, "already registered")
	_, err = registry.Default("custom")
	assert.ErrorContains(t, err, "custom location")
	assert.Panics(t, func() { registry.MustCustom("custom") })
}

func TestProviderLocationRegistry_IDBoundaries(t *testing.T) {
	defaultRegistry := newProviderLocationRegistryForTest()
	defaultRegistry.nextDefaultID = DefaultLocationEnd
	lastDefault, err := defaultRegistry.Default("last-default")
	require.NoError(t, err)
	assert.Equal(t, DefaultLocationEnd, lastDefault.GetLocationID())
	_, err = defaultRegistry.Default("exhausted")
	assert.ErrorContains(t, err, "exhausted")

	customRegistry := newProviderLocationRegistryForTest()
	customRegistry.nextCustomID = CustomLocationEnd
	lastCustom, err := customRegistry.Custom("last-custom")
	require.NoError(t, err)
	assert.Equal(t, CustomLocationEnd, lastCustom.GetLocationID())
}

func TestPLocationBind_AllowsDistinctManagersAtSameLocation(t *testing.T) {
	location := &PLocation{id: CustomLocationStart, name: "shared"}
	first := NewProviderManager(nil).SetName("first").SetOrBindToLocation(location)
	second := NewProviderManager(nil).SetName("second").SetOrBindToLocation(location)

	require.NoError(t, location.Bind(first))
	require.NoError(t, location.Bind(second))
	assert.Equal(t, []IProviderManager{first, second}, location.GetManagers())
}

func TestPLocationBind_RejectsNilAndExactDuplicate(t *testing.T) {
	location := &PLocation{id: CustomLocationStart, name: "shared"}
	manager := NewProviderManager(nil).SetName("manager").SetOrBindToLocation(location)

	assert.ErrorContains(t, location.Bind(nil), "cannot be nil")
	require.NoError(t, location.Bind(manager))
	assert.ErrorContains(t, location.Bind(manager), "already bound")
}

func TestPLocationGetManagers_ReturnsOrderedCopy(t *testing.T) {
	location := &PLocation{id: CustomLocationStart, name: "ordered"}
	assert.Nil(t, location.GetManagers())

	first := NewProviderManager(nil).SetName("first").SetOrBindToLocation(location)
	second := NewProviderManager(nil).SetName("second").SetOrBindToLocation(location)
	require.NoError(t, location.Bind(first))
	require.NoError(t, location.Bind(second))

	copyOfManagers := location.GetManagers()
	require.Equal(t, []IProviderManager{first, second}, copyOfManagers)
	copyOfManagers[0] = second
	assert.Equal(t, []IProviderManager{first, second}, location.GetManagers())
}

func TestPLocation_GettersAndDefaultBoundary(t *testing.T) {
	defaultBoundary := &PLocation{id: DefaultLocationEnd, name: "boundary"}
	customBoundary := &PLocation{id: CustomLocationStart, name: "custom"}

	assert.Equal(t, DefaultLocationEnd, defaultBoundary.GetLocationID())
	assert.Equal(t, "boundary", defaultBoundary.GetLocationName())
	assert.True(t, defaultBoundary.IsDefaultLocation())
	assert.False(t, customBoundary.IsDefaultLocation())
}
