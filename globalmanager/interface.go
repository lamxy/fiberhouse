// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package globalmanager

// Closable 定义可关闭行为
type Closable interface {
	Close() error
}

// HealthChecker 定义健康检查行为
type HealthChecker interface {
	IsHealthy() bool
}

// Rebuilder 定义重建行为
type Rebuilder interface {
	Rebuild(...interface{}) (interface{}, error)
	GetConfPath() string // 获取配置路径
}
