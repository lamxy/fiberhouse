// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse"
	frameUtils "github.com/lamxy/fiberhouse/utils"
)

// GetCached 通用缓存获取函数
func GetCached[R any](
	cacheOption *CacheOption,
	loader func(context.Context) (R, error),
	fallback ...func() (R, error),
) (R, error) {
	//defer cacheOption.Release()
	var zero R

	// 验证缓存选项
	err := cacheOption.Valid()
	if err != nil {
		return zero, err
	}

	// 如果缓存未启用，直接调用loader
	if !cacheOption.IsCache() {
		return loader(cacheOption.GetContextCtx())
	}

	// 根据策略获取相应的缓存实例
	var cacheInstance Cache
	switch cacheOption.GetCacheLevel() {
	case Local:
		cacheInstance = fiberhouse.GetMustInstance[Cache](cacheOption.GetContext().GetStarter().GetApplication().GetLocalCacheKey())
	case Remote:
		cacheInstance = fiberhouse.GetMustInstance[Cache](cacheOption.GetContext().GetStarter().GetApplication().GetRemoteCacheKey())
	case Level2:
		cacheInstance = fiberhouse.GetMustInstance[Cache](cacheOption.GetContext().GetStarter().GetApplication().GetLevel2CacheKey())
	default:
		return zero, fmt.Errorf("unsupported cache level: %d", cacheOption.GetCacheLevel())
	}

	// 尝试从缓存获取数据
	var (
		data     R
		jsonData string
	)

	jsonData, err = cacheInstance.Get(cacheOption.GetContextCtx(), cacheOption.GetCacheKey(), cacheOption)
	if err != nil {
		// 判断是否时特殊拦截错误
		// 是否被布隆过滤器拦截，避免缓存穿透
		var errRejectedByBloomFilter ErrRejectedByBloomFilter
		if errors.As(err, &errRejectedByBloomFilter) {
			return zero, errRejectedByBloomFilter
		}
		// 是否被熔断器拦截，避免缓存雪崩
		var errCircuitBreakerOpen ErrCircuitBreakerOpen
		if errors.As(err, &errCircuitBreakerOpen) {
			if len(fallback) > 0 {
				return fallback[0]()
			}
			return zero, errCircuitBreakerOpen
		}
		// 其他错误，视为缓存未命中，调用loader获取数据
		data, err = loader(cacheOption.GetContextCtx())
		if err != nil {
			return zero, err
		}

		// 序列化并存入缓存
		jsonBytes, err := cacheOption.GetJsonWrapper().Marshal(data)
		if err != nil {
			return zero, err
		}
		jsonData = frameUtils.UnsafeString(jsonBytes)

		// 写入缓存
		err = cacheInstance.Set(cacheOption.GetContextCtx(), cacheOption.GetCacheKey(), jsonData, cacheOption)

		if err != nil {
			// 记录日志，但不影响正常返回数据
			if cacheOption.GetContext() != nil {
				cacheOption.GetContext().GetLogger().Error(cacheOption.GetContext().GetConfig().LogOriginCache()).Msgf("failed to set cache for key %s: %v", cacheOption.GetCacheKey(), err)
			}
		}

		return data, nil
	}

	// 反序列化缓存数据
	err = cacheOption.GetJsonWrapper().Unmarshal(frameUtils.UnsafeBytes(jsonData), &data)
	if err != nil {
		return zero, err
	}

	return data, nil
}

// Factory 缓存工厂，根据缓存级别返回相应的缓存实例
type Factory struct {
	ctx fiberhouse.IContext
}

// NewFactory 创建缓存工厂
func NewFactory(ctx fiberhouse.IContext) *Factory {
	return &Factory{ctx: ctx}
}

// GetCache 根据缓存级别获取相应的缓存实例
func (cf *Factory) GetCache(l Level) Cache {
	switch l {
	case Local:
		return fiberhouse.GetMustInstance[Cache](cf.ctx.GetStarter().GetApplication().GetLocalCacheKey())
	case Remote:
		return fiberhouse.GetMustInstance[Cache](cf.ctx.GetStarter().GetApplication().GetRemoteCacheKey())
	case Level2:
		return fiberhouse.GetMustInstance[Cache](cf.ctx.GetStarter().GetApplication().GetLevel2CacheKey())
	default:
		panic(fmt.Sprintf("unsupported cache level: %d", l))
	}
}
