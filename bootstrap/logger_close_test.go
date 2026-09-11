// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package bootstrap

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
)

// 本文件锁定 LoggerWrap.Close 的幂等契约（PRD 任务 12 G3）。
//
// 日志器的关闭有多个入口：Fiber 的 clearApplicationGlobalsOnce、Gin 的 Shutdown
// 尾部、CLI 的 releaseCommandGlobals、以及应用自行调用。此前 Close 直接转发给
// 注入的 wCloser，重复关闭是否安全完全取决于该 writer 的实现——框架自带的两个
// 异步 writer 恰好幂等，但那是它们的实现细节，不是 LoggerWrapper 的契约。

// countingWriteCloser 记录自身被关闭的次数。
type countingWriteCloser struct {
	closeCalls atomic.Int32
	closeErr   error
}

func (w *countingWriteCloser) Write(p []byte) (int, error) { return len(p), nil }

func (w *countingWriteCloser) Close() error {
	w.closeCalls.Add(1)
	return w.closeErr
}

// TestLoggerWrapClose_ClosesUnderlyingWriterOnce 重复 Close 只穿透一次。
func TestLoggerWrapClose_ClosesUnderlyingWriterOnce(t *testing.T) {
	writer := &countingWriteCloser{}
	logger := zerolog.New(writer)
	wrap := NewLoggerWrap(&logger, writer)

	for i := 0; i < 5; i++ {
		if err := wrap.Close(); err != nil {
			t.Fatalf("Close() call %d error = %v", i+1, err)
		}
	}

	if got := writer.closeCalls.Load(); got != 1 {
		t.Fatalf("underlying Close calls = %d, want 1: "+
			"the guard must belong to LoggerWrap, not to the injected writer", got)
	}
}

// TestLoggerWrapClose_ReplaysFirstError 首次错误在后续调用中原样返回。
//
// 调用方可能在多个关闭入口各检查一次返回值；若第二次返回 nil，失败会被误读为
// 「已成功关闭」。
func TestLoggerWrapClose_ReplaysFirstError(t *testing.T) {
	sentinel := errors.New("writer close failed")
	writer := &countingWriteCloser{closeErr: sentinel}
	logger := zerolog.New(writer)
	wrap := NewLoggerWrap(&logger, writer)

	if err := wrap.Close(); !errors.Is(err, sentinel) {
		t.Fatalf("first Close() error = %v, want the writer error", err)
	}
	if err := wrap.Close(); !errors.Is(err, sentinel) {
		t.Fatalf("second Close() error = %v, want the cached first error", err)
	}
	if got := writer.closeCalls.Load(); got != 1 {
		t.Fatalf("underlying Close calls = %d, want 1", got)
	}
}

// TestLoggerWrapClose_ConcurrentCallsCloseOnce 并发关闭只穿透一次。
//
// 以 -race 执行。多个关闭入口可能来自不同 goroutine（例如信号处理与运行分支
// 同时收尾）。
func TestLoggerWrapClose_ConcurrentCallsCloseOnce(t *testing.T) {
	writer := &countingWriteCloser{}
	logger := zerolog.New(writer)
	wrap := NewLoggerWrap(&logger, writer)

	var wg sync.WaitGroup
	const closers = 16
	wg.Add(closers)
	for i := 0; i < closers; i++ {
		go func() {
			defer wg.Done()
			_ = wrap.Close()
		}()
	}
	wg.Wait()

	if got := writer.closeCalls.Load(); got != 1 {
		t.Fatalf("underlying Close calls = %d, want 1", got)
	}
}

// TestLoggerWrapClose_NoWriterIsNoOp 未注入 writer 时 Close 是空操作。
//
// NewLoggerWrap 的 wCloser 是可选参数；大量测试与仅控制台输出的场景都不注入。
func TestLoggerWrapClose_NoWriterIsNoOp(t *testing.T) {
	logger := zerolog.Nop()
	wrap := NewLoggerWrap(&logger)

	for i := 0; i < 3; i++ {
		if err := wrap.Close(); err != nil {
			t.Fatalf("Close() without a writer error = %v, want nil", err)
		}
	}
}
