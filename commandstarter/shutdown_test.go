// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package commandstarter

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件覆盖 CLI 的关闭段。
//
// 此前 commandstarter 没有任何关闭路径：RunCommandStarter 跑完 AppCoreRun 就返回，
// 应用注册的 MySQL 连接池、Mongo 客户端与 Redis 客户端从未 Close，只能等进程退出
// 由 OS 回收。短命命令影响有限，常驻型 CLI（daemon、cron worker）则是真实泄漏。
//
// 这些用例用 commandTestContext——它持有 per-test 的 NewGlobalManager()，而不是
// NewGlobalManagerOnce() 那个进程级单例，因此驱动真实释放不会污染同包其他测试。

// closableCommandResource 记录自己被关闭的次数与返回的错误。
type closableCommandResource struct {
	closeCalls atomic.Int32
	closeErr   error
}

func (r *closableCommandResource) Close() error {
	r.closeCalls.Add(1)
	return r.closeErr
}

// commandShutdownStarter 是驱动 RunCommandStarter 的最小替身：它持有一个真实上下文，
// 其余生命周期方法都是空操作。
type commandShutdownStarter struct {
	fiberhouse.CommandStarter
	ctx     fiberhouse.ICommandContext
	runErr  error
	runCall int
}

func (s *commandShutdownStarter) InitCoreApp()                                        {}
func (s *commandShutdownStarter) GetFrameCmdApp() fiberhouse.FrameCmdStarter          { return s }
func (s *commandShutdownStarter) GetContext() fiberhouse.ICommandContext              { return s.ctx }
func (s *commandShutdownStarter) RegisterGlobalErrHandler(fiberhouse.FrameCmdStarter) {}
func (s *commandShutdownStarter) RegisterCommands(fiberhouse.FrameCmdStarter)         {}
func (s *commandShutdownStarter) RegisterCoreGlobalOptional(fiberhouse.FrameCmdStarter) {
}
func (s *commandShutdownStarter) RegisterApplicationGlobals() {}
func (s *commandShutdownStarter) AppCoreRun() error {
	s.runCall++
	return s.runErr
}

// TestRunCommandStarter_ReleasesGlobalsAfterRun 锁定核心契约：命令执行完毕后，
// 容器内已初始化的全局对象会被 Close，条目会被清空。
func TestRunCommandStarter_ReleasesGlobalsAfterRun(t *testing.T) {
	ctx := newCommandTestContext()
	resource := &closableCommandResource{}

	container := ctx.GetContainer()
	require.True(t, container.Register("cmdResource", func() (interface{}, error) {
		return resource, nil
	}))
	// 必须 Get 一次让实例落位：ReleaseAll 只处理已初始化的 entry，
	// 否则用例会恒真通过而不检验任何东西。
	_, err := container.Get("cmdResource")
	require.NoError(t, err)

	RunCommandStarter(&commandShutdownStarter{ctx: ctx})

	assert.Equal(t, int32(1), resource.closeCalls.Load(),
		"the CLI must close globals it registered")
	assert.False(t, container.IsRegistered("cmdResource"),
		"container entries must be cleared after release")
}

// TestRunCommandStarter_ReleasesGlobalsWhenCommandFails 确认命令返回错误时资源同样
// 被回收——用 defer 调用正是为了覆盖这条路径。
func TestRunCommandStarter_ReleasesGlobalsWhenCommandFails(t *testing.T) {
	ctx := newCommandTestContext()
	resource := &closableCommandResource{}

	container := ctx.GetContainer()
	require.True(t, container.Register("cmdResource", func() (interface{}, error) {
		return resource, nil
	}))
	_, err := container.Get("cmdResource")
	require.NoError(t, err)

	starter := &commandShutdownStarter{ctx: ctx, runErr: errors.New("command failed")}
	RunCommandStarter(starter)

	assert.Equal(t, 1, starter.runCall)
	assert.Equal(t, int32(1), resource.closeCalls.Load(),
		"a failing command must still release its globals")
}

// TestRunCommandStarter_ReleaseFailureDoesNotStopRecovery 确认一个资源关不掉不会
// 阻止其余资源被释放：关闭失败只上报，不中断回收。
func TestRunCommandStarter_ReleaseFailureDoesNotStopRecovery(t *testing.T) {
	ctx := newCommandTestContext()
	failing := &closableCommandResource{closeErr: errors.New("close failed")}
	healthy := &closableCommandResource{}

	container := ctx.GetContainer()
	for name, instance := range map[string]*closableCommandResource{
		"failingResource": failing,
		"healthyResource": healthy,
	} {
		value := instance
		require.True(t, container.Register(name, func() (interface{}, error) {
			return value, nil
		}))
		_, err := container.Get(name)
		require.NoError(t, err)
	}

	RunCommandStarter(&commandShutdownStarter{ctx: ctx})

	assert.Equal(t, int32(1), failing.closeCalls.Load())
	assert.Equal(t, int32(1), healthy.closeCalls.Load(),
		"one failing Close must not prevent the others from being released")
}

// TestRunCommandStarter_NilContextIsSafe 确认替身不持有真实上下文时关闭段安静跳过。
//
// commandLifecycleRecorder 就是这种情况：嵌入的 CommandStarter 为 nil，方法分派会
// panic，因此它显式实现 GetContext 返回 nil。关闭段必须容忍这一点。
func TestRunCommandStarter_NilContextIsSafe(t *testing.T) {
	require.NotPanics(t, func() {
		RunCommandStarter(&commandShutdownStarter{ctx: nil})
	})
}
