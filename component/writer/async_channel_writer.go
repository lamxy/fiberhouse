// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package writer

import (
	"bufio"
	"fmt"
	"github.com/lamxy/fiberhouse/appconfig"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"sync"
	"time"
)

// AsyncChannelWriter 实现异步写日志功能，实现 io.Writer 接口
type AsyncChannelWriter struct {
	logChan chan []byte        // 用于接收日志数据
	wg      sync.WaitGroup     // 用于等待后台写入完成
	writer  *bufio.Writer      // 缓冲写入器，包装 lumberjack.Logger
	lumber  *lumberjack.Logger // lumberjack 实例，用于管理日志文件滚动等
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
		logChan: make(chan []byte, chSize),
		writer:  writer,
		lumber:  logRoller,
	}

	// 启动后台写入 goroutine
	al.wg.Add(1)
	go al.consume(1 * time.Second)
	return al
}

// start 后台 goroutine 不断从 logChan 中读取日志数据，并写入底层 Writer
func (a *AsyncChannelWriter) consume(flushInterval time.Duration) {
	defer a.wg.Done()
	ticker := time.NewTicker(flushInterval) // 定时 flush 缓冲区
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-a.logChan:
			if !ok {
				// 通道关闭时 flush 并退出
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

	// 将数据发送到通道，可能会阻塞但保证数据不丢失
	//a.logChan <- p

	// 当通道满了，写阻塞时，超时1s丢弃消息
	// 拷贝数据，避免传参 slice 被复用
	l := len(p)
	data := make([]byte, l)
	copy(data, p)
	select {
	case a.logChan <- data:
	case <-time.After(1 * time.Second):
		// TODO 指标监控drop计数
	}

	return len(p), nil
}

// Close 关闭日志记录器，等待后台 goroutine 完成
func (a *AsyncChannelWriter) Close() error {
	close(a.logChan)
	_ = a.writer.Flush()
	a.wg.Wait()
	return a.lumber.Close()
}
