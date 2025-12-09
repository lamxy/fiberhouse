// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache

import (
	"context"
	"github.com/lamxy/fiberhouse"
	"github.com/redis/go-redis/v9"
)

// Cache 缓存接口，定义了通用接口的方法
// 定义Get、Set、Del、Wait、Close等
type Cache interface {
	Get(ctx context.Context, key string, co *CacheOption) (string, error)
	Set(ctx context.Context, key string, value interface{}, co *CacheOption) error
	Delete(ctx context.Context, keys ...string) error
	Close() error
	Wait() error
	GetLevel() Level
	// more
}

// IRedisClient Redis客户端接口，定义获取Redis客户端的方法
type IRedisClient interface {
	GetRedisClient() *redis.Client
}

// CacheLocator 缓存定位器接口，继承自Locator接口
type CacheLocator interface {
	fiberhouse.Locator
	GetRemote() Cache
	GetLocal() Cache
	GetLevel2() Cache
	SetOrigin(locator fiberhouse.Locator) fiberhouse.Locator
	GetOrigin() fiberhouse.Locator
}

// CacheBloomFilter 布隆过滤器接口
type CacheBloomFilter interface {
	// Add 添加Key到布隆过滤器
	Add(key []byte)
	// Test 测试Key是否存在于布隆过滤器中，可能存在误判
	Test(key []byte) bool
	// TestAndAdd 测试Key是否存在于布隆过滤器中，若不存在则添加Key，返回true表示Key可能存在，false表示Key一定不存在
	TestAndAdd(key []byte) bool
	// Reset 如有需要，重置布隆过滤器
	Reset()
}

// CacheCircuitBreaker 熔断器接口
type CacheCircuitBreaker interface {
	// Call 受保护的方法调用
	Call(fn func() (string, error)) (interface{}, error)
	// ConvertCircuitBreakerOpenError 将熔断错误识别和转换为"熔断器打开时错误"，见 cache.ErrCircuitBreakerOpen
	ConvertCircuitBreakerOpenError(err error) error
	// Allow 如有需要，作为允许条件打开熔断器，返回true，否则返回false
	Allow() bool
	// Reset 如有需要，重置熔断器的状态
	Reset()
}
