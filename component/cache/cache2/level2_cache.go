// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/lamxy/fiberhouse/component/cache/cachelocal"
	"github.com/panjf2000/ants/v2"
)

const generationShardCount = 64

type generationEntry struct {
	mu         sync.Mutex
	generation uint64
	refs       int
}

type generationShard struct {
	mu      sync.Mutex
	entries map[string]*generationEntry
}

type generationToken struct {
	key        string
	shard      *generationShard
	entry      *generationEntry
	generation uint64
}

// Level2Cache 二级缓存实现（本地+远程）
type Level2Cache struct {
	Ctx    fiberhouse.IContext
	local  cache.Cache
	remote cache.Cache
	closed atomic.Bool
	level  cache.Level

	// 代际条目只在对应键的操作执行期间存在。固定分片限制常驻协调状态，
	// 同时保持按键隔离。
	generationShards [generationShardCount]generationShard

	// 使用ants goroutine池
	localPool  *ants.Pool
	remotePool *ants.Pool

	lifecycleMu  sync.Mutex
	pending      int
	drain        chan struct{}
	admitting    bool
	ownChildren  bool
	shutdownDone chan struct{}
	shutdownErr  error
}

func NewLevel2Cache(appCtx fiberhouse.IContext, local cache.Cache, remote cache.Cache) cache.Cache {
	l2c, err := NewLevel2CacheWithError(appCtx, local, remote)
	if err != nil {
		return nil
	}
	return l2c
}

func NewLevel2CacheWithError(appCtx fiberhouse.IContext, local cache.Cache, remote cache.Cache) (*Level2Cache, error) {
	cfg := appCtx.GetConfig()
	localSize := cfg.Int("cache.asyncPool.ants.local.size")
	remoteSize := cfg.Int("cache.asyncPool.ants.remote.size")
	if localSize <= 0 || remoteSize <= 0 {
		return nil, fmt.Errorf("cache async pool sizes must be positive: local=%d remote=%d", localSize, remoteSize)
	}
	// 创建本地缓存异步操作池（容量size）
	localPool, err := ants.NewPool(
		localSize,
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
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginCache()).
			Err(err).
			Msg("NewLevel2Cache create local goroutine pool error")
		return nil, fmt.Errorf("create local cache async pool: %w", err)
	}

	// 创建远程缓存异步操作池（容量size）
	remotePool, err := ants.NewPool(
		remoteSize,
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
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginCache()).
			Err(err).
			Msg("NewLevel2Cache create remote goroutine pool error")
		return nil, fmt.Errorf("create remote cache async pool: %w", err)
	}

	l2c := &Level2Cache{
		Ctx:        appCtx,
		local:      local,
		remote:     remote,
		closed:     atomic.Bool{},
		level:      cache.Level2,
		localPool:  localPool,
		remotePool: remotePool,
	}
	l2c.initLifecycle(true)
	return l2c, nil
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

	read := l2c.beginGenerationRead(key)

	// 从远程获取
	value, err := l2c.remote.Get(ctx, key, co)

	if err != nil {
		l2c.releaseGeneration(read)
		return "", err
	}

	// 根据策略选择回写方式
	switch co.GetSyncStrategy() {
	case cache.AsyncWriteBoth, cache.AsyncWriteRemoteOnly:
		// 异步写入本地缓存
		l2c.asyncSetLocalIfCurrent(ctx, key, value, co.Clone(), read)
	default:
		l2c.setLocalIfGenerationCurrent(ctx, key, value, co, read)
	}
	return value, nil
}

func (l2c *Level2Cache) generationShard(key string) *generationShard {
	const (
		offset32 = uint32(2166136261)
		prime32  = uint32(16777619)
	)
	hash := offset32
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= prime32
	}
	return &l2c.generationShards[hash%generationShardCount]
}

func (l2c *Level2Cache) acquireGeneration(key string) *generationToken {
	shard := l2c.generationShard(key)
	shard.mu.Lock()
	if shard.entries == nil {
		shard.entries = make(map[string]*generationEntry)
	}
	entry := shard.entries[key]
	if entry == nil {
		entry = &generationEntry{}
		shard.entries[key] = entry
	}
	entry.refs++
	shard.mu.Unlock()
	return &generationToken{key: key, shard: shard, entry: entry}
}

func (l2c *Level2Cache) releaseGeneration(token *generationToken) {
	token.shard.mu.Lock()
	token.entry.refs--
	if token.entry.refs == 0 && token.shard.entries[token.key] == token.entry {
		delete(token.shard.entries, token.key)
	}
	token.shard.mu.Unlock()
}

func (l2c *Level2Cache) beginGenerationRead(key string) *generationToken {
	token := l2c.acquireGeneration(key)
	token.entry.mu.Lock()
	token.generation = token.entry.generation
	token.entry.mu.Unlock()
	return token
}

func (l2c *Level2Cache) setLocalIfGenerationCurrent(ctx context.Context, key string, value interface{}, co *cache.CacheOption, token *generationToken) {
	defer l2c.releaseGeneration(token)
	token.entry.mu.Lock()
	defer token.entry.mu.Unlock()
	if token.entry.generation != token.generation {
		return
	}
	if err := l2c.local.Set(ctx, key, value, co); err != nil {
		l2c.Ctx.GetLogger().InfoWith(l2c.Ctx.GetConfig().LogOriginCache()).Err(err).Msg("getInternal: Set local error")
	}
}

// Set 设置缓存值
func (l2c *Level2Cache) Set(ctx context.Context, key string, value interface{}, co *cache.CacheOption) error {
	if l2c.closed.Load() {
		return cache.ErrCacheClosed
	}

	// 根据策略选择写入方式
	switch co.GetSyncStrategy() {
	case cache.WriteBoth:
		mutation := l2c.acquireGeneration(key)
		defer l2c.releaseGeneration(mutation)
		if err := l2c.remote.Set(ctx, key, value, co); err != nil {
			return fmt.Errorf("remote cache set error: %w", err)
		}
		mutation.entry.mu.Lock()
		defer mutation.entry.mu.Unlock()
		mutation.entry.generation++
		if err := l2c.local.Set(ctx, key, value, co); err != nil {
			return fmt.Errorf("local cache set error: %w", err)
		}
	case cache.WriteRemoteOnly:
		mutation := l2c.acquireGeneration(key)
		defer l2c.releaseGeneration(mutation)
		if err := l2c.remote.Set(ctx, key, value, co); err != nil {
			return fmt.Errorf("remote cache set error: %w", err)
		}
		mutation.entry.mu.Lock()
		defer mutation.entry.mu.Unlock()
		mutation.entry.generation++
		if err := l2c.local.Delete(ctx, key); err != nil {
			return fmt.Errorf("local cache invalidate error: %w", err)
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

func (l2c *Level2Cache) asyncSetLocalIfCurrent(ctx context.Context, key string, value interface{}, co *cache.CacheOption, read *generationToken) {
	if !l2c.admit() {
		l2c.releaseGeneration(read)
		co.Release()
		return
	}

	err := l2c.localPool.Submit(func() {
		defer l2c.complete()
		defer co.Release()
		timeoutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		l2c.setLocalIfGenerationCurrent(timeoutCtx, key, value, co, read)
	})
	if err != nil {
		l2c.complete()
		l2c.releaseGeneration(read)
		co.Release()
		l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
			Str("key", key).
			Err(err).
			Msg("AsyncSetLocal submit failed")
	}
}

// asyncSetLocal 异步写入本地缓存
func (l2c *Level2Cache) asyncSetLocal(ctx context.Context, key string, value interface{}, co *cache.CacheOption) {
	if !l2c.admit() {
		co.Release()
		return
	}

	// 使用ants池提交任务
	err := l2c.localPool.Submit(func() {
		defer l2c.complete()
		defer co.Release()

		// 带超时的context
		timeoutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()

		if err := l2c.local.Set(timeoutCtx, key, value, co); err != nil {
			l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
				Str("key", key).
				Msg("AsyncSetLocal error")
		}
	})

	if err != nil {
		l2c.complete()
		co.Release() // 提交失败时释放资源
		l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
			Str("key", key).
			Err(err).
			Msg("AsyncSetLocal submit failed")
	}
	// TODO fallback go func(){}
}

// asyncSetRemote 异步写入远程缓存
func (l2c *Level2Cache) asyncSetRemote(ctx context.Context, key string, value interface{}, co *cache.CacheOption) {
	if !l2c.admit() {
		co.Release()
		return
	}

	// 使用ants池提交任务
	err := l2c.remotePool.Submit(func() {
		defer l2c.complete()
		defer co.Release()

		// 带超时的context
		timeoutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		if err := l2c.remote.Set(timeoutCtx, key, value, co); err != nil {
			l2c.Ctx.GetLogger().Error(l2c.Ctx.GetConfig().LogOriginCache()).
				Str("key", key).
				Err(err).
				Msg("AsyncSetRemote error")
		}
	})

	if err != nil {
		l2c.complete()
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

	mutations := make([]*generationToken, len(keys))
	for i, key := range keys {
		mutations[i] = l2c.acquireGeneration(key)
	}
	defer func() {
		for _, mutation := range mutations {
			l2c.releaseGeneration(mutation)
		}
	}()

	if err := l2c.remote.Delete(ctx, keys...); err != nil {
		return fmt.Errorf("remote cache delete error: %w", err)
	}

	var deleteErrors []error
	for i, key := range keys {
		mutation := mutations[i]
		mutation.entry.mu.Lock()
		mutation.entry.generation++
		if err := l2c.local.Delete(ctx, key); err != nil {
			deleteErrors = append(deleteErrors, fmt.Errorf("local cache delete %q error: %w", key, err))
		}
		mutation.entry.mu.Unlock()
	}
	return errors.Join(deleteErrors...)
}

// Close 关闭缓存实例，释放资源（应用退出时调用Close关闭资源）
func (l2c *Level2Cache) Close() error {
	done, drain, leader := l2c.beginShutdown()
	if leader {
		go l2c.finishShutdown(drain)
	}
	<-done
	if !leader {
		return nil
	}
	l2c.lifecycleMu.Lock()
	err := l2c.shutdownErr
	l2c.lifecycleMu.Unlock()
	return err
}

func (l2c *Level2Cache) initLifecycle(ownChildren bool) {
	l2c.lifecycleMu.Lock()
	l2c.admitting = true
	l2c.ownChildren = ownChildren
	l2c.drain = make(chan struct{})
	close(l2c.drain)
	l2c.shutdownDone = make(chan struct{})
	l2c.lifecycleMu.Unlock()
}

func (l2c *Level2Cache) admit() bool {
	l2c.lifecycleMu.Lock()
	defer l2c.lifecycleMu.Unlock()
	if !l2c.admitting {
		return false
	}
	if l2c.pending == 0 {
		l2c.drain = make(chan struct{})
	}
	l2c.pending++
	return true
}

func (l2c *Level2Cache) complete() {
	l2c.lifecycleMu.Lock()
	l2c.pending--
	if l2c.pending == 0 {
		close(l2c.drain)
	}
	l2c.lifecycleMu.Unlock()
}

func (l2c *Level2Cache) Flush(ctx context.Context) error {
	l2c.lifecycleMu.Lock()
	drain := l2c.drain
	l2c.lifecycleMu.Unlock()
	select {
	case <-drain:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l2c *Level2Cache) Shutdown(ctx context.Context) error {
	done, drain, leader := l2c.beginShutdown()
	if leader {
		go l2c.finishShutdown(drain)
	}
	select {
	case <-done:
		l2c.lifecycleMu.Lock()
		err := l2c.shutdownErr
		l2c.lifecycleMu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l2c *Level2Cache) beginShutdown() (done, drain chan struct{}, leader bool) {
	l2c.lifecycleMu.Lock()
	defer l2c.lifecycleMu.Unlock()
	if !l2c.admitting {
		return l2c.shutdownDone, l2c.drain, false
	}
	l2c.admitting = false
	l2c.closed.Store(true)
	return l2c.shutdownDone, l2c.drain, true
}

func (l2c *Level2Cache) finishShutdown(drain chan struct{}) {
	<-drain
	l2c.localPool.Release()
	l2c.remotePool.Release()
	var shutdownErrors []error
	if l2c.ownChildren {
		if err := l2c.local.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("local cache close error: %w", err))
		}
		if err := l2c.remote.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("remote cache close error: %w", err))
		}
	}
	l2c.lifecycleMu.Lock()
	l2c.shutdownErr = errors.Join(shutdownErrors...)
	close(l2c.shutdownDone)
	l2c.lifecycleMu.Unlock()
}

// Wait 当写入后立即读取时，等待底层缓存完成写入操作
func (l2c *Level2Cache) Wait() error {
	if l2c.closed.Load() {
		return cache.ErrCacheClosed
	}
	if err := l2c.Flush(context.Background()); err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := l2c.local.Wait(); err != nil {
			errCh <- fmt.Errorf("local cache wait error: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := l2c.remote.Wait(); err != nil {
			errCh <- fmt.Errorf("remote cache wait error: %w", err)
		}
	}()

	wg.Wait()
	close(errCh)
	waitErrors := make([]error, 0, 2)
	for err := range errCh {
		waitErrors = append(waitErrors, err)
	}
	return errors.Join(waitErrors...)
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
