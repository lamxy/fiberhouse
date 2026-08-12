// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cacheremote

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/lamxy/fiberhouse/constant"
	frameUtils "github.com/lamxy/fiberhouse/utils"
	"github.com/redis/go-redis/v9"
)

// RedisDb 实现了 Cache和全局管理器相关 接口
type RedisDb struct {
	Client       *redis.Client
	Ctx          fiberhouse.IContext
	lock         sync.RWMutex
	confPathname string
	// 缓存调用close关闭则阻止所有方法执行逻辑
	closed         atomic.Bool
	level          cache.Level
	circuitBreaker cache.CacheCircuitBreaker
}

type redisOptions struct {
	confPath       string
	circuitBreaker cache.CacheCircuitBreaker
}

// RedisOption configures a RedisDb without changing the legacy NewRedisDb
// constructor signature.
type RedisOption func(*redisOptions)

// WithRedisConfigPath selects the Redis configuration subtree.
func WithRedisConfigPath(path string) RedisOption {
	return func(options *redisOptions) {
		if path != "" {
			options.confPath = path
		}
	}
}

// WithRedisCircuitBreaker installs the per-instance breaker used by requests
// that enable circuit-breaker protection.
func WithRedisCircuitBreaker(breaker cache.CacheCircuitBreaker) RedisOption {
	return func(options *redisOptions) {
		if !isNilCircuitBreaker(breaker) {
			options.circuitBreaker = breaker
		}
	}
}

func NewRedisDb(appCtx fiberhouse.IContext, confPath ...string) (cache.Cache, error) {
	options := make([]RedisOption, 0, 1)
	if len(confPath) > 0 {
		options = append(options, WithRedisConfigPath(confPath[0]))
	}
	return NewRedisDbWithOptions(appCtx, options...)
}

// NewRedisDbWithOptions constructs a Redis backend with explicit optional
// dependencies. The breaker remains disabled per request unless the caller also
// enables it on CacheOption.
func NewRedisDbWithOptions(appCtx fiberhouse.IContext, optionFns ...RedisOption) (*RedisDb, error) {
	options := redisOptions{confPath: constant.DefaultRedisDBConfName}
	for _, option := range optionFns {
		if option != nil {
			option(&options)
		}
	}
	ca := &RedisDb{
		Client:         NewClient(appCtx, options.confPath),
		Ctx:            appCtx,
		level:          cache.Remote,
		circuitBreaker: options.circuitBreaker,
	}
	ca.confPathname = options.confPath
	return ca, nil
}

// GetConfPath 获取 Redis 配置路径
func (rd *RedisDb) GetConfPath() string {
	return rd.confPathname
}

// GetRedisClient 获取底层 Redis 客户端实例
func (rd *RedisDb) GetRedisClient() *redis.Client {
	rd.lock.RLock()
	client := rd.Client
	rd.lock.RUnlock()
	return client
}

// GetLevel 获取缓存级别
func (rd *RedisDb) GetLevel() cache.Level {
	return rd.level
}

// Close 关闭 Redis 客户端连接
// 谨慎使用Close关闭链接
func (rd *RedisDb) Close() error {
	rd.lock.Lock()
	defer rd.lock.Unlock()
	if rd.closed.Load() {
		return nil
	}
	rd.closed.Store(true)
	client := rd.Client
	rd.Client = nil
	if client == nil {
		return nil
	}
	return client.Close()
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
	client, err := rd.clientSnapshot()
	if err != nil {
		return "", err
	}
	return rd.getInternal(ctx, client, key, co)
}

// getInternal 获取缓存内部方法
func (rd *RedisDb) getInternal(ctx context.Context, client *redis.Client, key string, co *cache.CacheOption) (string, error) {
	if co.GetCircuitBreakerState() && rd.circuitBreaker != nil {
		var operationErr error
		value, err := rd.circuitBreaker.Call(func() (string, error) {
			var value string
			value, operationErr = client.Get(ctx, key).Result()
			if errors.Is(operationErr, redis.Nil) || isCallerCancellation(ctx, operationErr) {
				return "", nil
			}
			return value, operationErr
		})
		if err != nil {
			return "", err
		}
		if operationErr != nil {
			return "", normalizeRedisGetError(key, operationErr)
		}
		return value.(string), nil
	}

	s, err := client.Get(ctx, key).Result()
	return s, normalizeRedisGetError(key, err)
}

// Set 设置指定 key 的值
func (rd *RedisDb) Set(ctx context.Context, key string, value interface{}, co *cache.CacheOption) error {
	client, err := rd.clientSnapshot()
	if err != nil {
		return err
	}

	// 序列化值
	serializedValue, err := rd.serializeValue(value, co)
	if err != nil {
		return err
	}

	// 执行设置操作
	return rd.setInternal(ctx, client, key, serializedValue, co)
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
func (rd *RedisDb) setInternal(ctx context.Context, client *redis.Client, key, value string, co *cache.CacheOption) error {
	return client.Set(ctx, key, value, co.GetRemoteTTL()).Err()
}

// Delete 删除指定的 key
func (rd *RedisDb) Delete(ctx context.Context, keys ...string) error {
	client, err := rd.clientSnapshot()
	if err != nil {
		return err
	}
	return client.Del(ctx, keys...).Err()
}

// Wait Redis 无需等待，直接返回 nil
func (rd *RedisDb) Wait() error {
	if rd.closed.Load() {
		return cache.ErrCacheClosed
	}
	return nil
}

// NewClient 创建一个新的 Redis 客户端连接
func NewClient(appCtx fiberhouse.IContext, confPath ...string) *redis.Client {
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
	path := rd.confPathname
	if len(confPath) > 0 && confPath[0] != "" {
		path = confPath[0]
	}
	client := NewClient(rd.Ctx, path)
	rd.lock.Lock()
	defer rd.lock.Unlock()
	if rd.closed.Load() {
		_ = client.Close()
		return rd, cache.ErrCacheClosed
	}
	old := rd.Client
	rd.Client = client
	if old != nil {
		_ = old.Close()
	}
	return rd, nil
}

// PingTry 尝试 ping Redis 服务器，检查连接是否可用
func (rd *RedisDb) PingTry(ctx context.Context) bool {
	client, err := rd.clientSnapshot()
	if err != nil {
		return false
	}
	_, err = client.Ping(ctx).Result()
	if err != nil {
		rd.Ctx.GetLogger().Error(rd.Ctx.GetConfig().LogOriginCache()).Err(err).Msg("Redis PingTry error")
		return false
	}
	return true
}

func (rd *RedisDb) clientSnapshot() (*redis.Client, error) {
	rd.lock.RLock()
	defer rd.lock.RUnlock()
	if rd.closed.Load() || rd.Client == nil {
		return nil, cache.ErrCacheClosed
	}
	return rd.Client, nil
}

func normalizeRedisGetError(key string, err error) error {
	if errors.Is(err, redis.Nil) {
		return cache.NewErrRedisNil(key)
	}
	return err
}

func isCallerCancellation(ctx context.Context, err error) bool {
	ctxErr := ctx.Err()
	return ctxErr != nil && errors.Is(err, ctxErr)
}

func isNilCircuitBreaker(breaker cache.CacheCircuitBreaker) bool {
	if breaker == nil {
		return true
	}
	value := reflect.ValueOf(breaker)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
