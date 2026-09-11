//go:build liveintegration

// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件补齐 feature-status.md 记录的异步任务验证空白：既有的
// TestLive_TaskWorkerDispatcher_EnqueueConsumeShutdown 只覆盖单条
// 入队-消费-关闭路径，不覆盖高并发与故障注入。
//
// DB 分配：cacheremote 用 14，既有 task live 测试用 15，本文件用 13，
// 避免三者的键空间与 asynq 队列互相污染。
const taskConcurrentLiveRedisDB = 13

func newTaskLiveRedisClient(t *testing.T, db int) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: db})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	require.NoError(t, client.Ping(ctx).Err(), "real redis container at 127.0.0.1:6379 must be reachable")
	// 清空专属 DB，避免历史残留的 asynq retry/archived 集合影响判定。
	require.NoError(t, client.FlushDB(ctx).Err())
	return client
}

// shutdownTaskWorker 在超时保护下关闭 worker，避免单个用例挂死整个 live job。
func shutdownTaskWorker(t *testing.T, worker *TaskWorker) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		worker.GetServer().Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("worker Shutdown did not complete within 5s")
	}
}

// TestLive_TaskWorker_ConcurrentEnqueueConsumeNoLossOrDuplication 验证高并发下
// 入队与消费不丢任务、不重复消费：N 个 goroutine 并发入队，worker 以多并发度消费，
// 断言每个 marker 恰好被消费一次。
//
// 去重依据 asynq 的 TaskID 选项：同一 ID 重复入队会返回 ErrTaskIDConflict，
// 因此"每个 ID 恰好一条"由服务端保证，这里断言的是消费侧不丢不重。
func TestLive_TaskWorker_ConcurrentEnqueueConsumeNoLossOrDuplication(t *testing.T) {
	appCtx := newTask6Context()
	redisClient := newTaskLiveRedisClient(t, taskConcurrentLiveRedisDB)

	const (
		producers        = 8
		tasksPerProducer = 25
		totalTasks       = producers * tasksPerProducer
	)

	runID := time.Now().UnixNano()
	taskType := fmt.Sprintf("live-concurrent-%d", runID)

	type payload struct {
		Marker string `json:"marker"`
	}

	var (
		mu       sync.Mutex
		consumed = make(map[string]int, totalTasks)
		received = make(chan struct{}, totalTasks)
	)

	worker := NewTaskWorker(appCtx, redisClient, asynq.Config{Concurrency: 8})
	worker.HandleFunc(taskType, func(_ context.Context, task *asynq.Task) error {
		var p payload
		if err := json.Unmarshal(task.Payload(), &p); err != nil {
			return err
		}
		mu.Lock()
		consumed[p.Marker]++
		mu.Unlock()
		received <- struct{}{}
		return nil
	})

	require.NoError(t, worker.RunAsync())
	t.Cleanup(func() { shutdownTaskWorker(t, worker) })

	dispatcher := NewTaskDispatcher(redisClient)

	var (
		wg         sync.WaitGroup
		enqueueErr atomic.Value
	)
	for producer := 0; producer < producers; producer++ {
		wg.Add(1)
		go func(producer int) {
			defer wg.Done()
			for i := 0; i < tasksPerProducer; i++ {
				marker := fmt.Sprintf("p%d-i%d", producer, i)
				body, err := json.Marshal(payload{Marker: marker})
				if err != nil {
					enqueueErr.Store(err)
					return
				}
				// TaskID 让同一 marker 在服务端唯一，消费侧的重复计数才是真实的重复消费。
				if _, err := dispatcher.Enqueue(
					asynq.NewTask(taskType, body),
					asynq.TaskID(fmt.Sprintf("%d-%s", runID, marker)),
				); err != nil {
					enqueueErr.Store(err)
					return
				}
			}
		}(producer)
	}
	wg.Wait()
	if err := enqueueErr.Load(); err != nil {
		require.NoError(t, err.(error))
	}

	deadline := time.After(60 * time.Second)
	for i := 0; i < totalTasks; i++ {
		select {
		case <-received:
		case <-deadline:
			mu.Lock()
			got := len(consumed)
			mu.Unlock()
			t.Fatalf("only %d/%d tasks consumed before deadline", got, totalTasks)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, consumed, totalTasks, "every enqueued marker must be consumed")
	for marker, count := range consumed {
		assert.Equalf(t, 1, count, "marker %s consumed %d times, want exactly once", marker, count)
	}
}

// TestLive_TaskWorker_HandlerFailureRetriesThenArchives 是故障注入用例：
// handler 持续返回错误时，asynq 必须按 MaxRetry 重试，耗尽后把任务归档，
// 且每次失败都经由 Config.ErrorHandler 上报。
//
// 这条路径此前完全没有 live 覆盖——既有用例的 handler 永远成功。
func TestLive_TaskWorker_HandlerFailureRetriesThenArchives(t *testing.T) {
	appCtx := newTask6Context()
	redisClient := newTaskLiveRedisClient(t, taskConcurrentLiveRedisDB)

	runID := time.Now().UnixNano()
	taskType := fmt.Sprintf("live-failure-%d", runID)
	taskID := fmt.Sprintf("failure-%d", runID)
	queueName := "default"

	const maxRetry = 2

	handlerErr := errors.New("handler fails deterministically")

	var (
		attempts     atomic.Int32
		reported     atomic.Int32
		lastReported atomic.Value
	)

	worker := NewTaskWorker(appCtx, redisClient, asynq.Config{
		Concurrency: 1,
		// 把重试延迟压到最小，否则默认退避会让用例等待数分钟。
		RetryDelayFunc: func(int, error, *asynq.Task) time.Duration {
			return 100 * time.Millisecond
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, _ *asynq.Task, err error) {
			reported.Add(1)
			lastReported.Store(err.Error())
		}),
	})
	worker.HandleFunc(taskType, func(context.Context, *asynq.Task) error {
		attempts.Add(1)
		return handlerErr
	})

	require.NoError(t, worker.RunAsync())
	t.Cleanup(func() { shutdownTaskWorker(t, worker) })

	dispatcher := NewTaskDispatcher(redisClient)
	_, err := dispatcher.Enqueue(
		asynq.NewTask(taskType, []byte(`{}`)),
		asynq.TaskID(taskID),
		asynq.Queue(queueName),
		asynq.MaxRetry(maxRetry),
	)
	require.NoError(t, err)

	inspector := asynq.NewInspectorFromRedisClient(redisClient)
	t.Cleanup(func() { _ = inspector.Close() })

	// 首次执行 + maxRetry 次重试 = maxRetry+1 次尝试，之后转入 archived。
	var info *asynq.TaskInfo
	require.Eventually(t, func() bool {
		got, err := inspector.GetTaskInfo(queueName, taskID)
		if err != nil {
			return false
		}
		info = got
		return got.State == asynq.TaskStateArchived
	}, 60*time.Second, 200*time.Millisecond, "task must end up archived after exhausting retries")

	require.NotNil(t, info)
	assert.Equal(t, asynq.TaskStateArchived, info.State)
	assert.Equal(t, maxRetry, info.MaxRetry)
	assert.Equal(t, maxRetry, info.Retried, "asynq records the retry count it actually performed")
	assert.Contains(t, info.LastErr, handlerErr.Error(), "the handler error must be preserved on the task")

	assert.EqualValues(t, maxRetry+1, attempts.Load(), "handler runs once plus one run per retry")
	assert.GreaterOrEqual(t, reported.Load(), int32(1), "Config.ErrorHandler must observe handler failures")
	if reportedErr, ok := lastReported.Load().(string); ok {
		assert.Contains(t, reportedErr, handlerErr.Error())
	}

	archived, err := inspector.ListArchivedTasks(queueName)
	require.NoError(t, err)
	require.NotEmpty(t, archived)
	assert.Condition(t, func() bool {
		for _, task := range archived {
			if task.ID == taskID {
				return true
			}
		}
		return false
	}, "the failed task must appear in the archived list")
}

// TestLive_TaskWorker_SkipRetryArchivesImmediately 验证 asynq.SkipRetry 语义：
// handler 返回该哨兵错误时任务立即归档，不消耗重试次数。这是框架使用方区分
// "可重试故障"与"确定性坏任务"的唯一手段，此前无任何覆盖。
func TestLive_TaskWorker_SkipRetryArchivesImmediately(t *testing.T) {
	appCtx := newTask6Context()
	redisClient := newTaskLiveRedisClient(t, taskConcurrentLiveRedisDB)

	runID := time.Now().UnixNano()
	taskType := fmt.Sprintf("live-skipretry-%d", runID)
	taskID := fmt.Sprintf("skipretry-%d", runID)
	queueName := "default"

	var attempts atomic.Int32

	worker := NewTaskWorker(appCtx, redisClient, asynq.Config{
		Concurrency: 1,
		RetryDelayFunc: func(int, error, *asynq.Task) time.Duration {
			return 100 * time.Millisecond
		},
	})
	worker.HandleFunc(taskType, func(context.Context, *asynq.Task) error {
		attempts.Add(1)
		return fmt.Errorf("poison payload: %w", asynq.SkipRetry)
	})

	require.NoError(t, worker.RunAsync())
	t.Cleanup(func() { shutdownTaskWorker(t, worker) })

	dispatcher := NewTaskDispatcher(redisClient)
	_, err := dispatcher.Enqueue(
		asynq.NewTask(taskType, []byte(`{}`)),
		asynq.TaskID(taskID),
		asynq.Queue(queueName),
		// 即使允许 5 次重试，SkipRetry 也必须让它一次就归档。
		asynq.MaxRetry(5),
	)
	require.NoError(t, err)

	inspector := asynq.NewInspectorFromRedisClient(redisClient)
	t.Cleanup(func() { _ = inspector.Close() })

	var info *asynq.TaskInfo
	require.Eventually(t, func() bool {
		got, err := inspector.GetTaskInfo(queueName, taskID)
		if err != nil {
			return false
		}
		info = got
		return got.State == asynq.TaskStateArchived
	}, 30*time.Second, 200*time.Millisecond, "SkipRetry must archive the task without retrying")

	require.NotNil(t, info)
	assert.Equal(t, asynq.TaskStateArchived, info.State)
	assert.Zero(t, info.Retried, "SkipRetry must not consume any retry")
	assert.EqualValues(t, 1, attempts.Load(), "handler must run exactly once under SkipRetry")
}
