package cache_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/lamxy/fiberhouse/component/cache/cache2"
	"github.com/lamxy/fiberhouse/component/cache/cachelocal"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/rs/zerolog"
)

type benchmarkCache struct {
	mu          sync.RWMutex
	values      map[string]string
	level       cache.Level
	alwaysMiss  bool
	storeWrites bool
}

var (
	benchmarkStringSink string
	benchmarkBoolSink   bool
)

func newBenchmarkCache(level cache.Level) *benchmarkCache {
	return &benchmarkCache{values: make(map[string]string), level: level, storeWrites: true}
}

func (c *benchmarkCache) Get(_ context.Context, key string, _ *cache.CacheOption) (string, error) {
	if c.alwaysMiss {
		return "", cache.ErrKeyNotFound
	}
	c.mu.RLock()
	value, ok := c.values[key]
	c.mu.RUnlock()
	if !ok {
		return "", cache.ErrKeyNotFound
	}
	return value, nil
}

func (c *benchmarkCache) Set(_ context.Context, key string, value interface{}, _ *cache.CacheOption) error {
	if !c.storeWrites {
		return nil
	}
	serialized, ok := value.(string)
	if !ok {
		return fmt.Errorf("benchmark cache received %T, want string", value)
	}
	c.mu.Lock()
	c.values[key] = serialized
	c.mu.Unlock()
	return nil
}

func (c *benchmarkCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	for _, key := range keys {
		delete(c.values, key)
	}
	c.mu.Unlock()
	return nil
}

func (*benchmarkCache) Close() error            { return nil }
func (*benchmarkCache) Wait() error             { return nil }
func (c *benchmarkCache) GetLevel() cache.Level { return c.level }

func benchmarkAppContext() fiberhouse.IContext {
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"benchmark.local.numCounters":                  int64(10_000),
		"benchmark.local.maxCost":                      int64(8 << 20),
		"benchmark.local.bufferItems":                  int64(64),
		"benchmark.local.metrics":                      false,
		"benchmark.local.ignoreInternalCost":           true,
		"cache.asyncPool.ants.local.size":              2,
		"cache.asyncPool.ants.local.expiryDuration":    1,
		"cache.asyncPool.ants.local.maxBlockingTasks":  8,
		"cache.asyncPool.ants.remote.size":             2,
		"cache.asyncPool.ants.remote.expiryDuration":   1,
		"cache.asyncPool.ants.remote.maxBlockingTasks": 8,
	})
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

func benchmarkOption(appCtx fiberhouse.IContext, key string, level cache.Level) *cache.CacheOption {
	return cache.NewCacheOption(appCtx).
		SetContextCtx(context.Background()).
		SetCacheKey(key).
		SetCacheLevel(level).
		SetJsonWrapper(jsoncodec.StdJsonDefault()).
		SetLocalTTL(time.Minute).
		SetRemoteTTL(time.Minute)
}

func BenchmarkCoordinatorHit(b *testing.B) {
	appCtx := benchmarkAppContext()
	backend := newBenchmarkCache(cache.Local)
	backend.values["hit"] = `"value"`
	coordinator, err := cache.NewCoordinator(backend)
	if err != nil {
		b.Fatal(err)
	}
	request := benchmarkOption(appCtx, "hit", cache.Local)
	loader := func(context.Context) (string, error) { return "unexpected", nil }

	b.ReportAllocs()
	b.ResetTimer()
	var value string
	var runErr error
	for i := 0; i < b.N; i++ {
		value, runErr = cache.GetOrLoad(coordinator, request, loader)
	}
	b.StopTimer()
	benchmarkStringSink = value
	if runErr != nil || value != "value" {
		b.Fatalf("GetOrLoad() = %q, %v", value, runErr)
	}
}

func BenchmarkCoordinatorMiss(b *testing.B) {
	appCtx := benchmarkAppContext()
	backend := newBenchmarkCache(cache.Local)
	backend.alwaysMiss = true
	backend.storeWrites = false
	coordinator, err := cache.NewCoordinator(backend)
	if err != nil {
		b.Fatal(err)
	}
	request := benchmarkOption(appCtx, "miss", cache.Local)
	loader := func(context.Context) (string, error) { return "value", nil }

	b.ReportAllocs()
	b.ResetTimer()
	var value string
	var runErr error
	for i := 0; i < b.N; i++ {
		value, runErr = cache.GetOrLoad(coordinator, request, loader)
	}
	b.StopTimer()
	benchmarkStringSink = value
	if runErr != nil || value != "value" {
		b.Fatalf("GetOrLoad() = %q, %v", value, runErr)
	}
}

func BenchmarkCoordinatorContendedMiss(b *testing.B) {
	appCtx := benchmarkAppContext()
	backend := newBenchmarkCache(cache.Local)
	backend.alwaysMiss = true
	backend.storeWrites = false
	coordinator, err := cache.NewCoordinator(backend)
	if err != nil {
		b.Fatal(err)
	}
	request := benchmarkOption(appCtx, "contended", cache.Local)
	var loads atomic.Int64
	loader := func(context.Context) (string, error) {
		loads.Add(1)
		time.Sleep(50 * time.Microsecond)
		return "value", nil
	}

	b.ReportAllocs()
	b.SetParallelism(4)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := cache.GetOrLoad(coordinator, request, loader); err != nil {
				b.Error(err)
			}
		}
	})
	b.StopTimer()
	loadCount := loads.Load()
	if b.N > 1 && loadCount >= int64(b.N) {
		b.Fatalf("loader calls = %d for %d operations; benchmark did not exercise coalescing", loadCount, b.N)
	}
	b.ReportMetric(float64(loadCount)/float64(b.N), "loader/op")
}

func BenchmarkMemoryBloomMayContain(b *testing.B) {
	filter, err := cache.NewMemoryBloomFilter(10_000, 0.01, 16)
	if err != nil {
		b.Fatal(err)
	}
	if err := filter.Preload(context.Background(), []string{"present"}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	var present bool
	var runErr error
	for i := 0; i < b.N; i++ {
		present, runErr = filter.MayContain(context.Background(), "present")
	}
	b.StopTimer()
	benchmarkBoolSink = present
	if runErr != nil || !present {
		b.Fatalf("MayContain() = %v, %v", present, runErr)
	}
}

func BenchmarkLocalCacheHit(b *testing.B) {
	appCtx := benchmarkAppContext()
	backend, err := cachelocal.NewLocalCache(appCtx, "benchmark.local")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = backend.Close() })
	request := benchmarkOption(appCtx, "hit", cache.Local)
	if err := backend.Set(context.Background(), "hit", "value", request); err != nil {
		b.Fatal(err)
	}
	if err := backend.Wait(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	var value string
	var runErr error
	for i := 0; i < b.N; i++ {
		value, runErr = backend.Get(context.Background(), "hit", request)
	}
	b.StopTimer()
	benchmarkStringSink = value
	if runErr != nil || value != "value" {
		b.Fatalf("Get() = %q, %v", value, runErr)
	}
}

func BenchmarkLevel2LocalHit(b *testing.B) {
	appCtx := benchmarkAppContext()
	local := newBenchmarkCache(cache.Local)
	local.values["hit"] = "value"
	remote := newBenchmarkCache(cache.Remote)
	backend, err := cache2.NewLevel2CacheWithError(appCtx, local, remote)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = backend.Shutdown(context.Background()) })
	request := benchmarkOption(appCtx, "hit", cache.Level2)

	b.ReportAllocs()
	b.ResetTimer()
	var value string
	var runErr error
	for i := 0; i < b.N; i++ {
		value, runErr = backend.Get(context.Background(), "hit", request)
	}
	b.StopTimer()
	benchmarkStringSink = value
	if runErr != nil || value != "value" {
		b.Fatalf("Get() = %q, %v", value, runErr)
	}
}

func BenchmarkLevel2RemoteHitBackfill(b *testing.B) {
	appCtx := benchmarkAppContext()
	local := newBenchmarkCache(cache.Local)
	local.alwaysMiss = true
	local.storeWrites = false
	remote := newBenchmarkCache(cache.Remote)
	remote.values["remote"] = "value"
	backend, err := cache2.NewLevel2CacheWithError(appCtx, local, remote)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = backend.Shutdown(context.Background()) })
	request := benchmarkOption(appCtx, "remote", cache.Level2)

	b.ReportAllocs()
	b.ResetTimer()
	var value string
	var runErr error
	for i := 0; i < b.N; i++ {
		value, runErr = backend.Get(context.Background(), "remote", request)
	}
	b.StopTimer()
	benchmarkStringSink = value
	if runErr != nil || value != "value" {
		b.Fatalf("Get() = %q, %v", value, runErr)
	}
}

func BenchmarkCacheOptionPool(b *testing.B) {
	appCtx := benchmarkAppContext()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		option := cache.OptionPoolGet(appCtx)
		cache.OptionPoolPut(option)
	}
}
