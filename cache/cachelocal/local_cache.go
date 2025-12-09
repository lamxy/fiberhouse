// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cachelocal

import (
	"context"
	"errors"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/cache"
	"github.com/lamxy/fiberhouse/constant"
	frameUtils "github.com/lamxy/fiberhouse/utils"
	"sync/atomic"
)

// LocalCache 本地缓存实现，基于 ristretto
type LocalCache struct {
	client *ristretto.Cache[string, []byte]
	Ctx    fiberhouse.IContext
	closed atomic.Bool
	level  cache.Level
}

func NewLocalCache(appCtx fiberhouse.IContext, confPath ...string) (cache.Cache, error) {
	var basePath string
	if len(confPath) > 0 && confPath[0] != "" {
		basePath = confPath[0]
	} else {
		basePath = constant.DefaultLocalCacheConfName
	}

	aConf := appCtx.GetConfig()

	ristrettoConfig := &ristretto.Config[string, []byte]{
		NumCounters:        aConf.Int64(basePath + ".numCounters"),
		MaxCost:            aConf.Int64(basePath + ".maxCost"),
		BufferItems:        aConf.Int64(basePath + ".bufferItems"),
		Metrics:            aConf.Bool(basePath + ".metrics"),
		IgnoreInternalCost: aConf.Bool(basePath + ".ignoreInternalCost"),
	}

	cached, err := ristretto.NewCache(ristrettoConfig)
	if err != nil {
		return nil, cache.NewCacheError("create", "", err)
	}

	return &LocalCache{
		client: cached,
		Ctx:    appCtx,
		closed: atomic.Bool{},
		level:  cache.Local,
	}, nil
}

// GetLevel 获取缓存级别
func (lc *LocalCache) GetLevel() cache.Level {
	return lc.level
}

// Get 获取缓存值
func (lc *LocalCache) Get(ctx context.Context, key string, co *cache.CacheOption) (string, error) {
	if lc.closed.Load() {
		return "", cache.ErrCacheClosed
	}

	value, found := lc.client.Get(key)
	if !found {
		return "", cache.ErrKeyNotFound
	}

	//return string(value), nil
	return frameUtils.UnsafeString(value), nil
}

// Set 设置缓存值
func (lc *LocalCache) Set(ctx context.Context, key string, value interface{}, co *cache.CacheOption) error {
	if lc.closed.Load() {
		return cache.ErrCacheClosed
	}

	// 序列化值
	var serializedValue []byte
	var err error

	switch v := value.(type) {
	case string:
		serializedValue = frameUtils.UnsafeBytes(v)
	case []byte:
		serializedValue = v
	default:
		serializedValue, err = co.GetJsonWrapper().Marshal(value)
		if err != nil {
			return cache.NewCacheError("serialize", key, err)
		}
	}

	// 计算成本
	itemCost := int64(len(serializedValue))
	ttl := co.GetLocalTTL()
	// 设置缓存项
	success := lc.client.SetWithTTL(key, serializedValue, itemCost, ttl)
	if !success {
		return cache.NewCacheError("set", key, errors.New("failed to set cache item"))
	}

	return nil
}

// Delete 删除缓存项
func (lc *LocalCache) Delete(ctx context.Context, keys ...string) error {
	if lc.closed.Load() {
		return cache.ErrCacheClosed
	}

	for _, key := range keys {
		lc.client.Del(key)
	}

	return nil
}

// Close 关闭缓存
func (lc *LocalCache) Close() error {
	if lc.closed.Load() {
		return nil
	}

	lc.client.Close()
	lc.closed.Store(true)
	return nil
}

// Wait 等待所有待处理的缓存操作完成，当Set后需要立即读取Get时，调用此方法等待
func (lc *LocalCache) Wait() error {
	if lc.closed.Load() {
		return cache.ErrCacheClosed
	}

	lc.client.Wait()
	return nil
}

// GetMetrics 获取原始缓存统计信息
func (lc *LocalCache) GetMetrics(co *cache.CacheOption) *ristretto.Metrics {
	if lc.closed.Load() {
		return nil
	}
	return lc.client.Metrics
}

// GetMetricsInfo 获取详细的缓存统计信息
func (lc *LocalCache) GetMetricsInfo(co *cache.CacheOption) *CacheMetricsInfo {
	if lc.closed.Load() {
		return nil
	}

	metrics := lc.client.Metrics
	if metrics == nil {
		return nil
	}

	totalRequests := metrics.Hits() + metrics.Misses()
	var hitRatio, missRatio float64

	if totalRequests > 0 {
		hitRatio = float64(metrics.Hits()) / float64(totalRequests)
		missRatio = float64(metrics.Misses()) / float64(totalRequests)
	}

	return &CacheMetricsInfo{
		HitRatio:      hitRatio,
		MissRatio:     missRatio,
		TotalHits:     metrics.Hits(),
		TotalMisses:   metrics.Misses(),
		TotalRequests: totalRequests,
		KeysAdded:     metrics.KeysAdded(),
		KeysEvicted:   metrics.KeysEvicted(),
		KeysUpdated:   metrics.KeysUpdated(),
		CostAdded:     metrics.CostAdded(),
		CostEvicted:   metrics.CostEvicted(),
		SetsDropped:   metrics.SetsDropped(),
		SetsRejected:  metrics.SetsRejected(),
		GetsDropped:   metrics.GetsDropped(),
		GetsKept:      metrics.GetsKept(),
	}
}
