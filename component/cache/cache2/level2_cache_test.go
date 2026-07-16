package cache2

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingCache struct {
	mu         sync.Mutex
	values     map[string]string
	getErr     error
	setErr     error
	deleteErr  error
	waitErr    error
	closeErr   error
	closeCalls int
	waitCalls  int
	level      cache.Level
}

func newRecordingCache(level cache.Level) *recordingCache {
	return &recordingCache{values: make(map[string]string), level: level}
}

func (c *recordingCache) Get(_ context.Context, key string, _ *cache.CacheOption) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.getErr != nil {
		return "", c.getErr
	}
	v, ok := c.values[key]
	if !ok {
		return "", cache.ErrKeyNotFound
	}
	return v, nil
}

func (c *recordingCache) Set(_ context.Context, key string, value interface{}, _ *cache.CacheOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.setErr != nil {
		return c.setErr
	}
	c.values[key] = value.(string)
	return nil
}

func (c *recordingCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deleteErr != nil {
		return c.deleteErr
	}
	for _, key := range keys {
		delete(c.values, key)
	}
	return nil
}

func (c *recordingCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeCalls++
	return c.closeErr
}

func (c *recordingCache) Wait() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.waitCalls++
	return c.waitErr
}

func (c *recordingCache) GetLevel() cache.Level { return c.level }

func newTestLevel2(t *testing.T, local, remote cache.Cache) *Level2Cache {
	t.Helper()
	localPool, err := ants.NewPool(2)
	require.NoError(t, err)
	remotePool, err := ants.NewPool(2)
	require.NoError(t, err)
	logger := zerolog.Nop()
	ctx := fiberhouse.NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
	return &Level2Cache{
		Ctx: ctx, local: local, remote: remote, level: cache.Level2,
		localPool: localPool, remotePool: remotePool, stopCh: make(chan struct{}),
	}
}

func TestLevel2Close_IsIdempotentAndClosesChildrenOnce(t *testing.T) {
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	l2 := newTestLevel2(t, local, remote)

	require.NoError(t, l2.Close())
	require.NoError(t, l2.Close())
	assert.Equal(t, 1, local.closeCalls)
	assert.Equal(t, 1, remote.closeCalls)
}

func TestLevel2Close_MarksClosed(t *testing.T) {
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), newRecordingCache(cache.Remote))
	require.NoError(t, l2.Close())
	assert.True(t, l2.closed.Load())
}

func TestLevel2OperationsAfterClose_ReturnErrCacheClosed(t *testing.T) {
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), newRecordingCache(cache.Remote))
	require.NoError(t, l2.Close())
	co := cache.NewCacheOption(nil)
	ctx := context.Background()
	_, err := l2.Get(ctx, "key", co)
	assert.ErrorIs(t, err, cache.ErrCacheClosed)
	assert.ErrorIs(t, l2.Set(ctx, "key", "value", co), cache.ErrCacheClosed)
	assert.ErrorIs(t, l2.Delete(ctx, "key"), cache.ErrCacheClosed)
	assert.ErrorIs(t, l2.Wait(), cache.ErrCacheClosed)
}

func TestLevel2Wait_PropagatesChildErrors(t *testing.T) {
	localErr, remoteErr := errors.New("local wait"), errors.New("remote wait")
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	local.waitErr, remote.waitErr = localErr, remoteErr
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })

	err := l2.Wait()
	assert.ErrorIs(t, err, localErr)
	assert.ErrorIs(t, err, remoteErr)
}

func TestLevel2Close_AggregatesChildErrors(t *testing.T) {
	localErr, remoteErr := errors.New("local close"), errors.New("remote close")
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	local.closeErr, remote.closeErr = localErr, remoteErr
	l2 := newTestLevel2(t, local, remote)

	err := l2.Close()
	assert.ErrorIs(t, err, localErr)
	assert.ErrorIs(t, err, remoteErr)
	assert.True(t, l2.closed.Load())
	require.NoError(t, l2.Close())
	assert.Equal(t, 1, local.closeCalls)
	assert.Equal(t, 1, remote.closeCalls)
}

func TestLevel2ReadThroughAndSynchronousOperations(t *testing.T) {
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	remote.values["remote"] = "value"
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })
	co := cache.NewCacheOption(nil).SetSyncStrategyWriteBoth()
	ctx := context.Background()

	got, err := l2.Get(ctx, "remote", co)
	require.NoError(t, err)
	assert.Equal(t, "value", got)
	assert.Equal(t, "value", local.values["remote"])

	require.NoError(t, l2.Set(ctx, "both", "written", co))
	assert.Equal(t, "written", local.values["both"])
	assert.Equal(t, "written", remote.values["both"])
	require.NoError(t, l2.Delete(ctx, "both"))
	assert.NotContains(t, local.values, "both")
	assert.NotContains(t, remote.values, "both")
	assert.Equal(t, cache.Level2, l2.GetLevel())
	assert.Same(t, local, l2.GetLocal())
	assert.Same(t, remote, l2.GetRemote())
	assert.NotEmpty(t, l2.GetPoolMetrics())
}
