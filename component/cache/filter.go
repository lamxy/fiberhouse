package cache

import (
	"context"
	"errors"
	"math"
	"sync"
)

// ExistenceFilter decides whether a cache key may exist. Implementations must
// return true while they are not authoritative, so callers never reject a key
// from incomplete filter state.
type ExistenceFilter interface {
	Ready() bool
	MayContain(context.Context, string) (bool, error)
	Add(context.Context, string) error
}

// PreloadableFilter can atomically replace its authoritative key set.
type PreloadableFilter interface {
	ExistenceFilter
	Preload(context.Context, []string) error
	Reset()
}

// MemoryBloomFilter is an in-process authoritative Bloom filter. It is
// unready until a complete preload succeeds.
type MemoryBloomFilter struct {
	mu          sync.RWMutex
	preloadMu   sync.Mutex
	capacity    uint
	errorRate   float64
	shards      []shard
	mask        uint32
	ready       bool
	preloadLive bool
	pendingAdds []string
}

const maxMemoryBloomCapacity uint = 1 << 26

// NewMemoryBloomFilter constructs an unready filter. Shard counts are rounded
// up to a power of two, with zero selecting one shard.
func NewMemoryBloomFilter(capacity uint, errorRate float64, shardCount int) (*MemoryBloomFilter, error) {
	if capacity == 0 {
		return nil, errors.New("memory bloom filter capacity must be positive")
	}
	if capacity > maxMemoryBloomCapacity {
		return nil, errors.New("memory bloom filter capacity is too large")
	}
	if errorRate <= 0 || errorRate >= 1 || math.IsNaN(errorRate) {
		return nil, errors.New("memory bloom filter error rate must be between zero and one")
	}
	shardCount, ok := normalizeShardCount(shardCount)
	if !ok {
		return nil, errors.New("memory bloom filter shard count is too large")
	}
	return newMemoryBloomFilter(capacity, errorRate, shardCount), nil
}

func newMemoryBloomFilter(capacity uint, errorRate float64, shardCount int) *MemoryBloomFilter {
	shards := uint(shardCount)
	perShard := capacity / shards
	if capacity%shards != 0 {
		perShard++
	}
	filter := &MemoryBloomFilter{
		capacity:  capacity,
		errorRate: errorRate,
		shards:    make([]shard, shardCount),
		mask:      uint32(shardCount - 1),
	}
	for i := range filter.shards {
		filter.shards[i].bf = newBloomFilter(perShard, errorRate)
	}
	return filter
}

// Ready reports whether the filter contains a complete preload.
func (f *MemoryBloomFilter) Ready() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ready
}

// MayContain returns true until the filter becomes authoritative.
func (f *MemoryBloomFilter) MayContain(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.ready {
		return true, nil
	}
	return f.test([]byte(key)), nil
}

// Add records a successfully loaded key but never changes readiness.
func (f *MemoryBloomFilter) Add(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.add([]byte(key))
	if f.preloadLive {
		f.pendingAdds = append(f.pendingAdds, key)
	}
	return nil
}

// Preload builds a complete replacement before atomically making it visible.
func (f *MemoryBloomFilter) Preload(ctx context.Context, keys []string) error {
	return f.preload(ctx, keys, nil)
}

// preload serializes construction and publication with Add. The hook exists
// only to deterministically exercise that transaction in package tests.
func (f *MemoryBloomFilter) preload(ctx context.Context, keys []string, hook func()) error {
	f.preloadMu.Lock()
	defer f.preloadMu.Unlock()

	f.mu.Lock()
	capacity, errorRate, shardCount := f.capacity, f.errorRate, len(f.shards)
	f.preloadLive = true
	f.pendingAdds = nil
	f.mu.Unlock()

	next := newMemoryBloomFilter(capacity, errorRate, shardCount)
	if hook != nil {
		hook()
	}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			f.finishPreloadFailure()
			return err
		}
		next.add([]byte(key))
	}
	if err := ctx.Err(); err != nil {
		f.finishPreloadFailure()
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		f.preloadLive = false
		f.pendingAdds = nil
		return err
	}
	for _, key := range f.pendingAdds {
		next.add([]byte(key))
	}
	f.shards = next.shards
	f.mask = next.mask
	f.ready = true
	f.preloadLive = false
	f.pendingAdds = nil
	return nil
}

func (f *MemoryBloomFilter) finishPreloadFailure() {
	f.mu.Lock()
	f.preloadLive = false
	f.pendingAdds = nil
	f.mu.Unlock()
}

// Reset discards authoritative state and returns to fail-open behavior.
func (f *MemoryBloomFilter) Reset() {
	f.preloadMu.Lock()
	defer f.preloadMu.Unlock()
	f.mu.RLock()
	capacity, errorRate, shardCount := f.capacity, f.errorRate, len(f.shards)
	f.mu.RUnlock()
	next := newMemoryBloomFilter(capacity, errorRate, shardCount)
	f.mu.Lock()
	f.shards = next.shards
	f.mask = next.mask
	f.ready = false
	f.mu.Unlock()
}

func (f *MemoryBloomFilter) test(key []byte) bool {
	i := f.idx(key)
	s := &f.shards[i]
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bf.Test(key)
}

func (f *MemoryBloomFilter) add(key []byte) {
	i := f.idx(key)
	s := &f.shards[i]
	s.mu.Lock()
	s.bf.Add(key)
	s.mu.Unlock()
}

func (f *MemoryBloomFilter) idx(key []byte) int {
	return bloomShardIndex(key, f.mask)
}
