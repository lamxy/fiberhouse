// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmysql

import (
	"strings"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/stretchr/testify/require"
)

// modelTestKey 返回一个测试专用的唯一 key。
//
// fiberhouse.NewAppContext 内部使用 NewGlobalManagerOnce，容器是进程级单例，
// 测试之间共享。用测试名派生 key 可以避免相互干扰，也不需要清空全局容器。
func modelTestKey(t *testing.T) fiberhouse.InstanceKey {
	t.Helper()
	return fiberhouse.InstanceKey("test_mysql_model_" + t.Name())
}

func newModelTestContext(t *testing.T) fiberhouse.IContext {
	t.Helper()
	return newTestMysqlAppContext(t, "unused-dsn")
}

// TestNewMysqlModelWithError_UnregisteredKeyReturnsError 确认 key 未注册时返回错误
// 而非 panic，且错误文本包含 key 名。
func TestNewMysqlModelWithError_UnregisteredKeyReturnsError(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)

	model, err := NewMysqlModelWithError(ctx, key)

	require.Error(t, err)
	require.Nil(t, model)
	require.Contains(t, err.Error(), string(key),
		"error must name the unresolved key so the failure can be located")
}

// TestNewMysqlModelWithError_WrongTypeReturnsError 确认 key 下注册了错误类型时返回
// 错误而非 panic。
//
// 这是未经检查的类型断言原本会 panic 的路径：容器的 Get 成功返回，断言在构造函数
// 内部炸掉，且 panic 信息不含 key 名。
func TestNewMysqlModelWithError_WrongTypeReturnsError(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)
	require.True(t, ctx.GetContainer().Register(key.String(),
		func() (interface{}, error) { return "not a MysqlDb", nil }))
	_, err := ctx.GetContainer().Get(key.String())
	require.NoError(t, err)

	model, err := NewMysqlModelWithError(ctx, key)

	require.Error(t, err)
	require.Nil(t, model)
	require.Contains(t, err.Error(), string(key), "error must name the conflicting key")
	require.Contains(t, err.Error(), "want *dbmysql.MysqlDb", "error must state the expected type")
}

// TestNewMysqlModelWithError_ResolvesRegisteredInstance 确认注册了正确类型时成功返回。
func TestNewMysqlModelWithError_ResolvesRegisteredInstance(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)
	want := &MysqlDb{Ctx: ctx}
	require.True(t, ctx.GetContainer().Register(key.String(),
		func() (interface{}, error) { return want, nil }))

	model, err := NewMysqlModelWithError(ctx, key)

	require.NoError(t, err)
	require.NotNil(t, model)
	require.Same(t, want, model.GetDB())
}

// TestNewMysqlModel_PanicsWithErrorValue 确认 panic 变体传出的是 error 值而非裸字符串。
//
// 原实现是 panic(err.Error())，恢复方拿到的是 string，无法用 errors.Is/As 判别根因。
func TestNewMysqlModel_PanicsWithErrorValue(t *testing.T) {
	ctx := newModelTestContext(t)
	key := modelTestKey(t)

	defer func() {
		recovered := recover()
		require.NotNil(t, recovered, "NewMysqlModel must panic for an unregistered key")

		err, ok := recovered.(error)
		require.True(t, ok, "panic value must be an error, got %T", recovered)
		require.Contains(t, err.Error(), string(key))
		require.False(t, strings.HasPrefix(err.Error(), "%!"),
			"error must not be a malformed format string")
	}()

	NewMysqlModel(ctx, key)
	t.Fatal("NewMysqlModel did not panic")
}
