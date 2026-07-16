# Context 与 Locator

Context 是 FiberHouse 各层共享基础设施的访问面；Locator 是 API、Service、Repository 按名称访问 Context 与全局对象的轻量基类。它们提供运行期定位，不等同于请求上下文，也不等同于 Wire 的编译期依赖注入。

## 三个 Context 接口

| 接口 | 共有或新增能力 | 当前实现 |
|---|---|---|
| `IContext` | 配置、日志、`GlobalManager`、Starter 回指、带 Origin 的子日志器、validator wrapper | Web 与 CLI 的最小交集 |
| `IApplicationContext` | `IContext` + Web `ApplicationStarter` 回指、应用状态、`BootConfig` | `AppContext` |
| `ICommandContext` | `IContext` + `CommandStarter` 回指、Dig 容器 | `CmdContext` |

`IContext.GetStarter()` 只返回共同的 `IStarter`，其公开能力仅有 `GetApplication()`。Web 中实际对象是 `ApplicationStarter`，CLI 中实际对象是 `CommandStarter`；需要形态专属方法时，应先持有正确的具体 Context 接口，再调用 `GetStarterApp()`，不要盲目跨形态断言。

这里的 Context 是进程/应用级对象，不是 Fiber `*fiber.Ctx` 或 Gin `*gin.Context`。请求参数、响应 writer、取消信号等仍属于 HTTP 引擎的请求 context。

## `AppContext` 的创建与内容

`New(cfg)` 先通过 bootstrap 建立配置和日志，再调用 `NewAppContextOnce(cfg, logger)`。`AppContext` 保存：

- `appconfig.IAppConfig`：`GetConfig()`；
- `bootstrap.LoggerWrapper`：`GetLogger()`；
- 进程级 `*globalmanager.GlobalManager`：`GetContainer()`；
- `*validate.Wrap`：通过 `GetValidateWrap()` 以接口返回；
- `BootConfig`：只允许 `RegisterBootConfig` 首次写入；
- `ApplicationStarter` 回指：由 `RunServer` 创建 `WebApplication` 后注册；
- 应用状态：只允许首次写入；
- 一个带 `RWMutex` 的 `DefaultStorage` 实现。

`IApplicationContext` 本身没有嵌入 `IStorage`，所以通过接口持有的普通调用方看不到 `Set` / `Get` 等存储方法。具体 `AppContext` 虽嵌入 `IStorage`，也不应把它当成跨进程缓存或业务状态仓库。

配置、日志、`AppContext` 与默认 `GlobalManager` 都带进程级单例语义。`NewAppContext(cfg, logger)` 可以构造一个新的 Web Context，但其容器仍来自 `NewGlobalManagerOnce()`；它不是完整的隔离应用沙箱。

## CLI Context 的差异

`NewCmdContextOnce` 创建 `CmdContext`，共享同一个 `GlobalManager` 单例，并额外持有 `*container.DigContainer`（`container` 对应包路径 `github.com/lamxy/fiberhouse/component/container`）。CLI 的 Starter 由命令装配代码注册，不经过 Web `RunServer`。

当前 `CmdContext.GetValidateWrap()` 返回 nil；虽然该方法属于 `IContext`，CLI 不能据此假设验证器可用。CLI keepalive 也只同步遍历一次全局对象，并非 Web 形态的 ticker 生命周期。CLI 整体在[功能状态](../reference/feature-status.md)中属于实验性。

## Starter 回指

Web 的回写顺序是：

```text
New / Default → AppContext（Starter 尚未注册）
RunServer → 创建 FrameStarter 与 CoreStarter
          → 组合 WebApplication
          → RegisterToCtx(WebApplication)
          → 后续注册器可通过 GetStarter() / GetStarterApp() 回到启动器
```

在 `RegisterToCtx` 之前，`GetStarter()` / `GetStarterApp()` 可能为 nil。任务 worker 获取实例 key 等代码依赖这个回指，因此不能把相关逻辑提前到 Context 创建阶段。

`AppContext.RegisterAppState` 使用 `sync.Once`，但当前 Fiber/Gin 都在 server 的阻塞监听调用返回后才写入 true；它不是可靠的“已经开始接受请求”探针。该布尔值的普通读写也没有独立锁或原子保护，不应作为并发健康状态通道。

## `GlobalManager` 访问

`GlobalManager` 保存 `KeyName → initializer / instance`。`Register` 只注册 initializer；第一次 `Get` 使用 `sync.Once` 延迟创建实例。initializer 返回错误或发生 panic 后，后续 `Get` 会重置 `once` 与初始化状态，并重新执行 initializer。

源码静态限制是：成功重试会保存新实例并把状态改为成功，但旧 `initErr` 没有被完整清除；执行这次重试的 `Get` 随后仍可能返回旧错误。再下一次 `Get` 会先命中成功状态并取得已经保存的实例。调用方不能把“第一次重试返回旧错误”直接解释为 initializer 没有成功，也不应把多调用一次当作稳定重试协议。

典型启动顺序是：

1. `ApplicationRegister.ConfigGlobalInitializers()` 返回 initializer map；
2. `FrameStarter` 在 `RegisterApplicationGlobals` 阶段调用 `Registers`；
3. `ConfigRequiredGlobalKeys()` 中的对象在启动期被主动 `Get`；
4. 其他对象由请求或业务组件首次 `Get` 时延迟初始化；
5. HTTP 关闭路径当前清空容器，但不会逐项调用 `Close`。

缺少 key 时，`Get` 返回 `entry '<key>' not found for loading`；类型断言仍由调用方负责。`Register` 对 nil initializer 或重复 key 返回 false，批量 `Registers` 和 `RegisterKeyInitializerFunc` 当前不会把这个结果向上传播。必要对象应在启动期主动读取并检查，而不是把拼写或重复注册错误拖到请求期。

`GlobalManager` 设计为读多写少。initializer 的单例创建和实例读取使用原子/锁保护，但 `Rebuild`、`Release`、`ClearAll` 与业务对象自身生命周期并不自动形成无缝并发切换。具体资源仍需定义所有者、停止流量、关闭、替换的顺序。

## Locator 与三种基类

`Locator` 只有四项能力：

```go
type Locator interface {
	GetContext() IContext
	GetName() string
	SetName(string) Locator
	GetInstance(string) (interface{}, error)
}
```

`ApiLocator`、`ServiceLocator`、`RepositoryLocator` 当前都是 `Locator` 的类型别名，不是三个不同的运行时协议。根包提供三个结构体基类：

| 基类 | 构造器参数 | 保存内容 | `GetInstance` 行为 |
|---|---|---|---|
| `Api` | `NewApi(IApplicationContext)` | `IContext` + 私有 name | 调用 Context 的 `GlobalManager.Get` |
| `Service` | `NewService(IContext)` | `IContext` + 私有 name | 同上；也可用于 CLI 公共 Context |
| `Repository` | `NewRepository(IApplicationContext)` | `IContext` + 私有 name | 同上 |

应用通常把 Locator 接口嵌入具体类型，用 `SetName` 保存容器 key 或命名空间标记。基类不会自动注册具体 API、Service 或 Repository，也不会校验层间依赖；`GetInstance` 返回 `interface{}`，类型错误在调用点出现。

`RegisterKeyName(name, ns...)` 只用 `.` 拼接命名空间和名称。注释描述的标识符规则当前没有由该函数执行校验。`RegisterKeyInitializerFunc(key, initializer)` 使用进程级 `GlobalManager` 注册 initializer；key 为空时直接返回，重复注册的 false 结果也被忽略。

## 泛型全局查找 helper

根包提供两种类型化读取：

```go
getUserService := fiberhouse.GetInstance[*UserService]
mustGetUserService := fiberhouse.GetMustInstance[*UserService]
service, err := getUserService(key)
mustService := mustGetUserService(key)
```

`GetInstance[T]` 在 key 缺失、initializer 失败或类型断言失败时返回错误；`GetMustInstance[T]` 对这些情况 panic。两者都直接取得 `NewGlobalManagerOnce()`，不经过调用方传入的 Context。若代码需要测试隔离、不同容器或明确依赖，优先保留 Context/构造器参数，而不是在深层函数中调用全局 helper。

Locator 的非泛型 `GetInstance` 与全局泛型 helper 访问的是同一个默认容器，但错误形态不同：前者把类型断言留给调用方，后者在 helper 内检查 `T`。

## 与 Wire 编译期注入的区别

| 维度 | Context / Locator / `GlobalManager` | Wire |
|---|---|---|
| 解析时机 | 运行期，按字符串 key 查找，可能延迟初始化 | 生成代码时解析构造函数图 |
| 依赖表达 | Context、key、initializer 和类型断言 | 构造函数参数与 `wire.ProviderSet` |
| 失败时机 | 注册、首次 `Get` 或类型断言时 | 缺少/冲突 provider 通常在生成期；构造函数 error 仍在运行期返回 |
| 生命周期 | 容器/资源所有者负责单例、重建、关闭 | 生成普通 Go 构造调用，本身不提供运行容器或资源管理 |
| 可见性 | 深层代码可通过全局 helper 隐式取依赖 | 生成的 injector 把构造链写成显式 Go 调用 |

仓库的示例 Wire 生成函数会依次调用 model、repository、service、handler 构造器；这条路径不需要在请求时按 key 查这些层。示例的其他路径又把 locator 对象注册到 `GlobalManager`，说明两种方式可以并存。不要把 Locator 称为 Wire 的运行时实现，也不要把 Wire 当成 GlobalManager 的自动关闭器。

## 启动写、运行读

推荐把边界固定为：

- 启动期：注册 `BootConfig`、Starter、日志 Origin、GlobalManager initializer、validator 语言/tag、Locator 对象和 Wire 构造出的根对象；
- 运行期：读取配置/日志/容器实例，调用已构造的 API、Service、Repository；
- 停止期：先停止请求、任务和其他生产者，再由资源所有者关闭数据库、缓存、writer 等，最后清理容器与日志。

`DefaultStorage` 的单项操作有锁，但这不使整个 Context 可热重配。`AppContext` 的 Starter 字段、应用状态以及 Provider/Manager 基类的启动字段没有为任意时刻并发写入设计；validator 内部注册表也要求启动写、运行读。

## 当前限制

- Web 与 CLI Context、配置、日志和默认容器都是进程级单例；同进程多应用和测试必须显式规划隔离。
- `IContext` 暴露 validator，但 CLI 实现返回 nil；公共接口只代表最小签名，不保证两种形态语义完全对称。
- Starter 回指在生命周期早期为 nil；应用状态写入时机晚且不适合作为 readiness。
- Locator 名称和容器 key 是字符串，没有编译期依赖完整性检查；`GetMust*` 会把配置/装配错误变成 panic。
- `RegisterKeyInitializerFunc` 与批量注册会丢弃重复注册结果；启动期应主动校验必需 key 和类型。
- HTTP Core 当前通过 `ClearAll(true)` 清空容器，不逐项关闭资源；Context/Locator 只提供可达性，不提供所有权证明。
- Wire 内容位于示例装配，不构成框架强制范式或稳定 API 承诺。

源码入口为 [`context_interface.go`](../../context_interface.go)、[`context_impl.go`](../../context_impl.go)、[`locator_interface.go`](../../locator_interface.go)、[`api_impl.go`](../../api_impl.go)、[`service_impl.go`](../../service_impl.go)、[`repository_impl.go`](../../repository_impl.go)、[`global_utils.go`](../../global_utils.go) 与 [`globalmanager/manager.go`](../../globalmanager/manager.go)。
