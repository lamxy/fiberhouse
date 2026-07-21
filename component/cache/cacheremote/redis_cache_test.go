// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cacheremote

import (
	"sync"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	err := rd.Close()
	assert.ErrorIs(t, err, cache.ErrCacheClosed)
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

	var successCount, closedErrCount int
	for _, err := range errs {
		if err == nil {
			successCount++
		} else {
			assert.ErrorIs(t, err, cache.ErrCacheClosed)
			closedErrCount++
		}
	}
	assert.Equal(t, 1, successCount, "exactly one concurrent Close() call must win and return nil")
	assert.Equal(t, n-1, closedErrCount)
}
