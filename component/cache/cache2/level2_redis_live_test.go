//go:build liveintegration

package cache2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/lamxy/fiberhouse/component/cache/cachelocal"
	"github.com/lamxy/fiberhouse/component/cache/cacheremote"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件补齐 Level2Cache 的 live integration 覆盖。
//
// cache2 的既有测试全部是 hermetic 的：子缓存是包内 recordingCache stub，
// 异步写入落在内存里。那些用例能证明排空计数与关闭排序的逻辑自洽，但**不能**
// 证明 L2 组合真实 Redis 时的行为——真实路径上 Set 要经 ants pool 异步提交、
// 跨网络写入 Redis，再由 Wait 保证「写入后立即读取」可见。
//
// 关闭排序尤其只能在 live 下证伪：L2 的 OwnsChildren 向容器声明它负责关闭
// local/remote，finishShutdown 必须先 `<-drain` 排空在途异步写、再关子缓存。
// 若顺序反了，排空期间的写就会打在已关闭的 Redis 客户端上——stub 子缓存永远
// 不会暴露这一点，因为它的 Close 不会让后续写入失败。

// newLiveLevel2 构造一个 local(ristretto) + remote(真实 Redis 容器) 的 L2。
//
// 使用 db 13：db 0 归单元测试、db 14 归 cacheremote 的 live 用例、db 15 预留给
// asynq 批次，彼此不干扰，允许并发 CI 运行。
func newLiveLevel2(t *testing.T) (*Level2Cache, cache.Cache) {
	t.Helper()

	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		// ants 池：容量必须为正，否则 NewLevel2CacheWithError 直接返回错误。
		"cache.asyncPool.ants.local.size":   4,
		"cache.asyncPool.ants.remote.size":  4,
		"cache.asyncPool.ants.local.nonblocking":  false,
		"cache.asyncPool.ants.remote.nonblocking": false,

		// ristretto 本地缓存。
		"cache.local.numCounters":        1000,
		"cache.local.maxCost":            1 << 20,
		"cache.local.bufferItems":        64,
		"cache.local.metrics":            true,
		"cache.local.ignoreInternalCost": true,

		// 真实 Redis 容器。
		"live.redis.host":     "127.0.0.1",
		"live.redis.port":     "6379",
		"live.redis.db":       13,
		"live.redis.poolSize": 5,
	})
	logger := zerolog.Nop()
	appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))

	local, err := cachelocal.NewLocalCache(appCtx, "cache.local")
	require.NoError(t, err, "local cache must be constructible")

	remote, err := cacheremote.NewRedisDb(appCtx, "live.redis")
	require.NoError(t, err, "must be able to construct a client for the live Redis container")

	l2, err := NewLevel2CacheWithError(appCtx, local, remote)
	require.NoError(t, err)

	return l2, remote
}

// liveCacheOption 返回带 JSON 编解码与两级 TTL 的选项。
//
// 必须显式选 AsyncWriteBoth：NewCacheOption 默认的 WriteRemoteOnly 走的是
// **同步** remote.Set（level2_cache.go:285-298），根本不向 ants pool 提交任务。
// 用默认策略写本文件的排空用例会是空测——Wait/Shutdown 无在途任务可等，
// 断言必然通过却什么都没验证。异步分支才会经 asyncSetLocal/asyncSetRemote
// 递增 pending 并 Submit，从而真正检验排空与关闭排序。
func liveCacheOption(t *testing.T, l2 *Level2Cache) *cache.CacheOption {
	t.Helper()
	return cache.NewCacheOption(l2.Ctx).
		SetSyncStrategyAsyncWriteBoth().
		SetJsonWrapper(jsoncodec.StdJsonDefault()).
		SetLocalTTL(time.Minute).
		SetRemoteTTL(time.Minute)
}

// TestLive_Level2Cache_WriteThenReadBackThroughRealRedis 验证 L2 组合真实 Redis 时
// 「写入 → Wait → 读取」语义成立。
//
// 这是 Wait 契约在 live 下的对应物：hermetic 用例只证明 Wait 会等到 pending 归零，
// 本用例证明 pending 归零时数据**确实已经落到 Redis**，而不只是任务计数被清掉。
func TestLive_Level2Cache_WriteThenReadBackThroughRealRedis(t *testing.T) {
	l2, remote := newLiveLevel2(t)
	co := liveCacheOption(t, l2)
	ctx := context.Background()

	key := fmt.Sprintf("l2-live-%s-%d", t.Name(), time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = remote.Delete(cleanupCtx, key)
		_ = l2.Close()
	})

	const value = "l2-live-value"
	require.NoError(t, l2.Set(ctx, key, value, co))

	// Wait 必须覆盖 ants 在途任务；没有它，下面的读取会是竞态。
	require.NoError(t, l2.Wait())

	got, err := l2.Get(ctx, key, co)
	require.NoError(t, err)
	assert.Equal(t, value, got, "value must be readable back through L2 after Wait")

	// 绕过 L2 直接问 Redis：证明异步写确实穿透到了远端，而不是只命中本地层。
	remoteGot, err := remote.Get(ctx, key, co)
	require.NoError(t, err, "the async write must have reached the live Redis")
	assert.Equal(t, value, remoteGot, "remote tier must hold the same value")
}

// TestLive_Level2Cache_ShutdownDrainsInflightWritesBeforeClosingRedis 验证关闭排序：
// 在途异步写必须在子缓存关闭之前排空。
//
// 这是 stub 子缓存无法证伪的那条路径。若 finishShutdown 先关 Redis 再排空，
// 在途写就会打在已关闭的客户端上；此时 Shutdown 会带回 Redis 的错误，或数据
// 根本没写进去——两者都会让本用例失败。
func TestLive_Level2Cache_ShutdownDrainsInflightWritesBeforeClosingRedis(t *testing.T) {
	l2, _ := newLiveLevel2(t)
	co := liveCacheOption(t, l2)
	ctx := context.Background()

	const writes = 16
	keys := make([]string, 0, writes)
	stamp := time.Now().UnixNano()
	for i := 0; i < writes; i++ {
		keys = append(keys, fmt.Sprintf("l2-live-drain-%d-%d", stamp, i))
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// 关闭后的 L2 拒绝操作，因此清理直接走独立的 Redis 客户端。
		cleanup, err := cacheremote.NewRedisDb(l2.Ctx, "live.redis")
		if err == nil {
			_ = cleanup.Delete(cleanupCtx, keys...)
			_ = cleanup.Close()
		}
	})

	for _, key := range keys {
		require.NoError(t, l2.Set(ctx, key, "drained", co))
	}

	// 立刻关闭，不先 Wait：关闭链自身必须负责排空。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, l2.Shutdown(shutdownCtx), "shutdown must drain in-flight writes without erroring")

	// L2 与其子缓存都已关闭，用一个新客户端核对数据确实落盘。
	verifier, err := cacheremote.NewRedisDb(l2.Ctx, "live.redis")
	require.NoError(t, err)
	t.Cleanup(func() { _ = verifier.Close() })

	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer verifyCancel()
	for _, key := range keys {
		got, err := verifier.Get(verifyCtx, key, co)
		require.NoError(t, err, "in-flight write for %s must have landed before Redis was closed", key)
		assert.Equal(t, "drained", got)
	}
}

// TestLive_Level2Cache_OperationsRejectedAfterClose 验证关闭后 L2 拒绝操作，
// 且重复关闭幂等——在持有真实网络连接时同样成立。
func TestLive_Level2Cache_OperationsRejectedAfterClose(t *testing.T) {
	l2, _ := newLiveLevel2(t)
	co := liveCacheOption(t, l2)
	ctx := context.Background()

	require.NoError(t, l2.Close())
	require.NoError(t, l2.Close(), "close must stay idempotent against a real client")

	_, err := l2.Get(ctx, "any", co)
	assert.ErrorIs(t, err, cache.ErrCacheClosed)
	assert.ErrorIs(t, l2.Set(ctx, "any", "v", co), cache.ErrCacheClosed)
	assert.ErrorIs(t, l2.Wait(), cache.ErrCacheClosed)
}
