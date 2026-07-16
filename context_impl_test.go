package fiberhouse

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStorage_CRUDRangeAndClear(t *testing.T) {
	storage := NewDefaultStorage()
	assert.Zero(t, storage.Len())
	assert.False(t, storage.Has("missing"))
	assert.Equal(t, "fallback", storage.GetOrDefault("missing", "fallback"))

	storage.Set("first", 1)
	storage.Set("second", 2)
	storage.Set("first", 3)
	value, exists := storage.Get("first")
	require.True(t, exists)
	assert.Equal(t, 3, value)
	assert.Equal(t, 3, storage.GetOrDefault("first", 0))
	assert.Equal(t, 2, storage.Len())

	keys := storage.Keys()
	sort.Strings(keys)
	assert.Equal(t, []string{"first", "second"}, keys)

	visited := 0
	storage.Range(func(string, interface{}) bool {
		visited++
		return false
	})
	assert.Equal(t, 1, visited)

	assert.False(t, storage.Delete("missing"))
	assert.True(t, storage.Delete("second"))
	assert.False(t, storage.Has("second"))
	storage.Clear()
	assert.Zero(t, storage.Len())
	assert.Empty(t, storage.Keys())
}

func TestDefaultStorage_ConcurrentReadersAndWriters(t *testing.T) {
	storage := NewDefaultStorage()
	const workers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			<-start
			storage.Set(fmt.Sprintf("key-%d", i), i)
		}(i)
		go func() {
			defer wg.Done()
			<-start
			_, _ = storage.Get("shared")
			_ = storage.Has("shared")
			_ = storage.Len()
		}()
	}

	close(start)
	wg.Wait()
	assert.Equal(t, workers, storage.Len())
	for i := 0; i < workers; i++ {
		value, exists := storage.Get(fmt.Sprintf("key-%d", i))
		require.True(t, exists)
		assert.Equal(t, i, value)
	}
}

func TestAppContext_BootConfigAndAppStateRegisterOnce(t *testing.T) {
	cfg := appconfig.NewAppConfig()
	logger := zerolog.Nop()
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	firstBoot := &BootConfig{AppName: "first"}
	secondBoot := &BootConfig{AppName: "second"}

	ctx.RegisterBootConfig(firstBoot)
	ctx.RegisterBootConfig(secondBoot)
	assert.Same(t, firstBoot, ctx.GetBootConfig())

	assert.False(t, ctx.GetAppState())
	ctx.RegisterAppState(false)
	ctx.RegisterAppState(true)
	assert.False(t, ctx.GetAppState())
	assert.Same(t, cfg, ctx.GetConfig())
	assert.Same(t, ctx.GetLogger(), ctx.GetLogger())
	assert.NotNil(t, ctx.GetContainer())
	assert.NotNil(t, ctx.GetValidateWrap())
}

func TestAppContext_StarterRegistration(t *testing.T) {
	ctx := newTask2AppContext(t, "fiber")
	starter := &WebApplication{}

	ctx.RegisterStarterApp(starter)
	assert.Same(t, starter, ctx.GetStarterApp())
	assert.Same(t, starter, ctx.GetStarter())
}

func TestAppContext_LoggerOriginMissingAndSuccess(t *testing.T) {
	cfg := appconfig.NewAppConfig()
	baseLogger := zerolog.Nop()
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&baseLogger))

	gotBase, err := ctx.GetLoggerWithOrigin("")
	require.NoError(t, err)
	assert.Same(t, &baseLogger, gotBase)
	assert.Same(t, &baseLogger, ctx.GetMustLoggerWithOrigin(""))

	missingOrigin := appconfig.LogOrigin("task2-missing-origin")
	ctx.GetContainer().Unregister(missingOrigin.InstanceKey())
	_, err = ctx.GetLoggerWithOrigin(missingOrigin)
	assert.ErrorContains(t, err, "not found")
	assert.Panics(t, func() { ctx.GetMustLoggerWithOrigin(missingOrigin) })

	registeredOrigin := appconfig.LogOrigin("task2-registered-origin")
	ctx.GetContainer().Unregister(registeredOrigin.InstanceKey())
	t.Cleanup(func() { ctx.GetContainer().Unregister(registeredOrigin.InstanceKey()) })
	originLogger := zerolog.Nop()
	require.True(t, ctx.GetContainer().Register(registeredOrigin.InstanceKey(), func() (interface{}, error) {
		return &originLogger, nil
	}))
	gotOrigin, err := ctx.GetLoggerWithOrigin(registeredOrigin)
	require.NoError(t, err)
	assert.Same(t, &originLogger, gotOrigin)
	assert.Same(t, &originLogger, ctx.GetMustLoggerWithOrigin(registeredOrigin))
}
