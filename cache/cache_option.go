// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package cache 提供高性能的多级缓存系统，支持本地缓存、远程缓存和二级缓存架构，内置多种缓存保护机制。
package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/samber/lo"
	"math/rand"
	"sync"
	"time"
)

/**
使用示例

// 使用选项池获取CacheOption实例

co := cache.OptionPoolGet(s.GetContext())
//defer co.Release()
defer cache.OptionPoolPut(co)

// new全新的CacheOption实例
co := cache.NewCacheOption(s.GetContext())

//----------------------------------------------

// 示例1：固定TTL
option := co.SetCacheKey("user:123").
    SetLocalTTL(5 * time.Minute).
    SetRemoteTTL(30 * time.Minute)

// 示例2：使用随机TTL（绝对时间范围）
option = co.SetCacheKey("product:456").
    LocalTTLRandom(5*time.Minute, 1*time.Minute).  // 4-6分钟随机
    RemoteTTLRandom(30*time.Minute, 5*time.Minute) // 25-35分钟随机

// 示例3：使用随机TTL（百分比范围）
option = co.SetCacheKey("order:789").
    LocalTTLRandomPercent(10*time.Minute, 0.2).  // 8-12分钟随机（±20%）
    RemoteTTLRandomPercent(60*time.Minute, 0.1)  // 54-66分钟随机（±10%）

// 示例4：链式调用
option = co.SetCacheKey("session:abc").
    Local().
    LocalTTLRandom(15*time.Minute, 3*time.Minute).
    EnableSingleFlight()

// 获取实际TTL值
localTTL := option.GetLocalTTL()   // 每次调用返回新的随机值
baseTTL := option.GetLocalBaseTTL() // 返回基础TTL，不含随机

// 完整使用示例： ExampleService.GetExamples 方法中启用缓存
import "github.com/lamxy/fiberhouse/cache"

co := cache.OptionPoolGet(s.GetContext())
defer cache.OptionPoolPut(co)

// 设置缓存参数: 二级缓存、开启缓存、设置缓存key、设置本地缓存过期时间(10秒±10%)、设置远程缓存过期时间(3分钟±1分钟)、写远程缓存同步策略、设置上下文、启用缓存全部保护措施
co.Level2().EnableCache().SetCacheKey("key:example:list:page:"+strconv.Itoa(page)+":size:"+strconv.Itoa(size)).SetLocalTTLRandomPercent(10*time.Second, 0.1).
		SetRemoteTTLWithRandom(3*time.Minute, 1*time.Minute).SetSyncStrategyWriteRemoteOnly().SetContextCtx(context.Background()).EnableProtectionAll()

// 获取缓存数据 GetCached泛型方法
	return cache.GetCached[[]responsevo.ExampleRespVo](co, func(ctx context.Context) ([]responsevo.ExampleRespVo, error) {
		// 调用实际获取数据的方法
		return s.Repo.GetExamples(page, size)
	})
*/

var (
	optionsPool *sync.Pool
	optsOnce    sync.Once
)

// Level 缓存级别
type Level int8

// Strategy 缓存在2级缓存的同步策略
type Strategy int8

// 缓存级别
const (
	Local Level = iota + 1 // 从1开始，避免零值问题
	Remote
	Level2
)

// 同步策略
const (
	WriteBoth Strategy = iota + 1
	WriteRemoteOnly
	AsyncWriteBoth
	AsyncWriteRemoteOnly
)

// TTLConfig TTL配置结构
type TTLConfig struct {
	BaseTTL     time.Duration // 基础TTL
	RandomRange time.Duration // 随机范围
	UseRandom   bool          // 是否使用随机TTL
}

// CacheOption 缓存配置选项
type CacheOption struct {
	// 全局上下文
	AppCtx fiberhouse.IContext
	// 是否启动缓存
	enable bool
	// context
	ctx context.Context

	// 缓存级别： 1本地缓存、2远程缓存、3二级缓存
	cacheLevel Level
	// 二级缓存同步策略： 1 write_both 异步双写、2 write_remote_only 只写远程
	syncStrategy Strategy
	// 缓存紧急策略： 不处理、熔断降级、布隆过滤器、击穿保护
	singleFlight   bool //击穿保护
	bloomFilter    bool // 穿透保护
	circuitBreaker bool // 雪崩保护

	// 缓存key
	cacheKey string
	// json序列化反序列化实例
	jsonWrapper fiberhouse.JsonWrapper

	// 本地缓存有效期配置
	localTTLConfig *TTLConfig
	// 远程缓存有效期配置
	remoteTTLConfig *TTLConfig

	// 默认缓存实例key
	defaultInstanceKey globalmanager.KeyName
}

// NewCacheOption 创建默认的CacheOption实例
func NewCacheOption(appCtx fiberhouse.IContext) *CacheOption {
	return &CacheOption{
		AppCtx:          appCtx,
		enable:          true,
		syncStrategy:    WriteRemoteOnly,
		localTTLConfig:  &TTLConfig{UseRandom: false},
		remoteTTLConfig: &TTLConfig{UseRandom: false},
	}
}

// OptionPoolGet 从选项池获取缓存选项
func OptionPoolGet(ctx fiberhouse.IContext) *CacheOption {
	optsOnce.Do(func() {
		optionsPool = &sync.Pool{
			New: func() any {
				return NewCacheOption(ctx)
			},
		}
	})
	return optionsPool.Get().(*CacheOption)
}

// OptionPoolPut 将使用完的CacheOption放回选项池
func OptionPoolPut(co *CacheOption) {
	if co != nil {
		co.Reset()
		optionsPool.Put(co)
	}
}

// Clone 克隆CacheOption实例，可以传入新的context.Context
func (c *CacheOption) Clone(ctx ...context.Context) *CacheOption {
	coNew := OptionPoolGet(c.AppCtx)
	if len(ctx) > 0 {
		coNew.ctx = ctx[0]
	}
	coNew.enable = c.enable
	coNew.cacheLevel = c.cacheLevel
	coNew.syncStrategy = c.syncStrategy
	coNew.singleFlight = c.singleFlight
	coNew.bloomFilter = c.bloomFilter
	coNew.circuitBreaker = c.circuitBreaker
	coNew.cacheKey = c.cacheKey
	coNew.jsonWrapper = c.jsonWrapper
	if c.localTTLConfig != nil {
		coNew.localTTLConfig = &TTLConfig{
			BaseTTL:     c.localTTLConfig.BaseTTL,
			RandomRange: c.localTTLConfig.RandomRange,
			UseRandom:   c.localTTLConfig.UseRandom,
		}
	}
	if c.remoteTTLConfig != nil {
		coNew.remoteTTLConfig = &TTLConfig{
			BaseTTL:     c.remoteTTLConfig.BaseTTL,
			RandomRange: c.remoteTTLConfig.RandomRange,
			UseRandom:   c.remoteTTLConfig.UseRandom,
		}
	}
	coNew.defaultInstanceKey = c.defaultInstanceKey
	return coNew
}

// Release 释放CacheOption实例到选项池
func (c *CacheOption) Release() {
	OptionPoolPut(c)
}

// Reset 重置CacheOption实例属性，保留AppCtx
func (c *CacheOption) Reset() *CacheOption {
	c.ctx = nil
	c.bloomFilter = false
	c.circuitBreaker = false
	c.singleFlight = false
	if c.localTTLConfig == nil {
		c.localTTLConfig = &TTLConfig{}
	} else {
		c.localTTLConfig.BaseTTL = 0
		c.localTTLConfig.RandomRange = 0
		c.localTTLConfig.UseRandom = false
	}
	if c.remoteTTLConfig == nil {
		c.remoteTTLConfig = &TTLConfig{}
	} else {
		c.remoteTTLConfig.BaseTTL = 0
		c.remoteTTLConfig.RandomRange = 0
		c.remoteTTLConfig.UseRandom = false
	}
	c.defaultInstanceKey = ""
	c.cacheKey = ""
	c.jsonWrapper = nil
	c.syncStrategy = WriteRemoteOnly
	c.cacheLevel = 0
	c.enable = true
	return c
}

// SetSyncStrategyWriteBoth 同步策略: 同步双写2级缓存
func (c *CacheOption) SetSyncStrategyWriteBoth() *CacheOption {
	c.SetSyncStrategy(WriteBoth)
	return c
}

// SetSyncStrategyWriteRemoteOnly 同步策略: 同步写远程缓存
func (c *CacheOption) SetSyncStrategyWriteRemoteOnly() *CacheOption {
	c.SetSyncStrategy(WriteRemoteOnly)
	return c
}

// SetSyncStrategyAsyncWriteBoth 同步策略: 异步双写2级缓存
func (c *CacheOption) SetSyncStrategyAsyncWriteBoth() *CacheOption {
	c.SetSyncStrategy(AsyncWriteBoth)
	return c
}

// SetSyncStrategyAsyncWriteRemoteOnly 同步策略: 异步写远程缓存
func (c *CacheOption) SetSyncStrategyAsyncWriteRemoteOnly() *CacheOption {
	c.SetSyncStrategy(AsyncWriteRemoteOnly)
	return c
}

// SetSyncStrategy 设置同步策略
func (c *CacheOption) SetSyncStrategy(sst Strategy) *CacheOption {
	c.syncStrategy = sst
	return c
}

// GetSyncStrategy 获取同步策略
func (c *CacheOption) GetSyncStrategy() Strategy {
	return c.syncStrategy
}

// EnableSingleFlight 启动单飞保护
func (c *CacheOption) EnableSingleFlight() *CacheOption {
	c.singleFlight = true
	return c
}

// EnableBloomFilter 启动布隆过滤器保护
func (c *CacheOption) EnableBloomFilter() *CacheOption {
	c.bloomFilter = true
	return c
}

// EnableCircuitBreaker 启动断路器保护
func (c *CacheOption) EnableCircuitBreaker() *CacheOption {
	c.circuitBreaker = true
	return c
}

// EnableProtectionAll 启动所有保护措施
func (c *CacheOption) EnableProtectionAll() *CacheOption {
	c.singleFlight = true
	c.bloomFilter = true
	c.circuitBreaker = true
	return c
}

// GetSingleFlightState 获取单飞保护启动状态
func (c *CacheOption) GetSingleFlightState() bool {
	return c.singleFlight
}

// GetBloomFilterState 获取布隆过滤器保护启动状态
func (c *CacheOption) GetBloomFilterState() bool {
	return c.bloomFilter
}

// GetCircuitBreakerState 获取断路器保护启动状态
func (c *CacheOption) GetCircuitBreakerState() bool {
	return c.circuitBreaker
}

// SetJsonWrapper 设置json编解码器
func (c *CacheOption) SetJsonWrapper(jpr fiberhouse.JsonWrapper) *CacheOption {
	c.jsonWrapper = jpr
	return c
}

// GetJsonWrapper 获取json编解码器，默认获取GetDefaultJsonCodecKey编解码器
func (c *CacheOption) GetJsonWrapper() fiberhouse.JsonWrapper {
	if c.jsonWrapper == nil {
		c.jsonWrapper = fiberhouse.GetMustInstance[fiberhouse.JsonWrapper](c.AppCtx.GetStarter().GetApplication().GetDefaultJsonCodecKey())
	}
	return c.jsonWrapper
}

// SetLocalTTL 设置本地缓存有效期
func (c *CacheOption) SetLocalTTL(ttl time.Duration) *CacheOption {
	if c.localTTLConfig == nil {
		c.localTTLConfig = &TTLConfig{
			BaseTTL:   ttl,
			UseRandom: false,
		}
	} else {
		c.localTTLConfig.BaseTTL = ttl
		c.localTTLConfig.UseRandom = false
		c.localTTLConfig.RandomRange = 0
	}
	return c
}

// SetLocalTTLWithRandom 设置带随机范围的本地缓存TTL
// baseTTL: 基础过期时间
// randomRange: 随机范围（±randomRange）
func (c *CacheOption) SetLocalTTLWithRandom(baseTTL, randomRange time.Duration) *CacheOption {
	if c.localTTLConfig == nil {
		c.localTTLConfig = &TTLConfig{
			BaseTTL:     baseTTL,
			RandomRange: randomRange,
			UseRandom:   true,
		}
	} else {
		c.localTTLConfig.BaseTTL = baseTTL
		c.localTTLConfig.RandomRange = randomRange
		c.localTTLConfig.UseRandom = true
	}
	return c
}

// SetLocalTTLRandomPercent 设置按百分比随机的本地缓存TTL
// baseTTL: 基础过期时间
// percent: 随机百分比（0.0-1.0）
func (c *CacheOption) SetLocalTTLRandomPercent(baseTTL time.Duration, percent float64) *CacheOption {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	randomRange := time.Duration(float64(baseTTL) * percent)
	return c.SetLocalTTLWithRandom(baseTTL, randomRange)
}

// GetLocalTTL 获取计算后的本地缓存有效期
func (c *CacheOption) GetLocalTTL() time.Duration {
	if c.localTTLConfig == nil {
		return 0
	}

	if !c.localTTLConfig.UseRandom {
		return c.localTTLConfig.BaseTTL
	}

	return c.calculateRandomTTL(c.localTTLConfig)
}

// GetLocalBaseTTL 获取本地缓存基础TTL（不含随机）
func (c *CacheOption) GetLocalBaseTTL() time.Duration {
	if c.localTTLConfig == nil {
		return 0
	}
	return c.localTTLConfig.BaseTTL
}

// SetRemoteTTL 设置固定的远程缓存TTL
func (c *CacheOption) SetRemoteTTL(ttl time.Duration) *CacheOption {
	if c.remoteTTLConfig == nil {
		c.remoteTTLConfig = &TTLConfig{
			BaseTTL:   ttl,
			UseRandom: false,
		}
	} else {
		c.remoteTTLConfig.BaseTTL = ttl
		c.remoteTTLConfig.UseRandom = false
		c.remoteTTLConfig.RandomRange = 0
	}
	return c
}

// SetRemoteTTLWithRandom 设置带随机范围的远程缓存TTL
func (c *CacheOption) SetRemoteTTLWithRandom(baseTTL, randomRange time.Duration) *CacheOption {
	if c.remoteTTLConfig == nil {
		c.remoteTTLConfig = &TTLConfig{
			BaseTTL:     baseTTL,
			RandomRange: randomRange,
			UseRandom:   true,
		}
	} else {
		c.remoteTTLConfig.BaseTTL = baseTTL
		c.remoteTTLConfig.RandomRange = randomRange
		c.remoteTTLConfig.UseRandom = true
	}

	return c
}

// SetRemoteTTLRandomPercent 设置按百分比随机的远程缓存TTL
func (c *CacheOption) SetRemoteTTLRandomPercent(baseTTL time.Duration, percent float64) *CacheOption {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	randomRange := time.Duration(float64(baseTTL) * percent)
	return c.SetRemoteTTLWithRandom(baseTTL, randomRange)
}

// GetRemoteTTL 获取远程缓存TTL（如果启用随机，返回随机值）
func (c *CacheOption) GetRemoteTTL() time.Duration {
	if c.remoteTTLConfig == nil {
		return 0
	}

	if !c.remoteTTLConfig.UseRandom {
		return c.remoteTTLConfig.BaseTTL
	}

	return c.calculateRandomTTL(c.remoteTTLConfig)
}

// GetRemoteBaseTTL 获取远程缓存存基础TTL（不含随机）
func (c *CacheOption) GetRemoteBaseTTL() time.Duration {
	if c.remoteTTLConfig == nil {
		return 0
	}
	return c.remoteTTLConfig.BaseTTL
}

// SetCacheKey 设置缓存key的值
func (c *CacheOption) SetCacheKey(key string) *CacheOption {
	c.cacheKey = key
	return c
}

// GetCacheKey 获取缓存key值
func (c *CacheOption) GetCacheKey() string {
	return c.cacheKey
}

// SetContextCtx 设置上下文对象
func (c *CacheOption) SetContextCtx(ctx context.Context) *CacheOption {
	c.ctx = ctx
	return c
}

// GetContextCtx 获取上下文对象
func (c *CacheOption) GetContextCtx() context.Context {
	return c.ctx
}

// GetContext 获取全局上下文对象
func (c *CacheOption) GetContext() fiberhouse.IContext {
	return c.AppCtx
}

// GetCacheLevel 获取缓存级别
func (c *CacheOption) GetCacheLevel() Level {
	return c.cacheLevel
}

// SetCacheLevel 设置缓存级别
func (c *CacheOption) SetCacheLevel(st Level) *CacheOption {
	c.cacheLevel = st
	return c
}

// Local 设置为本地缓存级别
func (c *CacheOption) Local() *CacheOption {
	c.SetCacheLevel(Local)
	return c
}

// Remote 设置为远程缓存级别
func (c *CacheOption) Remote() *CacheOption {
	c.SetCacheLevel(Remote)
	return c
}

// Level2 设置为二级缓存级别
func (c *CacheOption) Level2() *CacheOption {
	c.SetCacheLevel(Level2)
	return c
}

// SetDefaultInstanceKey 设置默认的缓存实例的key，用于从全局管理器获取默认缓存实例
func (c *CacheOption) SetDefaultInstanceKey(key string) *CacheOption {
	c.defaultInstanceKey = key
	return c
}

// GetDefaultInstanceKey 获取默认的缓存实例key值
func (c *CacheOption) GetDefaultInstanceKey() globalmanager.KeyName {
	return c.defaultInstanceKey
}

// EnableCache 开启缓存
func (c *CacheOption) EnableCache() *CacheOption {
	c.enable = true
	return c
}

// DisableCache 关闭缓存
func (c *CacheOption) DisableCache() *CacheOption {
	c.enable = false
	return c
}

// IsCache 判断是否开启缓存
func (c *CacheOption) IsCache() bool {
	return c.enable
}

// Valid 验证缓存选项是否有效
func (c *CacheOption) Valid() error {
	if !lo.Contains([]Level{Local, Remote, Level2}, c.cacheLevel) {
		return fmt.Errorf("invalid cache level: %d, must be one of [%d, %d, %d]",
			c.cacheLevel, Local, Remote, Level2)
	}

	if !lo.Contains([]Strategy{WriteBoth, WriteRemoteOnly, AsyncWriteBoth, AsyncWriteRemoteOnly}, c.syncStrategy) {
		return fmt.Errorf("invalid sync strategy: %d, must be one of [%d, %d, %d, %d]", c.syncStrategy, WriteBoth, WriteRemoteOnly, AsyncWriteBoth, AsyncWriteRemoteOnly)
	}

	if c.cacheKey == "" {
		return errors.New("cache key is required")
	}

	if c.AppCtx == nil {
		return errors.New("AppCtx is required")
	}

	return nil
}

// ===== 内部工具方法 =====

// calculateRandomTTL 计算随机TTL
func (c *CacheOption) calculateRandomTTL(config *TTLConfig) time.Duration {
	if config == nil {
		return 0
	}
	if !config.UseRandom || config.RandomRange <= 0 {
		return config.BaseTTL
	}

	// 生成 [-randomRange, +randomRange] 范围内的随机值
	randomOffset := time.Duration(rand.Int63n(int64(config.RandomRange*2))) - config.RandomRange
	finalTTL := config.BaseTTL + randomOffset

	// 确保TTL不会为负数
	if finalTTL <= 0 {
		return config.BaseTTL
	}

	return finalTTL
}

// IsLocalTTLRandom 检查本地缓存是否启用随机TTL
func (c *CacheOption) IsLocalTTLRandom() bool {
	return c.localTTLConfig != nil && c.localTTLConfig.UseRandom
}

// IsRemoteTTLRandom 检查远程缓存是否启用随机TTL
func (c *CacheOption) IsRemoteTTLRandom() bool {
	return c.remoteTTLConfig != nil && c.remoteTTLConfig.UseRandom
}

// GetTTLInfo 获取TTL配置信息（用于调试）
func (c *CacheOption) GetTTLInfo() map[string]interface{} {
	return map[string]interface{}{
		"local": map[string]interface{}{
			"base_ttl":    c.GetLocalBaseTTL(),
			"current_ttl": c.GetLocalTTL(),
			"use_random":  c.IsLocalTTLRandom(),
			"random_range": func() time.Duration {
				if c.localTTLConfig != nil {
					return c.localTTLConfig.RandomRange
				}
				return 0
			}(),
		},
		"remote": map[string]interface{}{
			"base_ttl":    c.GetRemoteBaseTTL(),
			"current_ttl": c.GetRemoteTTL(),
			"use_random":  c.IsRemoteTTLRandom(),
			"random_range": func() time.Duration {
				if c.remoteTTLConfig != nil {
					return c.remoteTTLConfig.RandomRange
				}
				return 0
			}(),
		},
	}
}
