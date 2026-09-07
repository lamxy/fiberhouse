//go:build liveintegration

// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmongo

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestLive_MongoDb_ConcurrentReadWriteWithHealthCheck 针对真实 MongoDB 容器验证
// 并发读写与并发健康检查同时进行时的行为。
//
// 为什么需要这个用例：MongoDb 刻意不实现 globalmanager.Rebuilder，其正确性论证是
// 「Client 构造后不再被替换，读侧无需加锁，失效恢复交给 mongo-driver 的 SDAM
// （Server Discovery and Monitoring）」。既有 live 测试只跑单 goroutine 的顺序
// 写入-读取，没有验证这个论证。本用例让 N 个 goroutine 并发读写同一个 collection，
// 同时另一组 goroutine 持续调用 IsHealthy（框架后台健康检查正是这么做的），确保：
//
//   - 并发读写不产生错误，写入的文档都能被准确读回
//   - 健康检查与业务流量并发进行时不干扰彼此
//   - 连接池在并发压力下正常复用
//
// 配合 `go test -race -tags=liveintegration` 运行，这个用例才同时覆盖并发正确性
// 与内存模型安全。
func TestLive_MongoDb_ConcurrentReadWriteWithHealthCheck(t *testing.T) {
	applyURI := "mongodb://admin:admin@127.0.0.1:27037/?authSource=admin"
	ctx := newTestMongoAppContext(t, applyURI)

	db, err := NewMongoDb(ctx, "test.mongodb")
	require.NoError(t, err, "must be able to connect to the live MongoDB container")
	t.Cleanup(func() { _ = db.Close() })

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	require.True(t, db.PingTry(pingCtx))

	collName := "concurrent_live_" + sanitizeForCollectionName(fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano()))
	require.LessOrEqual(t, len(collName), 255, "collection name must not exceed MongoDB's 255-byte limit")

	coll := db.Client.Database("test").Collection(collName)
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		_ = coll.Drop(dropCtx)
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

				insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := coll.InsertOne(insertCtx, liveTestDoc{Marker: marker})
				insertCancel()
				if err != nil {
					recordFailure("InsertOne(%s) error = %v", marker, err)
					return
				}

				findCtx, findCancel := context.WithTimeout(context.Background(), 5*time.Second)
				var got liveTestDoc
				err = coll.FindOne(findCtx, bson.D{{Key: "marker", Value: marker}}).Decode(&got)
				findCancel()
				if err != nil {
					recordFailure("FindOne(%s) error = %v", marker, err)
					return
				}
				if got.Marker != marker {
					recordFailure("FindOne(%s) returned marker %q", marker, got.Marker)
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

	// 所有写入都必须落库：并发写不能丢文档。
	countCtx, countCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer countCancel()
	total, err := coll.CountDocuments(countCtx, bson.D{})
	require.NoError(t, err)
	require.Equal(t, int64(writers*opsPerWriter), total, "concurrent writes lost documents")
}

// TestLive_MongoDb_CloseIsIdempotent 针对真实容器验证 Close 幂等。
//
// 关闭路径上同一个资源可能被关闭多次（别名 key、组合对象持有子资源）。MongoDb
// 此前没有幂等保护，第二次 Disconnect 会返回 mongo.ErrClientDisconnected —— 一个
// 已完成的关闭被上报为失败。单元测试只能覆盖 Client 为 nil 的分支，真正的幂等性
// 需要一个活的 client 才能验证。
func TestLive_MongoDb_CloseIsIdempotent(t *testing.T) {
	applyURI := "mongodb://admin:admin@127.0.0.1:27037/?authSource=admin"
	ctx := newTestMongoAppContext(t, applyURI)

	db, err := NewMongoDb(ctx, "test.mongodb")
	require.NoError(t, err, "must be able to connect to the live MongoDB container")

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	require.True(t, db.PingTry(pingCtx), "client must be live before closing")

	require.NoError(t, db.Close(), "first Close must succeed")
	require.NoError(t, db.Close(), "second Close must be a no-op, not ErrClientDisconnected")
	require.NoError(t, db.Close(), "third Close must remain a no-op")
}
