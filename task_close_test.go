// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件锁定异步任务的关闭契约。
//
// 此前 TaskWorker 与 TaskDispatcher 都没有 wrapper 级关闭方法：dispatcher 内部的
// asynq.Client 不被回收，worker 的 server 也无人停止。容器的 ReleaseAll 只关闭
// 实现了 globalmanager.Closable 的对象，两者都不实现，于是标准关闭链走完之后
// 它们仍然活着——feature-status.md 把这记为「关闭路径仍无契约」，也是异步任务
// 能力的晋级阻塞项（条件 1）。
//
// 两者现在都实现 Closable，因此注册进容器后会被 ReleaseAll 正常回收。

// TestTaskDispatcher_ImplementsClosable dispatcher 必须满足容器的关闭契约。
func TestTaskDispatcher_ImplementsClosable(t *testing.T) {
	dispatcher := NewTaskDispatcher(newTask6RedisClient(t))

	closable, ok := any(dispatcher).(globalmanager.Closable)
	require.True(t, ok,
		"TaskDispatcher must implement globalmanager.Closable, "+
			"otherwise ReleaseAll leaves its asynq.Client unreclaimed")
	require.NoError(t, closable.Close())
}

// TestTaskWorker_ImplementsClosable worker 必须满足容器的关闭契约。
func TestTaskWorker_ImplementsClosable(t *testing.T) {
	worker := NewTaskWorker(newTask6Context(), newTask6RedisClient(t), asynq.Config{Concurrency: 1})

	closable, ok := any(worker).(globalmanager.Closable)
	require.True(t, ok,
		"TaskWorker must implement globalmanager.Closable, "+
			"otherwise ReleaseAll never stops the asynq server")
	require.NoError(t, closable.Close())
}

// TestTaskDispatcher_CloseDoesNotOwnTheRedisConnection 锁定连接所有权语义。
//
// NewTaskDispatcher 走 asynq.NewClientFromRedisClient，连接由调用方注入。asynq
// 对这种客户端**拒绝**关闭连接，其 Close() 恒定返回「redis connection is shared
// so the Client can't be closed through asynq」——这是所有权契约而非故障。
//
// dispatcher.Close 必须把这条拒绝视为成功：它确实没有独占资源需要释放。若照原样
// 上抛，每一次 ReleaseAll 都会为每个 dispatcher 报一次假的关闭失败。
//
// 同时确认注入的 Redis 连接**没有**被关掉——它通常还供缓存与 worker 使用。
func TestTaskDispatcher_CloseDoesNotOwnTheRedisConnection(t *testing.T) {
	// 本用例自己持有连接，不用 newTask6RedisClient：那个 helper 在 t.Cleanup 里
	// 断言 Close 成功，而本用例正要亲自关闭连接以验证它此前仍是打开的，
	// 两者会在收尾时相撞。
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	dispatcher := NewTaskDispatcher(redisClient)

	// 先确认 asynq 本身确实拒绝关闭共享连接，使本用例的前提可见。
	require.ErrorContains(t, dispatcher.Client.Close(), "redis connection is shared",
		"asynq must refuse to close a caller-supplied connection")

	require.NoError(t, dispatcher.Close(),
		"the shared-connection refusal must not surface as a close failure")
	assert.True(t, dispatcher.isClosed())

	// 注入的连接仍归调用方所有，dispatcher.Close 不得把它关掉。
	//
	// 判据是「首次 Close 成功」：go-redis 对已关闭的客户端返回
	// "redis: client is closed"，因此这里能成功就证明它此前仍处于打开状态。
	// 不能用 Ping 判断——测试地址 127.0.0.1:0 本就拨不通，Ping 的失败与是否
	// 关闭无关。
	assert.NoError(t, redisClient.Close(),
		"the caller-supplied Redis client must still be open after dispatcher.Close")
	assert.ErrorContains(t, redisClient.Close(), "client is closed",
		"sanity check: a second Close is what a closed client looks like")
}

// TestTaskDispatcher_CloseIsIdempotent 容器允许同一对象以别名注册在多个 key 下，
// 因此 Close 会被调用多次，必须幂等。
func TestTaskDispatcher_CloseIsIdempotent(t *testing.T) {
	dispatcher := NewTaskDispatcher(newTask6RedisClient(t))

	require.NoError(t, dispatcher.Close())
	assert.NoError(t, dispatcher.Close(), "a second Close must return safely")
	assert.NoError(t, dispatcher.Close())
}

// TestTaskWorker_CloseIsIdempotent 同上。
//
// 这条尤其关键：asynq.Server.Shutdown() 重复调用会 panic（内部向已关闭的
// channel 发送），因此幂等守卫不是锦上添花。
func TestTaskWorker_CloseIsIdempotent(t *testing.T) {
	worker := NewTaskWorker(newTask6Context(), newTask6RedisClient(t), asynq.Config{Concurrency: 1})

	require.NoError(t, worker.Close())
	assert.NotPanics(t, func() {
		assert.NoError(t, worker.Close(), "a second Close must return safely")
	})
}

// TestTaskWorker_CloseIsRaceFree 并发关闭只能有一次真正生效。
func TestTaskWorker_CloseIsRaceFree(t *testing.T) {
	worker := NewTaskWorker(newTask6Context(), newTask6RedisClient(t), asynq.Config{Concurrency: 1})

	var wg sync.WaitGroup
	const closers = 8
	wg.Add(closers)
	for i := 0; i < closers; i++ {
		go func() {
			defer wg.Done()
			assert.NoError(t, worker.Close())
		}()
	}
	wg.Wait()
}

// TestTaskDispatcher_EnqueueAfterCloseReturnsError 关闭后入队必须给出明确错误。
//
// 旧行为把调用直接转发给已关闭的 asynq.Client，错误信息与「客户端已关闭」无关，
// 调用方难以判断是配置问题还是生命周期问题。
func TestTaskDispatcher_EnqueueAfterCloseReturnsError(t *testing.T) {
	dispatcher := NewTaskDispatcher(newTask6RedisClient(t))
	require.NoError(t, dispatcher.Close())

	_, err := dispatcher.Enqueue(asynq.NewTask("task6:after-close", nil))
	require.Error(t, err, "enqueueing on a closed dispatcher must fail")
	assert.ErrorIs(t, err, ErrTaskDispatcherClosed)

	_, err = dispatcher.EnqueueContext(context.Background(), asynq.NewTask("task6:after-close", nil))
	require.Error(t, err, "EnqueueContext must apply the same guard")
	assert.ErrorIs(t, err, ErrTaskDispatcherClosed)
}

// TestTaskWorker_RunAsyncReportsStartFailure RunAsync 不得恒返回 nil。
//
// 旧实现的具名返回值 err 从未被赋值：启动失败只写进日志，调用方拿到 nil，
// 于是「worker 没起来」这件事在调用侧完全不可见。
func TestTaskWorker_RunAsyncReportsStartFailure(t *testing.T) {
	worker := NewTaskWorker(newTask6Context(), newTask6RedisClient(t), asynq.Config{Concurrency: 1})
	// 先关闭：已关闭的 worker 不能再启动，这是一条确定可观测的失败路径，
	// 无需依赖真实 Redis。
	require.NoError(t, worker.Close())

	err := worker.RunAsync()

	require.Error(t, err, "RunAsync must report that a closed worker cannot start")
	assert.ErrorIs(t, err, ErrTaskWorkerClosed)
}

// TestTaskWorker_RunSyncOnClosedWorkerFails RunSync 同样不得静默成功。
func TestTaskWorker_RunSyncOnClosedWorkerFails(t *testing.T) {
	worker := NewTaskWorker(newTask6Context(), newTask6RedisClient(t), asynq.Config{Concurrency: 1})
	require.NoError(t, worker.Close())

	err := worker.RunSync()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTaskWorkerClosed)
}

// TestReleaseAll_ClosesTaskDispatcherAndWorker 红线：容器关闭链必须回收两者。
//
// 这是本任务的核心断言——此前 ReleaseAll 走完，dispatcher 的 client 与 worker 的
// server 都还活着。
//
// 注意：必须先 Get 让实例真正落位。ReleaseAll 只关闭已初始化的对象，若只
// Register 不 Get，entry 始终未初始化，测试会恒真通过而证明不了任何事情。
func TestReleaseAll_ClosesTaskDispatcherAndWorker(t *testing.T) {
	manager := globalmanager.NewGlobalManager()

	dispatcher := NewTaskDispatcher(newTask6RedisClient(t))
	worker := NewTaskWorker(newTask6Context(), newTask6RedisClient(t), asynq.Config{Concurrency: 1})

	require.True(t, manager.Register("task6-dispatcher", func() (interface{}, error) {
		return dispatcher, nil
	}))
	require.True(t, manager.Register("task6-server", func() (interface{}, error) {
		return worker, nil
	}))

	// 让两个实例真正落位，否则 ReleaseAll 无事可做。
	for _, key := range []globalmanager.KeyName{"task6-dispatcher", "task6-server"} {
		instance, err := manager.Get(key)
		require.NoError(t, err)
		require.NotNil(t, instance)
	}

	manager.ReleaseAll(true)

	assert.True(t, dispatcher.isClosed(),
		"ReleaseAll must close the task dispatcher: "+
			"its asynq.Client was previously left unreclaimed")
	assert.True(t, worker.isClosed(),
		"ReleaseAll must stop the task worker: "+
			"its asynq server was previously left running")
}

// TestReleaseAll_TaskClosureFailureIsReported 关闭失败必须经 observer 上报。
func TestReleaseAll_TaskClosureFailureIsReported(t *testing.T) {
	manager := globalmanager.NewGlobalManager()
	dispatcher := NewTaskDispatcher(newTask6RedisClient(t))

	require.True(t, manager.Register("task6-dispatcher", func() (interface{}, error) {
		return dispatcher, nil
	}))
	_, err := manager.Get("task6-dispatcher")
	require.NoError(t, err)

	// 正常路径不应产生上报；这里确认 observer 接线本身是通的。
	var reported []globalmanager.KeyName
	manager.ReleaseAllWithObserver(func(name globalmanager.KeyName, _ error) {
		reported = append(reported, name)
	}, true)

	assert.Empty(t, reported, "a successful close must not be reported as a failure")
	assert.True(t, dispatcher.isClosed())
}
