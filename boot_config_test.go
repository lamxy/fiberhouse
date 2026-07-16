package fiberhouse

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootConfigOptions_SetEveryConfigurableField(t *testing.T) {
	config := &BootConfig{}
	options := []BootConfigOption{
		WithAppId("app-id"),
		WithAppName("app-name"),
		WithVersion("v1.2.3"),
		WithDate("2026-07-17"),
		WithFrameType("frame"),
		WithCoreType("gin"),
		WithTrafficCodec("codec"),
		WithConfigPath("config-path"),
		WithLogPath("log-path"),
		WithCustomKV("custom", "value"),
	}
	for _, option := range options {
		option(config)
	}

	assert.Equal(t, "app-id", config.AppId)
	assert.Equal(t, "app-name", config.AppName)
	assert.Equal(t, "v1.2.3", config.Version)
	assert.Equal(t, "2026-07-17", config.Date)
	assert.Equal(t, "frame", config.FrameType)
	assert.Equal(t, "gin", config.CoreType)
	assert.Equal(t, "codec", config.TrafficCodec)
	assert.Equal(t, "config-path", config.ConfigPath)
	assert.Equal(t, "log-path", config.LogPath)
	value, err := config.GetValue("custom")
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

func TestBootConfig_CustomValuesFinallyAndMissing(t *testing.T) {
	config := &BootConfig{}
	_, err := config.GetValue("missing")
	assert.ErrorContains(t, err, "kvStorage is nil")
	assert.Panics(t, func() { config.GetMustValue("missing") })

	assert.Same(t, config, config.WithCustom("first", 1))
	assert.Equal(t, 1, config.GetMustValue("first"))
	_, err = config.GetValue("missing")
	assert.ErrorContains(t, err, "not found")
	assert.Panics(t, func() { config.GetMustValue("missing") })

	assert.Same(t, config, config.Finally())
	config.WithCustom("second", 2)
	_, err = config.GetValue("second")
	assert.ErrorContains(t, err, "not found")
	assert.Equal(t, 1, config.GetMustValue("first"))
}

func TestBootConfig_ConcurrentCustomReadsAndWrites(t *testing.T) {
	config := (&BootConfig{}).WithCustom("shared", 0)
	const iterations = 100
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			config.WithCustom("shared", i)
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			_, _ = config.GetValue("shared")
			_ = config.GetMustValue("shared")
		}
	}()

	close(start)
	wg.Wait()
	_, err := config.GetValue("shared")
	require.NoError(t, err)
}
