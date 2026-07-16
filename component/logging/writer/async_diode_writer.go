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

	"code.cloudfoundry.org/go-diodes"
	"github.com/lamxy/fiberhouse/appconfig"
	"gopkg.in/natefinch/lumberjack.v2"
)

// AsyncDiodeWriter 实现异步写日志功能，实现 io.Writer 接口
type AsyncDiodeWriter struct {
	diode       diodes.Diode       // 二极管
	wg          sync.WaitGroup     // 用于等待后台写入完成
	stopCh      chan struct{}      // 通知子goroutine退出
	closeDone   chan struct{}      // 通知并发 Close 调用原始关闭流程已完成
	writer      *bufio.Writer      // 缓冲写入器，接入lumberjack.Logger
	lumber      *lumberjack.Logger // lumberjack 实例，用于管理日志文件滚动等
	closed      int32              // atomic 标志，防止 Close 后 Write 触发未定义行为
	closeErr    error              // 首次 Close 的结果，由 closeDone 保护发布
	droppedLogs int64              // 因 diode 满而丢弃的日志条数（atomic）
}

// NewAsyncDiodeWriter 创建一个新的异步日志记录器
func NewAsyncDiodeWriter(cfg appconfig.IAppConfig, filename string) *AsyncDiodeWriter {
	maxSize := cfg.Int("application.appLog.rollConf.maxSize")
	maxBackups := cfg.Int("application.appLog.rollConf.maxBackups")
	maxAge := cfg.Int("application.appLog.rollConf.maxAge")
	compress := cfg.Bool("application.appLog.rollConf.compress")
	diodeSize := cfg.Int("application.appLog.asyncConf.diodeConf.size", 33554432) // 必要配置，否则报错: 除数为0 panic
	diodeBuf := cfg.Int("application.appLog.asyncConf.diodeConf.bufferSize", 4096)
	diodeInterval := cfg.Duration("application.appLog.asyncConf.diodeConf.flushInterval", 1000) * time.Millisecond

	logRoller := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize, // megabytes
		MaxBackups: maxBackups,
		MaxAge:     maxAge,   //days
		Compress:   compress, // disabled by default
	}

	writer := bufio.NewWriterSize(logRoller, diodeBuf)

	aw := &AsyncDiodeWriter{
		stopCh:    make(chan struct{}),
		closeDone: make(chan struct{}),
		writer:    writer,
		lumber:    logRoller,
	}

	dd := diodes.NewManyToOne(diodeSize, diodes.AlertFunc(func(missed int) {
		dropped := atomic.AddInt64(&aw.droppedLogs, int64(missed))
		_, _ = fmt.Fprintf(os.Stderr, "AsyncDiodeWriter: diode full, +%d dropped, total: %d\n", missed, dropped)
	}))
	aw.diode = dd

	// 启动后台写入 goroutine
	aw.wg.Add(1)
	go aw.consume(diodeInterval)
	return aw
}

// consume 后台 goroutine 不断从二极管中读取日志数据，并写入底层 Writer
func (a *AsyncDiodeWriter) consume(flushInterval time.Duration) {
	defer a.wg.Done()
	ticker := time.NewTicker(flushInterval) // 定时 flush 缓冲区
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			// 排空 diode 中残留数据，避免关闭时丢失
			for {
				data, ok := a.diode.TryNext()
				if !ok || data == nil {
					break
				}
				b := *(*[]byte)(data)
				_, _ = a.writer.Write(b)
			}
			// 由 consume 负责最终刷盘，Close 不再并发 Flush
			_ = a.writer.Flush()
			return
		case <-ticker.C:
			_ = a.writer.Flush()
		default:
			data, ok := a.diode.TryNext()
			if !ok || data == nil {
				time.Sleep(1000 * time.Microsecond) // 适当睡眠 100 ~ 1000 微秒
				continue
			}
			b := *(*[]byte)(data)
			_, _ = a.writer.Write(b)
		}
	}
}

// Write 方法实现 io.Writer 接口，将数据写入二极管。
// Close 开始后，Write 返回 (0, error)，不会接受更多数据。
func (a *AsyncDiodeWriter) Write(p []byte) (int, error) {
	if atomic.LoadInt32(&a.closed) == 1 {
		return 0, fmt.Errorf("AsyncDiodeWriter: writer is closed")
	}

	// 拷贝数据，避免传参 slice 被复用
	l := len(p)
	data := make([]byte, l)
	copy(data, p)

	a.diode.Set(diodes.GenericDataType(&data))
	return l, nil
}

// DroppedLogs 返回因 diode 满而丢弃的日志总条数
func (a *AsyncDiodeWriter) DroppedLogs() int64 {
	return atomic.LoadInt64(&a.droppedLogs)
}

// Close 关闭日志记录器，等待后台 goroutine 排空并刷盘后再返回。
// Close 可以重复或并发调用；只有首次调用执行关闭，其余调用等待它完成。
func (a *AsyncDiodeWriter) Close() error {
	if !atomic.CompareAndSwapInt32(&a.closed, 0, 1) {
		<-a.closeDone
		return a.closeErr
	}

	close(a.stopCh)
	// 不在此处 Flush：consume goroutine 排空 diode 并完成最终 Flush 后才退出，避免并发写 bufio.Writer
	a.wg.Wait()
	a.closeErr = a.lumber.Close()
	close(a.closeDone)
	return a.closeErr
}
