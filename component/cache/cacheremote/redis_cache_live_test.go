//go:build liveintegration

package cacheremote

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// newLiveTestRedisDb 构造一个连接真实 Redis 容器（127.0.0.1:6379）的 RedisDb 实例，
// 使用独立的 db 14，避开单元测试使用的 db 0 与预留给 asynq 批次的 db 15。
func newLiveTestRedisDb(t *testing.T) *RedisDb {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"live.redis.host":     "127.0.0.1",
		"live.redis.port":     "6379",
		"live.redis.db":       14, // 独立于单元测试(db 0)与预留给 asynq 批次(db 15)
		"live.redis.poolSize": 5,
	})
	logger := zerolog.Nop()
	appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	origin, err := NewRedisDb(appCtx, "live.redis")
	require.NoError(t, err)
	rd := origin.(*RedisDb)
	t.Cleanup(func() { _ = rd.Close() })
	return rd
}

// TestLive_RedisDb_PingSetGetDeleteClose 针对真实 Redis 容器验证 RedisDb 的
// Ping/Set/Get/Delete/Close 完整读写流程，确保真正发起网络 I/O（而不是像
// newTestRedisDb 那样依赖懒连接从未真正连接）。
func TestLive_RedisDb_PingSetGetDeleteClose(t *testing.T) {
	rd := newLiveTestRedisDb(t)

	// Ping：验证能连通真实容器
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	require.True(t, rd.PingTry(pingCtx), "must be able to reach the live Redis container")

	// 唯一 key，避免并发 CI 运行或重跑相互污染
	key := fmt.Sprintf("p1-4a-live-%s-%d", t.Name(), time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = rd.Delete(cleanupCtx, key)
	})

	co := cache.NewCacheOption(rd.Ctx).SetJsonWrapper(jsoncodec.StdJsonDefault()).SetRemoteTTL(time.Minute)
	value := "p1-4a-live-value"

	// Set：写入一个带唯一后缀的 key
	setCtx, setCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer setCancel()
	require.NoError(t, rd.Set(setCtx, key, value, co))

	// Get：读回验证值相同
	getCtx, getCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer getCancel()
	got, err := rd.Get(getCtx, key, co)
	require.NoError(t, err)
	require.Equal(t, value, got)

	// Delete：删除该 key
	delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer delCancel()
	require.NoError(t, rd.Delete(delCtx, key))

	// 删除后再次 Get：核实实际返回的是 cache.ErrRedisNil（redis_cache.go 的
	// getInternal 在 errors.Is(err, redis.Nil) 时返回 cache.NewErrRedisNil(key)，
	// 不是 cache.ErrKeyNotFound）。
	getAfterDelCtx, getAfterDelCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer getAfterDelCancel()
	_, err = rd.Get(getAfterDelCtx, key, co)
	require.Error(t, err)
	var errRedisNil cache.ErrRedisNil
	require.True(t, errors.As(err, &errRedisNil), "expected cache.ErrRedisNil, got %T: %v", err, err)

	// Close 成功后再次调用任意方法（这里再次 Close），验证返回 cache.ErrCacheClosed
	require.NoError(t, rd.Close())
	closeErr := rd.Close()
	require.ErrorIs(t, closeErr, cache.ErrCacheClosed)
}
