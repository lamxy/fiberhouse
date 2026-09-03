// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cacheremote

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scriptedRedisHook struct {
	err   error
	value string
}

func (h scriptedRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h scriptedRedisHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		if stringCmd, ok := cmd.(*redis.StringCmd); ok {
			stringCmd.SetVal(h.value)
		}
		return h.err
	}
}

func (h scriptedRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func newScriptedRedisClient(err error, value string) *redis.Client {
	client := redis.NewClient(&redis.Options{Addr: "scripted.invalid:6379"})
	client.AddHook(scriptedRedisHook{err: err, value: value})
	return client
}

func newTestRedisDb(t *testing.T) *RedisDb {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"test.redis.host":     "127.0.0.1",
		"test.redis.port":     "6379",
		"test.redis.db":       0,
		"test.redis.poolSize": 5,
	})
	logger := zerolog.Nop()
	appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	origin, err := NewRedisDb(appCtx, "test.redis")
	require.NoError(t, err)
	return origin.(*RedisDb)
}

func TestRedisDb_CloseAndPostCloseErrors(t *testing.T) {
	rd := newTestRedisDb(t)
	require.NoError(t, rd.Close())
	require.NoError(t, rd.Close())

	co := cache.NewCacheOption(rd.Ctx)
	_, err := rd.Get(context.Background(), "closed", co)
	assert.ErrorIs(t, err, cache.ErrCacheClosed)
	assert.ErrorIs(t, rd.Set(context.Background(), "closed", "value", co), cache.ErrCacheClosed)
	assert.ErrorIs(t, rd.Delete(context.Background(), "closed"), cache.ErrCacheClosed)
	_, err = rd.ReNewClient()
	assert.ErrorIs(t, err, cache.ErrCacheClosed)
	assert.False(t, rd.PingTry(context.Background()))
}

func TestRedisDb_ConcurrentCloseIsIdempotentAndPanicFree(t *testing.T) {
	rd := newTestRedisDb(t)

	const n = 50
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = rd.Close()
		}(i)
	}
	wg.Wait()

	var successCount int
	for _, err := range errs {
		assert.NoError(t, err)
		successCount++
	}
	assert.Equal(t, n, successCount)
}

func TestRedisDb_MissIsCanonicalAndDoesNotOpenBreaker(t *testing.T) {
	rd := newTestRedisDb(t)
	old := rd.Client
	rd.Client = newScriptedRedisClient(redis.Nil, "")
	require.NoError(t, old.Close())
	t.Cleanup(func() { _ = rd.Close() })
	rd.circuitBreaker = cache.NewCircuitBreakerWrap("redis-miss", &gobreaker.Settings{
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.TotalFailures >= 1
		},
	})
	co := cache.NewCacheOption(rd.Ctx).EnableCircuitBreaker()

	for i := 0; i < 5; i++ {
		_, err := rd.Get(context.Background(), "absent", co)
		if !cache.IsMiss(err) {
			t.Fatalf("Get miss %d error = %v, want canonical cache miss", i, err)
		}
	}
	if !rd.circuitBreaker.Allow() {
		t.Fatal("cache misses must not open the circuit breaker")
	}
}

func TestRedisDb_CallerCancellationDoesNotOpenBreaker(t *testing.T) {
	rd := newTestRedisDb(t)
	old := rd.Client
	rd.Client = newScriptedRedisClient(context.Canceled, "")
	require.NoError(t, old.Close())
	t.Cleanup(func() { _ = rd.Close() })
	rd.circuitBreaker = cache.NewCircuitBreakerWrap("redis-cancel", &gobreaker.Settings{
		ReadyToTrip: func(counts gobreaker.Counts) bool { return counts.TotalFailures >= 1 },
	})
	co := cache.NewCacheOption(rd.Ctx).EnableCircuitBreaker()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := rd.Get(ctx, "cancelled", co)
	require.ErrorIs(t, err, context.Canceled)
	if !rd.circuitBreaker.Allow() {
		t.Fatal("caller cancellation must not open the circuit breaker")
	}
}

func TestRedisDb_InfrastructureErrorOpensBreaker(t *testing.T) {
	rd := newTestRedisDb(t)
	old := rd.Client
	infraErr := errors.New("redis transport unavailable")
	rd.Client = newScriptedRedisClient(infraErr, "")
	require.NoError(t, old.Close())
	t.Cleanup(func() { _ = rd.Close() })
	rd.circuitBreaker = cache.NewCircuitBreakerWrap("redis-infra", &gobreaker.Settings{
		ReadyToTrip: func(counts gobreaker.Counts) bool { return counts.TotalFailures >= 1 },
	})
	co := cache.NewCacheOption(rd.Ctx).EnableCircuitBreaker()

	_, err := rd.Get(context.Background(), "unavailable", co)
	require.ErrorIs(t, err, infraErr)
	if rd.circuitBreaker.Allow() {
		t.Fatal("infrastructure failure must open the circuit breaker")
	}
}

func TestRedisDb_RebuildClosesOldClient(t *testing.T) {
	rd := newTestRedisDb(t)
	old := rd.GetRedisClient()
	_, err := rd.ReNewClient()
	require.NoError(t, err)
	assert.NotSame(t, old, rd.GetRedisClient())
	assert.ErrorIs(t, old.Close(), redis.ErrClosed)
	require.NoError(t, rd.Close())
}

func TestRedisDb_ConcurrentLifecycleIsRaceFree(t *testing.T) {
	rd := newTestRedisDb(t)
	co := cache.NewCacheOption(rd.Ctx)
	var wg sync.WaitGroup
	var unexpected atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()
			switch i % 5 {
			case 0:
				_ = rd.GetRedisClient()
			case 1:
				_ = rd.PingTry(ctx)
			case 2:
				_, err := rd.Get(ctx, "key", co)
				if err != nil && !errors.Is(err, cache.ErrCacheClosed) && !errors.Is(err, redis.ErrClosed) && !errors.Is(err, context.DeadlineExceeded) && !isNetworkError(err) {
					unexpected.Add(1)
				}
			case 3:
				_, err := rd.ReNewClient()
				if err != nil && !errors.Is(err, cache.ErrCacheClosed) {
					unexpected.Add(1)
				}
			case 4:
				if err := rd.Close(); err != nil {
					unexpected.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()
	assert.Zero(t, unexpected.Load())
	assert.NoError(t, rd.Close())
}

func isNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}
