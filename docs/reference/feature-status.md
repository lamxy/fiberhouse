# 功能状态

本文按当前源码记录 FiberHouse 的能力边界，并作为逐能力状态、启用方式、生命周期、验证证据和限制的唯一详细事实源。这里的“默认”分为两层：`Default()` 只生成默认 `BootConfig` 并调用 `New()`；`DefaultProviders()` 与 `DefaultPManagers(ctx)` 只是提供默认集合，应用仍须通过 `WithProviders`、`WithPManagers` 把集合交给 `FiberHouse`。因此“源码已实现”不等于“调用 `Default()` 后自动启用”。

## 如何理解状态

- **实现阶段**：`占位`表示没有完整运行链；`已实现`表示有可执行实现但未证明进入框架或示例主链；`已接入`表示存在明确入口和可达运行路径。
- **支持级别**：`实验性`表示兼容、生命周期、错误、并发或验证仍未达到晋级门槛；`稳定公共能力`必须满足本文末尾全部晋级条件。本页当前不把任何能力新增标记为稳定。
- **API 受众**：`公共 API`面向使用方；`内部工具`描述设计受众，不覆盖 Go package 实际可导入的兼容事实；占位能力使用`未承诺`。

表中的“默认集合”表示对象是否由 `DefaultProviders()` 或 `DefaultPManagers(ctx)` 收集，不表示 `Default()` 自动装配。状态以完整能力为单位，而不是以单个导出符号为单位；阅读每一行时必须同时检查启用方式，以及创建、运行、失败、关闭四段生命周期。

当前工作树的 hermetic 基线是 `go test ./... -count=1`、`go vet ./...` 和 `go test -race ./... -count=1`。CI 的 `smoke` job 还构建并启动示例、检查一条 HTTP 路径，但当前没有针对 Redis、MongoDB、MySQL、task worker 或 keepalive 的可重复 live integration suite。表中必须显式保留这些验证空白。

## 已接入的核心能力

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Fiber HTTP 内核 | 已接入 | 实验性 | 公共 API | Fiber core provider 在默认集合中，`Default()` 的 `CoreType` 也选择 Fiber，但集合仍需显式装配；应用还需注册 `ApplicationRegister`、`ModuleRegister` 和监听配置 | `CoreWithFiber` 的创建、中间件/监听运行、失败和信号关闭均有路径，但错误与资源关闭尚未形成项目级统一契约 | HTTP smoke | `example_main` 实际选择 Fiber 并追加中间件、hook 与路由；smoke 只检查 `/example/hello/world`；见[Web 运行时](../guides/web-runtime.md) |
| Provider / Manager / Location | 已接入 | 实验性 | 公共 API | 默认集合与预定义 location 需显式传给 `WithProviders`、`WithPManagers`；`DefaultProviders()`/`DefaultPManagers(ctx)` 集合是进程级单例，`Add`/`Except` 只应在启动装配期修改；自定义能力还需匹配 type、target、manager/location 和初始化输入 | type、manager 与 location 驱动创建、运行和失败分发；没有统一的 provider 关闭契约 | 单元/契约 | 未匹配 provider 会交给默认 manager；`example_main` 展示集合合并而非自动发现；见[Provider 系统](../concepts/provider-system.md) |
| bootstrap、配置与日志 | 已接入 | 实验性 | 公共 API | `New()` 自动初始化配置与日志单例，不经过 provider 集合；应用需提供可读配置目录，异步日志由配置选择 | 文件/环境配置和 console/轮转文件、同步/异步 writer 的创建、运行、失败有路径；关闭存在 writer 入口，但停止生产者和关闭顺序由应用负责 | 单元/契约 | `Default()` 使用 `./config`、`./logs`，示例改用 `./example_config`、`./example_main/logs`；见[配置指南](../guides/configuration.md)、[日志指南](../guides/logging.md) |
| JSON 流量编解码与 JSON 响应 | 已接入 | 实验性 | 公共 API | Fiber/Gin 的 Std/Sonic provider 与 JSON manager 在默认集合中但需显式装配；`CoreType`、`TrafficCodec` 和 default/fast global key 必须按消费者匹配 | codec 与统一 `RespInfo` JSON 的创建、运行、失败回退有路径；没有独立关闭资源 | 单元/契约 | 示例注册两个 Sonic 实例并选择 `sonic_json_codec`；基础响应、缓存、task payload 与 recovery stack 使用的 codec key 不是统一前置；空 Go JSON 文件不是可运行实现；见[响应与序列化](../guides/response-and-serialization.md) |
| panic recovery 与错误响应 | 已接入 | 实验性 | 公共 API | Fiber/Gin recovery provider 与 manager 在默认集合中，需随所选内核显式装配 | 两种 recovery 和核心错误中间件的创建、运行、失败响应有路径；没有独立关闭资源，装配失败仍可能 panic 或 fatal | 单元/契约 | 调试信息受 recovery 配置控制，生产环境应关闭详细输出；示例的 `debugMode` 只适合本地演示；见[错误与恢复](../guides/errors-and-recovery.md) |
| 本地缓存与 Redis 缓存 | 已接入 | 实验性 | 公共 API | 不在默认集合；应用通过 GlobalManager 显式注册实例，Redis 还需服务、配置和 `CacheOption` | `cachelocal`、`cacheremote` 的创建、TTL/序列化运行、失败/健康检查和关闭均有入口；Redis 重建、读写和关闭未形成可重复外部验证 | 未验证外部 live integration | 示例注册本地与 Redis initializer，但只把 Redis 列为启动必需项；本地缓存有 hermetic 测试，不能据此推导 Redis；见[缓存指南](../guides/cache.md) |
| 参数验证 | 已接入 | 实验性 | 公共 API | Web `AppContext` 自动调用 `validate.NewWrap(cfg)`；CLI 必须自行构造、注册并持有 wrapper | en、zh-cn、zh-tw、错误映射及自定义 tag/translator 的创建、运行、失败映射有路径；没有独立关闭资源，可变注册只适合启动期 | 单元/契约 | Web 未配置语言时只注册 en，`CmdContext.GetValidateWrap()` 固定返回 nil；示例还追加日语、韩语和自定义 tag；见[验证指南](../guides/validation.md) |

## 实验性或存在明显限制的公共能力

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Gin HTTP 内核 | 已接入 | 实验性 | 公共 API | Gin core provider 在默认集合中但 `Default()` 仍选择 Fiber；启用时设置 `CoreType` 为 `gin` 并显式装配 Gin codec、recovery、中间件和路由 provider/manager | `CoreWithGin` 的创建、运行、失败、关闭有路径；有效证书可填充 `TLSConfig` 并选择 TLS serve，缺失路径仍保留 HTTP 路径 | 单元/契约 | 有效/无效证书加载已有回归测试，TLS/HTTP 启动调用已有 AST 核对，但没有真实 listener/握手集成验证；Gin 仍为实验性；见[Web 运行时](../guides/web-runtime.md) |
| MsgPack / Protobuf 响应 | 已接入 | 实验性 | 公共 API | 两种 MIME provider 与响应 manager 在默认集合中但需显式装配；还需启用 `EnableBinaryProtocolSupport` 并命中 `application/msgpack` 或 `application/x-protobuf` | 两种 HTTP body 实现的创建、运行、失败回退有路径；没有独立关闭资源 | 单元/契约 | 未命中或加载失败时回退 JSON，协商只取首个媒体类型；这是 HTTP body 编码而非通用 RPC；见[响应与序列化](../guides/response-and-serialization.md) |
| GlobalManager | 已接入 | 实验性 | 公共 API | `New()` 获取进程级单例；应用显式注册具体 initializer，且应在启动期完成 | 注册、懒初始化、健康检查、重建、释放、清空覆盖创建、运行、失败、关闭入口；同一已注册 entry generation 内，`Rebuild`/`Release` 维护操作以 fail-fast 方式互斥，冲突调用返回普通的实验性 busy error；删除不取消已经开始的 `Get` 初始化；默认 keepalive 已具备取消、等待退出和重复停止语义，内置 Fiber/Gin 会在 deletion-only 清空前停止并等待它；其余并发与统一资源关闭契约不完整 | 单元/契约 + race | busy error 的 private sentinel 不是稳定公开的 retry 分类；`Get`/`Rebuild`/`Release` 的完整并发状态机和调用方已取得引用的存活期契约尚未闭合，`Rebuild` 不会安全退役旧实例，`ClearAll` 仅删除条目而不逐项 `Close`；GlobalManager 的 owner/locator 责任、共享 alias/组合资源所有权和 task lifecycle 仍未统一，自定义 `FrameStarter` 的 keepalive 停止由自定义实现负责；见[GlobalManager](../guides/global-manager.md) |
| L2 缓存与 Redis 保护机制 | 已接入 | 实验性 | 公共 API | 不默认创建；应用显式构造 local、Redis、L2 并选择回填、同步/异步写、singleflight、Bloom filter 和 circuit breaker | 创建、组合运行、失败保护、关闭有代码路径；策略组合和关闭状态不完整 | 未验证外部 live integration | singleflight 未形成完整 loader 合并，Bloom/breaker miss 语义不一致；现有 hermetic 测试不证明 Redis live 行为；见[缓存指南](../guides/cache.md) |
| 异步任务 | 已接入 | 实验性 | 公共 API | 无默认 task register；应用需提供 Redis、initializer、handler、`TaskRegister` 并启用 `application.task.enableServer` | asynq `TaskWorker`/`TaskDispatcher` 的创建、同步/异步运行和失败记录有路径；统一关闭、dispatcher 回收不完整 | 未验证外部 live integration | 异步启动内部错误只记录，示例依赖外部 Redis，当前无 task worker live suite；见[异步任务指南](../guides/background-tasks.md) |
| CLI | 已接入 | 实验性 | 公共 API | 不属于 Web 默认集合；应用单独创建 `CmdContext`、应用注册器和基于 urfave/cli 的 `CMDLineApplication` | 创建、命令注册和运行有路径；`AppCoreRun` 失败传播、健康检查循环与资源关闭不完整 | 单元/契约 | 健康检查只执行一次，`RunCommandStarter` 丢弃返回值；见[命令行指南](../guides/command-line.md) |
| MySQL / MongoDB | 已接入 | 实验性 | 公共 API | 不默认创建；由应用 initializer 显式注册 GORM/MySQL、MongoDB v2 client，并决定是否在启动期强制初始化 | client/连接池/模型 locator 的创建、运行、失败/健康检查、关闭均有入口；替换时旧 client 关闭与读侧并发契约不完整 | 未验证外部 live integration | Mongo decimal codec 随 client 构造；连接失败会使需要资源的装配失败；smoke 启动服务容器不证明读写、重建或关闭；见[数据库指南](../guides/database.md) |
| 扩展运行位点与关闭链 | 已实现 | 实验性 | 公共 API | 应用可显式绑定自定义 manager；没有通用 before/after provider，关闭由具体 core 决定 | 创建声明和部分运行位点可达；失败契约不统一，关闭 before/after 未消费 | 无专项测试 | `ServerShutdownBefore`/`ServerShutdownAfter` 只有 location 声明；Fiber shutdown 会先停止并等待默认 keepalive，再以 deletion-only 语义清空容器，但仍未消费这些位点；见[Web 启动生命周期](../concepts/startup-lifecycle.md) |

## 内部工具

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| bufferpool 与对象池 | 已实现 | 实验性 | 内部工具 | 不注册、不自动启用；调用方直接构造或借用分片 `bytes.Buffer` 池、泛型 `sync.Pool` | 创建、运行有实现；失败与关闭没有独立生命周期 | 单元/契约 | 当前仓库没有生产调用者，池中对象不能被调用方并发复用；见[组件目录](components.md) |
| Dig 容器 | 已接入 | 实验性 | 内部工具 | `CmdContext` 持有单例容器，CLI ORM 示例显式用于启动和命令装配 | 创建、运行、失败返回和 reset 有路径；没有通用资源关闭编排 | 单元/契约 | Web 运行时不推荐使用，`ResetDigContainer` 非并发安全；见[组件目录](components.md)、[命令行指南](../guides/command-line.md) |
| jsonconvert | 已接入 | 实验性 | 内部工具 | recovery 错误处理按调用借用 wrapper，用后显式 `Release` | 创建、运行、失败回退、释放均有路径；单个 wrapper 不支持并发复用 | 单元/契约 | 主要把任意数据转换为 JSON 或字符串；见[组件目录](components.md)、[错误与恢复](../guides/errors-and-recovery.md) |
| mongodecimal | 已接入 | 实验性 | 内部工具 | 随 `dbmongo.NewClient` 构造自动注册到 BSON registry，不是独立服务 | 创建和运行随 Mongo client；失败与关闭也依附 client，没有独立生命周期 | 无专项测试 | 只服务 `decimal.Decimal` BSON 编解码，外部 Mongo live 行为未验证；见[组件目录](components.md)、[数据库指南](../guides/database.md) |
| logging writer 与 task logger adaptor | 已接入 | 实验性 | 内部工具 | bootstrap 按配置选择同步/异步 writer；task logger adaptor 由示例任务显式装配 | 创建、运行、失败和关闭入口存在；异步停止生产者与关闭顺序未统一编排 | 单元/契约 + race | 异步 writer 可能丢日志，task adaptor 目前主要服务示例；见[组件目录](components.md)、[日志指南](../guides/logging.md)、[异步任务指南](../guides/background-tasks.md) |

## 预留与占位

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| plugins | 占位 | 不适用 | 未承诺 | 不适用；不在默认集合，配置字段不会启用它 | 创建、运行、失败、关闭均无完整路径 | 不适用 | 只有 `Plugin` 接口，没有 loader、registry 或应用调用链；见本页、[扩展 FiberHouse](../guides/extending-fiberhouse.md) |
| RPC | 占位 | 不适用 | 未承诺 | 不适用 | 创建、运行、失败、关闭均无完整路径 | 不适用 | `component/rpc` 仅占位；`response/pb` 的 Protobuf HTTP 响应不是 RPC；见本页、[响应与序列化](../guides/response-and-serialization.md) |
| MQ | 占位 | 不适用 | 未承诺 | 不适用；配置中的 `mq` 不会创建能力 | 创建、运行、失败、关闭均无完整路径 | 不适用 | 只有 RabbitMQ 方向说明，没有 provider、client 或 consumer；见本页 |
| i18n | 占位 | 不适用 | 未承诺 | 不适用 | 创建、运行、失败、关闭均无完整路径 | 不适用 | `component/i18n` 没有 Go 实现；validate 翻译不等于通用 i18n；见本页、[验证指南](../guides/validation.md) |
| Go JSON codec | 占位 | 不适用 | 未承诺 | 不适用；默认集合没有 Go JSON provider | 创建、运行、失败、关闭均无完整路径 | 不适用 | `component/codec/json/gojson.go` 只有 package 声明，即使常量存在也不可选择；见[响应与序列化](../guides/response-and-serialization.md) |
| 空 component/middleware 目录说明 | 占位 | 不适用 | 未承诺 | 不适用 | 创建、运行、失败、关闭均无完整路径 | 不适用 | placeholder 文档和示例空分支只表达目录意图；见[组件目录](components.md)、[示例目录](examples.md) |
| 未消费的生命周期 hook | 占位 | 不适用 | 未承诺 | 不适用；自定义代码不能假设声明的位点会执行 | 创建只有声明；运行、失败、关闭均无消费路径 | 不适用 | `ServerShutdownBefore` 与 `ServerShutdownAfter` 已声明但启动链未读取；见[Web 启动生命周期](../concepts/startup-lifecycle.md) |

## 判断依据

本页以 `default.go`、`boot.go`、具体 core starter、`component/`、`plugins/`、`response/pb` 和三个示例目录的当前调用路径为准：类型、目录或配置键不算接入；默认集合不算自动启用；缺失 provider、manager、配置或 global 时可能返回错误、记录 fatal 或 panic；后台 goroutine、连接池、缓存、writer 和 task worker 的资源所有权仍须由应用明确。`example_main`、`example_config` 与 `example_application` 只证明调用路径存在，不构成兼容性或成熟度承诺。

复核入口包括 `Default()`、`New()`、`RunServer()`、`RunApplicationStarter()`、默认 provider/manager 集合、预定义 location、Fiber/Gin/response/recovery 的启动实现，以及 GlobalManager、缓存、数据库、任务和 CLI 的构造与关闭方法。后续指南若与本页冲突，应先按当前源码复核调用路径，再更新本页。

## 稳定公共能力晋级条件

能力只有同时满足以下条件，才能从“实验性”晋级为“稳定公共能力”：

1. 创建、运行、失败、关闭四条路径都有明确契约。
2. 公开 API 有兼容承诺，默认与显式启用方式无歧义。
3. 单元/契约测试进入 CI；并发能力通过 race，外部依赖通过可重复 live integration test。
4. 不依靠 panic 或 fatal 表达可恢复的装配或运行错误。
5. 文档示例确实执行该路径，并明确资源所有权和限制。
