// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"os"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/globalmanager"
)

// 本文件覆盖运行分支失败后的资源回收。
//
// coordinateServerRun 的 select 有两条分支。信号分支调用 Shutdown，回收由那里完成；
// 运行分支（case runErr = <-runResult）直接返回，从不调用 Shutdown——它覆盖监听绑定
// 失败与运行 goroutine panic 两种情况。这两条路径此前没有任何回收：OnShutdown hook
// 只在 coreApp.Shutdown() 时触发，而这条分支从不调用它，因此注册的 MySQL 连接池、
// Mongo 与 Redis 客户端一直保持到进程退出。

// closableRunResource 记录自己被关闭的次数。
type closableRunResource struct {
	closeCalls atomic.Int32
}

func (r *closableRunResource) Close() error {
	r.closeCalls.Add(1)
	return nil
}

// runFailureStarter 是最小的 ApplicationStarter 替身：持有真实上下文，
// AppCoreRun 立即返回错误，Shutdown 记录自己是否被调用过。
type runFailureStarter struct {
	ApplicationStarter
	ctx           IApplicationContext
	runErr        error
	shutdownCalls int
}

func (s *runFailureStarter) GetAppContext() IApplicationContext { return s.ctx }
func (s *runFailureStarter) AppCoreRun(...IProviderManager) error {
	return s.runErr
}
func (s *runFailureStarter) Shutdown(...IProviderManager) error {
	s.shutdownCalls++
	return nil
}

func newRunFailureContext(t *testing.T) IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	cfg.Initialize()
	logger := zerolog.Nop()
	return NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

// registerRunResource 注册并实例化一个可关闭资源。
//
// 必须 Get 一次让实例落位：ReleaseAll 只处理已初始化的 entry，否则用例会恒真通过。
func registerRunResource(t *testing.T, ctx IApplicationContext, key string) *closableRunResource {
	t.Helper()
	resource := &closableRunResource{}
	container := ctx.GetContainer()
	require.True(t, container.Register(globalmanager.KeyName(key), func() (interface{}, error) {
		return resource, nil
	}))
	_, err := container.Get(globalmanager.KeyName(key))
	require.NoError(t, err)
	t.Cleanup(func() { container.Unregister(globalmanager.KeyName(key)) })
	return resource
}

// TestCoordinateServerRun_RunErrorBranchSkipsShutdown 锁定前提：运行分支确实不调用
// Shutdown。这正是该分支此前零回收的原因，也是需要单独补回收的依据。
func TestCoordinateServerRun_RunErrorBranchSkipsShutdown(t *testing.T) {
	starter := &runFailureStarter{runErr: errors.New("listen failed")}
	stopCh := make(chan os.Signal, 1)

	runErr, shutdownErr, shutdownRequested := coordinateServerRun(starter, nil, nil, stopCh)

	require.Error(t, runErr, "the run error must propagate")
	require.NoError(t, shutdownErr)
	assert.False(t, shutdownRequested, "the run branch must not report a shutdown")
	assert.Zero(t, starter.shutdownCalls,
		"the run branch returns without calling Shutdown -- this is why cleanup must be added separately")
}

// TestReleaseGlobalsAfterRunFailure_ClosesRegisteredGlobals 验证补上的回收确实关闭
// 已初始化的全局对象并清空容器。
func TestReleaseGlobalsAfterRunFailure_ClosesRegisteredGlobals(t *testing.T) {
	ctx := newRunFailureContext(t)
	resource := registerRunResource(t, ctx, "runFailureResource")

	releaseGlobalsAfterRunFailure(&runFailureStarter{ctx: ctx})

	assert.Equal(t, int32(1), resource.closeCalls.Load(),
		"a failed run must still release the globals it registered")
	assert.False(t, ctx.GetContainer().IsRegistered("runFailureResource"),
		"container entries must be cleared after release")
}

// TestReleaseGlobalsAfterRunFailure_NilInputsAreSafe 确认防护到位：启动流程可能在
// 核心装配完成前就失败，此时 starter 或上下文可能为 nil。
func TestReleaseGlobalsAfterRunFailure_NilInputsAreSafe(t *testing.T) {
	require.NotPanics(t, func() { releaseGlobalsAfterRunFailure(nil) })
	require.NotPanics(t, func() { releaseGlobalsAfterRunFailure(&runFailureStarter{ctx: nil}) })
}
