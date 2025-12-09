// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache

import (
	"errors"
	"fmt"
)

var (
	ErrKeyNotFound           = errors.New("cache: key not found")
	ErrCacheClosed           = errors.New("cache: cache is closed")
	ErrSerializationFailed   = errors.New("cache: serialization failed")
	ErrDeserializationFailed = errors.New("cache: deserialization failed")
)

// CacheError 包含缓存操作失败的详细信息
type CacheError struct {
	Op  string
	Key string
	Err error
}

func (e *CacheError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("cache %s operation failed for key '%s': %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("cache %s operation failed: %v", e.Op, e.Err)
}

func (e *CacheError) Unwrap() error {
	return e.Err
}

func NewCacheError(op, key string, err error) *CacheError {
	return &CacheError{
		Op:  op,
		Key: key,
		Err: err,
	}
}

// ErrRejectedByBloomFilter 预定义缓存穿透、雪崩时特殊的错误
type ErrRejectedByBloomFilter struct {
	msg string
}

// ErrCircuitBreakerOpen 预定义熔断器打开时的特殊的错误
type ErrCircuitBreakerOpen struct {
	msg string
}

// ErrRedisNil 预定义明确Redis key 不存在的错误
type ErrRedisNil struct {
	msg string
}

func NewErrCircuitBreakerOpen(text string) ErrCircuitBreakerOpen {
	return ErrCircuitBreakerOpen{
		msg: "circuit breaker is open: " + text,
	}
}

func NewErrRejectedByBloomFilter(key string) ErrRejectedByBloomFilter {
	return ErrRejectedByBloomFilter{
		msg: "Cache key: '" + key + "' definitely does not exits",
	}
}

func NewErrRedisNil(key string) ErrRedisNil {
	return ErrRedisNil{
		msg: "Cache key: '" + key + "' is Nil in Redis",
	}
}

func (e ErrRejectedByBloomFilter) Error() string {
	return e.msg
}

func (e ErrCircuitBreakerOpen) Error() string {
	return e.msg
}

func (e ErrRedisNil) Error() string {
	return e.msg
}
