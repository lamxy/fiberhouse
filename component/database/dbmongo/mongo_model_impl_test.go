// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmongo

import (
	"strings"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// newModelTestContext 构造用于测试的最小 fiberhouse.IContext。
//
// 不能复用 mongo_live_test.go 中的 newTestMongoAppContext：那个文件带
// liveintegration build tag，默认构建下不参与编译。
func newModelTestContext(t *testing.T) fiberhouse.IContext {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{})
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

// modelTestKey 返回一个测试专用的唯一 key。
//
// fiberhouse.NewAppContext 内部使用 NewGlobalManagerOnce，容器是进程级单例，
// 测试之间共享。用测试名派生 key 可以避免相互干扰。
func modelTestKey(t *testing.T) fiberhouse.InstanceKey {
	t.Helper()
	return fiberhouse.InstanceKey("test_mongo_model_" + t.Name())
}

// TestNewMongoModelWithError_UnregisteredKeyReturnsError 确认 key 未注册时返回错误
// 而非 panic，且错误文本包含 key 名。
func TestNewMongoModelWithError_UnregisteredKeyReturnsError(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)

	model, err := NewMongoModelWithError(ctx, key)

	require.Error(t, err)
	require.Nil(t, model)
	require.Contains(t, err.Error(), string(key),
		"error must name the unresolved key so the failure can be located")
}

// TestNewMongoModelWithError_WrongTypeReturnsError 确认 key 下注册了错误类型时返回
// 错误而非 panic。
//
// 这是未经检查的类型断言原本会 panic 的路径：容器的 Get 成功返回，断言在构造函数
// 内部炸掉，且 panic 信息不含 key 名。
func TestNewMongoModelWithError_WrongTypeReturnsError(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)
	require.True(t, ctx.GetContainer().Register(key.String(),
		func() (interface{}, error) { return "not a MongoDb", nil }))
	_, err := ctx.GetContainer().Get(key.String())
	require.NoError(t, err)

	model, err := NewMongoModelWithError(ctx, key)

	require.Error(t, err)
	require.Nil(t, model)
	require.Contains(t, err.Error(), string(key), "error must name the conflicting key")
	require.Contains(t, err.Error(), "want *dbmongo.MongoDb", "error must state the expected type")
}

// TestNewMongoModelWithError_ResolvesRegisteredInstance 确认注册了正确类型时成功返回。
func TestNewMongoModelWithError_ResolvesRegisteredInstance(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)
	want := &MongoDb{Ctx: ctx}
	require.True(t, ctx.GetContainer().Register(key.String(),
		func() (interface{}, error) { return want, nil }))

	model, err := NewMongoModelWithError(ctx, key)

	require.NoError(t, err)
	require.NotNil(t, model)
	require.Same(t, want, model.GetDB())
}

// TestNewMongoModel_PanicsWithErrorValue 确认 panic 变体传出的是 error 值而非裸字符串。
//
// 原实现是 panic(err.Error())，恢复方拿到的是 string，无法用 errors.Is/As 判别根因。
func TestNewMongoModel_PanicsWithErrorValue(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)

	defer func() {
		recovered := recover()
		require.NotNil(t, recovered, "NewMongoModel must panic for an unregistered key")

		err, ok := recovered.(error)
		require.True(t, ok, "panic value must be an error, got %T", recovered)
		require.Contains(t, err.Error(), string(key))
		require.False(t, strings.HasPrefix(err.Error(), "%!"),
			"error must not be a malformed format string")
	}()

	NewMongoModel(ctx, key)
	t.Fatal("NewMongoModel did not panic")
}

// TestMongoDbClose_NilClientIsSafe 确认未初始化 Client 时重复 Close 安全返回。
//
// 注意本用例的边界：Client 为 nil 时 Close 在 CAS 之前就返回了，因此它验证的是
// nil 分支，**不**证明 closed 标志本身生效。真正的幂等性（第二次 Disconnect 不被
// 发起）需要一个活的 client，由 liveintegration 下的
// TestLive_MongoDb_CloseIsIdempotent 覆盖。
func TestMongoDbClose_NilClientIsSafe(t *testing.T) {
	ctx := newModelTestContext(t)
	db := &MongoDb{Ctx: ctx}

	require.NoError(t, db.Close())
	require.NoError(t, db.Close())
	require.NoError(t, db.Close())
}
