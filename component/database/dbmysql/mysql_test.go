// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmysql

import (
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMysqlAppContext 构造用于测试的最小 fiberhouse.IContext，
// 参照 component/cache/cachelocal/local_cache_test.go、
// component/cache/cacheremote/redis_cache_test.go 的既有写法。
func newTestMysqlAppContext(t *testing.T, dsn string) fiberhouse.IContext {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"test.mysql.dsn":                  dsn,
		"test.mysql.gorm.maxIdleConns":    2,
		"test.mysql.gorm.maxOpenConns":    5,
		"test.mysql.gorm.connMaxLifetime": int64(60),
		"test.mysql.gorm.connMaxIdleTime": int64(60),
		"test.mysql.gorm.logger.enable":   false,
	})
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
}

// TestNewClient_PingFailureReturnsError 验证行为基线：
// PingContext 失败时 NewClient 必须返回非 nil error，db 为 nil。
// 这是 P1-2（sqlDb.Close() 泄漏修复）不能破坏的既有行为，必须在修复前后都
// 稳定通过。
//
// 关于连接池泄漏的直接/间接检测：本任务评估了两种规格文件建议的间接观测
// 方式，均在本地环境下不可靠，已放弃：
//
//  1. runtime.NumGoroutine() 计数对比（连续调用 NewClient 指向一个明显未
//     监听的端口 127.0.0.1:1，观察前后 goroutine 数量变化）：实测该地址上
//     连接会被系统立即以 "connection refused" 拒绝，database/sql 的连接器
//     goroutine 在拨号失败后会同步退出，不会残留可观测的 goroutine（本地
//     20 次调用观测 before=after，无法作为泄漏证据，无论修复前后表现一致，
//     不具区分度）。
//  2. net.Listen 建立本地 TCP listener 但不 Accept，模拟连接建立后端服务器
//     无响应的挂起场景：实测该方式会使每次 NewClient 调用真正阻塞到 MySQL
//     driver / DSN 层超时或更久，10 次调用即导致测试整体运行超过 60 秒，
//     不满足"测试执行时间要合理"的约束，且容易在不同环境/驱动版本下出现
//     不稳定的超时行为，判断为规格文件停止条件中警告的"过度复杂或脆弱的
//     并发/时序断言"。
//
// 因此采用规格文件允许的"退而求其次"方案：仅保留本行为基线测试，防止
// 修复破坏既有的错误返回行为；泄漏修复本身（在 PingContext 失败分支补上
// sqlDb.Close()）风险低、逻辑直观，其正确性依赖任务 2 的代码审查确认，而
// 非依赖脆弱的间接观测测试。
func TestNewClient_PingFailureReturnsError(t *testing.T) {
	// 指向本地明显不会有 MySQL 服务监听的地址，DSN 层 timeout 保持测试快速失败。
	ctx := newTestMysqlAppContext(t, "root:root@tcp(127.0.0.1:1)/test?timeout=200ms")
	db, err := NewClient(ctx, "test.mysql")
	require.Error(t, err)
	assert.Nil(t, db)

	// 任务 2（P1-2 修复）新增了 sqlDb.Close() 调用，仅记录 Close 自身的错误
	// 日志，不得包装/替换原始 ping 错误。断言返回的 error 仍然来自底层
	// 拨号失败（"connection refused"），而不包含 close 相关字样，确认
	// 调用方看到的错误信息未被清理动作污染。
	assert.Contains(t, err.Error(), "connect: connection refused")
	assert.NotContains(t, err.Error(), "close")
}
