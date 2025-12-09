// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache2

import (
	"context"
	"fmt"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/cache"
	"github.com/lamxy/fiberhouse/cache/cachelocal"
	"github.com/panjf2000/ants/v2"
	"sync"
	"sync/atomic"
	"time"
)

// Level2Cache 二级缓存实现（本地+远程）
type Level2Cache struct {
	Ctx    fiberhouse.IContext
	local  cache.Cache
	remote cache.Cache
	closed atomic.Bool
	level  cache.Level

	// 使用ants goroutine池
	localPool  *ants.Pool
	remotePool *ants.Pool
	stopCh     chan struct{}
}

func NewLevel2Cache(appCtx fiberhouse.IContext, local cache.Cache, remote cache.Cache) cache.Cache {
	cfg := appCtx.GetConfig()
	// 创建本地缓存异步操作池（容量size）
	localPool, err := ants.NewPool(
		cfg.Int("cache.asyncPool.ants.local.size"),
		ants.WithOptions(ants.Options{
			ExpiryDuration:   time.Duration(cfg.Int("cache.asyncPool.ants.local.expiryDuration")) * time.Second,
			PreAlloc:         cfg.Bool("cache.asyncPool.ants.local.preAlloc"),
			MaxBlockingTasks: cfg.Int("cache.asyncPool.ants.local.maxBlockingTasks"),
			Nonblocking:      cfg.Bool("cache.asyncPool.ants.local.nonblocking"),
			PanicHandler: func(i interface{}) {
				appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginCache()).
					Interface("panic", i).
					Msg("LocalPool goroutine panic")
			},
		}),
	)

	if err != nil {
		appCtx.GetLogger().Fatal(appCtx.GetConfig().LogOriginCache()).
			Err(err).
			Msg("NewLevel2Cache create local goroutine pool error")
		return nil
	}

	// 创建远程缓存异步操作池（容量size）
	remotePool, err := ants.NewPool(
		cfg.Int("cache.asyncPool.ants.remote.size"),
		ants.WithOptions(ants.Options{
			ExpiryDuration:   time.Duration(cfg.Int("cache.asyncPool.ants.remote.expiryDuration")) * time.Second,
			PreAlloc:         cfg.Bool("cache.asyncPool.ants.remote.preAlloc"),
			MaxBlockingTasks: cfg.Int("cache.asyncPool.ants.remote.maxBlockingTasks"),
			Nonblocking:      cfg.Bool("cache.asyncPool.ants.remote.nonblocking"),
			PanicHandler: func(i interface{}) {
				appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginCache()).
					Interface("panic", i).
					Msg("RemotePool goroutine panic")
			},
		}),
	)

	if err != nil {
		localPool.Release()
		appCtx.GetLogger().Fatal(appCtx.GetConfig().LogOriginCache()).
			Err(err).
			Msg("NewLevel2Cache create remote goroutine pool error")
		return nil
	}

	l2c := &Level2Cache{
		Ctx:        appCtx,
		local:      local,
		remote:     remote,
		closed:     atomic.Bool{},
		level:      cache.Level2,
		localPool:  localPool,
		remotePool: remotePool,
		stopCh:     make(chan struct{}),
	}

	return l2c
}

// GetLevel 获取缓存级别
func (l2c *Level2Cache) GetLevel() cache.Level {
	return l2c.level
}

// GetLocal 获取本地缓存实例
func (l2c *Level2Cache) GetLocal() cache.Cache {
	return l2c.local
}

// GetRemote 获取远程缓存实例
func (l2c *Level2Cache) GetRemote() cache.Cache {
	return l2c.remote
}

// Get 获取缓存值
func (l2c *Level2Cache) Get(ctx context.Context, key string, co *cache.CacheOption) (string, error) {
	if l2c.closed.Load() {
		return "", cache.ErrCacheClosed
	}
	// 直接获取
	return l2c.getInternal(ctx, key, co)
}

func (l2c *Level2Cache) getInternal(ctx context.Context, key string, co *cache.CacheOption) (string, error) {
	// 优先从本地获取
	if value, err := l2c.local.Get(ctx, key, co); err == nil {
		return value, nil
	}

	// 从远程获取
	value, err := l2c.remote.Get(ctx, key, co)

	if err != nil {
		return "", err
	}

	// 根据策略选择回写方式
	switch co.GetSyncStrategy() {
	case cache.AsyncWriteBoth, cache.AsyncWriteRemoteOnly:
		// 异步写入本地缓存
		l2c.asyncSetLocal(ctx, key, value, co.Clone())
	default:
		// 同步写入本地缓存
		err := l2c.local.Set(ctx, key, value, co)
		if err != nil {
			l2c.Ctx.GetLogger().InfoWith(l2c.Ctx.GetConfig().LogOriginCache()).Err(err).Msg("getInternal: Set local error")
		}
	}
	return value, nil
}

// Set 设置缓存值
func (l2c *Level2Cache) Set(ctx context.Context, key string, value interface{}, co *cache.CacheOption) error {
	if l2c.closed.Load() {
		return cache.ErrCacheClosed
	}

	// 根据策略选择写入方式
	switch co.GetSyncStrategy() {
	case cache.WriteBoth:
		// 同步双写两级缓存
		var wg sync.WaitGroup
		errCh := make(chan error, 2)

		wg.Add(2)

		// 并行写入本地缓存
		go func() {
			defer wg.Done()
			if err := l2c.local.Set(ctx, key, value, co); err != nil {
				errCh <- err
			}
		}()

		// 并行写入远程缓存
		go func() {
			defer wg.Done()
			if err := l2c.remote.Set(ctx, key, value, co); err != nil {
				errCh <- err
			}
		}()

		wg.Wait()
		close(errCh)

		var errors []error
		for err := range errCh {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			return fmt.Errorf("cache set errors: %v", errors)
		}
	case cache.WriteRemoteOnly:
		// 同步写远程
		if err := l2c.remote.Set(ctx, key, value, co); err != nil {
			return err
		}
	case cache.AsyncWriteBoth:
		// 异步写入本地和远程缓存
		l2c.asyncSetLocal(ctx, key, value, co.Clone())
		l2c.asyncSetRemote(ctx, key, value, co.Clone())
	case cache.AsyncWriteRemoteOnly:
		// 异步只写远程
		l2c.asyncSetRemote(ctx, key, value, co.Clone())
	default:
		return fmt.Errorf("unsupported sync strategy: %d", co.GetSyncStrategy())
	}

	return nil
}

// asyncSetLocal 异步写入本地缓存
func (l2c *Level2Cache) asyncSetLocal(ctx context.Context, key string, value interface{}, co *cache.CacheOption) {
	// 首先检查是否已关闭
	select {
	case <-l2c.stopCh:
		co.Release()
		return
	default:
	}

	// 使用ants池提交任务
	err := l2c.localPool.Submit(func() {
		defer co.Release()

		// 带超时的context
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		if err := l2c.local.Set(timeoutCtx, key, value, co); err != nil {
			l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
				Str("key", key).
				Msg("AsyncSetLocal error")
		}
	})

	if err != nil {
		co.Release() // 提交失败时释放资源
		l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
			Str("key", key).
			Err(err).
			Msg("AsyncSetLocal submit failed")
	}
}

// asyncSetRemote 异步写入远程缓存
func (l2c *Level2Cache) asyncSetRemote(ctx context.Context, key string, value interface{}, co *cache.CacheOption) {
	// 首先检查是否已关闭
	select {
	case <-l2c.stopCh:
		co.Release()
		return
	default:
	}

	// 使用ants池提交任务
	err := l2c.remotePool.Submit(func() {
		defer co.Release()

		// 带超时的context
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		if err := l2c.remote.Set(timeoutCtx, key, value, co); err != nil {
			l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
				Str("key", key).
				Err(err).
				Msg("AsyncSetRemote error")
		}
	})

	if err != nil {
		co.Release() // 提交失败时释放资源
		l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
			Str("key", key).
			Err(err).
			Msg("AsyncSetRemote submit failed")
	}
}

// Delete 删除缓存值
func (l2c *Level2Cache) Delete(ctx context.Context, keys ...string) error {
	if l2c.closed.Load() {
		return cache.ErrCacheClosed
	}

	var wg sync.WaitGroup
	var errors []error
	var mu sync.Mutex

	// 同时删除两级缓存
	wg.Add(2)

	// 删除本地缓存
	go func() {
		defer wg.Done()
		if err := l2c.local.Delete(ctx, keys...); err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("local delete error: %w", err))
			mu.Unlock()
		}
	}()

	// 删除远程缓存
	go func() {
		defer wg.Done()
		err := l2c.remote.Delete(ctx, keys...)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("remote delete error: %w", err))
			mu.Unlock()
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("cache delete errors: %v", errors)
	}

	return nil
}

// Close 关闭缓存实例，释放资源（应用退出时调用Close关闭资源）
func (l2c *Level2Cache) Close() error {
	//if !l2c.closed.CompareAndSwap(false, true) {
	//	return nil // 已关闭
	//}

	if l2c.closed.Load() {
		return nil
	}

	// 停止接收新任务
	close(l2c.stopCh)

	// 等待池中的任务完成（带超时）
	timeout := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if l2c.localPool.Running() == 0 && l2c.remotePool.Running() == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 关闭goroutine池
	l2c.localPool.Release()
	l2c.remotePool.Release()

	// 关闭底层缓存
	var errors []error
	if err := l2c.local.Close(); err != nil {
		errors = append(errors, fmt.Errorf("local cache close error: %w", err))
	}
	if err := l2c.remote.Close(); err != nil {
		errors = append(errors, fmt.Errorf("remote cache close error: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("close errors: %v", errors)
	}
	return nil
}

// Wait 当写入后立即读取时，等待底层缓存完成写入操作
func (l2c *Level2Cache) Wait() error {
	if l2c.closed.Load() {
		return cache.ErrCacheClosed
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = l2c.local.Wait()
	}()

	go func() {
		defer wg.Done()
		_ = l2c.remote.Wait()
	}()

	wg.Wait()
	return nil
}

// GetLocalMetrics 获取本地缓存指标
func (l2c *Level2Cache) GetLocalMetrics(co *cache.CacheOption) *ristretto.Metrics {
	return l2c.local.(*cachelocal.LocalCache).GetMetrics(co)
}

// GetLocalMetricsInfo 获取本地缓存指标
func (l2c *Level2Cache) GetLocalMetricsInfo(co *cache.CacheOption) *cachelocal.CacheMetricsInfo {
	return l2c.local.(*cachelocal.LocalCache).GetMetricsInfo(co)
}

// GetPoolMetrics 获取goroutine池指标
func (l2c *Level2Cache) GetPoolMetrics() map[string]interface{} {
	return map[string]interface{}{
		"local_pool": map[string]interface{}{
			"capacity": l2c.localPool.Cap(),
			"running":  l2c.localPool.Running(),
			"free":     l2c.localPool.Free(),
		},
		"remote_pool": map[string]interface{}{
			"capacity": l2c.remotePool.Cap(),
			"running":  l2c.remotePool.Running(),
			"free":     l2c.remotePool.Free(),
		},
		"total_capacity": l2c.localPool.Cap() + l2c.remotePool.Cap(),
		"total_running":  l2c.localPool.Running() + l2c.remotePool.Running(),
	}
}
