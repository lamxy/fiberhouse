// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package writer

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lamxy/fiberhouse/appconfig"
	"gopkg.in/natefinch/lumberjack.v2"
)

// AsyncChannelWriter 实现异步写日志功能，实现 io.Writer 接口
type AsyncChannelWriter struct {
	logChan        chan []byte        // 用于接收日志数据
	wg             sync.WaitGroup     // 用于等待后台写入完成
	writer         *bufio.Writer      // 缓冲写入器，包装 lumberjack.Logger
	lumber         *lumberjack.Logger // lumberjack 实例，用于管理日志文件滚动等
	closed         int32              // atomic 标志，防止 Close 后 Write 触发 panic
	activeWriters  int64              // 已获准或正在检查准入的 Write 数量（atomic）
	writersDrained chan struct{}      // 通知 Close 所有已获准 Write 已退出
	closeDone      chan struct{}      // 首次 Close 完成后关闭
	closeErr       error              // 首次 Close 的结果，由 closeDone 发布
	droppedLogs    int64              // 因通道满而丢弃的日志条数（atomic）
}

// NewAsyncChannelWriter 创建一个新的异步日志记录器
func NewAsyncChannelWriter(cfg appconfig.IAppConfig, filename string) *AsyncChannelWriter {
	maxSize := cfg.Int("application.appLog.rollConf.maxSize")
	maxBackups := cfg.Int("application.appLog.rollConf.maxBackups")
	maxAge := cfg.Int("application.appLog.rollConf.maxAge")
	compress := cfg.Bool("application.appLog.rollConf.compress")
	bufSize, chSize := cfg.Int("application.appLog.asyncConf.chanConf.bufferSize"), cfg.Int("application.appLog.asyncConf.chanConf.chanSize")
	logRoller := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize, // megabytes
		MaxBackups: maxBackups,
		MaxAge:     maxAge,   //days
		Compress:   compress, // disabled by default
	}

	writer := bufio.NewWriterSize(logRoller, bufSize)

	al := &AsyncChannelWriter{
		logChan:        make(chan []byte, chSize),
		writer:         writer,
		lumber:         logRoller,
		writersDrained: make(chan struct{}, 1),
		closeDone:      make(chan struct{}),
	}

	// 启动后台写入 goroutine
	al.wg.Add(1)
	go al.consume(1 * time.Second)
	return al
}

// consume 后台 goroutine 不断从 logChan 中读取日志数据，并写入底层 Writer
func (a *AsyncChannelWriter) consume(flushInterval time.Duration) {
	defer a.wg.Done()
	ticker := time.NewTicker(flushInterval) // 定时 flush 缓冲区
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-a.logChan:
			if !ok {
				// 通道关闭且已排空，执行最终刷盘后退出
				_ = a.writer.Flush()
				return
			}
			// 写入数据到缓冲区
			_, err := a.writer.Write(data)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "AsyncLogger Write error: %v\n", err)
			}
		case <-ticker.C:
			_ = a.writer.Flush()
		}
	}
}

// Write 方法实现 io.Writer 接口，将数据放入 logChan
func (a *AsyncChannelWriter) Write(p []byte) (int, error) {
	atomic.AddInt64(&a.activeWriters, 1)
	if atomic.LoadInt32(&a.closed) == 1 {
		a.finishWrite()
		return 0, fmt.Errorf("AsyncChannelWriter: writer is closed")
	}

	// 拷贝数据，避免传参 slice 被复用
	data := make([]byte, len(p))
	copy(data, p)

	// 通道有空位时立即写入；通道持续满超过 1s 才丢弃，期间调用方阻塞等待
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	var dropped int64
	select {
	case a.logChan <- data:
	case <-timer.C:
		// 超过 1s 仍无法写入，丢弃并计数
		dropped = atomic.AddInt64(&a.droppedLogs, 1)
	}

	a.finishWrite()
	if dropped%100 == 1 {
		_, _ = fmt.Fprintf(os.Stderr, "AsyncChannelWriter: log channel full, total dropped: %d\n", dropped)
	}

	return len(p), nil
}

func (a *AsyncChannelWriter) finishWrite() {
	if atomic.AddInt64(&a.activeWriters, -1) == 0 && atomic.LoadInt32(&a.closed) == 1 {
		select {
		case a.writersDrained <- struct{}{}:
		default:
		}
	}
}

// DroppedLogs 返回因通道满而丢弃的日志总条数
func (a *AsyncChannelWriter) DroppedLogs() int64 {
	return atomic.LoadInt64(&a.droppedLogs)
}

// Close 关闭日志记录器，等待后台 goroutine 完成所有写入后再关闭底层文件
func (a *AsyncChannelWriter) Close() error {
	if !atomic.CompareAndSwapInt32(&a.closed, 0, 1) {
		<-a.closeDone
		return a.closeErr
	}

	for atomic.LoadInt64(&a.activeWriters) != 0 {
		<-a.writersDrained
	}
	close(a.logChan)
	// 不在此处 Flush：consume goroutine 在通道耗尽后负责最终 Flush，避免并发写 bufio.Writer
	a.wg.Wait()
	a.closeErr = a.lumber.Close()
	close(a.closeDone)
	return a.closeErr
}
