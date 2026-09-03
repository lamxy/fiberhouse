// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package globalmanager

// Closable 定义可关闭行为，用于优雅关闭时释放资源。
type Closable interface {
	Close() error
}

// HealthChecker 定义健康检查行为，用于观测与上报全局对象的可用状态。
//
// 实现方应保证 IsHealthy 是轻量、快速、只读的探测，并自带较短超时。
// 框架侧仅将其结果用于日志上报与外部探针（如 K8s readiness），
// 不会据此自动重建对象——连接池型对象的失效恢复由驱动层负责。
type HealthChecker interface {
	IsHealthy() bool
}

// Rebuilder 定义重建行为，用于配置热更新等需要替换底层客户端的场景。
//
// 注意：Rebuild 仅供显式手动调用（如配置热更新、运维指令），框架的周期性健康检查
// 不会自动触发它。原因是连接池型对象（MySQL/Redis/MongoDB）的驱动层已具备失效连接
// 自愈能力，而基于单次探测失败就整池重建，会丢弃正在服务请求的连接，
// 并在故障期形成"重建风暴"放大故障。
//
// 实现方必须在替换客户端后关闭旧客户端，否则每次重建都会泄漏一整池连接；
// 且被替换的字段若存在并发读取，必须以读写锁保护。
type Rebuilder interface {
	Rebuild(...interface{}) (interface{}, error)
	GetConfPath() string // 获取配置路径
}
