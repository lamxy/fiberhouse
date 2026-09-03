// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

/*
pprof 性能剖析配置模块，定义独立诊断服务的配置结构、默认值与配置读取逻辑。

该模块的配置项统一位于 yaml 的 application.pprof 命名空间下，默认关闭。
诊断服务独立于业务引擎监听单独端口，因此与核心引擎类型（fiber/gin/hertz）无关。

# 安全默认

  - enable 默认 false：未显式开启时不监听任何端口，零开销
  - host 默认 127.0.0.1：仅回环可访问，避免生产环境公网暴露
  - blockRate / mutexFraction 默认 0：不开启阻塞与锁竞争采样，避免运行时开销

# 配置示例

	application:
	  pprof:
	    enable: false
	    host: "127.0.0.1"
	    port: 6060
	    prefix: "/debug/pprof"
	    blockRate: 0
	    mutexFraction: 0
	    readTimeout: 10
	    writeTimeout: 120
	    shutdownTimeout: 5
	    auth:
	      enable: false
	      username: ""
	      password: ""
*/

package fiberhouse

import (
	"time"

	"github.com/lamxy/fiberhouse/appconfig"
)

// pprof 配置在 yaml 中的命名空间前缀
const pprofConfigPrefix = "application.pprof."

// pprof 各配置项的默认值
const (
	defaultPprofHost            = "127.0.0.1"
	defaultPprofPort            = 6060
	defaultPprofPrefix          = "/debug/pprof"
	defaultPprofReadTimeout     = 10 * time.Second
	defaultPprofWriteTimeout    = 120 * time.Second
	defaultPprofShutdownTimeout = 5 * time.Second
)

// PprofConfig pprof 诊断服务配置
type PprofConfig struct {
	// Enable 总开关，false 时不启动任何监听
	Enable bool

	// Host 监听地址，默认 127.0.0.1（仅回环）
	Host string
	// Port 监听端口，与业务端口分离
	Port int
	// Prefix 路由前缀，默认 /debug/pprof
	Prefix string

	// BlockRate 阻塞采样率，传给 runtime.SetBlockProfileRate。
	// 单位为纳秒，表示平均每发生 BlockRate 纳秒的阻塞事件采样一次。
	// 0 表示关闭；设为 1 会记录所有阻塞事件，开销极大，仅短期诊断使用。
	// 未开启时 /debug/pprof/block 返回空 profile。
	BlockRate int
	// MutexFraction 锁竞争采样分母，传给 runtime.SetMutexProfileFraction。
	// 表示平均每 MutexFraction 次争用事件采样一次，典型值 100。
	// 0 表示关闭；未开启时 /debug/pprof/mutex 返回空 profile。
	MutexFraction int

	// ReadTimeout 读超时
	ReadTimeout time.Duration
	// WriteTimeout 写超时。必须大于 CPU profile 请求的 seconds 参数，
	// 否则 go tool pprof 采集长时间 profile 时会被服务端写超时截断。
	// 设为 0 表示不限制。
	WriteTimeout time.Duration
	// ShutdownTimeout 优雅关闭等待超时
	ShutdownTimeout time.Duration

	// AuthEnable 是否启用 BasicAuth 鉴权
	AuthEnable bool
	// AuthUsername BasicAuth 用户名
	AuthUsername string
	// AuthPassword BasicAuth 密码
	AuthPassword string
}

// LoadPprofConfig 从应用配置读取 pprof 配置并填充默认值
func LoadPprofConfig(cfg appconfig.IAppConfig) PprofConfig {
	return PprofConfig{
		Enable: cfg.Bool(pprofConfigPrefix + "enable"),

		Host:   cfg.String(pprofConfigPrefix+"host", defaultPprofHost),
		Port:   cfg.Int(pprofConfigPrefix+"port", defaultPprofPort),
		Prefix: normalizePprofPrefix(cfg.String(pprofConfigPrefix+"prefix", defaultPprofPrefix)),

		BlockRate:     cfg.Int(pprofConfigPrefix + "blockRate"),
		MutexFraction: cfg.Int(pprofConfigPrefix + "mutexFraction"),

		// Duration 语义与框架其他配置保持一致：yaml 中填写秒数
		ReadTimeout:     cfg.Duration(pprofConfigPrefix+"readTimeout", defaultPprofReadTimeout/time.Second) * time.Second,
		WriteTimeout:    cfg.Duration(pprofConfigPrefix+"writeTimeout", defaultPprofWriteTimeout/time.Second) * time.Second,
		ShutdownTimeout: cfg.Duration(pprofConfigPrefix+"shutdownTimeout", defaultPprofShutdownTimeout/time.Second) * time.Second,

		AuthEnable:   cfg.Bool(pprofConfigPrefix + "auth.enable"),
		AuthUsername: cfg.String(pprofConfigPrefix+"auth.username", ""),
		AuthPassword: cfg.String(pprofConfigPrefix+"auth.password", ""),
	}
}

// normalizePprofPrefix 规范化路由前缀：确保以 / 开头且不以 / 结尾
func normalizePprofPrefix(prefix string) string {
	if prefix == "" {
		return defaultPprofPrefix
	}
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	for len(prefix) > 1 && prefix[len(prefix)-1] == '/' {
		prefix = prefix[:len(prefix)-1]
	}
	if prefix == "/" {
		return defaultPprofPrefix
	}
	return prefix
}
