// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package writer

import (
	"bufio"
	"code.cloudfoundry.org/go-diodes"
	"github.com/lamxy/fiberhouse/frame/appconfig"
	"gopkg.in/natefinch/lumberjack.v2"
	"sync"
	"time"
)

// AsyncDiodeWriter 实现异步写日志功能，实现 io.Writer 接口
type AsyncDiodeWriter struct {
	diode  diodes.Diode       // 二极管
	wg     sync.WaitGroup     // 用于等待后台写入完成
	stopCh chan struct{}      // 通知子goroutine退出
	writer *bufio.Writer      // 缓冲写入器，接入lumberjack.Logger
	lumber *lumberjack.Logger // lumberjack 实例，用于管理日志文件滚动等
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

	dd := diodes.NewManyToOne(diodeSize, diodes.AlertFunc(func(missed int) {
		// TODO 指标记录drop计数
		//fmt.Printf("AsyncDiodeWriter: %d messages dropped due to full diode buffer\n", missed)
	}))

	aw := &AsyncDiodeWriter{
		diode:  dd,
		wg:     sync.WaitGroup{},
		stopCh: make(chan struct{}),
		writer: writer,
		lumber: logRoller,
	}

	// 启动后台写入 goroutine
	aw.wg.Add(1)
	go aw.consume(diodeInterval)
	return aw
}

// consume 后台 goroutine 不断从 二极管 中读取日志数据，并写入底层 Writer
func (a *AsyncDiodeWriter) consume(flushInterval time.Duration) {
	defer a.wg.Done()
	ticker := time.NewTicker(flushInterval) // 定时 flush 缓冲区
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
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

// Write 方法实现 io.Writer 接口，将数据写入二极管
func (a *AsyncDiodeWriter) Write(p []byte) (int, error) {
	// 拷贝数据，避免传参 slice 被复用
	l := len(p)
	data := make([]byte, l)
	copy(data, p)

	a.diode.Set(diodes.GenericDataType(&data))
	return l, nil
}

// Close 关闭日志记录器，等待后台 goroutine 完成
func (a *AsyncDiodeWriter) Close() error {
	close(a.stopCh)
	_ = a.writer.Flush()
	a.wg.Wait()
	return a.lumber.Close()
}
