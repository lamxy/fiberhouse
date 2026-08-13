package cache

import (
	"context"
	"errors"
	"math"
	"sync"
)

// ExistenceFilter 判断缓存键是否可能存在。过滤器尚未具备权威状态时必须返回 true，
// 防止调用方依据不完整数据错误拒绝有效键。
type ExistenceFilter interface {
	Ready() bool
	MayContain(context.Context, string) (bool, error)
	Add(context.Context, string) error
}

// PreloadableFilter 可以原子替换权威键集合。
type PreloadableFilter interface {
	ExistenceFilter
	Preload(context.Context, []string) error
	Reset()
}

// MemoryBloomFilter 是进程内权威布隆过滤器，完整预加载成功前不会进入就绪状态。
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

// NewMemoryBloomFilter 创建未就绪的过滤器。分片数向上取整到 2 的幂，零表示一个分片。
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

// Ready 返回过滤器是否已经完成一次完整预加载。
func (f *MemoryBloomFilter) Ready() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ready
}

// MayContain 在过滤器具备权威状态前始终返回 true。
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

// Add 记录成功加载的键，但不会改变过滤器的就绪状态。
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

// Preload 先构建完整替代集合，再以原子方式发布。
func (f *MemoryBloomFilter) Preload(ctx context.Context, keys []string) error {
	return f.preload(ctx, keys, nil)
}

// preload 将集合构建、发布与 Add 串行协调。hook 仅用于包内测试确定性验证事务过程。
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

// Reset 丢弃权威状态并恢复故障开放行为。
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
