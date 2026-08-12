package cache2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

type pausedGetCache struct {
	*recordingCache
	readOldValue chan struct{}
	releaseGet   <-chan struct{}
	readOnce     sync.Once
}

type gatedSetCache struct {
	*recordingCache
	started  chan struct{}
	release  <-chan struct{}
	finished atomic.Int32
}

func (c *gatedSetCache) Set(ctx context.Context, key string, value interface{}, co *cache.CacheOption) error {
	c.started <- struct{}{}
	select {
	case <-c.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := c.recordingCache.Set(ctx, key, value, co); err != nil {
		return err
	}
	c.finished.Add(1)
	return nil
}

func (c *pausedGetCache) Get(ctx context.Context, key string, co *cache.CacheOption) (string, error) {
	value, err := c.recordingCache.Get(ctx, key, co)
	c.readOnce.Do(func() { close(c.readOldValue) })
	<-c.releaseGet
	return value, err
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
	l2 := &Level2Cache{
		Ctx: ctx, local: local, remote: remote, level: cache.Level2,
		localPool: localPool, remotePool: remotePool,
	}
	l2.initLifecycle(true)
	return l2
}

func newLevel2Context(values map[string]interface{}) fiberhouse.IContext {
	cfg := appconfig.NewAppConfig().LoadDefault(values)
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
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

func TestLevel2Wait_DrainsQueuedAsyncWorkBeforeWaitingChildren(t *testing.T) {
	release := make(chan struct{})
	remote := &gatedSetCache{
		recordingCache: newRecordingCache(cache.Remote),
		started:        make(chan struct{}, 2),
		release:        release,
	}
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), remote)
	l2.remotePool.Release()
	pool, err := ants.NewPool(1, ants.WithMaxBlockingTasks(2))
	require.NoError(t, err)
	l2.remotePool = pool
	co := cache.NewCacheOption(nil).SetSyncStrategyAsyncWriteRemoteOnly()
	require.NoError(t, l2.Set(context.Background(), "running", "one", co))
	<-remote.started
	secondReturned := make(chan error, 1)
	go func() { secondReturned <- l2.Set(context.Background(), "queued", "two", co) }()
	require.Eventually(t, func() bool {
		l2.lifecycleMu.Lock()
		defer l2.lifecycleMu.Unlock()
		return l2.pending == 2
	}, time.Second, time.Millisecond)

	waited := make(chan error, 1)
	go func() { waited <- l2.Wait() }()
	select {
	case err := <-waited:
		t.Fatalf("Wait returned before async work drained: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-secondReturned)
	require.NoError(t, <-waited)
	assert.Equal(t, int32(2), remote.finished.Load())
	require.NoError(t, l2.Shutdown(context.Background()))
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

func TestLevel2Cache_RemoteFirstSetFailureLeavesLocalUntouched(t *testing.T) {
	remoteErr := errors.New("remote set failed")
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	local.values["key"] = "old"
	remote.setErr = remoteErr
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })

	err := l2.Set(context.Background(), "key", "new", cache.NewCacheOption(nil).SetSyncStrategyWriteBoth())

	require.ErrorIs(t, err, remoteErr)
	assert.Equal(t, "old", local.values["key"])
}

func TestLevel2Cache_RemoteFirstWriteRemoteOnlyInvalidatesLocalAfterRemoteSuccess(t *testing.T) {
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	local.values["key"] = "old"
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })

	require.NoError(t, l2.Set(context.Background(), "key", "new", cache.NewCacheOption(nil).SetSyncStrategyWriteRemoteOnly()))

	assert.Equal(t, "new", remote.values["key"])
	assert.NotContains(t, local.values, "key")
}

func TestLevel2Cache_RemoteFirstDeleteFailureLeavesLocalUntouched(t *testing.T) {
	remoteErr := errors.New("remote delete failed")
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	local.values["key"] = "old"
	remote.deleteErr = remoteErr
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })

	err := l2.Delete(context.Background(), "key")

	require.ErrorIs(t, err, remoteErr)
	assert.Equal(t, "old", local.values["key"])
}

func TestLevel2Cache_NoStaleBackfillAfterSet(t *testing.T) {
	releaseGet := make(chan struct{})
	local := newRecordingCache(cache.Local)
	remote := &pausedGetCache{
		recordingCache: newRecordingCache(cache.Remote),
		readOldValue:   make(chan struct{}),
		releaseGet:     releaseGet,
	}
	remote.values["key"] = "old"
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })
	co := cache.NewCacheOption(nil).SetSyncStrategyWriteBoth()

	getResult := make(chan string, 1)
	getErr := make(chan error, 1)
	go func() {
		value, err := l2.Get(context.Background(), "key", co)
		getResult <- value
		getErr <- err
	}()
	<-remote.readOldValue
	require.NoError(t, l2.Set(context.Background(), "key", "new", co))
	close(releaseGet)

	require.NoError(t, <-getErr)
	assert.Equal(t, "old", <-getResult)
	assert.Equal(t, "new", local.values["key"])
}

func TestLevel2Cache_NoStaleBackfillAfterDelete(t *testing.T) {
	releaseGet := make(chan struct{})
	local := newRecordingCache(cache.Local)
	remote := &pausedGetCache{
		recordingCache: newRecordingCache(cache.Remote),
		readOldValue:   make(chan struct{}),
		releaseGet:     releaseGet,
	}
	remote.values["key"] = "old"
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })
	co := cache.NewCacheOption(nil).SetSyncStrategyWriteBoth()

	getResult := make(chan string, 1)
	getErr := make(chan error, 1)
	go func() {
		value, err := l2.Get(context.Background(), "key", co)
		getResult <- value
		getErr <- err
	}()
	<-remote.readOldValue
	require.NoError(t, l2.Delete(context.Background(), "key"))
	close(releaseGet)

	require.NoError(t, <-getErr)
	assert.Equal(t, "old", <-getResult)
	assert.NotContains(t, local.values, "key")
}

func TestLevel2Cache_UnrelatedSetDoesNotSuppressBackfill(t *testing.T) {
	releaseGet := make(chan struct{})
	local := newRecordingCache(cache.Local)
	remote := &pausedGetCache{
		recordingCache: newRecordingCache(cache.Remote),
		readOldValue:   make(chan struct{}),
		releaseGet:     releaseGet,
	}
	remote.values["a"] = "a-old"
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })
	co := cache.NewCacheOption(nil).SetSyncStrategyWriteBoth()

	getResult := make(chan string, 1)
	getErr := make(chan error, 1)
	go func() {
		value, err := l2.Get(context.Background(), "a", co)
		getResult <- value
		getErr <- err
	}()
	<-remote.readOldValue
	require.NoError(t, l2.Set(context.Background(), "b", "b-new", co))
	close(releaseGet)

	require.NoError(t, <-getErr)
	assert.Equal(t, "a-old", <-getResult)
	assert.Equal(t, "a-old", local.values["a"])
}

func TestLevel2Cache_UnrelatedDeleteDoesNotSuppressBackfill(t *testing.T) {
	releaseGet := make(chan struct{})
	local := newRecordingCache(cache.Local)
	remote := &pausedGetCache{
		recordingCache: newRecordingCache(cache.Remote),
		readOldValue:   make(chan struct{}),
		releaseGet:     releaseGet,
	}
	remote.values["a"] = "a-old"
	remote.values["b"] = "b-old"
	local.values["b"] = "b-old"
	l2 := newTestLevel2(t, local, remote)
	t.Cleanup(func() { _ = l2.Close() })
	co := cache.NewCacheOption(nil).SetSyncStrategyWriteBoth()

	getResult := make(chan string, 1)
	getErr := make(chan error, 1)
	go func() {
		value, err := l2.Get(context.Background(), "a", co)
		getResult <- value
		getErr <- err
	}()
	<-remote.readOldValue
	require.NoError(t, l2.Delete(context.Background(), "b"))
	close(releaseGet)

	require.NoError(t, <-getErr)
	assert.Equal(t, "a-old", <-getResult)
	assert.Equal(t, "a-old", local.values["a"])
}

func TestLevel2Cache_FlushWaitsForQueuedAndRunningWork(t *testing.T) {
	release := make(chan struct{})
	remote := &gatedSetCache{
		recordingCache: newRecordingCache(cache.Remote),
		started:        make(chan struct{}, 2),
		release:        release,
	}
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), remote)
	l2.remotePool.Release()
	pool, err := ants.NewPool(1, ants.WithMaxBlockingTasks(2))
	require.NoError(t, err)
	l2.remotePool = pool
	co := cache.NewCacheOption(nil).SetSyncStrategyAsyncWriteRemoteOnly()

	require.NoError(t, l2.Set(context.Background(), "running", "one", co))
	<-remote.started
	secondReturned := make(chan error, 1)
	go func() { secondReturned <- l2.Set(context.Background(), "queued", "two", co) }()
	require.Eventually(t, func() bool {
		l2.lifecycleMu.Lock()
		defer l2.lifecycleMu.Unlock()
		return l2.pending == 2
	}, time.Second, time.Millisecond)

	flushed := make(chan error, 1)
	go func() { flushed <- l2.Flush(context.Background()) }()
	select {
	case err := <-flushed:
		t.Fatalf("Flush returned before queued and running work completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-secondReturned)
	require.NoError(t, <-flushed)
	assert.Equal(t, int32(2), remote.finished.Load())
	require.NoError(t, l2.Shutdown(context.Background()))
}

func TestLevel2Cache_FlushDeadlineDoesNotCancelPendingWork(t *testing.T) {
	release := make(chan struct{})
	remote := &gatedSetCache{
		recordingCache: newRecordingCache(cache.Remote),
		started:        make(chan struct{}, 1),
		release:        release,
	}
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), remote)
	co := cache.NewCacheOption(nil).SetSyncStrategyAsyncWriteRemoteOnly()
	require.NoError(t, l2.Set(context.Background(), "key", "value", co))
	<-remote.started

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, l2.Flush(ctx), context.DeadlineExceeded)
	close(release)
	require.NoError(t, l2.Flush(context.Background()))
	assert.Equal(t, int32(1), remote.finished.Load())
	require.NoError(t, l2.Shutdown(context.Background()))
}

func TestLevel2Cache_ShutdownStopsAdmissionWaitsAndClosesOwnedChildrenOnce(t *testing.T) {
	release := make(chan struct{})
	local := newRecordingCache(cache.Local)
	remote := &gatedSetCache{
		recordingCache: newRecordingCache(cache.Remote),
		started:        make(chan struct{}, 1),
		release:        release,
	}
	l2 := newTestLevel2(t, local, remote)
	co := cache.NewCacheOption(nil).SetSyncStrategyAsyncWriteRemoteOnly()
	require.NoError(t, l2.Set(context.Background(), "running", "value", co))
	<-remote.started

	const callers = 8
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() { errs <- l2.Shutdown(context.Background()) }()
	}
	require.Eventually(t, func() bool { return l2.closed.Load() }, time.Second, time.Millisecond)
	require.ErrorIs(t, l2.Set(context.Background(), "late", "value", co), cache.ErrCacheClosed)
	assert.Equal(t, 0, local.closeCalls, "Shutdown must not close children while a task is running")
	assert.Equal(t, 0, remote.closeCalls, "Shutdown must not close children while a task is running")

	close(release)
	for i := 0; i < callers; i++ {
		require.NoError(t, <-errs)
	}
	assert.Equal(t, 1, local.closeCalls)
	assert.Equal(t, 1, remote.closeCalls)
	require.NoError(t, l2.Shutdown(context.Background()))
	assert.Equal(t, 1, local.closeCalls)
	assert.Equal(t, 1, remote.closeCalls)
}

func TestLevel2Cache_SubmitAndFlushRaceDoesNotLoseAdmittedWork(t *testing.T) {
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	l2 := newTestLevel2(t, local, remote)
	co := cache.NewCacheOption(nil).SetSyncStrategyAsyncWriteRemoteOnly()

	const tasks = 100
	var submitted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < tasks; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if err := l2.Set(context.Background(), fmt.Sprintf("key-%d", i), "value", co); err == nil {
				submitted.Add(1)
			}
		}(i)
		go func() {
			defer wg.Done()
			_ = l2.Flush(context.Background())
		}()
	}
	wg.Wait()
	require.NoError(t, l2.Flush(context.Background()))
	remote.mu.Lock()
	executed := len(remote.values)
	remote.mu.Unlock()
	assert.Equal(t, int(submitted.Load()), executed)
	require.NoError(t, l2.Shutdown(context.Background()))
}

func TestLevel2Cache_NewLevel2CacheWithErrorRejectsInvalidPoolConfig(t *testing.T) {
	ctx := newLevel2Context(map[string]interface{}{
		"cache.asyncPool.ants.local.size":           1,
		"cache.asyncPool.ants.local.expiryDuration": -1,
		"cache.asyncPool.ants.remote.size":          1,
	})
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)

	l2, err := NewLevel2CacheWithError(ctx, local, remote)
	require.Error(t, err)
	assert.Nil(t, l2)
	assert.Nil(t, NewLevel2Cache(ctx, local, remote))
}

func TestLevel2Cache_NewConstructorsOwnChildren(t *testing.T) {
	ctx := newLevel2Context(map[string]interface{}{
		"cache.asyncPool.ants.local.size":  1,
		"cache.asyncPool.ants.remote.size": 1,
	})
	local, remote := newRecordingCache(cache.Local), newRecordingCache(cache.Remote)
	l2, err := NewLevel2CacheWithError(ctx, local, remote)
	require.NoError(t, err)
	require.NoError(t, l2.Shutdown(context.Background()))
	assert.Equal(t, 1, local.closeCalls)
	assert.Equal(t, 1, remote.closeCalls)
}
