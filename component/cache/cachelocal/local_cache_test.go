package cachelocal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLocalCache(t *testing.T, metrics bool) (*LocalCache, *cache.CacheOption) {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"test.local.numCounters":        int64(1_000),
		"test.local.maxCost":            int64(1 << 20),
		"test.local.bufferItems":        int64(64),
		"test.local.metrics":            metrics,
		"test.local.ignoreInternalCost": true,
	})
	logger := zerolog.Nop()
	appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	origin, err := NewLocalCache(appCtx, "test.local")
	require.NoError(t, err)
	lc := origin.(*LocalCache)
	co := cache.NewCacheOption(appCtx).SetJsonWrapper(jsoncodec.StdJsonDefault()).SetLocalTTL(time.Minute)
	t.Cleanup(func() { _ = lc.Close() })
	return lc, co
}

func TestLocalCache_SetWaitGetStringBytesAndObject(t *testing.T) {
	lc, co := newTestLocalCache(t, true)
	ctx := context.Background()

	for key, value := range map[string]interface{}{
		"string": "value",
		"bytes":  []byte("bytes"),
		"object": struct {
			Name string `json:"name"`
		}{Name: "fiberhouse"},
	} {
		require.NoError(t, lc.Set(ctx, key, value, co))
	}
	require.NoError(t, lc.Wait())

	got, err := lc.Get(ctx, "string", co)
	require.NoError(t, err)
	assert.Equal(t, "value", got)
	got, err = lc.Get(ctx, "bytes", co)
	require.NoError(t, err)
	assert.Equal(t, "bytes", got)
	got, err = lc.Get(ctx, "object", co)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"fiberhouse"}`, got)
	assert.Equal(t, cache.Local, lc.GetLevel())
}

func TestLocalCache_MissDeleteAndMetrics(t *testing.T) {
	lc, co := newTestLocalCache(t, true)
	ctx := context.Background()
	_, err := lc.Get(ctx, "missing", co)
	assert.ErrorIs(t, err, cache.ErrKeyNotFound)

	require.NoError(t, lc.Set(ctx, "key", "value", co))
	require.NoError(t, lc.Wait())
	_, err = lc.Get(ctx, "key", co)
	require.NoError(t, err)
	require.NoError(t, lc.Delete(ctx, "key", "also-missing"))
	_, err = lc.Get(ctx, "key", co)
	assert.ErrorIs(t, err, cache.ErrKeyNotFound)

	assert.NotNil(t, lc.GetMetrics(co))
	info := lc.GetMetricsInfo(co)
	require.NotNil(t, info)
	assert.GreaterOrEqual(t, info.TotalRequests, uint64(2))
	assert.Equal(t, info.TotalRequests, info.TotalHits+info.TotalMisses)
}

func TestLocalCache_TTLExpiresWithBoundedPolling(t *testing.T) {
	lc, co := newTestLocalCache(t, false)
	co.SetLocalTTL(20 * time.Millisecond)
	ctx := context.Background()
	require.NoError(t, lc.Set(ctx, "ttl", "value", co))
	require.NoError(t, lc.Wait())
	_, err := lc.Get(ctx, "ttl", co)
	require.NoError(t, err)

	deadline := time.NewTimer(2 * time.Second)
	ticker := time.NewTicker(2 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_, err = lc.Get(ctx, "ttl", co)
			if errors.Is(err, cache.ErrKeyNotFound) {
				return
			}
		case <-deadline.C:
			t.Fatalf("cache entry did not expire before deadline; last error: %v", err)
		}
	}
}

func TestLocalCache_CloseAndPostCloseErrors(t *testing.T) {
	lc, co := newTestLocalCache(t, true)
	require.NoError(t, lc.Close())
	require.NoError(t, lc.Close())
	ctx := context.Background()
	_, err := lc.Get(ctx, "key", co)
	assert.ErrorIs(t, err, cache.ErrCacheClosed)
	assert.ErrorIs(t, lc.Set(ctx, "key", "value", co), cache.ErrCacheClosed)
	assert.ErrorIs(t, lc.Delete(ctx, "key"), cache.ErrCacheClosed)
	assert.ErrorIs(t, lc.Wait(), cache.ErrCacheClosed)
	assert.Nil(t, lc.GetMetrics(co))
	assert.Nil(t, lc.GetMetricsInfo(co))
}

func TestLocalCache_SetObjectReportsSerializationFailure(t *testing.T) {
	lc, co := newTestLocalCache(t, false)
	err := lc.Set(context.Background(), "bad", make(chan int), co)
	require.Error(t, err)
	var cacheErr *cache.CacheError
	assert.ErrorAs(t, err, &cacheErr)
	assert.Equal(t, "serialize", cacheErr.Op)
}
