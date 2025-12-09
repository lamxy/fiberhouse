// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package writer

import (
	"github.com/lamxy/fiberhouse/appconfig"
	"gopkg.in/natefinch/lumberjack.v2"
)

// SyncLumberjackWriter 实现同步写日志功能，实现 io.Writer 接口
type SyncLumberjackWriter struct {
	lumber *lumberjack.Logger // lumberjack 实例，用于管理日志文件滚动等
}

func NewSyncLumberjackWriter(cfg appconfig.IAppConfig, filename string) *SyncLumberjackWriter {
	maxSize := cfg.Int("application.appLog.rollConf.maxSize")
	maxBackups := cfg.Int("application.appLog.rollConf.maxBackups")
	maxAge := cfg.Int("application.appLog.rollConf.maxAge")
	compress := cfg.Bool("application.appLog.rollConf.compress")

	logRoller := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize, // megabytes
		MaxBackups: maxBackups,
		MaxAge:     maxAge,   // days
		Compress:   compress, // disabled by default
	}
	return &SyncLumberjackWriter{
		lumber: logRoller,
	}
}

func (sw *SyncLumberjackWriter) Write(p []byte) (int, error) {
	return sw.lumber.Write(p)
}

func (sw *SyncLumberjackWriter) Close() error {
	return sw.lumber.Close()
}
