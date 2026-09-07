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

// CompositeClosable 标记一个「持有并负责关闭其他全局对象」的组合对象。
//
// 容器允许同一个底层资源以别名形式注册在多个 key 下，也允许一个组合对象把其他
// 全局对象作为子资源持有。示例应用就是这样：Redis 客户端同时注册在 KEY_REDIS 与
// KEY_REMOTE_CACHE 下，而 Level2Cache 又把 local 与 remote 两个缓存作为子资源持有
// 并在自己的 Close 中关闭它们。
//
// 这带来一个关闭顺序约束。组合对象通常需要先排空在途异步操作、再关闭子资源
// （Level2Cache.finishShutdown 就是先 <-drain 再 Close 子缓存）。如果容器先关闭了
// 子资源，组合对象排空期间的操作就会打在已关闭的资源上——一次关闭路径上的
// use-after-close。
//
// 而遍历顺序本身是不确定的：ReleaseAll 基于 sync.Map.Range，其迭代顺序未定义且每次
// 运行都可能不同。因此「先关组合对象」不能依赖运气，必须显式表达。
//
// 实现本接口即向容器声明：本对象拥有子资源，必须**先于**普通对象关闭。
// ReleaseAll 会分两轮执行——先释放所有 CompositeClosable，再释放其余对象。
//
// 实现方仍须保证 Close 幂等：子资源被组合对象关闭后，容器随后仍会对它自己的 key
// 调用一次 Release，此时第二次 Close 必须安全返回。
type CompositeClosable interface {
	Closable
	// OwnsChildren 返回本对象是否确实持有子资源的关闭责任。
	//
	// 返回 false 时按普通 Closable 对待：某些组合对象可以被构造为「借用」子资源
	// 而不负责关闭它们，这种对象没有关闭顺序约束。
	OwnsChildren() bool
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
