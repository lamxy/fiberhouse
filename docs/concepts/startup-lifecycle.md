# Web 启动生命周期

本文记录 `(*FiberHouse).RunServer` 的当前执行顺序。它不是对所有 `DefaultPLocation` 名称的推测，也不表示每个传入 Starter 的 Manager 都会被内置实现加载。Provider 系统整体为[已接入](../reference/feature-status.md)，但扩展运行位点与关闭链仍是实验性能力。

## `RunServer` 之前已经发生什么

调用 `New(cfg)` 时，框架已经完成这些进程级动作：

1. 获取 `GlobalManager` 单例。
2. `bootstrap.NewConfigOnce(cfg.ConfigPath)` 加载配置。
3. `bootstrap.NewLoggerOnce(appCfg, cfg.LogPath)` 建立日志器。
4. `NewAppContextOnce(appCfg, logger)` 建立 `AppContext`。
5. 把上下文 initializer 注册到全局容器，注册 `BootConfig`，并写入 `FiberHouse.AppCtx`。

因此 `RunServer` 开头的 `LocationBootStrapConfig` 是一个“bootstrap 之后的扩展位置”，不是配置和日志的实际创建点。配置、日志、Web Context 和容器均带进程级单例语义；同一进程内再次调用 `New` 不应被理解为建立完全隔离的应用。

`Default()` 只用 Fiber、Sonic、`./config`、`./logs` 等默认值构造 `BootConfig`。它不会自动调用 `DefaultProviders()` 或 `DefaultPManagers(ctx)`。

## `RunServer` 的精确顺序

下表按 [`boot.go`](../../boot.go) 当前代码排序。“读取 Manager”表示 `RunServer` 调用了该 Location 的 `GetManagers()`；是否执行 Provider 还要看本列说明和具体 Starter。

| 顺序 | 动作 | 当前行为与错误出口 |
|---:|---|---|
| 1 | bootstrap location | 读取 `LocationBootStrapConfig`；只检查 Manager 列表的第一项，若它 `IsUnique()` 才调用并把 `*FiberHouse` 传给 `ProviderLoadFunc`。返回值和错误都被丢弃 |
| 2 | 确定 fallback Manager | 若 `RunServer(manager...)` 没有参数，创建 `NewDefaultPManager(appContext)`；否则只取第一个参数作为 fallback。无论哪种情况都会追加到 `fh.managers` |
| 3 | 分发 Provider | 对每个 `fh.providers`，按 `Type().GetTypeID()` 找第一个匹配 Manager 并 `Register`；注册错误只记 Error 日志。未找到匹配类型的 Provider 收集为剩余项 |
| 4 | fallback 注册 | 把剩余 Provider 注册到 fallback Manager；失败只记 Error 日志 |
| 5 | 加载 zero-location Manager | 遍历 `fh.managers`，凡 `Location()` 为 `ZeroLocation` 都调用 `LoadProvider()`；返回值和错误被丢弃 |
| 6 | 再次加载 fallback | fallback 非空时再调用一次 `LoadProvider()`；返回值和错误同样被丢弃。因为 fallback 通常也是 zero location，这可能造成重复初始化调用 |
| 7 | 取得 Frame Options | 优先使用 `WithFrameStarterOptions` 收集的值；为空时警告并读取 `LocationFrameStarterOptionInit` 的第一个 Manager。加载失败或返回值不是 `[]FrameStarterOption` 时 fatal |
| 8 | 创建 `FrameStarter` | 读取 `LocationFrameStarterCreate` 的第一个 Manager，把 Frame Options 传入 `LoadProvider`。没有 Manager、加载错误或返回值不是 `FrameStarter` 时 fatal |
| 9 | 取得 Core Options | 优先使用 `WithCoreStarterOptions`；为空时警告并读取 `LocationCoreStarterOptionInit` 的第一个 Manager。加载/类型错误时 fatal；位置为空则继续使用空切片 |
| 10 | 创建 `CoreStarter` | 读取 `LocationCoreStarterCreate` 的第一个 Manager，把 Core Options 传入。没有 Manager、加载错误或类型不符时 fatal |
| 11 | 组合并回写 Context | 用两个 Starter 创建 `WebApplication`，调用 `RegisterToCtx(appStarter)`；此后 `IContext.GetStarter()` 可回到完整 `ApplicationStarter` |
| 12 | globals | 调用 `RegisterApplicationGlobals(LocationGlobalInit.GetManagers()...)` |
| 13 | core engine | 调用 `InitCoreApp(frame, LocationCoreEngineInit.GetManagers()...)` |
| 14 | hooks | 调用 `RegisterAppHooks(frame, LocationCoreHookInit.GetManagers()...)` |
| 15 | application middleware | 调用 `RegisterAppMiddleware(frame, LocationAppMiddlewareInit.GetManagers()...)` |
| 16 | module middleware + routes | 合并 `LocationModuleMiddlewareInit` 与 `LocationRouteRegisterInit` 的 Manager，再调用 `RegisterModuleInitialize` |
| 17 | Swagger | 调用 `RegisterModuleSwagger(frame, LocationModuleSwaggerInit.GetManagers()...)` |
| 18 | task server | 调用 `RegisterTaskServer(LocationTaskServerInit.GetManagers()...)` |
| 19 | global keepalive | 调用 `RegisterGlobalsKeepalive(LocationGlobalKeepaliveInit.GetManagers()...)` |
| 20 | before-run | 读取 `LocationServerRunBefore`；只执行第一个 unique Manager，把 `ApplicationStarter` 传给它，忽略返回值和错误 |
| 21 | run + shutdown | 合并 `LocationServerRun` 与 `LocationServerShutdown` 的 Manager，传给 `AppCoreRun`；内置 Fiber/Gin 实现当前不读取这些 Manager |
| 22 | after-run | `AppCoreRun` 返回后读取 `LocationServerRunAfter`；只执行第一个 unique Manager，把 `ApplicationStarter` 传给它，忽略返回值和错误 |

`RunServer` 没有返回 `error`。若日志器的 fatal 语义终止进程，调用者也无法在上层统一恢复这些失败；另一些阶段则只记录、panic 或直接忽略错误。应用应把必要依赖校验放在可观察的启动阶段，不要依赖运行期补装配。

## Starter 内部的实际工作

### 全局对象与验证器

`FrameApplication.RegisterApplicationGlobals` 当前不使用传入的 `LocationGlobalInit` Manager。它按固定顺序：

1. 为配置中的各个 `LogOrigin` 注册子日志器 initializer；
2. 调用 `ApplicationRegister.ConfigGlobalInitializers()` 批量注册应用 initializer；
3. 对 `ConfigRequiredGlobalKeys()` 逐项 `Get`；失败只记录 Error 日志，不中止；
4. 注册自定义语言验证器；
5. 注册自定义 validator tag，错误汇总后记录但不 panic；
6. 若存在 `TaskRegister`，注册 task worker 与 dispatcher initializer。

`ApplicationRegister` 缺失会在全局 initializer 阶段 panic。注册表与验证器应在这里写完，进入请求并发阶段后只读。

### Core、hook 和中间件

`InitCoreApp` 是内置 Starter 真正检查传入 Manager 的主要阶段。Fiber/Gin 都从 `LocationCoreEngineInit` 的 Manager 中寻找 `GroupTrafficCodecChoose`；没有匹配 Manager、加载失败或返回 codec 类型错误会 panic。若没有传入任何 Manager，则从应用定义的默认 codec key 通过 `GlobalManager` 获取实例。

之后的 Manager 参数并未被内置 Core 直接消费：

- `RegisterAppHooks` 调用 `ApplicationRegister.RegisterCoreHook(core)`；应用注册器可以自行读取 `LocationCoreHookInit` 并加载 Manager。
- `RegisterAppMiddleware` 先安装框架 recovery、错误处理/请求日志，再调用 `ApplicationRegister.RegisterAppMiddleware(core)`；应用注册器可以自行读取 `LocationAppMiddlewareInit`。
- `RegisterModuleInitialize` 调用 `ModuleRegister.RegisterModuleRouteHandlers(core)`；模块注册器可以自行读取路由 Location。当前接口没有单独的模块中间件回调。
- `RegisterModuleSwagger` 只在 `application.swagger.enable` 为 true 且存在模块注册器时调用 `RegisterSwagger`。
- `RegisterTaskServer` 与 `RegisterGlobalsKeepalive` 由 Frame Starter 直接读取配置和注册器，不使用传入 Manager。

这意味着“`RunServer` 把某个 Location 的 Manager 传给方法”不等于“该 Manager 已自动执行”。应用若选择 Provider 化的 hook、中间件或路由，必须在对应注册器实现中显式调用 `LoadProvider`，并处理错误。

## 任务、keepalive 与并发开始点

当 `application.task.enableServer` 为 true 且存在 `TaskRegister` 时，Frame Starter 从全局容器取得 task worker、注册 handler map，然后调用 `RunServer()` 启动后台 worker。取得 worker 失败会 panic；worker 异步运行错误当前主要记录在任务实现内部。

当 `application.globalManage.keepAlive` 为 true 时，Frame Starter 按 `application.globalManage.interval`（缺省 180 秒）启动 ticker goroutine，遍历全局容器并对实现健康检查/重建接口的实例执行检查。当前没有由 `RunServer` 管理的取消句柄，日志也可能在关闭期间先于 goroutine 停止。

因此并发并不只从 HTTP 监听开始：task server 与 keepalive goroutine 在 `AppCoreRun` 之前就可能启动。Provider、Manager、Location、默认集合、validator 注册表以及应用 initializer 列表都应在此之前冻结。

## 监听、信号与关闭

Fiber 与 Gin 都在 goroutine 中启动 HTTP server，在调用 `AppCoreRun` 的 goroutine 中等待 `SIGINT` / `SIGTERM`：

| Core | 运行 | 收到信号后 | 当前限制 |
|---|---|---|---|
| Fiber | `fiber.App.Listen(host + ":" + port)` | 调用 `fiber.App.Shutdown()`；`OnShutdown` hook 清空 `GlobalManager` 并关闭日志器 | `RegisterAppState(true)` 在 `Listen` 返回后才执行，并非“开始监听成功”标志；清空容器不逐项关闭资源 |
| Gin | `http.Server.ListenAndServe()` | 最多等待 30 秒 `Shutdown`，随后清空容器并关闭日志器 | TLS 分支未完成且仍走 `ListenAndServe`；应用状态同样在 server 返回后才写入 |

`LocationServerRun` 与 `LocationServerShutdown` 的 Manager 虽被传给 `AppCoreRun`，当前两种 Core 都不读取它们。`LocationServerRunAfter` 只有在 `AppCoreRun` 返回后才执行；如果 fatal/panic/进程强退阻断返回，after-run 也没有保证。

## 已声明 Location 与当前消费范围

`ProviderLocationDefault()` 声明了一组单例 Location，但声明只提供名称和 Manager 容器。当前 `RunServer` 的读取范围如下：

| 分类 | Location |
|---|---|
| zero-location 判定 | `ZeroLocation`（通过比较 Manager 的 Location ID，不读取其 Manager 列表） |
| `RunServer` 直接读取 | `LocationBootStrapConfig`、`LocationFrameStarterOptionInit`、`LocationCoreStarterOptionInit`、`LocationFrameStarterCreate`、`LocationCoreStarterCreate`、`LocationGlobalInit`、`LocationCoreEngineInit`、`LocationCoreHookInit`、`LocationAppMiddlewareInit`、`LocationModuleMiddlewareInit`、`LocationRouteRegisterInit`、`LocationModuleSwaggerInit`、`LocationTaskServerInit`、`LocationGlobalKeepaliveInit`、`LocationServerRunBefore`、`LocationServerRun`、`LocationServerShutdown`、`LocationServerRunAfter` |
| 已声明但 `RunServer` 不读取 | `LocationAdaptCoreCtxChoose`、`LocationServerShutdownBefore`、`LocationServerShutdownAfter`、`LocationResponseInfoInit` |

`LocationResponseInfoInit` 由响应 facade 在响应路径单独读取，不属于启动顺序。`LocationAdaptCoreCtxChoose` 当前没有启动链消费者；适配 Fiber/Gin context 仍需显式选择。`LocationServerShutdownBefore` 与 `LocationServerShutdownAfter` 只有声明，不能当作可用关闭 hook。

自定义 Location 也不会因注册而自动进入 `RunServer`。它需要一个明确调用 `GetManagers()` / `LoadProvider()` 的消费者；否则只是一条元数据记录。

## 错误与生命周期边界

- Provider 注册重复或 fallback 注册失败：记录日志后继续；原 Provider 可能未进入任何可执行 Manager。
- zero-location、fallback、bootstrap、before-run、after-run 加载错误：当前入口忽略；不能从 `RunServer` 返回给调用者。
- Frame/Core 创建、Options 类型不符：使用 fatal 日志；应用注册器缺失、codec manager 缺失、task worker 获取失败等路径会 panic。
- `ProviderManager.List()` 来自 map，Provider 执行顺序不稳定。Location 使用切片保存 Manager，但当前重复绑定检查会拒绝同一 Location 的后续 Manager，而 `SetOrBindToLocation` 又忽略这个错误；不能依赖“同一位置多个 Manager 按绑定顺序执行”。
- `RunApplicationStarter` 是另一个导出 helper：它把同一组 Manager 传给各阶段，但不完成 `RunServer` 的 Provider 分发、Starter 选择和 Location 编排，不能与完整入口等同。
- Fiber/Gin 关闭链只清空全局容器；数据库、缓存、任务、writer 与其他 goroutine 的资源所有者仍需定义停止顺序、超时和错误出口。

相关机制见[《Provider 系统》](provider-system.md)，上下文的单例与并发边界见[《Context 与 Locator》](context-and-locators.md)。
