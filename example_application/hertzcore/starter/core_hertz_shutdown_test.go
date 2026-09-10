// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package starter

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// 本文件覆蓋 hertzcore 的關閉段。
//
// 此前這個套件沒有任何測試檔，而它的 OnShutdown 鉤子只寫一行日誌、Shutdown 也不
// 釋放全域物件——應用註冊的 MySQL 連線池、Mongo 與 Redis 客戶端從未 Close。
//
// 重點驗證 defer 放置：Shutdown 有五處提前返回，寫在方法尾端的釋放會被它們全部
// 跳過。這正是 Fiber 側同型缺陷的成因，不能在這裡重蹈。

// closableHertzResource 記錄自己被關閉的次數。
type closableHertzResource struct {
	closeCalls atomic.Int32
	closeErr   error
}

func (r *closableHertzResource) Close() error {
	r.closeCalls.Add(1)
	return r.closeErr
}

func newHertzTestContext(t *testing.T) fiberhouse.IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	cfg.Initialize()
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

// registerHertzResource 註冊並實例化一個可關閉資源。
//
// 必須 Get 一次讓實例落位：ReleaseAll 只處理已初始化的 entry，否則用例會恆真通過
// 而不檢驗任何東西。
func registerHertzResource(t *testing.T, ctx fiberhouse.IApplicationContext, key string) *closableHertzResource {
	t.Helper()
	resource := &closableHertzResource{}
	container := ctx.GetContainer()
	require.True(t, container.Register(key, func() (interface{}, error) {
		return resource, nil
	}))
	_, err := container.Get(key)
	require.NoError(t, err)
	t.Cleanup(func() { container.Unregister(key) })
	return resource
}

// TestHertzShutdown_ReleasesGlobalsOnEarlyReturn 鎖定 defer 放置。
//
// 初始化失敗時 Shutdown 在第一個檢查就返回，遠早於方法尾端。若釋放寫在尾端，這條
// 路徑不會釋放任何東西；用 defer 才會。
func TestHertzShutdown_ReleasesGlobalsOnEarlyReturn(t *testing.T) {
	ctx := newHertzTestContext(t)
	resource := registerHertzResource(t, ctx, "hertzEarlyReturnResource")

	core := NewCoreWithHertz(ctx).(*CoreWithHertz)
	core.initErr = errors.New("initialization failed")

	err := core.Shutdown()

	require.Error(t, err, "an initialization failure must surface")
	assert.Equal(t, int32(1), resource.closeCalls.Load(),
		"globals must be released even when Shutdown returns early")
}

// TestHertzShutdown_ReleaseIsIdempotent 確認 once 守衛生效。
//
// 正常路徑會在關閉日誌器前顯式呼叫一次，defer 再呼叫一次；資源的 Close 只能記到
// 一次。這個守衛不是美化——LoggerWrap.Close 沒有冪等保護，重複釋放會把錯誤上報到
// 已關閉的 writer。
func TestHertzShutdown_ReleaseIsIdempotent(t *testing.T) {
	ctx := newHertzTestContext(t)
	resource := registerHertzResource(t, ctx, "hertzIdempotentResource")

	core := NewCoreWithHertz(ctx).(*CoreWithHertz)

	core.releaseHertzGlobals()
	core.releaseHertzGlobals()
	core.releaseHertzGlobals()

	assert.Equal(t, int32(1), resource.closeCalls.Load(),
		"the once guard must collapse repeated releases into one")
}

// TestHertzShutdown_ReleaseFailureDoesNotStopRecovery 確認一個資源關不掉不會阻止
// 其餘資源被釋放：關閉失敗只上報，不中斷回收。
func TestHertzShutdown_ReleaseFailureDoesNotStopRecovery(t *testing.T) {
	ctx := newHertzTestContext(t)

	failing := registerHertzResource(t, ctx, "hertzFailingResource")
	failing.closeErr = errors.New("close failed")
	healthy := registerHertzResource(t, ctx, "hertzHealthyResource")

	core := NewCoreWithHertz(ctx).(*CoreWithHertz)
	core.releaseHertzGlobals()

	assert.Equal(t, int32(1), failing.closeCalls.Load())
	assert.Equal(t, int32(1), healthy.closeCalls.Load(),
		"one failing Close must not strand the others")
}

// TestHertzShutdown_ReleaseGuardIsPerInstance 確認 once 守衛掛在實例上。
//
// 若守衛是套件層級的，同進程的第二個應用實例會被前一個的釋放狀態擋掉，永遠拿不到
// 清理。
func TestHertzShutdown_ReleaseGuardIsPerInstance(t *testing.T) {
	firstCtx := newHertzTestContext(t)
	firstResource := registerHertzResource(t, firstCtx, "hertzFirstInstanceResource")
	first := NewCoreWithHertz(firstCtx).(*CoreWithHertz)
	first.releaseHertzGlobals()
	require.Equal(t, int32(1), firstResource.closeCalls.Load())

	secondCtx := newHertzTestContext(t)
	secondResource := registerHertzResource(t, secondCtx, "hertzSecondInstanceResource")
	second := NewCoreWithHertz(secondCtx).(*CoreWithHertz)
	second.releaseHertzGlobals()

	assert.Equal(t, int32(1), secondResource.closeCalls.Load(),
		"a second core instance must still release its own globals")
}
