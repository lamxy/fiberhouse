//go:build liveintegration

package fiberhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TestLive_TaskWorkerDispatcher_EnqueueConsumeShutdown 验证 TaskWorker/TaskDispatcher
// 针对真实 Redis（DB 15）的入队-消费-优雅关闭完整流程。
//
// 依赖：本机已通过 Docker 启动 Redis 容器，监听 127.0.0.1:6379。
// 独立于 P1-4a（cacheremote，使用 db: 14）使用的 db: 15，避免键空间/队列冲突。
func TestLive_TaskWorkerDispatcher_EnqueueConsumeShutdown(t *testing.T) {
	appCtx := newTask6Context() // 复用 task_test.go 已有的最小上下文构造（同包跨文件可见）

	redisClient := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   15,
	})
	t.Cleanup(func() { _ = redisClient.Close() })

	ctx := context.Background()
	require.NoError(t, redisClient.Ping(ctx).Err(), "real redis container at 127.0.0.1:6379 must be reachable")

	// DB 15 是本次测试专属的隔离 DB，清空以避免历史残留的 asynq
	// completed/archived 集合数据影响本次判定。
	require.NoError(t, redisClient.FlushDB(ctx).Err())

	taskType := fmt.Sprintf("p1-4b-live-%s-%d", t.Name(), time.Now().UnixNano())
	type payload struct {
		Marker string `json:"marker"`
	}
	marker := fmt.Sprintf("marker-%d", time.Now().UnixNano())

	consumed := make(chan payload, 1)

	worker := NewTaskWorker(appCtx, redisClient, asynq.Config{Concurrency: 1})
	worker.HandleFunc(taskType, func(_ context.Context, task *asynq.Task) error {
		var p payload
		if err := json.Unmarshal(task.Payload(), &p); err != nil {
			return err
		}
		consumed <- p
		return nil
	})

	// RunAsync 是非阻塞的：内部用 goroutine 执行 server.Run，错误只在内部被
	// recover/记录日志，函数本身永远返回 nil。因此这里不依赖其返回值判断
	// worker 是否成功启动，而是用下方"10 秒内是否收到消费信号"作为证据。
	require.NoError(t, worker.RunAsync())
	t.Cleanup(func() {
		shutdownDone := make(chan struct{})
		go func() {
			worker.GetServer().Shutdown()
			close(shutdownDone)
		}()
		select {
		case <-shutdownDone:
		case <-time.After(3 * time.Second):
			t.Error("worker Shutdown did not complete within 3s")
		}
	})

	dispatcher := NewTaskDispatcher(redisClient)
	body, err := json.Marshal(payload{Marker: marker})
	require.NoError(t, err)
	_, err = dispatcher.Enqueue(asynq.NewTask(taskType, body))
	require.NoError(t, err)

	select {
	case got := <-consumed:
		require.Equal(t, marker, got.Marker)
	case <-time.After(10 * time.Second):
		t.Fatal("task was not consumed by worker within 10s")
	}
}
