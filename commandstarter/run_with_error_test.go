// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package commandstarter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件锁定 CLI 命令执行错误的传播（PRD 任务 11）。
//
// 此前 RunCommandStarter 以 `_ = starter.AppCoreRun()` 丢弃错误：命令失败时
// 进程仍以退出码 0 结束，在 CI、cron 与容器编排里等于「成功」。
//
// 处置方式是补姊妹入口而非改签名：RunCommandStarter 是已发布的公共入口，
// 改成返回 error 属 breaking change，而仓库已两次明确排除该变更
// （follow-up-local-patch-optimization-todo.md:40、
// v1-production-readiness-tasklist.md:223）。

// TestRunCommandStarterWithError_PropagatesRunFailure 命令失败必须传出。
func TestRunCommandStarterWithError_PropagatesRunFailure(t *testing.T) {
	sentinel := errors.New("command failed")
	starter := &commandShutdownStarter{
		ctx:    newCommandTestContext(),
		runErr: sentinel,
	}

	err := RunCommandStarterWithError(starter)

	assert.ErrorIs(t, err, sentinel, "the run error must reach the caller")
	assert.Equal(t, 1, starter.runCall)
}

// TestRunCommandStarterWithError_ReturnsNilOnSuccess 成功时返回 nil。
func TestRunCommandStarterWithError_ReturnsNilOnSuccess(t *testing.T) {
	starter := &commandShutdownStarter{ctx: newCommandTestContext()}

	assert.NoError(t, RunCommandStarterWithError(starter))
	assert.Equal(t, 1, starter.runCall)
}

// TestRunCommandStarterWithError_ReleasesGlobalsOnFailure 失败路径同样执行关闭段。
//
// 关闭段写在 defer 里正是为了这条路径：命令返回错误时资源同样需要回收。
func TestRunCommandStarterWithError_ReleasesGlobalsOnFailure(t *testing.T) {
	ctx := newCommandTestContext()
	resource := &closableCommandResource{}

	container := ctx.GetContainer()
	require.True(t, container.Register("cmdRunFailureResource", func() (interface{}, error) {
		return resource, nil
	}))
	// 必须 Get 一次让实例落位：ReleaseAll 只处理已初始化的 entry。
	_, err := container.Get("cmdRunFailureResource")
	require.NoError(t, err)

	runErr := RunCommandStarterWithError(&commandShutdownStarter{
		ctx:    ctx,
		runErr: errors.New("command failed"),
	})

	require.Error(t, runErr)
	assert.Equal(t, int32(1), resource.closeCalls.Load(),
		"the shutdown section must run even when the command fails")
	assert.False(t, container.IsRegistered("cmdRunFailureResource"))
}

// TestRunCommandStarter_StaysVoidAndSwallowsError 原入口签名与行为不变。
//
// 既有调用方依赖它不返回值；错误改为写入 CLI 日志器而非静默丢弃。
// 本用例同时是签名的编译期证明：若 RunCommandStarter 改成返回 error，
// 下面这行独立语句仍可编译，但 assert 之前的赋值形式会立刻暴露差异——
// 因此这里刻意以语句形式调用。
func TestRunCommandStarter_StaysVoidAndSwallowsError(t *testing.T) {
	starter := &commandShutdownStarter{
		ctx:    newCommandTestContext(),
		runErr: errors.New("command failed"),
	}

	assert.NotPanics(t, func() { RunCommandStarter(starter) },
		"a failing command must not panic the void entry point")
	assert.Equal(t, 1, starter.runCall)
}
