// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cacheremote

import (
	"context"
	"errors"
	"github.com/lamxy/fiberhouse/frame"
	"github.com/lamxy/fiberhouse/frame/cache"
	"github.com/lamxy/fiberhouse/frame/constant"
	frameUtils "github.com/lamxy/fiberhouse/frame/utils"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker/v2"
	"golang.org/x/sync/singleflight"
	"sync"
	"sync/atomic"
	"time"
)

// RedisDb 实现了 Cache和全局管理器相关 接口
type RedisDb struct {
	Client       *redis.Client
	Ctx          frame.IContext
	lock         *sync.RWMutex
	confPathname string
	// 缓存调用close关闭则阻止所有方法执行逻辑
	closed atomic.Bool
	level  cache.Level
	// 缓存保护组件
	sf             *singleflight.Group       // 击穿保护
	bloomFilter    cache.CacheBloomFilter    // 穿透保护
	circuitBreaker cache.CacheCircuitBreaker // 雪崩熔断保护
}

func NewRedisDb(appCtx frame.IContext, confPath ...string) (cache.Cache, error) {
	ca := &RedisDb{
		Client: NewClient(appCtx, confPath...),
		Ctx:    appCtx,
		sf:     &singleflight.Group{},
		lock:   &sync.RWMutex{},
		level:  cache.Remote,
	}

	var basePath string
	if len(confPath) > 0 && confPath[0] != "" {
		basePath = confPath[0]
	} else {
		basePath = constant.DefaultRedisDBConfName
	}
	ca.confPathname = basePath
	aConf := appCtx.GetConfig()

	// 读取缓存保护配置
	cacheProtection := aConf.Bool(basePath + ".protection.enable")

	if cacheProtection {
		/**
		注册默认保护器
		*/
		// 注册分片锁的布隆过滤器
		appCtx.GetContainer().Register(constant.CacheProtectionKeyPrefix+"shardedBloomFilter", func() (interface{}, error) {
			return cache.NewShardedBloomFilter(aConf.Int(basePath+".protection.shardedBloomFilter.shards", 16),
				uint(aConf.Int(basePath+".protection.shardedBloomFilter.estPerShard", 100000)),
				aConf.Float64(basePath+".protection.shardedBloomFilter.fpRate", 0.01),
			), nil
		})
		// 注册包装的熔断器
		appCtx.GetContainer().Register(constant.CacheProtectionKeyPrefix+"wrapCircuitBreaker", func() (interface{}, error) {
			name := aConf.String(basePath+".protection.wrapCircuitBreaker.name", "cacheCircuitBreaker")
			return cache.NewCircuitBreakerWrap(name, &gobreaker.Settings{
				MaxRequests:  uint32(aConf.Int(basePath+".protection.wrapCircuitBreaker.maxRequests", 5)),
				Interval:     aConf.Duration(basePath+".protection.wrapCircuitBreaker.interval", 60) * time.Second,
				Timeout:      aConf.Duration(basePath+".protection.wrapCircuitBreaker.timeout", 30) * time.Second,
				BucketPeriod: aConf.Duration(basePath+".protection.wrapCircuitBreaker.bucketPeriod", 10) * time.Second,
			}), nil
		})

		// 获取配置type
		bloomFilterType := aConf.String(basePath+".protection.type.bloomFilter.selected", "shardedBloomFilter")       // 默认稳定的布隆过滤器
		circuitBreakerType := aConf.String(basePath+".protection.type.circuitBreaker.selected", "wrapCircuitBreaker") // 默认包装的熔断器

		appCtx.GetLogger().Info(appCtx.GetConfig().LogOriginCache()).Msgf("Redis Cache Protection enabled, BloomFilter type: %s, CircuitBreaker type: %s", bloomFilterType, circuitBreakerType)

		// 初始化布隆过滤器
		ca.bloomFilter = frame.GetMustInstance[cache.CacheBloomFilter](constant.CacheProtectionKeyPrefix + bloomFilterType)
		// 初始化熔断器
		ca.circuitBreaker = frame.GetMustInstance[cache.CacheCircuitBreaker](constant.CacheProtectionKeyPrefix + circuitBreakerType)
	}
	return ca, nil
}

// GetConfPath 获取 Redis 配置路径
func (rd *RedisDb) GetConfPath() string {
	return rd.confPathname
}

// GetRedisClient 获取底层 Redis 客户端实例
func (rd *RedisDb) GetRedisClient() *redis.Client {
	return rd.Client
}

// GetLevel 获取缓存级别
func (rd *RedisDb) GetLevel() cache.Level {
	return rd.level
}

// Close 关闭 Redis 客户端连接
// 谨慎使用Close关闭链接
func (rd *RedisDb) Close() error {
	if rd.closed.Load() {
		return cache.ErrCacheClosed
	}
	rd.closed.Store(true)
	return rd.Client.Close()
}

// IsHealthy 检查 Redis 客户端连接是否健康
func (rd *RedisDb) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return rd.PingTry(ctx)
}

// Rebuild 重建 Redis 客户端连接
func (rd *RedisDb) Rebuild(name ...interface{}) (interface{}, error) {
	if len(name) > 0 {
		return rd.ReNewClient(name[0].(string))
	}
	return rd.ReNewClient()
}

// Get 获取指定 key 的值
func (rd *RedisDb) Get(ctx context.Context, key string, co *cache.CacheOption) (string, error) {
	if rd.closed.Load() {
		return "", cache.ErrCacheClosed
	}

	// 布隆过滤器预检查
	if co.GetBloomFilterState() && rd.bloomFilter != nil {
		if !rd.bloomFilter.Test(frameUtils.UnsafeBytes(key)) {
			// 新key值被拦截, 尝试获取一次，在成功时添加key+检查标识到布隆过滤器
			if co.GetSingleFlightState() && rd.sf != nil {
				// 开启单飞保护，避免并发穿透
				value, err, _ := rd.sf.Do(key+"_bloom_check", func() (interface{}, error) {
					return rd.getWithBloomUpdate(ctx, key, co)
				})
				if err != nil {
					return "", err
				}
				return value.(string), nil
			}
			return rd.getWithBloomUpdate(ctx, key, co)
		}
	}

	// 正常获取流程
	if co.GetSingleFlightState() && rd.sf != nil {
		value, err, _ := rd.sf.Do(key, func() (interface{}, error) {
			return rd.getInternal(ctx, key, co)
		})
		if err != nil {
			return "", err
		}
		// 成功获取后添加到布隆过滤器
		if co.GetBloomFilterState() && rd.bloomFilter != nil {
			rd.bloomFilter.Add(frameUtils.UnsafeBytes(key))
		}
		return value.(string), nil
	}

	// 直接获取
	res, err := rd.getInternal(ctx, key, co)
	if err == nil && co.GetBloomFilterState() && rd.bloomFilter != nil {
		rd.bloomFilter.Add(frameUtils.UnsafeBytes(key))
	}
	return res, err
}

// getWithBloomUpdate 带布隆过滤器更新的获取内部方法
func (rd *RedisDb) getWithBloomUpdate(ctx context.Context, key string, co *cache.CacheOption) (string, error) {
	res, err := rd.getInternal(ctx, key, co)
	if err == nil {
		rd.bloomFilter.Add(frameUtils.UnsafeBytes(key))
		return res, nil
	}

	var errRedNil cache.ErrRedisNil
	if errors.As(err, &errRedNil) {
		return "", cache.NewErrRejectedByBloomFilter(key)
	}
	return "", err
}

// getInternal 获取缓存内部方法
func (rd *RedisDb) getInternal(ctx context.Context, key string, co *cache.CacheOption) (string, error) {
	if co.GetCircuitBreakerState() && rd.circuitBreaker != nil {
		// 开启熔断保护
		value, err := rd.circuitBreaker.Call(func() (string, error) {
			return rd.Client.Get(ctx, key).Result()
		})

		if err != nil {
			return "", err
		}
		return value.(string), nil
	}

	// 直接获取
	s, err := rd.Client.Get(ctx, key).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", cache.NewErrRedisNil(key)
		}
		return "", err
	}
	return s, nil
}

// Set 设置指定 key 的值
func (rd *RedisDb) Set(ctx context.Context, key string, value interface{}, co *cache.CacheOption) error {
	if rd.closed.Load() {
		return cache.ErrCacheClosed
	}

	// 序列化值
	serializedValue, err := rd.serializeValue(value, co)
	if err != nil {
		return err
	}

	// 执行设置操作
	err = rd.setInternal(ctx, key, serializedValue, co)
	if err != nil {
		return err
	}

	// 成功设置后才添加到布隆过滤器
	if co.GetBloomFilterState() && rd.bloomFilter != nil {
		rd.bloomFilter.Add(frameUtils.UnsafeBytes(key))
	}
	return nil
}

// serializeValue 内部对值序列化方法
func (rd *RedisDb) serializeValue(value interface{}, co *cache.CacheOption) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return frameUtils.UnsafeString(v), nil
	default:
		data, err := co.GetJsonWrapper().Marshal(value)
		if err != nil {
			return "", cache.NewCacheError("serialize", "", err)
		}
		return frameUtils.UnsafeString(data), nil
	}
}

// setInternal 内部设置方法
func (rd *RedisDb) setInternal(ctx context.Context, key, value string, co *cache.CacheOption) error {
	// 写入不设置断路保护
	/*if co.GetCircuitBreakerState() && rd.circuitBreaker != nil {
		// 开启断路保护
		_, err := rd.circuitBreaker.Call(func() (string, error) {
			return "", rd.Client.Set(ctx, key, value, co.GetRemoteTTL()).Err()
		})
		return err
	}*/
	return rd.Client.Set(ctx, key, value, co.GetRemoteTTL()).Err()
}

// Delete 删除指定的 key
func (rd *RedisDb) Delete(ctx context.Context, keys ...string) error {
	if rd.closed.Load() {
		return cache.ErrCacheClosed
	}
	return rd.Client.Del(ctx, keys...).Err()
}

// Wait Redis 无需等待，直接返回 nil
func (rd *RedisDb) Wait() error {
	if rd.closed.Load() {
		return cache.ErrCacheClosed
	}
	return nil
}

// NewClient 创建一个新的 Redis 客户端连接
func NewClient(appCtx frame.IContext, confPath ...string) *redis.Client {
	var basePath string
	if len(confPath) > 0 && confPath[0] != "" {
		basePath = confPath[0]
	} else {
		basePath = constant.DefaultRedisDBConfName
	}

	// 读取配置
	aConf := appCtx.GetConfig()
	addr := aConf.String(basePath+".host") + ":" + aConf.String(basePath+".port")
	return redis.NewClient(&redis.Options{
		Addr:            addr,                                                      // Redis 服务器地址
		Password:        aConf.String(basePath + ".password"),                      // Redis 服务器密码
		DB:              aConf.Int(basePath + ".db"),                               // 使用的数据库编号
		PoolSize:        aConf.Int(basePath + ".poolSize"),                         // 连接池大小
		MinIdleConns:    aConf.Int(basePath + ".minIdleConns"),                     // 最小空闲连接数
		DialTimeout:     aConf.Duration(basePath+".dialTimeout") * time.Second,     // 连接建立超时时间
		ReadTimeout:     aConf.Duration(basePath+".readTimeout") * time.Second,     // 读操作超时时间
		WriteTimeout:    aConf.Duration(basePath+".writeTimeout") * time.Second,    // 写操作超时时间
		PoolTimeout:     aConf.Duration(basePath+".poolTimeout") * time.Second,     // 连接池最大等待时间
		ConnMaxIdleTime: aConf.Duration(basePath+".connMaxIdleTime") * time.Second, // 空闲连接超时时间
		ConnMaxLifetime: aConf.Duration(basePath+".connMaxLifetime") * time.Second, // 连接的最大生命周期
	})
}

// ReNewClient 重新创建 Redis 客户端连接
func (rd *RedisDb) ReNewClient(confPath ...string) (*RedisDb, error) {
	rd.lock.Lock()
	defer rd.lock.Unlock()
	rd.Client = NewClient(rd.Ctx, confPath...)
	return rd, nil
}

// PingTry 尝试 ping Redis 服务器，检查连接是否可用
func (rd *RedisDb) PingTry(ctx context.Context) bool {
	_, err := rd.Client.Ping(ctx).Result()
	if err != nil {
		rd.Ctx.GetLogger().Error(rd.Ctx.GetConfig().LogOriginCache()).Err(err).Msg("Redis PingTry error")
		return false
	}
	return true
}
