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

// ErrRedisBloomUnavailable indicates that the target Redis server has no
// RedisBloom module installed.
var ErrRedisBloomUnavailable = errors.New("redis bloom module is unavailable")

type commandExecutor interface {
	Do(context.Context, ...interface{}) *redis.Cmd
}

// RedisBloomConfig defines one namespaced RedisBloom filter.
type RedisBloomConfig struct {
	Key           string
	Capacity      uint
	ErrorRate     float64
	BatchSize     int
	AllowExisting bool
}

// RedisBloomFilter is an optional remote ExistenceFilter backed by RedisBloom.
// It remains permissive until Preload completes in full.
type RedisBloomFilter struct {
	executor commandExecutor
	config   RedisBloomConfig

	mu        sync.RWMutex
	preloadMu sync.Mutex
	ready     bool
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

// Prepare reserves the remote filter. A BUSYKEY response is accepted only when
// explicitly configured, since an accidental key collision must remain visible.
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

// Preload fills the remote filter in bounded BF.MADD batches. Readiness changes
// only after every batch succeeds, so callers never reject from partial state.
func (f *RedisBloomFilter) Preload(ctx context.Context, keys []string) error {
	f.preloadMu.Lock()
	defer f.preloadMu.Unlock()

	f.setReady(false)
	for start := 0; start < len(keys); start += f.config.BatchSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := start + f.config.BatchSize
		if end > len(keys) {
			end = len(keys)
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

// MayContain is permissive until a complete preload establishes authority.
func (f *RedisBloomFilter) MayContain(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	if !f.Ready() {
		return true, nil
	}
	result, err := f.executor.Do(ctx, "BF.EXISTS", f.config.Key, key).Int64()
	if err != nil {
		return true, normalizeRedisBloomError(err)
	}
	return result != 0, nil
}

// Add records a key but never makes the filter authoritative.
func (f *RedisBloomFilter) Add(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return normalizeRedisBloomError(f.executor.Do(ctx, "BF.ADD", f.config.Key, key).Err())
}

// Reset removes local authority; it deliberately does not delete a shared
// remote Redis key.
func (f *RedisBloomFilter) Reset() {
	f.preloadMu.Lock()
	defer f.preloadMu.Unlock()
	f.setReady(false)
}

func (f *RedisBloomFilter) setReady(ready bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ready = ready
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
