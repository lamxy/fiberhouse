# Web 运行时

FiberHouse 用 `BootConfig.CoreType` 在同一套启动位点中选择 Fiber 或 Gin。两条路径共享应用 Context、Provider/Manager、业务注册器和响应/异常抽象，但不会把两个 HTTP 引擎包装成完全相同的 API：路由与 handler 仍直接使用 `*fiber.Ctx` 或 `*gin.Context`。Provider 装配与完整位点顺序见[《Provider 系统》](../concepts/provider-system.md)和[《Web 启动生命周期》](../concepts/startup-lifecycle.md)。

## 选择与装配条件

`Default()` 选择 `CoreType="fiber"`、`TrafficCodec="sonic_json_codec"`，且关闭二进制响应。使用 `New(&fiberhouse.BootConfig{...})` 时不会补齐这些字段，调用方应显式给出 `CoreType`、`TrafficCodec`、配置目录和日志目录；配置来源与单例边界见[《配置与引导》](configuration.md)。

核心启动器和 JSON codec Manager 都按启动配置筛选 Provider：前者选择目标 `CoreType`，后者同时要求 Provider 的 `Version()` 等于 `TrafficCodec`、`Target()` 等于 `CoreType`。默认 Provider 集合为 Fiber/Gin 各注册 std 与 Sonic 实现。传入了 Manager 列表却缺少 `GroupTrafficCodecChoose` Manager，或者没有匹配的 codec Provider，都会在 `InitCoreApp` 阶段失败，而不是静默改用其他 codec。

标准 Web 装配的主线是：

```text
CoreType 选择 CoreStarter
  → InitCoreApp：安装框架日志桥接、创建引擎、安装 JSON codec/错误入口
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
| `InitCoreApp` | 将选中的 `JsonWrapper.Marshal/Unmarshal` 固化为该 app 的 `JSONEncoder/JSONDecoder`，并在 `fiber.Config` 安装全局 `ErrorHandler` | 先取得 Gin 框架日志桥接的进程级 lease，再调用 `gin.New(...)`，随后把选中的 codec 写入 `gin/codec/json.API` 并构造 `http.Server` |
| 内建中间件顺序 | recover → `fiberzerolog` → `ApplicationRegister.RegisterAppMiddleware` | recover → 尾部错误处理 → 请求日志 → `ApplicationRegister.RegisterAppMiddleware` |
| 普通错误入口 | Fiber handler 返回 `error`，由 `fiber.Config.ErrorHandler` 处理 | handler 调用 `c.Error(err)`，或在没有 `c.Errors` 时用 `c.Set("error", err)`；尾部中间件在 `c.Next()` 后处理 |
| panic 入口 | Fiber recovery Provider | Gin recovery Provider |
| 路由注册 | `ModuleRegister.RegisterModuleRouteHandlers` 接收 Fiber starter | 同一接口接收 Gin starter |
| 监听 | `fiber.App.Listen(host+":"+port)` | 无 TLS 配置时调用 `http.Server.ListenAndServe()`；已加载 TLS 配置时调用 `ListenAndServeTLS("", "")` |
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

Gin 的规范运行模式键是 `application.plugins.engine.servers.gin.mode`，接受 Gin 定义的 `debug`、`release`、`test` 三个值。解析时第一个非空值优先：先读取规范键；规范键缺失或为空时读取旧键 `application.plugins.server.gin.mode` 作为兼容 fallback；两者都为空时使用 `release`。不受支持的非空值仍由 `gin.SetMode` 按 Gin 的既有行为拒绝。`application.recover.debugMode` 只控制 recovery 响应细节与堆栈行为，不再改变 Gin mode；依赖旧耦合行为的应用必须显式设置规范键。

同一 Gin 分组中的服务参数还包括：`host=0.0.0.0`、`port=8080`、`readTimeout=30` 秒、`writeTimeout=30` 秒、`idleTimeout=120` 秒、`readHeaderTimeout=10` 秒；`maxHeaderBytes` 的配置值先按 KiB 读取，fallback `1024`，再乘 `1024`。这些数值是具体消费方的源码 fallback，不是示例 YAML 的默认声明。

两个引擎的 duration getter 结果都会再次乘 `time.Second`，当前约定应填写数值秒；不要传带 `ms`/`s` 单位的 duration 字符串并期待原样使用。

## Gin 框架诊断与请求日志

选择 Gin core 会自动把 Gin 的启动、路由注册、通用 debug、默认 writer、错误 writer 和 `http.Server` 内部错误接入 FiberHouse `LoggerWrapper`。debug hook 与路由记录使用 Debug，默认 writer 使用 Info，错误 writer 和默认 server error logger 使用 Error；所有记录都带 `Component="Gin"`，并按来源带 `Channel`。Debug 记录仍受 `application.appLog.level` 过滤；Gin 没有为启动 warning 提供独立级别，因此经 debug hook 到达的 warning 也按 Debug 处理，而不会解析私有消息文本猜测级别。应用显式提供的 `http.Server.ErrorLog` 不会被替换。

这些 Gin hook 和 writer 是 package 全局变量。一个活跃的 FiberHouse Gin core 取得独占 lease，初始化失败、server 返回或 shutdown 时恢复取得 lease 前的原值；第二个 FiberHouse core 不能在 lease 活跃时覆盖它。lease 活跃期间创建的其他 Gin engine 也会把原生诊断写到同一个框架日志器，不能获得逐 engine 的原生 debug 隔离；外部代码同时改写这些全局变量也不受支持。

现有 FiberHouse 请求日志、recovery 和尾部错误处理中间件仍是访问与恢复记录的权威来源。框架不会额外安装 Gin 原生 Logger 或 Recovery，因此默认请求路径不会因日志桥接增加第二条访问或恢复记录；现有请求日志继续使用 Info、`LogOriginCoreHttp` 和原有 HTTP 字段，并增加 `Component="Gin"`。

## JSON codec 是启动期引擎配置

内建可用组合是 `std_json_codec` 与 `sonic_json_codec` × Fiber/Gin。`go_json_codec` 只有常量，当前 [`component/codec/json/gojson.go`](../../component/codec/json/gojson.go) 没有实现，也没有默认 Provider，不能作为可用选项宣传。Sonic 的 `Unmarshal` 失败后会再尝试标准库 `encoding/json`；`Marshal` 不做同类 fallback。

两条安装路径的作用域不同：

- Fiber 把函数保存在单个 `fiber.App` 的配置中，作用域是该引擎实例。
- Gin 修改 `gin/codec/json.API` 包级变量。这是进程全局副作用：同进程创建多套 Gin 应用并选择不同 codec 不具备隔离保证，初始化先后也会影响 Gin 的其他使用者。

这里的引擎 JSON codec 负责请求绑定和 JSON 写出。starter 还把同一个 marshal 函数放进 `RecoverConfig.JsonCodec`，但 recovery 只在 debug 模式处理“既不是 `error` 也不是已知异常”的 panic 值时用它尝试 JSON 转换。`DefaultStackTraceHandler` 编码 params/query/headers 与异常 data 时读取的则是应用注册器 `GetFastTrafficCodecKey()` 指向的容器实例，这是第三个、由应用装配的 codec 入口；它不等同于 `BootConfig.TrafficCodec` 的选择结果。`DebugStackLines` 是例外：`GetJsonIndent` 忽略传入的 codec 参数，固定调用标准库 `encoding/json.MarshalIndent`。示例应用可以让前述 codec 都指向 Sonic，但框架没有强制它们相同。

以上三个 codec 入口都不决定统一响应是否改用 MsgPack/Protobuf；后者是 `ResponseWrap.SendWithCtx` 的独立协商，见[《响应与序列化》](response-and-serialization.md)。

传入自定义 Fiber `CoreCfg` 时，当前 `InitCoreApp` 会在 `fiber.New(*CoreCfg)` 后提前返回，也不会执行标准分支中 `ErrorHandler: adaptorerrorhandler.FiberErrorHandler(eh.ErrorHandler)` 的装配；调用方只有在自己的 `fiber.Config` 中显式设置等价 `ErrorHandler` 才能保留统一普通错误入口。`cf.json` 的装配已修复：标准启动链传入非 nil `fs` 时，该路径同样会调用 `resolveJSONCodec` 完成编解码器解析并赋值给 `cf.json`，`RegisterAppMiddleware` 引用 `cf.json.Marshal` 不再有 nil 解引用风险；只有在 `fs` 为 nil（例如仅验证自定义配置的单元测试）时才会跳过这一步。这条自定义路径仍不能视为已完整支持的标准装配，因为 `ErrorHandler` 的等价装配仍需调用方自行补齐。

## `ICoreContext`：最小跨引擎边界

[`adaptor/context`](../../adaptor/context/) 的 `ICoreContext` 只统一五件事：取得原生 Context、读请求头、写响应头、按 HTTP status 发送 JSON、按 HTTP status 发送原始字节。路由参数、query/body 绑定、重定向、文件发送、`c.Error` 等仍使用引擎原生 API；需要时用 `GetCtx()` 做明确类型断言。

`WithFiberContext` 与 `WithGinContext` 从各自的 `sync.Pool` 取得 adaptor。`Release` 不属于 `ICoreContext`，只有 adaptor 的 `JSON`/`Send` 会在返回时自动归还对象。只读 header、正常穿过 recovery 而未发送响应，或提前跳过中间件的路径没有统一归还动作。这里描述的是源码所有权边界，不代表已通过长期压测确认泄漏量。

## listen、shutdown 与 TLS 边界

Fiber 和 Gin 都在 goroutine 中启动服务，并在主 goroutine 等待 `SIGINT`/`SIGTERM`。两者都只在监听函数返回后才把 `AppState` 设为 `true`，因此该字段不是“已经开始接流量”的 ready 标记。受控停止都会调用 `GlobalManager.ClearAll(true)` 而不是逐项 `Close`；清空容器不等于数据库、缓存、后台 worker 已被释放，详见[《GlobalManager》](global-manager.md)。

Fiber 的 `OnShutdown` 在 `Shutdown()` 触发时先停止并等待默认 keepalive，再清空容器、记录 shutdown 日志并关闭日志器。Gin 使用固定 30 秒 shutdown context，随后按停止并等待默认 keepalive、清空容器、记录完成日志、恢复 Gin 日志全局变量、关闭日志器的顺序清理；server 先返回时也会幂等释放同一个 lease。该协调只覆盖默认 `FrameApplication` 的健康检查，不包含 task worker、应用自建 goroutine 或逐项资源关闭。

当前 Gin TLS 配置已接通证书加载和启动选择：`tls.enable=true` 且证书/私钥路径有效时会填充 `TLSConfig`，运行阶段随后调用 `ListenAndServeTLS("", "")`；没有 TLS 配置时仍调用 `ListenAndServe()`。无效的非空证书/私钥会在加载失败后 fail-stop；缺失任一路径时则只记录错误并保持 `TLSConfig == nil`，因此显式启用 TLS 的应用仍应在部署前校验配置，避免落到 HTTP 路径。现有测试覆盖有效证书加载和无效证书失败，并用 AST 核对 TLS/HTTP 调用分支；另有一条以 `127.0.0.1:0` 建立 loopback listener、通过 `http.Server.ServeTLS` 驱动真实 TLS 握手与 HTTP 请求-响应、并验证 `Shutdown` 在 3 秒内正常完成的回归测试。Fiber 默认路径同样调用普通 `Listen`；`OnListen` 能显示 TLS 标志并不等于框架已经装配证书。

## 已知限制

- `ICoreContext` 不是完整 Web API，也没有公开的统一 `Release` 生命周期。
- Gin JSON codec、mode 和原生日志 hook 都是进程级副作用；同一时刻只有一个 FiberHouse core 能持有日志 lease，其他 Gin engine 会共享该 lease 的框架日志器，不能假设逐 engine 隔离。
- 自定义 Fiber `CoreCfg` 早退路径不安装标准 `FiberErrorHandler`；`cf.json` 会在标准启动链（非 nil `fs`）下正确装配，但仅验证配置本身、不传 `fs` 的调用方式仍会跳过这一步。
- Gin TLS 的证书加载与 HTTPS 启动路径已接通，并有真实 loopback listener 握手回归测试；但缺失路径仍可能保留 HTTP 路径。
- 两条受控停止路径清空全局容器，但不提供所有资源和后台 goroutine 的统一关闭协议。

源码入口：[`core_fiber_starter_impl.go`](../../core_fiber_starter_impl.go)、[`core_gin_starter_impl.go`](../../core_gin_starter_impl.go)、[`json_codec_manager.go`](../../json_codec_manager.go)、[`component/codec/json`](../../component/codec/json/)、[`adaptor/context`](../../adaptor/context/)、[`adaptor/errorhandler`](../../adaptor/errorhandler/) 与 [`adaptor/logging`](../../adaptor/logging/)。
