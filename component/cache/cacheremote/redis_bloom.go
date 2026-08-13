package cacheremote

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/redis/go-redis/v9"
)

// ErrRedisBloomUnavailable 表示目标 Redis 服务未安装 RedisBloom 模块。
var ErrRedisBloomUnavailable = errors.New("redis bloom module is unavailable")

type commandExecutor interface {
	Do(context.Context, ...interface{}) *redis.Cmd
}

// RedisBloomConfig 定义一个带命名空间的 RedisBloom 过滤器。
type RedisBloomConfig struct {
	Key           string
	Capacity      uint
	ErrorRate     float64
	BatchSize     int
	AllowExisting bool
}

// RedisBloomFilter 是由 RedisBloom 支撑的可选远程存在性过滤器。
// Preload 完整成功前始终保持放行。
type RedisBloomFilter struct {
	executor commandExecutor
	config   RedisBloomConfig

	mu         sync.RWMutex
	preloadMu  sync.Mutex
	ready      bool
	generation uint64
}

var _ cache.ExistenceFilter = (*RedisBloomFilter)(nil)
var _ cache.PreloadableFilter = (*RedisBloomFilter)(nil)

func NewRedisBloomFilter(executor commandExecutor, config RedisBloomConfig) (*RedisBloomFilter, error) {
	if executor == nil {
		return nil, errors.New("redis bloom executor must not be nil")
	}
	if strings.TrimSpace(config.Key) == "" {
		return nil, errors.New("redis bloom filter key must not be empty")
	}
	if config.Capacity == 0 {
		return nil, errors.New("redis bloom filter capacity must be positive")
	}
	if config.ErrorRate <= 0 || config.ErrorRate >= 1 || math.IsNaN(config.ErrorRate) || math.IsInf(config.ErrorRate, 0) {
		return nil, errors.New("redis bloom filter error rate must be between zero and one")
	}
	if config.BatchSize < 0 {
		return nil, errors.New("redis bloom filter batch size must not be negative")
	}
	if config.BatchSize == 0 {
		config.BatchSize = 1000
	}
	return &RedisBloomFilter{executor: executor, config: config}, nil
}

// Prepare 创建远程过滤器。只有显式配置后才接受 BUSYKEY 响应，避免隐藏意外的键冲突。
func (f *RedisBloomFilter) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := f.executor.Do(ctx, "BF.RESERVE", f.config.Key, f.config.ErrorRate, f.config.Capacity)
	if err := normalizeRedisBloomError(cmd.Err()); err != nil {
		if f.config.AllowExisting && isBusyKey(err) {
			return nil
		}
		return err
	}
	return nil
}

// Preload 使用有界的 BF.MADD 批次填充远程过滤器。全部批次成功后才进入就绪状态，
// 防止调用方根据部分数据拒绝有效键。
func (f *RedisBloomFilter) Preload(ctx context.Context, keys []string) error {
	f.preloadMu.Lock()
	defer f.preloadMu.Unlock()

	f.setReady(false)
	for start := 0; start < len(keys); start += f.config.BatchSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		remaining := len(keys) - start
		end := len(keys)
		if f.config.BatchSize < remaining {
			end = start + f.config.BatchSize
		}
		args := make([]interface{}, 0, end-start+2)
		args = append(args, "BF.MADD", f.config.Key)
		for _, key := range keys[start:end] {
			args = append(args, key)
		}
		if err := normalizeRedisBloomError(f.executor.Do(ctx, args...).Err()); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f.setReady(true)
	return nil
}

func (f *RedisBloomFilter) Ready() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ready
}

// MayContain 在完整预加载建立权威状态前始终放行。
func (f *RedisBloomFilter) MayContain(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	f.mu.RLock()
	ready, generation := f.ready, f.generation
	f.mu.RUnlock()
	if !ready {
		return true, nil
	}
	result, err := f.executor.Do(ctx, "BF.EXISTS", f.config.Key, key).Int64()
	if err != nil {
		return true, normalizeRedisBloomError(err)
	}
	f.mu.RLock()
	stillAuthoritative := f.ready && f.generation == generation
	f.mu.RUnlock()
	if !stillAuthoritative {
		return true, nil
	}
	return result != 0, nil
}

// Add 记录单个键，但不会让过滤器进入权威状态。
func (f *RedisBloomFilter) Add(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return normalizeRedisBloomError(f.executor.Do(ctx, "BF.ADD", f.config.Key, key).Err())
}

// Reset 仅撤销本地权威状态，不会删除共享的远程 Redis 键。
func (f *RedisBloomFilter) Reset() {
	f.preloadMu.Lock()
	defer f.preloadMu.Unlock()
	f.setReady(false)
}

func (f *RedisBloomFilter) setReady(ready bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ready = ready
	f.generation++
}

func normalizeRedisBloomError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "unknown command") {
		return fmt.Errorf("%w: %v", ErrRedisBloomUnavailable, err)
	}
	return err
}

func isBusyKey(err error) bool {
	return strings.Contains(strings.ToUpper(err.Error()), "BUSYKEY")
}
