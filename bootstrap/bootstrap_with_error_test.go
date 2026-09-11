// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package bootstrap

import (
	"errors"
	"io"
	"testing"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/constant"
)

// 本文件锁定 bootstrap 异步日志 writer 装配 panic 的替代入口（PRD C1）。
//
// NewWriterAsync 在 writer 类型未注册或注册实例类型不符时 panic。日志 writer
// 装配失败是**可恢复**的：调用方可以退回同步 writer 或 console，让应用带着
// 降级的日志继续启动，而不是在启动期直接终止进程。

// TestNewWriterAsyncWithError_ReportsWrongInstanceType 容器里的实例类型不符时
// 返回错误。
//
// 预先用同一个 key 注册一个非 io.WriteCloser 的实例：NewWriterAsync 的
// Register 对已存在的 key 不会覆盖，因此解析时拿到的就是这个错误类型的实例。
func TestNewWriterAsyncWithError_ReportsWrongInstanceType(t *testing.T) {
	cfg := appconfig.NewAppConfig()
	cfg.LoadDefault(map[string]interface{}{
		"application.appLog.asyncConf.type": "diode",
	})

	// 抢先占用 key，放入一个不是 io.WriteCloser 的实例。
	cfg.GetContainer().Register(constant.LogWriterKeyPrefix+"diode", func() (interface{}, error) {
		return "not a write closer", nil
	})

	logWriter, err := NewWriterAsyncWithError(cfg, "probe.log")

	if err == nil {
		t.Fatal("a wrongly typed writer instance must be reported")
	}
	if !errors.Is(err, ErrAsyncWriterUnavailable) {
		t.Fatalf("error = %v, want ErrAsyncWriterUnavailable in the chain", err)
	}
	if logWriter != nil {
		t.Fatal("a failed resolution must return a nil writer")
	}
}

// TestNewWriterAsync_KeepsPanicking 原入口保留 panic。
func TestNewWriterAsync_KeepsPanicking(t *testing.T) {
	cfg := appconfig.NewAppConfig()
	cfg.LoadDefault(map[string]interface{}{
		"application.appLog.asyncConf.type": "diode",
	})
	cfg.GetContainer().Register(constant.LogWriterKeyPrefix+"diode", func() (interface{}, error) {
		return "not a write closer", nil
	})

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("NewWriterAsync must keep panicking on a wrongly typed writer")
		}
	}()
	NewWriterAsync(cfg, "probe.log")
}

// TestNewWriterAsyncWithError_ResolvesConfiguredWriter 正常路径返回可用 writer。
func TestNewWriterAsyncWithError_ResolvesConfiguredWriter(t *testing.T) {
	cfg := appconfig.NewAppConfig()
	cfg.LoadDefault(map[string]interface{}{
		"application.appLog.asyncConf.type": "chan",
	})

	logWriter, err := NewWriterAsyncWithError(cfg, t.TempDir()+"/probe.log")

	if err != nil {
		t.Fatalf("NewWriterAsyncWithError() error = %v", err)
	}
	if logWriter == nil {
		t.Fatal("a successful resolution must return a writer")
	}
	t.Cleanup(func() { _ = logWriter.Close() })

	var _ io.WriteCloser = logWriter
}

// TestErrAsyncWriterUnavailable_IsClassifiable 哨兵必须可分类。
func TestErrAsyncWriterUnavailable_IsClassifiable(t *testing.T) {
	if ErrAsyncWriterUnavailable.Error() == "" {
		t.Fatal("the sentinel must carry a message")
	}
}
