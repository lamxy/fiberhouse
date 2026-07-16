# Web 运行时

FiberHouse 用 `BootConfig.CoreType` 在同一套启动位点中选择 Fiber 或 Gin。两条路径共享应用 Context、Provider/Manager、业务注册器和响应/异常抽象，但不会把两个 HTTP 引擎包装成完全相同的 API：路由与 handler 仍直接使用 `*fiber.Ctx` 或 `*gin.Context`。Provider 装配与完整位点顺序见[《Provider 系统》](../concepts/provider-system.md)和[《Web 启动生命周期》](../concepts/startup-lifecycle.md)。

## 选择与装配条件

`Default()` 选择 `CoreType="fiber"`、`TrafficCodec="sonic_json_codec"`，且关闭二进制响应。使用 `New(&fiberhouse.BootConfig{...})` 时不会补齐这些字段，调用方应显式给出 `CoreType`、`TrafficCodec`、配置目录和日志目录；配置来源与单例边界见[《配置与引导》](configuration.md)。

核心启动器和 JSON codec Manager 都按启动配置筛选 Provider：前者选择目标 `CoreType`，后者同时要求 Provider 的 `Version()` 等于 `TrafficCodec`、`Target()` 等于 `CoreType`。默认 Provider 集合为 Fiber/Gin 各注册 std 与 Sonic 实现。传入了 Manager 列表却缺少 `GroupTrafficCodecChoose` Manager，或者没有匹配的 codec Provider，都会在 `InitCoreApp` 阶段失败，而不是静默改用其他 codec。

标准 Web 装配的主线是：

```text
CoreType 选择 CoreStarter
  → InitCoreApp：创建引擎、安装 JSON codec/错误入口
  → RegisterAppHooks
  → RegisterAppMiddleware：recover、错误/访问日志、应用中间件
  → RegisterModuleInitialize：模块注册路由
  → RegisterModuleSwagger（按开关）
  → AppCoreRun：listen、等待信号、shutdown
```

应用注册的中间件先后次序就是请求进入时的外到内次序；路由在应用中间件之后注册。`RegisterModuleInitialize` 本身只把当前 `CoreStarter` 交给 `ModuleRegister.RegisterModuleRouteHandlers`，具体路由组、模块中间件和 handler 顺序仍由应用实现决定。

## Fiber 与 Gin 对照

| 关注点 | Fiber | Gin |
|---|---|---|
| 引擎对象 | `*fiber.App` | `*gin.Engine`，外加 `*http.Server` |
| `InitCoreApp` | 将选中的 `JsonWrapper.Marshal/Unmarshal` 固化为该 app 的 `JSONEncoder/JSONDecoder`，并在 `fiber.Config` 安装全局 `ErrorHandler` | `gin.New(...)` 后把选中的 codec 写入 `gin/codec/json.API`，再构造 `http.Server` |
| 内建中间件顺序 | recover → `fiberzerolog` → `ApplicationRegister.RegisterAppMiddleware` | recover → 尾部错误处理 → 请求日志 → `ApplicationRegister.RegisterAppMiddleware` |
| 普通错误入口 | Fiber handler 返回 `error`，由 `fiber.Config.ErrorHandler` 处理 | handler 调用 `c.Error(err)`，或在没有 `c.Errors` 时用 `c.Set("error", err)`；尾部中间件在 `c.Next()` 后处理 |
| panic 入口 | Fiber recovery Provider | Gin recovery Provider |
| 路由注册 | `ModuleRegister.RegisterModuleRouteHandlers` 接收 Fiber starter | 同一接口接收 Gin starter |
| 监听 | `fiber.App.Listen(host+":"+port)` | `http.Server.ListenAndServe()` |
| 停止 | 等待 `SIGINT`/`SIGTERM` 后调用 `Shutdown()`；`OnShutdown` 清空容器并关闭日志器 | 等待相同信号，以 30 秒 context 调用 `http.Server.Shutdown`，随后清空容器并关闭日志器 |

Fiber 的全局错误处理器不在 `Use` 链中；表中的 recover 和访问日志是中间件顺序，错误处理器由 `fiber.Config` 单独调用。Gin 的错误处理中间件必须包住后续 handler，因此注册在请求日志和应用中间件之前。

## 配置键与源码 fallback

Fiber 主要读取 `application.server`：

| 键 | 当前消费方式 |
|---|---|
| `host`、`port` | `AppCoreRun` 直接读取并拼接，没有非空 fallback |
| `caseSensitive`、`strictRouting`、`disableKeepalive`、`enablePrintRoutes`、`streamRequestBody` | bool 缺失时为 `false` |
| `appServerHeader`、`appConcurrency` | 直接读取，缺失时分别为空字符串、`0` |
| `readBufferSize`、`writeBufferSize`、`bodyLimit` | fallback 均为 `4096`；`bodyLimit` 值直接交给 Fiber，源码注释虽称 KB，但这里没有额外单位换算 |
| `idleTimeout`、`readTimeout`、`writeTimeout` | fallback 分别为 `60`、`30`、`30`，随后乘 `time.Second` |
| `requestMethods` | fallback 为空切片，交由 Fiber 解释 |

Gin 的运行模式键是 `application.plugins.server.gin.mode`，fallback 为 `release`；`application.recover.debugMode=true` 会强制调用 `gin.SetMode(gin.DebugMode)`。服务参数位于另一分组 `application.plugins.engine.servers.gin`：`host=0.0.0.0`、`port=8080`、`readTimeout=30` 秒、`writeTimeout=30` 秒、`idleTimeout=120` 秒、`readHeaderTimeout=10` 秒；`maxHeaderBytes` 的配置值先按 KiB 读取，fallback `1024`，再乘 `1024`。这些数值是具体消费方的源码 fallback，不是示例 YAML 的默认声明。

两个引擎的 duration getter 结果都会再次乘 `time.Second`，当前约定应填写数值秒；不要传带 `ms`/`s` 单位的 duration 字符串并期待原样使用。

## JSON codec 是启动期引擎配置

内建可用组合是 `std_json_codec` 与 `sonic_json_codec` × Fiber/Gin。`go_json_codec` 只有常量，当前 [`component/codec/json/gojson.go`](../../component/codec/json/gojson.go) 没有实现，也没有默认 Provider，不能作为可用选项宣传。Sonic 的 `Unmarshal` 失败后会再尝试标准库 `encoding/json`；`Marshal` 不做同类 fallback。

两条安装路径的作用域不同：

- Fiber 把函数保存在单个 `fiber.App` 的配置中，作用域是该引擎实例。
- Gin 修改 `gin/codec/json.API` 包级变量。这是进程全局副作用：同进程创建多套 Gin 应用并选择不同 codec 不具备隔离保证，初始化先后也会影响 Gin 的其他使用者。

这里的引擎 JSON codec 负责请求绑定和 JSON 写出。starter 还把同一个 marshal 函数放进 `RecoverConfig.JsonCodec`，但 recovery 只在 debug 模式处理“既不是 `error` 也不是已知异常”的 panic 值时用它尝试 JSON 转换。`DefaultStackTraceHandler` 编码 params/query/headers 与异常 data 时读取的则是应用注册器 `GetFastTrafficCodecKey()` 指向的容器实例，这是第三个、由应用装配的 codec 入口；它不等同于 `BootConfig.TrafficCodec` 的选择结果。`DebugStackLines` 是例外：`GetJsonIndent` 忽略传入的 codec 参数，固定调用标准库 `encoding/json.MarshalIndent`。示例应用可以让前述 codec 都指向 Sonic，但框架没有强制它们相同。

以上三个 codec 入口都不决定统一响应是否改用 MsgPack/Protobuf；后者是 `ResponseWrap.SendWithCtx` 的独立协商，见[《响应与序列化》](response-and-serialization.md)。

传入自定义 Fiber `CoreCfg` 时，当前 `InitCoreApp` 会在 `fiber.New(*CoreCfg)` 后提前返回，不再设置 `cf.json`，也不会执行标准分支中 `ErrorHandler: adaptorerrorhandler.FiberErrorHandler(eh.ErrorHandler)` 的装配。调用方只有在自己的 `fiber.Config` 中显式设置等价 `ErrorHandler` 才能保留统一普通错误入口；即便如此，后续 `RegisterAppMiddleware` 构造 recovery 配置时仍引用 `cf.json.Marshal`，存在 nil 解引用的静态风险。这条自定义路径不能视为已完整支持的标准装配。

## `ICoreContext`：最小跨引擎边界

[`adaptor/context`](../../adaptor/context/) 的 `ICoreContext` 只统一五件事：取得原生 Context、读请求头、写响应头、按 HTTP status 发送 JSON、按 HTTP status 发送原始字节。路由参数、query/body 绑定、重定向、文件发送、`c.Error` 等仍使用引擎原生 API；需要时用 `GetCtx()` 做明确类型断言。

`WithFiberContext` 与 `WithGinContext` 从各自的 `sync.Pool` 取得 adaptor。`Release` 不属于 `ICoreContext`，只有 adaptor 的 `JSON`/`Send` 会在返回时自动归还对象。只读 header、正常穿过 recovery 而未发送响应，或提前跳过中间件的路径没有统一归还动作。这里描述的是源码所有权边界，不代表已通过长期压测确认泄漏量。

## listen、shutdown 与 TLS 边界

Fiber 和 Gin 都在 goroutine 中启动服务，并在主 goroutine 等待 `SIGINT`/`SIGTERM`。两者都只在监听函数返回后才把 `AppState` 设为 `true`，因此该字段不是“已经开始接流量”的 ready 标记。受控停止都会调用 `GlobalManager.ClearAll(true)` 而不是逐项 `Close`；清空容器不等于数据库、缓存、后台 worker 已被释放，详见[《GlobalManager》](global-manager.md)。

Fiber 的 `OnShutdown` 在 `Shutdown()` 触发时清空容器并关闭日志器。Gin 使用固定 30 秒 shutdown context，随后执行同样清理；它在关闭日志器后仍写一条完成日志，该条是否可见取决于 logger/writer 状态。内建 keepalive 没有绑定这两个 shutdown 流程。

当前 Gin TLS 配置不能描述为可用 HTTPS 能力：`tls.enable=true` 且证书/私钥路径都非空时初始化直接 panic；路径为空时会尝试加载空路径并只记录错误；运行阶段无论如何都调用 `ListenAndServe()`，而不是 `ListenAndServeTLS()`。即使通过自定义选项预置 `TLSConfig`，默认 run 路径也没有建立 TLS listener。Fiber 默认路径同样调用普通 `Listen`；`OnListen` 能显示 TLS 标志并不等于框架已经装配证书。

## 已知限制

- `ICoreContext` 不是完整 Web API，也没有公开的统一 `Release` 生命周期。
- Gin JSON codec 和 Gin mode 都是进程级副作用；多应用/并行测试不能假设隔离。
- 自定义 Fiber `CoreCfg` 早退路径既不安装标准 `FiberErrorHandler`，也没有满足 recovery 的 codec 依赖。
- Gin TLS 逻辑和默认启动调用不构成可工作的 HTTPS 链路。
- 两条受控停止路径清空全局容器，但不提供所有资源和后台 goroutine 的统一关闭协议。

源码入口：[`core_fiber_starter_impl.go`](../../core_fiber_starter_impl.go)、[`core_gin_starter_impl.go`](../../core_gin_starter_impl.go)、[`json_codec_manager.go`](../../json_codec_manager.go)、[`component/codec/json`](../../component/codec/json/)、[`adaptor/context`](../../adaptor/context/) 与 [`adaptor/errorhandler`](../../adaptor/errorhandler/)。
