// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

/*
Package recover 的配置模块，定义中间件的配置结构和默认值。

该文件包含 recover 中间件的所有配置选项定义，支持灵活的自定义配置，
包括堆栈跟踪、调试模式、日志记录等功能的开关控制。

# 配置结构说明

Config 结构体包含以下配置选项：

  - Next: 条件跳过中间件的函数
  - EnableStackTrace: 堆栈跟踪功能开关
  - StackTraceHandler: 自定义堆栈跟踪处理器
  - Logger: 日志记录器接口
  - AppContext: 应用框架上下文
  - Stdout: 标准输出开关
  - DebugMode: 调试模式开关

# 使用示例

使用默认配置：

	app.Use(recover.New())

使用自定义配置：

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		DebugMode: false,
		StackTraceHandler: customHandler,
	}))
*/
package recover

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

// Config 结构体用于定义 recover 中间件的配置项。
type Config struct {
	// Next 定义了一个函数，当返回 true 时跳过该中间件。
	//
	// 可选。 默认: nil
	Next func(c *fiber.Ctx) bool

	// EnableStackTrace 表示是否启用堆栈跟踪功能
	//
	// 可选。 默认: false
	EnableStackTrace bool

	// StackTraceHandler 定义了一个处理堆栈跟踪的函数
	//
	// 可选配置。默认值：defaultStackTraceHandler
	StackTraceHandler func(c providerCtx.ContextProvider, e interface{})

	// Logger for record messages
	Logger interface{}

	// AppContext 应用上下文
	AppContext fiberhouse.IApplicationContext

	// 默认输出目标是 os.Stdout
	Stdout bool
	// 调试模式：true 将详细错误信息响应给客户端，否则仅记入日志
	DebugMode bool
}

// ConfigDefault 默认配置
var ConfigDefault = Config{
	Next:              nil,
	EnableStackTrace:  false,
	StackTraceHandler: func(c providerCtx.ContextProvider, e interface{}) {},
	Logger:            nil,
	AppContext:        nil,
	Stdout:            true,
	DebugMode:         false,
}

// ConfigConfigured Configured 已配置
var ConfigConfigured Config

// 辅助函数，用于设置默认配置值
func configDefault(config ...Config) Config {
	// 如果未提供任何配置，则返回默认配置
	if len(config) < 1 {
		ConfigConfigured = ConfigDefault
		return ConfigConfigured
	}

	// 覆盖默认配置
	ConfigConfigured = config[0]

	if ConfigConfigured.EnableStackTrace && ConfigConfigured.StackTraceHandler == nil {
		ConfigConfigured.StackTraceHandler = ConfigDefault.StackTraceHandler
	}

	return ConfigConfigured
}
