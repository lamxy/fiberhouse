# 功能状态

本文按当前源码记录 FiberHouse 的能力边界。这里的“默认”分为两层：`Default()` 只生成默认 `BootConfig` 并调用 `New()`；`DefaultProviders()` 与 `DefaultPManagers(ctx)` 只是提供默认集合，应用仍须通过 `WithProviders`、`WithPManagers` 把集合交给 `FiberHouse`。因此“源码已实现”不等于“调用 `Default()` 后自动启用”。

## 如何理解状态

- `已接入`：存在可达的运行时实现和明确入口。它仍可能要求应用注册依赖或选择配置，并不等同于生产就绪承诺。
- `实验性`：已有实现和调用路径，但在生命周期、错误处理、并发边界或外部依赖上存在明显限制，采用前应阅读表中所列主指南。
- `内部工具`：主要服务于框架内部或示例装配，不作为稳定公共抽象推荐给业务代码。
- `预留/占位`：只有接口、配置项、生成类型、空目录说明或未接线声明；不能据此推断已有完整运行链。

表中的“默认集合”表示对象是否由 `DefaultProviders()` 或 `DefaultPManagers(ctx)` 收集，不表示 `Default()` 自动装配。

阅读每一行时应同时检查四个维度：

- “框架实现”回答仓库里是否存在可执行代码，而不是只有名称或目录。
- “默认注册”回答对象是否进入框架提供的集合，或是否由 `New()` 直接初始化。
- “应用启用”回答业务应用还必须提供哪些注册器、配置、实例或外部服务。
- “示例边界”回答示例是否真的走到该路径，以及示例之外还缺少什么。

状态以完整能力为单位，而不是以单个导出符号为单位。某个类型可被外部引用，不代表它已经拥有创建、运行、报错和关闭的完整生命周期。

## 已接入的核心能力

| 能力 | 状态 | 框架实现 | 默认注册 | 应用启用 | 示例边界 |
|---|---|---|---|---|---|
| Fiber HTTP 内核 | 已接入 | `CoreWithFiber` 负责引擎初始化、中间件、监听信号与关闭 | Fiber core provider 在默认集合中；`Default()` 的 `CoreType` 也选择 Fiber，但集合不会自动传入 | 装配默认 provider/manager，并注册 `ApplicationRegister`、`ModuleRegister`；配置监听参数 | `example_main` 实际选择 Fiber，并追加中间件、hook 与路由 provider |
| Provider / Manager / Location | 已接入 | 类型、管理器与执行位点驱动启动顺序；未匹配 provider 会交给默认 manager | 提供默认 provider/manager 集合和预定义 location；需显式 `WithProviders`、`WithPManagers` | 自定义能力必须同时提供匹配的类型、target、manager/location 和初始化输入 | `example_main` 展示默认集合与应用 provider/manager 的合并，不是自动发现机制 |
| bootstrap、配置与日志 | 已接入 | `New()` 调用配置、日志单例；配置支持文件与环境覆盖；日志支持 console、轮转文件及同步/异步 writer | 随 `New()` 初始化，不经过 provider 集合；`Default()` 使用 `./config`、`./logs` | 必须提供可读配置目录；异步日志由配置选择，关闭责任属于应用生命周期 | 示例改用 `./example_config` 与 `./example_main/logs`；详细边界见[配置指南](../guides/configuration.md)、[日志指南](../guides/logging.md) |
| JSON 流量编解码与 JSON 响应 | 已接入 | Fiber/Gin 的 Std JSON、Sonic provider，以及统一 `RespInfo` JSON 回退路径 | Std/Sonic provider 与 JSON manager 在默认集合中，但不会自动装配 | 基础引擎 codec 需要 `CoreType`/`TrafficCodec` 与已装配的 codec Provider/Manager 匹配；未传 codec Manager 时会从 `GetDefaultTrafficCodecKey()` 回退取实例，Sonic Provider 也依赖该 key。缓存默认序列化、task payload 与 recovery stack 等独立消费者再按需注册 default/fast key，它们不是基础 JSON 响应的统一前置 | 示例注册两个 Sonic 实例并选择 `sonic_json_codec`；空的 Go JSON 文件不构成第三种实现 |
| panic recovery 与错误响应 | 已接入 | Fiber、Gin recovery provider 和核心错误处理中间件均有运行路径；调试信息受 recovery 配置控制 | 两种 recovery provider 与 recovery manager 在默认集合中 | 随所选 HTTP 内核和 manager 装配；生产环境应关闭详细错误输出 | 示例配置启用 `debugMode` 仅适合本地演示；错误路径可能记录日志、返回统一响应或在装配失败时 panic |
| 本地缓存与 Redis 缓存 | 已接入 | `cachelocal`、`cacheremote` 实现统一 `Cache`，包含 TTL、序列化、健康检查与关闭入口 | 不在默认 provider 集合中 | 应用通过 GlobalManager 注册实例；Redis 需要可用服务和配置，调用者持有 `CacheOption` | Web 示例注册本地与 Redis initializer，但只把 Redis 列为启动必需项；完整用法见[缓存指南](../guides/cache.md) |
| 参数验证 | 已接入 | `component/validate` 提供 en、zh-cn、zh-tw、错误映射和自定义 tag/translator 注册 | 只有 Web `AppContext` 自动调用 `validate.NewWrap(cfg)`；`CmdContext.GetValidateWrap()` 固定返回 nil | Web 按 `application.validate.langFlags` 注册语言，未配置时仅注册 en；CLI 需自行构造、注册和持有 wrapper，且所有可变注册只在启动期完成 | 示例配置选择 en/zh-CN/zh-TW，并追加日语、韩语和自定义 tag；包装器不支持运行期并发读写，详见[验证指南](../guides/validation.md) |

## 实验性或存在明显限制的能力

| 能力 | 状态 | 框架实现 | 默认注册 | 应用启用 | 限制与主指南 |
|---|---|---|---|---|---|
| Gin HTTP 内核 | 实验性 | `CoreWithGin`、Gin codec、recovery、中间件和路由 provider 已存在 | Gin core provider 在默认集合中；`Default()` 仍选择 Fiber | 设置 `CoreType` 为 `gin`，并装配 Gin 对应 provider/manager | TLS 分支在证书路径有效时仍会 panic，不能称为完整 HTTPS 能力；见[Web 运行时](../guides/web-runtime.md) |
| MsgPack / Protobuf 响应 | 实验性 | `response` 有两种二进制响应实现，provider 按 MIME type 获取 | 两种 provider 与响应 manager 在默认集合中，但不自动装配 | `EnableBinaryProtocolSupport` 必须为 true，且请求 `Content-Type`/`Accept` 命中 `application/msgpack` 或 `application/x-protobuf` | 未命中或加载失败时回退 JSON，内容协商只取首个媒体类型；这不是通用 RPC；见[响应与序列化](../guides/response-and-serialization.md) |
| GlobalManager | 实验性 | 支持注册、懒初始化、健康检查、重建、释放与清空 | `New()` 获取进程级单例；具体资源由应用注册 | initializer 应在启动期注册，运行期以读为主；资源所有者必须规划关闭 | `Release` 存储 nil、失败状态残留、keepalive 无取消和清空时不逐项关闭等边界需规避；见[GlobalManager](../guides/global-manager.md) |
| L2 缓存与 Redis 保护机制 | 实验性 | 本地/远端组合、回填、同步/异步写、singleflight、Bloom filter、circuit breaker 均有代码 | 不默认创建 | 应用自行构造 local、Redis、L2，并显式选择策略和保护开关 | singleflight 未形成完整 loader 合并，Bloom/breaker miss 语义不一致，L2 关闭状态不完整；见[缓存指南](../guides/cache.md) |
| 异步任务 | 实验性 | `TaskWorker`、`TaskDispatcher` 基于 asynq，支持 handler map 与同步/异步启动 | 无默认 task register；只在应用提供 `TaskRegister` 后进入启动链 | 需要 Redis、任务 initializer、handler 注册和 `application.task.enableServer` | 异步启动内部错误只记录，统一关闭与 dispatcher 回收未完整编排；示例依赖外部 Redis；见[异步任务指南](../guides/background-tasks.md) |
| CLI | 实验性 | `commandstarter` 基于 urfave/cli，包含上下文、命令注册和错误处理 | 不属于 Web 默认集合 | 单独创建 `CmdContext`、应用注册器和 `CMDLineApplication` | `RunCommandStarter` 丢弃 `AppCoreRun` 返回值，健康检查只执行一次，未统一回收资源；见[命令行指南](../guides/command-line.md) |
| MySQL / MongoDB | 实验性 | GORM/MySQL、MongoDB v2、连接池、健康检查、模型 locator 与 Mongo decimal codec 已实现 | 不默认创建 | 由应用 initializer 注册并决定是否在启动期强制初始化 | 重建不会关闭旧 client，读侧也没有与替换锁配套；连接失败会使需要该资源的装配失败；见[数据库指南](../guides/database.md) |
| 扩展运行位点与关闭链 | 实验性 | `ServerRunBefore`、`ServerRun`、`ServerRunAfter`、`ServerShutdown` 有部分消费路径 | 没有通用 before/after provider；关闭行为由具体 core 实现决定 | 应用可绑定自定义 manager，但必须验证阻塞、信号与资源所有权 | `ServerShutdownBefore`/`ServerShutdownAfter` 只有 location 声明，Fiber shutdown 直接清空容器；见[Web 启动生命周期](../concepts/startup-lifecycle.md) |

## 内部工具

| 能力 | 状态 | 框架中的用途 | 默认/应用边界 | 主指南 |
|---|---|---|---|---|
| bufferpool 与对象池 | 内部工具 | 提供分片 `bytes.Buffer` 池和泛型 `sync.Pool`，当前仓库没有生产调用者 | 不注册、不自动启用；池中对象不能被调用方并发复用 | [组件目录](components.md) |
| Dig 容器 | 内部工具 | `CmdContext` 持有单例容器，CLI ORM 示例用它在启动/命令执行阶段组装依赖 | Web 运行时不推荐使用；`ResetDigContainer` 非并发安全 | [组件目录](components.md)、[命令行指南](../guides/command-line.md) |
| jsonconvert | 内部工具 | recovery 错误处理把任意数据转换成 JSON 或字符串 | 每次借用后必须 `Release`；单个 wrapper 不是并发对象 | [组件目录](components.md)、[错误与恢复](../guides/errors-and-recovery.md) |
| mongodecimal | 内部工具 | `dbmongo.NewClient` 将 `decimal.Decimal` 注册到 BSON registry | 随 Mongo client 构造生效，不是独立服务 | [组件目录](components.md)、[数据库指南](../guides/database.md) |
| writer 与 tasklog | 内部工具 | bootstrap 的同步/异步文件 writer；示例 asynq 日志适配器 | 异步 writer 可能丢日志，必须在停止生产者后关闭；tasklog 目前由示例任务装配 | [组件目录](components.md)、[日志指南](../guides/logging.md)、[异步任务指南](../guides/background-tasks.md) |

## 预留与占位

| 能力 | 状态 | 当前证据 | 默认/应用边界 | 主指南 |
|---|---|---|---|---|
| plugins | 预留/占位 | 只有 `Plugin` 接口；loader、registry 没有实现，且没有应用启动或关闭调用链 | 不在默认集合中，配置中的 `plugins` 字段也不等于插件系统 | 本页、[扩展 FiberHouse](../guides/extending-fiberhouse.md) |
| RPC | 预留/占位 | `component/rpc` 只有空占位文件；统一响应的 Protobuf schema 与生成代码位于 `response/pb` | 没有 client/server、注册、监听或关闭生命周期；二进制 HTTP 响应不能视为 RPC | 本页、[响应与序列化](../guides/response-and-serialization.md) |
| MQ | 预留/占位 | `component/mq` 仅有 RabbitMQ 方向说明，配置中 `mq` 为空 | 没有 provider、client、consumer 或生命周期 | 本页 |
| i18n | 预留/占位 | `component/i18n` 没有 Go 实现 | 验证消息翻译只属于 validate 组件，不代表通用 i18n | 本页、[验证指南](../guides/validation.md) |
| Go JSON codec | 预留/占位 | `component/jsoncodec/gojson.go` 只有 package 声明，默认集合也没有 Go JSON provider | 即使常量存在，当前不能选择为可运行 codec | [响应与序列化](../guides/response-and-serialization.md) |
| 空 component/middleware 目录说明 | 预留/占位 | placeholder 文档和若干示例空分支只表达目录意图 | 不形成可注册中间件或组件 | [组件目录](components.md)、[示例目录](examples.md) |
| 未消费的生命周期 hook | 预留/占位 | `ServerShutdownBefore` 与 `ServerShutdownAfter` 已声明但当前启动链未读取 | 自定义代码不能假设这些位点会执行 | [Web 启动生命周期](../concepts/startup-lifecycle.md) |

## 判断依据

本页以 `default.go`、`boot.go`、具体 core starter、`component/`（包含 `component/container/`、`component/cache/`、`component/database/` 等组件）、`plugins/`、`response/pb` 和三个示例目录的当前调用路径为准。判断时遵循以下规则：

1. 有类型或配置键不算接入；必须能从框架或应用入口到达初始化、运行和关闭路径。
2. 在默认集合中不算自动启用；应用必须显式把集合交给 `FiberHouse`。这些集合是进程级单例，`Add`/`Except` 虽有锁，仍应只在启动装配期修改。
3. 启动所需 provider、manager、配置或全局实例缺失时，当前代码可能返回错误、记录 fatal 日志或 panic；可选二进制响应加载失败则回退 JSON。应用应在启动阶段暴露错误，而不是依赖运行期补装配。
4. 后台 goroutine、连接池、缓存、日志 writer 和任务 worker 都有独立生命周期。当前关闭链不能替应用证明资源已完整释放，调用方仍需明确所有权、停止顺序与超时。
5. `example_main`、`example_config` 与 `example_application` 只证明某条装配和调用路径存在，不构成兼容性或成熟度承诺。

复核状态时使用的入口包括：

- `Default()`、`New()`、`RunServer()` 与 `RunApplicationStarter()`；
- `DefaultProviders()`、`DefaultPManagers(ctx)` 和预定义 provider location；
- Fiber、Gin、response、recovery 的 provider/manager 与 core starter；
- GlobalManager、缓存、数据库、任务和 CLI 的构造与关闭方法；
- `example_main` 的 Web 装配、`example_application/command` 的 CLI 装配；
- plugins、RPC、MQ、i18n 目录是否存在可达初始化和停止链。

后续指南若与本页冲突，应先回到当前源码重新确认调用路径，再更新状态，而不是沿用旧文档中的能力描述。
