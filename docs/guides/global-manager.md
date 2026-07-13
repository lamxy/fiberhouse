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

首次 `Get(key)` 通过 entry 内的 `sync.Once` 调用 initializer，并把实例保存到 `atomic.Value`。并发首次读取正常情况下只执行一次 initializer；成功后同一 entry 的后续 `Get` 返回同一实例。key 不存在时返回：

```text
entry '<key>' not found for loading
```

initializer 返回 error 时，容器把状态记为失败并返回包装错误；initializer panic 会被 `Get` 捕获并转成包含 key 的 error。下一次 `Get` 看到失败状态后会替换 `sync.Once` 并再次执行 initializer。

当前错误缓存有一项静态限制：重试成功后实例与成功状态会保存，但旧 `initErr` 没有清空，因此执行成功重试的这次 `Get` 仍可能返回旧错误；再下一次 `Get` 才从成功状态直接取得实例。这不应被应用当作稳定的“两次重试协议”。

initializer 必须返回非 nil 实例。`atomic.Value` 不能保存 nil；返回 `(nil, nil)` 会在保存时 panic，并被本次初始化恢复为失败。实例重建也必须保持可存储、类型兼容的具体值。

## 泛型查找 helper

根 package 提供：

```go
db, err := fiberhouse.GetInstance[*Database](key)
db := fiberhouse.GetMustInstance[*Database](key)
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

`FrameApplication.RegisterGlobalsKeepalive` 在 `application.globalManage.keepAlive=true` 时启动 ticker goroutine。`application.globalManage.interval` 通过 `Duration(key, 180) * time.Second` 计算，当前约定是正的数值秒；不要传已经带 duration 单位的字符串并期待不再相乘。零或负 interval 会在 `time.NewTicker` 处 panic，而且 ticker 在 goroutine 创建前构造，内部 recover 无法接住该 panic。

每次 tick 会 `Range` 全容器：

1. 调用 `CheckHealth`；错误只记录后继续下一个 key。
2. 健康结果为 false 时记录错误并调用 `Rebuild`。
3. 重建错误记录为失败；当前代码随后仍会再记录一条“rebuild success”日志，即使 `err` 非 nil。运维不能只按该 success 文本判断重建成功。

传给 `RegisterGlobalsKeepalive` 的 Provider Manager 参数当前未使用。keepalive 没有返回 cancel 函数，也没有连接 HTTP shutdown context；goroutine 只会在自身 panic 后由 defer 停止 ticker。Fiber/Gin 关闭路径可能已经 `ClearAll(true)` 并关闭日志器，而扫描 goroutine仍然存活。这是当前生命周期的源码静态限制，不能把 keepalive 当作完整的资源监管器。

## Rebuild、Release 与 Clear

这些 API 的语义不同：

| 操作 | key | initializer | 实例/资源 |
|---|---|---|---|
| `Rebuild(key)` | 保留 | 保留旧 initializer | 用 `Rebuilder` 返回值替换实例；不关闭旧实例 |
| `Release(key)` | 设计意图为保留 | 设计意图为保留并重置 `sync.Once` | 仅在对象实现 `Closable` 时调用 `Close` |
| `ReleaseAll(true)` | 保留 | 保留 | 遍历调用 `Release`；单项错误打印到 stdout |
| `Clear(key)` / `Unregister(key)` | 删除 | 删除 | 不调用 `Close` |
| `ClearAll(true)` | 删除全部 | 删除全部 | 直接替换内部 `sync.Map`，不逐项 `Close` |

`Release` 当前不能可靠完成它的设计意图：对象成功 `Close` 后，代码调用 `atomic.Value.Store(nil)` 清空实例，而 Go 的 `atomic.Value` 禁止存储 nil，因此该路径会 panic，后面的状态与 `sync.Once` 重置无法执行。对象未实现 `Closable` 时 `Release` 返回 nil，但也不会重置实例。应用不应在生产关闭链中依赖 `Release` / `ReleaseAll` 完成回收。

`ReleaseAll` 和 `ClearAll` 只有显式传入 true 才执行。`ClearAll(true)` 也没有与并发 `Get`、`Range`、`Register` 或业务对象使用者协调；它适合已经停流后的最终清理，不是运行期热重置。

当前 Fiber 与 Gin 受控关闭路径调用的是 `ClearAll(true)`，不是 `ReleaseAll(true)`。因此 GlobalManager 清空不等于数据库、缓存、writer 或 task client 已关闭，资源创建者仍须安排各自的 `Close` / `Disconnect`。

## 启动与运行期边界

推荐顺序是：

- 启动期：完成所有 `Register` / `Registers`，检查重复结果，对必需 key 调用 `Get` 并验证具体类型。
- 运行期：以 `Get` 和已持有实例的只读访问为主，不动态替换 initializer。
- 重建期：先停止或隔离流量，由资源所有者创建新实例、切换引用并关闭旧实例；不要假设 `Rebuild` 已完成这些步骤。

内建 keepalive 是进程生命周期 goroutine：`RegisterGlobalsKeepalive` 不返回 cancel，也没有把 ticker 绑定到 shutdown context；一旦启用，只能依赖进程退出终止，应用无法在受控关闭流程中取消它。这是依据当前源码得出的静态生命周期限制，不表示已经通过运行时故障复现。

如果需要可控关闭，应禁用 `application.globalManage.keepAlive`，由应用自建扫描器并持有其 `context.CancelFunc`、`time.Ticker` 和退出等待机制。此时停止顺序才是：先 cancel context、stop ticker 并等待自建扫描器退出，再停止请求与任务生产者，逐项释放资源，清理容器，最后关闭日志。不要在启用内建 keepalive 时照搬这套顺序，因为应用没有可调用的内建取消句柄。

框架当前也没有统一逐项关闭链，因此资源回收仍需应用补齐。GlobalManager 的原子字段与 `sync.Map` 保护局部读写，不会让业务对象自身变成线程安全，也不会让 `Rebuild`、`Release`、`ClearAll` 成为无缝并发切换。

## 已知限制

- 默认容器是进程级单例；Web、CLI、配置、日志 writer 和泛型 helper 可能共享同一 key 空间。
- 批量注册和根 package 注册 helper 丢弃重复注册结果。
- `Get` 的失败重试残留旧 `initErr`，成功重试的当次调用仍可能报旧错误。
- `Rebuild` 不关闭旧实例，也不与并发读者协调；返回 nil 或不兼容具体类型还会触发 `atomic.Value` 限制。
- `Release` 的 `Store(nil)` 是可由源码确定的 panic 风险；`Clear` / `ClearAll` 则完全不关闭资源。
- keepalive 不初始化懒对象、没有取消句柄，并可能在 logger 与容器清理之后继续运行。

因此 [功能状态](../reference/feature-status.md) 将 GlobalManager 归为实验性生命周期能力。源码入口见 [`globalmanager/manager.go`](../../globalmanager/manager.go)、[`globalmanager/interface.go`](../../globalmanager/interface.go)、[`global_utils.go`](../../global_utils.go) 与 [`frame_starter_impl.go`](../../frame_starter_impl.go)。
