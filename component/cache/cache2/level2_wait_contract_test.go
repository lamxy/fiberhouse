// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package cache2

import (
	"context"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse/component/cache"
)

// 本文件把 PRD 任务 9 的 I1 裁决转成可执行断言。
//
// PRD 主张 Wait() 不等待 ants pool 在途任务，并建议改用 localPool.Running()。
// 该主张不成立：Wait() 首先调用 Flush，而 Flush 阻塞在 drain channel 上，
// 后者只在 pending 计数归零时关闭；pending 在 Submit **之前**由 admit() 递增，
// 因此覆盖「已受理但未完成」的全部任务，含仍在队列中的。
//
// 建议的 Running() 方案反而更弱——它只统计正在执行的 worker，漏掉排队任务。
// 这里把这条结论钉死，避免将来有人依 PRD 字面「修复」成更弱的版本。

// TestLevel2Wait_CountsQueuedWorkNotJustRunning Wait() 必须覆盖排队中的任务。
//
// 构造：pool 容量 1，提交两个任务——第一个占住唯一 worker 并阻塞，第二个只能
// 排队。此时 ants 的 Running() 为 1，而待完成的任务实为 2。若 Wait() 依据
// Running() 判断，会在第二个任务尚未执行时就返回。
func TestLevel2Wait_CountsQueuedWorkNotJustRunning(t *testing.T) {
	release := make(chan struct{})
	remote := &gatedSetCache{
		recordingCache: newRecordingCache(cache.Remote),
		started:        make(chan struct{}, 2),
		release:        release,
	}
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), remote)
	l2.remotePool.Release()
	pool, err := ants.NewPool(1, ants.WithMaxBlockingTasks(2))
	require.NoError(t, err)
	l2.remotePool = pool

	co := cache.NewCacheOption(nil).SetSyncStrategyAsyncWriteRemoteOnly()
	require.NoError(t, l2.Set(context.Background(), "running", "one", co))
	<-remote.started

	secondReturned := make(chan error, 1)
	go func() { secondReturned <- l2.Set(context.Background(), "queued", "two", co) }()
	require.Eventually(t, func() bool {
		l2.lifecycleMu.Lock()
		defer l2.lifecycleMu.Unlock()
		return l2.pending == 2
	}, time.Second, time.Millisecond, "both tasks must be admitted")

	// 关键对照：ants 只认为 1 个在跑，但 pending 是 2。
	assert.Equal(t, 1, l2.remotePool.Running(),
		"ants reports only the running worker, not the queued task")
	l2.lifecycleMu.Lock()
	pendingCount := l2.pending
	l2.lifecycleMu.Unlock()
	assert.Equal(t, 2, pendingCount,
		"the drain counter must cover queued work too — this is why Running() is insufficient")

	waited := make(chan error, 1)
	go func() { waited <- l2.Wait() }()
	select {
	case err := <-waited:
		t.Fatalf("Wait returned while queued work was outstanding: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-secondReturned)
	require.NoError(t, <-waited)
	assert.Equal(t, int32(2), remote.finished.Load(),
		"every admitted task must have finished before Wait returned")
	require.NoError(t, l2.Shutdown(context.Background()))
}

// TestLevel2Wait_ReturnsPromptlyWhenIdle 无在途任务时 Wait() 不应阻塞。
//
// 排空机制的另一端：drain 在 pending 为 0 时保持关闭状态（initLifecycle 即
// 先 make 再 close），因此空闲时 Flush 立即返回。
func TestLevel2Wait_ReturnsPromptlyWhenIdle(t *testing.T) {
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), newRecordingCache(cache.Remote))

	done := make(chan error, 1)
	go func() { done <- l2.Wait() }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Wait blocked even though no async work was outstanding")
	}
	require.NoError(t, l2.Shutdown(context.Background()))
}

// TestLevel2Wait_RejectedAfterClose 关闭之后 Wait() 返回 ErrCacheClosed。
func TestLevel2Wait_RejectedAfterClose(t *testing.T) {
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), newRecordingCache(cache.Remote))
	require.NoError(t, l2.Shutdown(context.Background()))

	assert.ErrorIs(t, l2.Wait(), cache.ErrCacheClosed)
}

// TestLevel2Wait_SubmitFailureStillDecrementsPending 提交失败不得泄漏计数。
//
// admit() 在 Submit 之前递增 pending；若提交失败时不补一次 complete()，
// 计数永不归零，drain 永不关闭，Wait()/Flush() 将永久阻塞。
func TestLevel2Wait_SubmitFailureStillDecrementsPending(t *testing.T) {
	l2 := newTestLevel2(t, newRecordingCache(cache.Local), newRecordingCache(cache.Remote))

	// 提前释放 pool：后续 Submit 必然失败。
	l2.remotePool.Release()

	co := cache.NewCacheOption(nil).SetSyncStrategyAsyncWriteRemoteOnly()
	require.NoError(t, l2.Set(context.Background(), "doomed", "value", co))

	done := make(chan error, 1)
	go func() { done <- l2.Wait() }()

	select {
	case err := <-done:
		require.NoError(t, err, "a failed submit must not leave the drain counter stuck")
	case <-time.After(time.Second):
		t.Fatal("Wait blocked forever: the failed submit leaked a pending count")
	}
	require.NoError(t, l2.Shutdown(context.Background()))
}
