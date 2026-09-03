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

// Rebuilder 定义重建行为，用于**配置类**全局对象的热更新。
//
// 适用对象：持有业务配置、规则表、模板、特性开关等内存态数据的全局对象。
// 这类对象的「重建」是纯内存替换，无在途请求，语义清晰且代价可控。
//
// 不适用对象：连接池型对象（MySQL / Redis / MongoDB 等）。框架内置的三个封装
// 均**不实现**本接口：
//   - 驱动层已具备失效连接自愈能力（database/sql 坏连接淘汰、go-redis 重试、
//     mongo-driver SDAM），配合 connMaxLifetime / connMaxIdleTime 即可覆盖
//     中间设备静默掐断的场景；
//   - 换库、换实例、主从切换属于基础设施变更，应由 DNS、VIP、Proxy 层处理，
//     应用侧无感；
//   - 重建会中断正在使用旧连接的在途请求，除非实现引用计数或排空机制。
//
// 不提供任何重建连接的方法：凭证轮换等运维场景应在进程外解决
// （Vault Agent / IAM token / sidecar 重写连接串 + 滚动重启），
// 由应用进程自行替换连接池会中断在途请求且缺乏排空机制。
//
// 注意：Rebuild 仅供显式手动调用，框架的周期性健康检查不会自动触发它。
type Rebuilder interface {
	Rebuild(...interface{}) (interface{}, error)
	GetConfPath() string // 获取配置路径
}
