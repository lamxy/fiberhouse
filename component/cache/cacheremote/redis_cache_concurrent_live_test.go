//go:build liveintegration

// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cacheremote

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/component/cache"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/stretchr/testify/require"
)

// TestLive_RedisDb_ConcurrentReadWriteWithHealthCheck 针对真实 Redis 容器验证并发
// 读写与并发健康检查同时进行时的行为。
//
// 为什么需要这个用例：RedisDb 刻意不实现 globalmanager.Rebuilder，其正确性论证是
// 「Client 构造后不再被替换，因此读侧无需担心实例被换走，失效恢复交给 go-redis 的
// 连接池与重试」。既有 live 测试只跑单 goroutine 的顺序读写，完全没有验证这个论证。
// 本用例让 N 个 goroutine 并发读写，同时另一组 goroutine 持续调用 IsHealthy
// （健康检查正是框架后台会做的事），确保：
//
//   - 并发读写不产生错误，也不互相污染（每个 goroutine 用自己的 key 空间）
//   - 健康检查与业务流量并发进行时不干扰彼此
//   - 整个过程在 -race 下无数据竞争
//
// 配合 `go test -race -tags=liveintegration` 运行，这个用例才同时覆盖并发正确性
// 与内存模型安全。
func TestLive_RedisDb_ConcurrentReadWriteWithHealthCheck(t *testing.T) {
	rd := newLiveTestRedisDb(t)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	require.True(t, rd.PingTry(pingCtx), "must be able to reach the live Redis container")

	const (
		writers            = 8
		opsPerWriter       = 25
		healthCheckers     = 2
		healthCheckMinRuns = 1
	)

	co := cache.NewCacheOption(rd.Ctx).SetJsonWrapper(jsoncodec.StdJsonDefault()).SetRemoteTTL(time.Minute)
	prefix := fmt.Sprintf("concurrent-live-%s-%d", t.Name(), time.Now().UnixNano())

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for w := 0; w < writers; w++ {
			for i := 0; i < opsPerWriter; i++ {
				_ = rd.Delete(cleanupCtx, fmt.Sprintf("%s-w%d-k%d", prefix, w, i))
			}
		}
	})

	// stop 在读写结束后关闭，用于让健康检查 goroutine 退出。
	// 读写者与健康检查者分属两个 WaitGroup：必须先等读写跑完，才能停止健康检查，
	// 否则健康检查可能在业务流量开始前就退出，用例就证明不了「并发」。
	stop := make(chan struct{})
	var (
		writersWG    sync.WaitGroup
		healthWG     sync.WaitGroup
		mu           sync.Mutex
		failures     []string
		healthRuns   int
		healthyCount int
	)

	recordFailure := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		failures = append(failures, fmt.Sprintf(format, args...))
	}

	// 并发读写：每个 goroutine 只操作自己的 key 空间，写入后立即读回校验。
	for w := 0; w < writers; w++ {
		writersWG.Add(1)
		go func(worker int) {
			defer writersWG.Done()
			for i := 0; i < opsPerWriter; i++ {
				key := fmt.Sprintf("%s-w%d-k%d", prefix, worker, i)
				want := fmt.Sprintf("value-w%d-i%d", worker, i)

				setCtx, setCancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := rd.Set(setCtx, key, want, co)
				setCancel()
				if err != nil {
					recordFailure("Set(%s) error = %v", key, err)
					return
				}

				getCtx, getCancel := context.WithTimeout(context.Background(), 5*time.Second)
				got, err := rd.Get(getCtx, key, co)
				getCancel()
				if err != nil {
					recordFailure("Get(%s) error = %v", key, err)
					return
				}
				if got != want {
					recordFailure("Get(%s) = %q, want %q", key, got, want)
					return
				}
			}
		}(w)
	}

	// 并发健康检查：模拟框架后台健康检查与业务流量同时进行。
	for c := 0; c < healthCheckers; c++ {
		healthWG.Add(1)
		go func() {
			defer healthWG.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				healthy := rd.IsHealthy()
				mu.Lock()
				healthRuns++
				if healthy {
					healthyCount++
				}
				mu.Unlock()
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	// 先等所有读写完成，再停止健康检查：这样健康检查必然覆盖了整个业务流量窗口。
	writersWG.Wait()
	close(stop)
	healthWG.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, failures, "concurrent read/write reported failures:\n%v", failures)
	require.GreaterOrEqual(t, healthRuns, healthCheckMinRuns,
		"health check never ran; the test would not prove concurrent safety")
	require.Equal(t, healthRuns, healthyCount,
		"health check reported unhealthy while the live container was reachable")
}
