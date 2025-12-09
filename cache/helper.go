// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache

import (
	"errors"
	"fmt"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/sony/gobreaker/v2"
	"hash/fnv"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerWrap gobreaker熔断器封装
type CircuitBreakerWrap struct {
	cb *gobreaker.CircuitBreaker[string]
}

func NewCircuitBreakerWrap(name string, st ...*gobreaker.Settings) *CircuitBreakerWrap {
	// 默认配置
	defaultSettings := gobreaker.Settings{
		Name:         name,
		MaxRequests:  3,                // 半开状态最多3个请求
		Interval:     60 * time.Second, // 60秒统计窗口
		Timeout:      30 * time.Second, // 30秒后尝试恢复
		BucketPeriod: 10 * time.Second, // 10秒一个桶
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			const (
				minSamples          = 10  // 最小样本数，避免冷启动误判
				maxFailureRate      = 0.5 // 失败率阈值：50%
				maxConsecutiveFails = 10  // 连续失败阈值（样本不足时兜底）
				hardConsecFails     = 5   // 样本足够时的强触发阈值
			)

			// 强触发：连续失败过多
			if counts.ConsecutiveFailures >= hardConsecFails {
				return true
			}

			// 样本不足时，使用更严格的连续失败阈值
			if counts.Requests < minSamples {
				return counts.ConsecutiveFailures >= maxConsecutiveFails
			}

			// 样本足够时，用失败率判定
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRate >= maxFailureRate
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// 这里可以集成日志系统或监控系统
			fmt.Printf("CircuitBreaker '%s' state changed from %s to %s\n", name, from.String(), to.String())
		},
	}

	// 如果传入配置不为空，则按需覆盖默认配置
	if len(st) > 0 && st[0] != nil {
		nst := st[0]
		if nst.Name != "" {
			defaultSettings.Name = nst.Name
		}
		if nst.MaxRequests > 0 {
			defaultSettings.MaxRequests = nst.MaxRequests
		}
		if nst.Interval > 0 {
			defaultSettings.Interval = nst.Interval
		}
		if nst.Timeout > 0 {
			defaultSettings.Timeout = nst.Timeout
		}
		if nst.BucketPeriod > 0 {
			defaultSettings.BucketPeriod = nst.BucketPeriod
		}
		if nst.ReadyToTrip != nil {
			defaultSettings.ReadyToTrip = nst.ReadyToTrip
		}
		if nst.OnStateChange != nil {
			defaultSettings.OnStateChange = nst.OnStateChange
		}
	}

	return &CircuitBreakerWrap{
		cb: gobreaker.NewCircuitBreaker[string](defaultSettings),
	}
}

func (cbw *CircuitBreakerWrap) Call(fn func() (string, error)) (interface{}, error) {
	res, err := cbw.cb.Execute(fn)
	if err != nil {
		return "", cbw.ConvertCircuitBreakerOpenError(err)
	}
	return res, nil
}

func (cbw *CircuitBreakerWrap) Allow() bool {
	return true
}

func (cbw *CircuitBreakerWrap) Reset() {
	// 空函数体
}

// ConvertCircuitBreakerOpenError 转换熔断器打开错误为自定义错误类型
func (cbw *CircuitBreakerWrap) ConvertCircuitBreakerOpenError(err error) error {
	if errors.Is(err, gobreaker.ErrTooManyRequests) {
		return NewErrCircuitBreakerOpen(err.Error())
	}
	if errors.Is(err, gobreaker.ErrOpenState) {
		return NewErrCircuitBreakerOpen(err.Error())
	}
	return err
}

/*
	分片锁布隆过滤器实现
*/

type shard struct {
	mu sync.RWMutex
	bf *bloom.BloomFilter
}

// ShardedBloomFilter 分片锁布隆过滤器
type ShardedBloomFilter struct {
	shards []shard
	mask   uint32
}

func NewShardedBloomFilter(shardCount int, estPerShard uint, fpRate float64) CacheBloomFilter {
	// shardCount 建议为 2 的幂，方便用 mask
	if shardCount <= 0 {
		shardCount = 16
	}
	sb := &ShardedBloomFilter{
		shards: make([]shard, shardCount),
		mask:   uint32(shardCount - 1),
	}
	for i := range sb.shards {
		sb.shards[i].bf = bloom.NewWithEstimates(estPerShard, fpRate)
	}
	return sb
}

func (sb *ShardedBloomFilter) idx(key []byte) int {
	h := fnv.New32a()
	_, _ = h.Write(key)
	return int(h.Sum32() & sb.mask)
}

func (sb *ShardedBloomFilter) Test(key []byte) bool {
	i := sb.idx(key)
	s := &sb.shards[i]
	s.mu.RLock()
	ok := s.bf.Test(key)
	s.mu.RUnlock()
	return ok
}

func (sb *ShardedBloomFilter) Add(key []byte) {
	i := sb.idx(key)
	s := &sb.shards[i]
	s.mu.Lock()
	s.bf.Add(key)
	s.mu.Unlock()
}

func (sb *ShardedBloomFilter) TestAndAdd(key []byte) bool {
	i := sb.idx(key)
	s := &sb.shards[i]
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bf.TestAndAdd(key)
}

func (sb *ShardedBloomFilter) Reset() {
	// 空函数体
}

// StableBloomFilter 支持陈旧信息驱逐的稳定布隆过滤器
// 稳定的布隆过滤器
// 适合缓存场景，能够自动清理陈旧的key信息，同时保持较低的误报率和稳定的内存消耗
// 热点数据会被频繁添加，不易被驱逐
// 冷数据计数器逐渐减少，最终被清理
// 内存占用保持稳定，适合长期运行
type StableBloomFilter struct {
	// 配置参数
	buckets        uint32 // 桶数量 (m)
	cellsPerBucket uint32 // 每个桶的单元数 (k)
	maxCount       uint8  // 每个单元的最大计数值

	// 存储结构 - 使用原子操作保证并发安全
	cells []uint64 // 存储单元数组，每8位表示一个计数器

	// 概率参数
	p float64 // 驱逐概率
}

// NewStableBloomFilter 创建稳定布隆过滤器
// capacity: 预期容量
// errorRate: 期望误报率
// evictRate: 驱逐率，控制旧元素被移除的速度
func NewStableBloomFilter(capacity uint32, errorRate float64, evictRate float64) *StableBloomFilter {
	// 计算最优参数
	// m = capacity * ln(2) / ln(1/errorRate)
	buckets := uint32(float64(capacity) * math.Ln2 / math.Log(1.0/errorRate))

	// k = ln(2) * m / capacity
	cellsPerBucket := uint32(math.Ln2 * float64(buckets) / float64(capacity))
	if cellsPerBucket == 0 {
		cellsPerBucket = 1
	}

	// 每个单元最大计数值设为3（2位）
	maxCount := uint8(3)

	// 计算驱逐概率 P = evictRate / maxCount
	p := evictRate / float64(maxCount)

	// 每个计数器2位，一个uint64存储32个计数器
	totalCells := buckets * cellsPerBucket
	cellsArraySize := (totalCells + 31) / 32

	return &StableBloomFilter{
		buckets:        buckets,
		cellsPerBucket: cellsPerBucket,
		maxCount:       maxCount,
		cells:          make([]uint64, cellsArraySize),
		p:              p,
	}
}

// Add 添加元素到过滤器
func (sbf *StableBloomFilter) Add(key string) {
	// 为每个桶计算hash
	for i := uint32(0); i < sbf.cellsPerBucket; i++ {
		hash := sbf.hash(key, i)
		bucket := hash % sbf.buckets

		// 在桶内选择一个单元
		cellIndex := sbf.selectCell(bucket, key, i)

		sbf.incrementCell(cellIndex)
	}
}

// Test 测试元素是否可能存在
func (sbf *StableBloomFilter) Test(key string) bool {
	for i := uint32(0); i < sbf.cellsPerBucket; i++ {
		hash := sbf.hash(key, i)
		bucket := hash % sbf.buckets

		// 检查桶内对应单元的计数
		cellIndex := sbf.selectCell(bucket, key, i)
		if sbf.getCellCount(cellIndex) == 0 {
			return false // 绝对不存在
		}
	}
	return true // 可能存在
}

// incrementCell 原子性地增加单元计数
func (sbf *StableBloomFilter) incrementCell(cellIndex uint32) {
	arrayIndex := cellIndex / 32
	bitOffset := (cellIndex % 32) * 2 // 每个计数器2位

	for {
		oldValue := atomic.LoadUint64(&sbf.cells[arrayIndex])

		// 提取当前计数值（8位）
		mask := uint64(0x3) << bitOffset // 2位掩码
		currentCount := uint8((oldValue & mask) >> bitOffset)

		var newCount uint8
		if currentCount < sbf.maxCount {
			// 直接递增
			newCount = currentCount + 1
		} else {
			// 已达最大值，使用概率驱逐
			if sbf.shouldEvict() {
				newCount = currentCount - 1 // 驱逐：计数减1
			} else {
				newCount = currentCount // 保持不变
			}
		}

		// 构造新值
		newValue := (oldValue &^ mask) | (uint64(newCount) << bitOffset)

		// 原子更新
		if atomic.CompareAndSwapUint64(&sbf.cells[arrayIndex], oldValue, newValue) {
			break
		}
		// CAS失败，重试
	}
}

// getCellCount 获取单元计数值
func (sbf *StableBloomFilter) getCellCount(cellIndex uint32) uint8 {
	arrayIndex := cellIndex / 32
	bitOffset := (cellIndex % 32) * 2

	value := atomic.LoadUint64(&sbf.cells[arrayIndex])
	mask := uint64(0x3) << bitOffset

	return uint8((value & mask) >> bitOffset)
}

// selectCell 在指定桶中选择单元（使用一致性哈希）
func (sbf *StableBloomFilter) selectCell(bucket uint32, key string, seed uint32) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))

	// 使用字节数组避免字符串拼接问题
	seedBytes := make([]byte, 8)
	seedBytes[0] = byte(seed)
	seedBytes[1] = byte(seed >> 8)
	seedBytes[2] = byte(seed >> 16)
	seedBytes[3] = byte(seed >> 24)
	seedBytes[4] = byte(bucket)
	seedBytes[5] = byte(bucket >> 8)
	seedBytes[6] = byte(bucket >> 16)
	seedBytes[7] = byte(bucket >> 24)
	_, _ = h.Write(seedBytes)

	cellHash := h.Sum32()
	cellInBucket := cellHash % sbf.cellsPerBucket
	return bucket*sbf.cellsPerBucket + cellInBucket
}

// shouldEvict 根据概率决定是否驱逐
func (sbf *StableBloomFilter) shouldEvict() bool {
	return rand.Float64() < sbf.p // 使用全局安全的随机数
}

// hash 计算哈希值
func (sbf *StableBloomFilter) hash(key string, seed uint32) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte{byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24)})
	return h.Sum32()
}

// Reset 重置过滤器
func (sbf *StableBloomFilter) Reset() {
	for i := range sbf.cells {
		atomic.StoreUint64(&sbf.cells[i], 0)
	}
}

// GetStats 获取统计信息
func (sbf *StableBloomFilter) GetStats() StableBloomStats {
	var totalCount, nonZeroCells uint64

	totalCells := sbf.buckets * sbf.cellsPerBucket

	for i := uint32(0); i < totalCells; i++ {
		count := sbf.getCellCount(i)
		if count > 0 {
			nonZeroCells++
			totalCount += uint64(count)
		}
	}

	utilizationRate := float64(nonZeroCells) / float64(totalCells)
	avgCount := float64(totalCount) / float64(nonZeroCells)
	if nonZeroCells == 0 {
		avgCount = 0
	}

	return StableBloomStats{
		Buckets:         sbf.buckets,
		CellsPerBucket:  sbf.cellsPerBucket,
		TotalCells:      totalCells,
		NonZeroCells:    nonZeroCells,
		UtilizationRate: utilizationRate,
		AverageCount:    avgCount,
		MemoryBytes:     uint64(len(sbf.cells) * 8), // 8字节每个uint64
	}
}

// StableBloomStats 稳定布隆过滤器统计信息
type StableBloomStats struct {
	Buckets         uint32  `json:"buckets"`
	CellsPerBucket  uint32  `json:"cells_per_bucket"`
	TotalCells      uint32  `json:"total_cells"`
	NonZeroCells    uint64  `json:"non_zero_cells"`
	UtilizationRate float64 `json:"utilization_rate"`
	AverageCount    float64 `json:"average_count"`
	MemoryBytes     uint64  `json:"memory_bytes"`
}
