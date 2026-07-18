# GlobalManager

`GlobalManager` 是 FiberHouse 的字符串 key 全局对象容器，适合启动期注册 initializer、运行期延迟读取的读多写少场景。它提供单例创建、健康检查、重建、释放和清理操作，但不等同于依赖注入编译器，也不自动证明资源已经关闭。

## 容器与注册入口

`globalmanager.NewGlobalManagerOnce()` 返回进程级容器；`New`、`AppConfig`、Web/CLI Context 和根 package 泛型 helper 都使用它。`globalmanager.NewGlobalManager()` 可以创建独立容器，但默认 Context 不会自动改用该容器。

注册一个 initializer：

```go
ok := gm.Register("database", func() (interface{}, error) {
	return newDatabase()
})
```

`Register` 只保存函数，不立即构造对象。initializer 为 nil、key 已存在或并发注册中输给另一调用者时返回 false；已有 initializer 不会被覆盖。`Registers(InitializerMap)` 批量调用 `Register`，但丢弃每一项的 bool 结果，因此业务必须在启动验证中主动读取必需 key，不能靠批量注册发现重复。

根 package 的 `RegisterKeyName(name, ns...)` 只用 `.` 拼接命名空间和名称，并不执行注释中描述的标识符校验。`RegisterKeyInitializerFunc` 对空 key 静默返回，对重复注册同样忽略 false。

标准 Web 装配发生在 `FrameApplication.RegisterApplicationGlobals`：先注册日志 Origin 子日志器，再批量注册 `ApplicationRegister.ConfigGlobalInitializers()`，随后对 `ConfigRequiredGlobalKeys()` 逐项 `Get`。必需对象初始化失败只记录 Error 日志，不会阻止后续启动阶段；应用若要求 fail-fast，需要在自己的可观察启动入口返回或终止。

## Get 与延迟单例

首次 `Get(key)` 通过 entry 内的 `sync.Once` 调用 initializer，并把实例包装后保存到 `atomic.Pointer[storedValue]`。并发首次读取正常情况下只执行一次 initializer；成功后同一 entry 的后续 `Get` 返回同一实例。key 不存在时返回：

```text
entry '<key>' not found for loading
```

initializer 返回 error 时，容器把状态记为失败并返回包装错误；initializer panic 会被 `Get` 捕获并转成包含 key 的 error。下一次 `Get` 看到失败状态后会替换 `sync.Once` 并再次执行 initializer；成功重试会清空旧 `initErr`，同一次调用即可返回新实例。

initializer 必须返回非 nil 实例。返回 `(nil, nil)` 会被转换为初始化错误并进入可重试失败状态，不会把 nil 实例标记为成功。`Rebuild` 同样拒绝 nil 返回值，但容器不替调用方保证新旧具体类型兼容。

## 泛型查找 helper

根 package 提供：

```go
getDatabase := fiberhouse.GetInstance[*Database]
mustGetDatabase := fiberhouse.GetMustInstance[*Database]
db, err := getDatabase(key)
mustDB := mustGetDatabase(key)
```

`GetInstance[T]` 在 key 缺失、初始化失败或类型断言失败时返回 `T` 的零值和 error。`GetMustInstance[T]` 对同样情况 panic。两者始终访问 `NewGlobalManagerOnce()`，不能传入测试容器。

Context/Locator 的非泛型 `GetInstance` 访问同一默认容器，但把类型断言留给调用方。需要显式依赖、独立测试或不同容器时，应通过构造器传入依赖，而不是在深层代码中调用全局 helper；相关区别见[《Context 与 Locator》](../concepts/context-and-locators.md)。

## Health 合约

对象可选择实现：

```go
type HealthChecker interface {
	IsHealthy() bool
}

type Rebuilder interface {
	Rebuild(...interface{}) (interface{}, error)
	GetConfPath() string
}
```

`CheckHealth(key)` 不会触发 initializer。尚未经过 `Get` 的 entry 没有实例，不实现 `HealthChecker`，因此被视为健康；未实现该接口的已初始化对象也默认健康。只有已初始化并实现 `HealthChecker` 的对象会调用 `IsHealthy()`。

`Rebuild(key)` 要求对象已经初始化且实现 `Rebuilder`。它调用当前实例的 `Rebuild(current.GetConfPath())`，然后把返回值直接替换到 entry。容器不会更新 initializer，不会先关闭旧实例，也不会等待旧引用停止使用；数据库 client 等资源必须由应用定义停流、迁移和关闭顺序。

## Web keepalive 扫描

`FrameApplication.RegisterGlobalsKeepalive` 在 `application.globalManage.keepAlive=true` 时启动 ticker goroutine。`application.globalManage.interval` 通过 `Duration(key, 180) * time.Second` 计算，当前约定是正的数值秒；不要传已经带 duration 单位的字符串并期待不再相乘。零或负 interval 会记录错误并跳过启动，不再进入 `time.NewTicker`。

每次 tick 会 `Range` 全容器：

1. 调用 `CheckHealth`；错误只记录后继续下一个 key。
2. 健康结果为 false 时记录错误并调用 `Rebuild`。
3. 重建错误记录为失败并继续下一个 key；只有成功重建才记录“rebuild success”。

传给 `RegisterGlobalsKeepalive` 的 Provider Manager 参数当前未使用。默认 `FrameApplication` 在内部保存 cancel 函数和 `WaitGroup`；内置 Fiber/Gin 关闭路径会先取消并等待正在执行的健康检查，再以 deletion-only 语义清空容器。该停止入口不是公共 API，自定义 `FrameStarter` 若自行启动 keepalive，仍须自行实现停止与等待。

## Rebuild、Release 与 Clear

这些 API 的语义不同：

| 操作 | key | initializer | 实例/资源 |
|---|---|---|---|
| `Rebuild(key)` | 保留 | 保留旧 initializer | 用 `Rebuilder` 返回值替换实例；不关闭旧实例 |
| `Release(key)` | 保留 | 对 `Closable` 成功关闭后保留并重置 `sync.Once` | `Closable` 成功关闭后清空实例，后续 `Get` 可重新初始化；非 `Closable` 不重置 |
| `ReleaseAll(true)` | 保留 | 保留 | 遍历调用 `Release`；单项错误打印到 stdout |
| `Clear(key)` / `Unregister(key)` | 删除 | 删除 | 不调用 `Close` |
| `ClearAll(true)` | 删除全部 | 删除全部 | 在原 `sync.Map` 上调用 `Clear`，不逐项 `Close` |

`Release` 对实现 `Closable` 的对象调用 `Close`；成功后通过原子指针清空实例和错误状态，并重置初始化标志与 `sync.Once`。`Close` 返回错误时保留原实例；对象未实现 `Closable` 时返回 nil，但不会重置实例。同一已注册 entry generation 内，`Release` 与 `Rebuild` 共享 fail-fast maintenance gate；冲突维护调用返回 busy error，不等待第二个回调。

`ReleaseAll` 和 `ClearAll` 只有显式传入 true 才执行。`sync.Map.Clear` 可与 map 操作并发，但 `ClearAll(true)` 不取消已经取得 entry 并开始执行的 initializer，也不协调调用方已持有的业务引用；它适合已经停流后的最终删除，不是资源关闭或运行期热重置。

当前 Fiber 与 Gin 受控关闭路径先停止并等待默认 keepalive，再调用 `ClearAll(true)`，不是 `ReleaseAll(true)`。因此 GlobalManager 清空不等于数据库、缓存、writer 或 task client 已关闭，资源创建者仍须安排各自的 `Close` / `Disconnect`。

## 启动与运行期边界

推荐顺序是：

- 启动期：完成所有 `Register` / `Registers`，检查重复结果，对必需 key 调用 `Get` 并验证具体类型。
- 运行期：以 `Get` 和已持有实例的只读访问为主，不动态替换 initializer。
- 重建期：先停止或隔离流量，由资源所有者创建新实例、切换引用并关闭旧实例；不要假设 `Rebuild` 已完成这些步骤。

默认 `FrameApplication` 的 keepalive 由框架内部持有取消与等待状态；内置 Fiber/Gin 会在清空容器和关闭日志前停止它，重复停止和并发停止均可返回。停止后不会重新启动同一 `FrameApplication` 的健康检查。该契约不扩展到自定义 `FrameStarter`，也不构成通用后台任务取消树。

框架当前也没有统一逐项关闭链，因此资源回收仍需应用补齐。GlobalManager 的原子字段与 `sync.Map` 保护局部读写，不会让业务对象自身变成线程安全，也不会让 `Rebuild`、`Release`、`ClearAll` 成为无缝并发切换。

## 已知限制

- 默认容器是进程级单例；Web、CLI、配置、日志 writer 和泛型 helper 可能共享同一 key 空间。
- 批量注册和根 package 注册 helper 丢弃重复注册结果。
- `Rebuild` 不关闭旧实例，也不与调用方已持有的引用协调；新旧具体类型兼容仍由调用方负责。
- `Release` 只重置成功关闭的 `Closable`；`Clear` / `ClearAll` 仍完全不关闭资源。
- 同一 entry generation 的维护门禁不定义删除后同名重注册、普通 `Get` 或业务引用的完整状态机。
- keepalive 不初始化懒对象；取消与等待只由默认 `FrameApplication` 和内置 Fiber/Gin 关闭路径消费。

因此 [功能状态](../reference/feature-status.md) 将 GlobalManager 归为实验性生命周期能力。源码入口见 [`globalmanager/manager.go`](../../globalmanager/manager.go)、[`globalmanager/interface.go`](../../globalmanager/interface.go)、[`global_utils.go`](../../global_utils.go) 与 [`frame_starter_impl.go`](../../frame_starter_impl.go)。
