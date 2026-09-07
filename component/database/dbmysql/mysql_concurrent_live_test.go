//go:build liveintegration

// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmysql

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestLive_MysqlDb_CloseIsIdempotent 针对真实容器验证 Close 幂等。
//
// MysqlDb 不带自己的 closed 标志，依赖 database/sql 保证 sql.DB.Close 幂等。
// 那是一个外部契约，关闭路径上重复关闭又是常态（别名 key、组合对象持有子资源），
// 因此必须实测而不是假定。
func TestLive_MysqlDb_CloseIsIdempotent(t *testing.T) {
	dsn := "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"
	ctx := newTestMysqlAppContext(t, dsn)

	db, err := NewMysqlDb(ctx, "test.mysql")
	require.NoError(t, err, "must be able to connect to the live MySQL container")

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	require.True(t, db.PingTry(pingCtx), "client must be live before closing")

	require.NoError(t, db.Close(), "first Close must succeed")
	require.NoError(t, db.Close(), "second Close must be a no-op")
	require.NoError(t, db.Close(), "third Close must remain a no-op")
}

// TestLive_MysqlDb_ConcurrentReadWriteWithHealthCheck 针对真实 MySQL 容器验证并发
// 读写与并发健康检查同时进行时的行为。
//
// 为什么需要这个用例：MysqlDb 刻意不实现 globalmanager.Rebuilder，其正确性论证是
// 「Client 构造后不再被替换，读侧无需加锁，失效恢复交给 database/sql 的坏连接淘汰
// 与 connMaxLifetime/connMaxIdleTime」。既有 live 测试只跑单 goroutine 的顺序
// 建表-写入-读取，没有验证这个论证。本用例让 N 个 goroutine 并发读写同一张表，
// 同时另一组 goroutine 持续调用 IsHealthy（框架后台健康检查正是这么做的），确保：
//
//   - 并发读写不产生错误，写入的行都能被准确读回
//   - 健康检查与业务流量并发进行时不干扰彼此
//   - 连接池在并发压力下正常复用，不出现连接耗尽
//
// 配合 `go test -race -tags=liveintegration` 运行，这个用例才同时覆盖并发正确性
// 与内存模型安全。
func TestLive_MysqlDb_ConcurrentReadWriteWithHealthCheck(t *testing.T) {
	dsn := "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"
	ctx := newTestMysqlAppContext(t, dsn)

	db, err := NewMysqlDb(ctx, "test.mysql")
	require.NoError(t, err, "must be able to connect to the live MySQL container")
	t.Cleanup(func() { _ = db.Close() })

	tableSuffix := sanitizeForTableName(fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano()))
	tableName := "concurrent_live_" + tableSuffix

	require.NoError(t, db.Client.Table(tableName).Migrator().AutoMigrate(&liveTestRecord{}))
	t.Cleanup(func() {
		_ = db.Client.Table(tableName).Migrator().DropTable(&liveTestRecord{})
	})

	const (
		writers        = 6
		opsPerWriter   = 10
		healthCheckers = 2
	)

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

	// 并发读写：每个 goroutine 写入自己的 marker 并立即读回，验证互不串扰。
	for w := 0; w < writers; w++ {
		writersWG.Add(1)
		go func(worker int) {
			defer writersWG.Done()
			for i := 0; i < opsPerWriter; i++ {
				marker := fmt.Sprintf("w%d-i%d-%d", worker, i, time.Now().UnixNano())

				if err := db.Client.Table(tableName).Create(&liveTestRecord{Marker: marker}).Error; err != nil {
					recordFailure("Create(%s) error = %v", marker, err)
					return
				}

				var got liveTestRecord
				if err := db.Client.Table(tableName).Where("marker = ?", marker).First(&got).Error; err != nil {
					recordFailure("First(%s) error = %v", marker, err)
					return
				}
				if got.Marker != marker {
					recordFailure("First(%s) returned marker %q", marker, got.Marker)
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
				healthy := db.IsHealthy()
				mu.Lock()
				healthRuns++
				if healthy {
					healthyCount++
				}
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// 先等读写跑完，再停止健康检查：确保健康检查覆盖整个业务流量窗口。
	writersWG.Wait()
	close(stop)
	healthWG.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, failures, "concurrent read/write reported failures:\n%v", failures)
	require.Positive(t, healthRuns, "health check never ran; the test would not prove concurrent safety")
	require.Equal(t, healthRuns, healthyCount,
		"health check reported unhealthy while the live container was reachable")

	// 所有写入都必须落库：并发写不能丢行。
	var total int64
	require.NoError(t, db.Client.Table(tableName).Model(&liveTestRecord{}).Count(&total).Error)
	require.Equal(t, int64(writers*opsPerWriter), total, "concurrent writes lost rows")
}
