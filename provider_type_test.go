package fiberhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProviderTypeRegistryForTest() *ProviderTypeRegistry {
	return &ProviderTypeRegistry{
		defaultTypes:  make(map[string]IProviderType),
		customTypes:   make(map[string]IProviderType),
		nextDefaultID: uint16(DefaultTypeStart),
		nextCustomID:  uint16(CustomTypeStart),
	}
}

func TestProviderTypeRegistry_DefaultAndCustomNamespaces(t *testing.T) {
	registry := newProviderTypeRegistryForTest()

	defaultType, err := registry.Default("default")
	require.NoError(t, err)
	assert.Equal(t, DefaultTypeStart, defaultType.GetTypeID())
	assert.Equal(t, "default", defaultType.GetTypeName())
	assert.True(t, defaultType.IsDefaultType())

	customType, err := registry.Custom("custom")
	require.NoError(t, err)
	assert.Equal(t, CustomTypeStart, customType.GetTypeID())
	assert.Equal(t, "custom", customType.GetTypeName())
	assert.False(t, customType.IsDefaultType())

	gotDefault, err := registry.Type("default")
	require.NoError(t, err)
	assert.Same(t, defaultType, gotDefault)
	assert.Same(t, customType, registry.MustType("custom"))

	_, err = registry.Type("missing")
	assert.ErrorContains(t, err, "not found")
	assert.Panics(t, func() { registry.MustType("missing") })
}

func TestProviderTypeRegistry_RejectsDuplicateNamesAcrossNamespaces(t *testing.T) {
	registry := newProviderTypeRegistryForTest()
	require.NotNil(t, registry.MustDefault("shared"))

	_, err := registry.Default("shared")
	assert.ErrorContains(t, err, "already registered")
	_, err = registry.Custom("shared")
	assert.ErrorContains(t, err, "default type")
	assert.Panics(t, func() { registry.MustDefault("shared") })

	require.NotNil(t, registry.MustCustom("custom"))
	_, err = registry.Custom("custom")
	assert.ErrorContains(t, err, "already registered")
	_, err = registry.Default("custom")
	assert.ErrorContains(t, err, "custom type")
	assert.Panics(t, func() { registry.MustCustom("custom") })
}

func TestProviderTypeRegistry_IDBoundaries(t *testing.T) {
	defaultRegistry := newProviderTypeRegistryForTest()
	defaultRegistry.nextDefaultID = uint16(DefaultTypeEnd)
	lastDefault, err := defaultRegistry.Default("last-default")
	require.NoError(t, err)
	assert.Equal(t, DefaultTypeEnd, lastDefault.GetTypeID())
	_, err = defaultRegistry.Default("exhausted")
	assert.ErrorContains(t, err, "exhausted")

	customRegistry := newProviderTypeRegistryForTest()
	customRegistry.nextCustomID = uint16(CustomTypeEnd)
	lastCustom, err := customRegistry.Custom("last-custom")
	require.NoError(t, err)
	assert.Equal(t, CustomTypeEnd, lastCustom.GetTypeID())
	_, err = customRegistry.Custom("custom-exhausted")
	assert.ErrorContains(t, err, "exhausted")
}

func TestPType_GettersAndDefaultBoundary(t *testing.T) {
	defaultBoundary := &PType{id: DefaultTypeEnd, name: "boundary"}
	customBoundary := &PType{id: CustomTypeStart, name: "custom"}

	assert.Equal(t, DefaultTypeEnd, defaultBoundary.GetTypeID())
	assert.Equal(t, "boundary", defaultBoundary.GetTypeName())
	assert.True(t, defaultBoundary.IsDefaultType())
	assert.False(t, customBoundary.IsDefaultType())
}
