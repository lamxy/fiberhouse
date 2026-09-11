# 功能状态

本文按当前源码记录 FiberHouse 的能力边界，并作为逐能力状态、启用方式、生命周期、验证证据和限制的唯一详细事实源。这里的“默认”分为两层：`Default()` 只生成默认 `BootConfig` 并调用 `New()`；`DefaultProviders()` 与 `DefaultPManagers(ctx)` 只是提供默认集合，应用仍须通过 `WithProviders`、`WithPManagers` 把集合交给 `FiberHouse`。因此“源码已实现”不等于“调用 `Default()` 后自动启用”。

## 如何理解状态

- **实现阶段**：`已实现`表示有可执行实现但未证明进入框架或示例主链；`已接入`表示存在明确入口和可达运行路径。
- **支持级别**：`实验性`表示兼容、生命周期、错误、并发或验证仍未达到晋级要求；`稳定公共能力`必须满足本文末尾全部晋级条件（条件 2 在 v2.0.0 前豁免，见文末说明）。GlobalManager、MySQL / MongoDB、本地缓存与 Redis 缓存、JSON 流量编解码与 JSON 响应四项已完成晋级，其余能力仍为实验性。
- **API 受众**：`公共 API`面向使用方；`内部工具`描述设计受众，不覆盖 Go package 实际可导入的兼容事实。

表中的“默认集合”表示对象是否由 `DefaultProviders()` 或 `DefaultPManagers(ctx)` 收集，不表示 `Default()` 自动装配。状态以完整能力为单位，而不是以单个导出符号为单位；阅读每一行时必须同时检查启用方式，以及创建、运行、失败、关闭四段生命周期。

当前工作树的 hermetic 基线是 `go test ./... -count=1`、`go vet ./...` 和 `go test -race ./... -count=1`。CI 的 `smoke` job 还构建并启动示例、检查一条 HTTP 路径，并定向执行带 `liveintegration` build tag 的回归测试（`go test -tags=liveintegration ./... -run 'TestLive_' -v -count=1`），覆盖 Redis cache、asynq、MySQL、MongoDB 各自的创建-读写-关闭路径。

live 测试现已覆盖：三个封装各自的并发读写与并发健康检查同时进行（N 个读写 goroutine 叠加持续 `IsHealthy`，断言无错误、无丢行/丢文档、健康检查确实执行且未误报）、MySQL 与 MongoDB 的 `Close` 幂等，以及就绪探针对真实依赖故障的端到端反应（依赖可达 200 → TCP 层切断 503 并点名不健康 key → 恢复后重新 200，全程框架不做任何重建）。上述用例均在 `-race` 下通过。

仍未闭合的验证空白：asynq 任务仍只有单条入队-消费-关闭路径。关闭路径的资源释放现已覆盖四条：Fiber 与 Gin 走 `clearApplicationGlobals`，CLI（`RunCommandStarter`）与 `example_application/hertzcore` 因上下文类型与匯出边界不同，各自直接调用容器已匯出的 `ReleaseAllWithObserver` + `ClearAll`。四条路径的释放失败均按 key 上报且不中断回收。

## 稳定公共能力

以下能力满足文末全部晋级条件（条件 2 在 v2.0.0 前豁免）：四段生命周期契约明确、
测试与 race 进入 CI、外部依赖有可重复 live integration 覆盖、可恢复错误不依赖 panic
表达、示例确实执行对应路径。晋级不冻结导出符号，见文末豁免说明。

晋级过程与逐条证据见
[GlobalManager 稳定性晋级设计](../superpowers/specs/2026-09-08-globalmanager-stability-promotion-design.md)。

JSON 流量编解码逐条对照：条件 1，四条路径有契约，且「没有独立关闭资源」是完整契约——
`JsonWrapper` 只暴露 `Marshal`/`Unmarshal`，包内无任何 `Close`/`Shutdown`；条件 3，
`component/codec/json` 覆盖率 100%，随 `go test ./...` 与独立 race job 进入 CI，且不依赖外部
服务，无需 live integration；条件 4，全包不含 `panic`/`fatal`，错误一律以 error 返回；条件 5，
[响应与序列化](../guides/response-and-serialization.md)描述该路径，示例经 `resolveJSONCodec`
实际执行。条件 2 依 v2.0.0 前豁免不作门槛。

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| GlobalManager | 已接入 | 稳定公共能力 | 公共 API | `New()` 获取进程级单例；应用显式注册具体 initializer，且应在启动期完成 | 注册、懒初始化、健康检查、重建、释放、清空四段路径均有明确契约；`Rebuild`/`Release` 在同一 entry 上 fail-fast 互斥，冲突返回包装 `ErrMaintenanceInProgress` 的错误，可 `errors.Is` 分类重试；`Release` 对已初始化对象一律重置（非 `Closable` 也生效），`Close` 失败保留实例并返回错误，未初始化 entry 为幂等空操作；`ReleaseAll` 按嵌套深度由外向内释放组合对象（`CompositeClosable` / `CompositeOwnershipInspector`），失败经 `ReleaseAllWithObserver` 逐 key 上报；`ClearAll` 只删除条目、不关闭资源，须在 `ReleaseAll` 之后调用；初始化失败不会发布 typed-nil 实例；删除不取消已开始的 `Get` 初始化；健康检查具备取消、等待退出与重复停止语义 | 单元/契约 + race（含 200/300 轮关闭顺序、嵌套排序、持有成环终止、包外 sentinel 分类） | `Get`/`Rebuild`/`Release` 的完整并发状态机与调用方已取得引用的存活期契约仍未闭合，`Rebuild` 不会安全退役旧实例；持有关系成环时只保证终止、不保证顺序；`ReleaseAll` 现由四条关闭路径调用——Fiber/Gin 经 `clearApplicationGlobals`，CLI 与 hertzcore 因上下文类型与匯出边界不同而直接调用 `ReleaseAllWithObserver` + `ClearAll`；自定义 `FrameStarter` 的健康检查停止由自定义实现负责；见[GlobalManager](../guides/global-manager.md) |
| MySQL / MongoDB | 已接入 | 稳定公共能力 | 公共 API | 不默认创建；由应用 initializer 显式注册 GORM/MySQL、MongoDB v2 client，并决定是否在启动期强制初始化 | client/连接池/模型 locator 的创建、运行、失败/健康检查、关闭四条路径均有明确契约；两者均刻意不实现 `Rebuilder`，`Client` 构造后不再被替换，读侧无需加锁，连接失效恢复交由 database/sql 驱动层与 mongo-driver SDAM 负责，配合 `connMaxLifetime`/`connMaxIdleTime`；`Close` 幂等——Mongo 自带 `closed` 原子标志吸收 `ErrClientDisconnected`，MySQL 依赖 `sql.DB.Close` 的幂等语义（已由 live 测试实测）；模型构造提供 `NewMysqlModelWithError`/`NewMongoModelWithError`，把 key 未注册与类型不符作为可恢复错误返回并保留错误链 | 单元/契约 + race + live integration（建表/collection-写入-读取-清理、并发读写叠加健康检查、`Close` 幂等） | Mongo decimal codec 随 client 构造；连接失败会使需要资源的装配失败；`NewMysqlModel`/`NewMongoModel` 仍为 panic 变体，供 wire 生成的装配链使用，能处理错误的调用方应改用 `*WithError`；关闭在 Fiber/Gin/CLI/hertzcore 四条路径上均生效；见[数据库指南](../guides/database.md) |
| JSON 流量编解码与 JSON 响应 | 已接入 | 稳定公共能力 | 公共 API | Fiber/Gin 的 Std/Sonic provider 与 JSON manager 在默认集合中但需显式装配；`CoreType`、`TrafficCodec` 和 default/fast global key 必须按消费者匹配 | codec 与统一 `RespInfo` JSON 的创建、运行、失败回退四条路径均有明确契约；**没有独立关闭资源**是完整契约而非缺口——`JsonWrapper` 只有 `Marshal`/`Unmarshal`，包内不存在任何 `Close`/`Shutdown`，故无资源所有权需要交接 | 单元/契约 + race（`component/codec/json` 覆盖率 100%，随 `go test ./...` 与 race job 进入 CI） | 全包无 `panic`/`fatal`，装配与运行错误一律以 error 返回；示例注册两个 Sonic 实例并选择 `sonic_json_codec`，`resolveJSONCodec` 是实际执行路径；基础响应、缓存、task payload 与 recovery stack 使用的 codec key 不是统一前置；`gojson.go` 仅有包声明、不是可运行实现（`go-json` 未接线）；见[响应与序列化](../guides/response-and-serialization.md) |
| 本地缓存与 Redis 缓存 | 已接入 | 稳定公共能力 | 公共 API | 不在默认集合；应用通过 GlobalManager 显式注册实例，Redis 还需服务、配置和 `CacheOption` | `cachelocal`、`cacheremote` 的创建、TTL/序列化运行、失败/健康检查和关闭四条路径均有明确契约；Redis 刻意不实现 `Rebuilder`，连接失效恢复交由 go-redis 内建连接池与重试负责，读侧通过 `clientSnapshot()` 加锁访问；`Close` 幂等（`closed` 原子标志），关闭后操作返回 `ErrCacheClosed`；应用关闭时由 `clearApplicationGlobals` 先释放再清空 | 单元/契约 + race + Redis live integration（创建-读写-关闭、并发读写叠加健康检查、就绪探针故障与恢复端到端） | 示例把 Redis、本地缓存、远端缓存别名与 L2 缓存一并列入 `ConfigRequiredGlobalKeys`，`InitializeGlobalRequired` 会在启动期逐个 `Get` 从而真正构造它们，使组合对象关闭排序进入示例路径（注意该初始化失败只记录日志、不中断启动，见 `frame_starter_impl.go:257-263`）；资源释放在 Fiber/Gin/CLI/hertzcore 四条路径上均生效（后两者直接调用容器已匯出的释放入口，不经 `clearApplicationGlobals`）；见[缓存指南](../guides/cache.md) |

## 已接入的核心能力

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Fiber HTTP 内核 | 已接入 | 实验性 | 公共 API | Fiber core provider 在默认集合中，`Default()` 的 `CoreType` 也选择 Fiber，但集合仍需显式装配；应用还需注册 `ApplicationRegister`、`ModuleRegister` 和监听配置 | `CoreWithFiber` 的创建、中间件/监听、运行错误传递和信号关闭均有路径；运行链消费 before/main 位点，关闭链消费 before/main/after 位点，但跨组件资源所有权尚未形成项目级统一契约 | 单元/契约 + HTTP smoke | `example_main` 实际选择 Fiber 并追加中间件、hook 与路由；smoke 只检查 `/example/hello/world`，专项测试覆盖运行返回与关闭位点顺序；见[Web 运行时](../guides/web-runtime.md) |
| Provider / Manager / Location | 已接入 | 实验性 | 公共 API | 默认集合与预定义 location 需显式传给 `WithProviders`、`WithPManagers`；`DefaultProviders()`/`DefaultPManagers(ctx)` 集合是进程级单例，`Add`/`Except` 只应在启动装配期修改；自定义能力还需匹配 type、target、manager/location 和初始化输入 | type、manager 与 location 驱动创建、运行和失败分发；Provider 使用不可变状态值，Manager 缓存初始化结果或错误、避免重复初始化，`GroupExtendReplace` 只替代同一 location 的默认逻辑；没有统一的 provider 关闭契约 | 单元/契约 | 未匹配 provider 会交给默认 manager；`example_main` 展示集合合并而非自动发现；状态 API 近期存在不兼容调整，见[Provider 系统](../concepts/provider-system.md) |
| bootstrap、配置与日志 | 已接入 | 实验性 | 公共 API | `New()` 自动初始化配置与日志单例，不经过 provider 集合；应用需提供可读配置目录，异步日志由配置选择 | 文件/环境配置和 console/轮转文件、同步/异步 writer 的创建、运行、失败有路径；关闭存在 writer 入口，但停止生产者和关闭顺序由应用负责 | 单元/契约 | `Default()` 使用 `./config`、`./logs`，示例改用 `./example_config`、`./example_main/logs`；见[配置指南](../guides/configuration.md)、[日志指南](../guides/logging.md) |
| panic recovery 与错误响应 | 已接入 | 实验性 | 公共 API | Fiber/Gin recovery provider 与 manager 在默认集合中，需随所选内核显式装配 | 两种 recovery 和核心错误中间件的创建、运行、失败响应有路径；没有独立关闭资源，装配失败仍可能 panic 或 fatal | 单元/契约 | 调试信息受 recovery 配置控制，生产环境应关闭详细输出；示例的 `debugMode` 只适合本地演示；见[错误与恢复](../guides/errors-and-recovery.md) |
| 参数验证 | 已接入 | 实验性 | 公共 API | Web `AppContext` 自动调用 `validate.NewWrap(cfg)`；CLI 必须自行构造、注册并持有 wrapper | en、zh-cn、zh-tw、错误映射及自定义 tag/translator 的创建、运行、失败映射有路径；没有独立关闭资源，可变注册只适合启动期 | 单元/契约 | Web 未配置语言时只注册 en，`CmdContext.GetValidateWrap()` 固定返回 nil；示例还追加日语、韩语和自定义 tag；见[验证指南](../guides/validation.md) |

## 实验性或存在明显限制的公共能力

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Gin HTTP 内核 | 已接入 | 实验性 | 公共 API | Gin core provider 在默认集合中但 `Default()` 仍选择 Fiber；启用时设置 `CoreType` 为 `gin` 并显式装配 Gin codec、recovery、中间件和路由 provider/manager；原生诊断自动接入框架日志器 | `CoreWithGin` 的创建、运行错误传递和信号关闭均有路径；路由 location 已处理时不会再次执行模块默认注册；日志 bridge 在引擎创建前固定稳定转发入口并取得独占 lease，初始化失败、server 返回或 shutdown 时幂等停用 owner，无 owner 时按行为回退到首次捕获的 Gin 输出；有效证书可填充 `TLSConfig` 并选择 TLS serve，缺失路径仍保留 HTTP 路径 | 单元/契约 + race | adapter 与 core 测试覆盖级别/字段、稳定入口与回退、安装冲突、并发 release、mode fallback、server error logger、重复路由防护、单条访问记录及各退出路径，另有 loopback listener 驱动的真实 TLS 握手与 `Shutdown` 回归；运行期不写回 Gin 全局变量以避免与无同步读取竞争，多 Gin engine 仍共享一个框架日志器，逐 engine 原生诊断隔离不受支持，Gin 保持实验性；见[Web 运行时](../guides/web-runtime.md) |
| MsgPack / Protobuf 响应 | 已接入 | 实验性 | 公共 API | 两种 MIME provider 与响应 manager 在默认集合中但需显式装配；还需启用 `EnableBinaryProtocolSupport` 并命中 `application/msgpack` 或 `application/x-protobuf` | 两种 HTTP body 实现的创建、运行、失败回退有路径；没有独立关闭资源 | 单元/契约 | 未命中或加载失败时回退 JSON，协商只取首个媒体类型；这是 HTTP body 编码而非通用 RPC；见[响应与序列化](../guides/response-and-serialization.md) |
| L2 缓存与 Redis 保护机制 | 已接入 | 实验性 | 公共 API | 不默认创建；应用显式构造 local、Redis、L2 并选择回填、同步/异步写、singleflight、Bloom filter 和 circuit breaker | 创建、组合运行和失败保护有代码路径；关闭已具备原子幂等、关闭后拒绝操作、子缓存关闭与错误聚合；实现 `globalmanager.CompositeClosable` 与 `CompositeOwnershipInspector`，向容器声明它持有 local/remote 的关闭责任，使 `ReleaseAll` 先释放它、再释放子缓存各自的 key（先 `<-drain` 排空再关子缓存，避免排空期间打在已关闭的 Redis 上）；异步 flush 所有权仍不完整 | 单元/契约；未验证外部 live integration | singleflight 未形成完整 loader 合并，Bloom/breaker miss 语义不一致；L2 `Wait` 不等待 ants pool 异步任务，现有 hermetic 测试不证明 Redis live 行为；见[缓存指南](../guides/cache.md) |
| 异步任务 | 已接入 | 实验性 | 公共 API | 无默认 task register；应用需提供 Redis、initializer、handler、`TaskRegister` 并启用 `application.task.enableServer` | asynq `TaskWorker`/`TaskDispatcher` 的创建、同步/异步运行和失败记录有路径；统一关闭、dispatcher 回收不完整 | 单元/契约 + live integration（唯一 task 入队、worker 消费、优雅关闭） | 异步启动内部错误只记录，示例依赖外部 Redis；live 测试覆盖单个 task 的入队-消费-关闭路径，不覆盖高并发或故障注入场景；见[异步任务指南](../guides/background-tasks.md) |
| CLI | 已接入 | 实验性 | 公共 API | 不属于 Web 默认集合；应用单独创建 `CmdContext`、应用注册器和基于 urfave/cli 的 `CMDLineApplication` | 创建、命令注册、运行与资源关闭有路径；`AppCoreRun` 失败传播与健康检查循环仍不完整 | 单元/契约 | 健康检查只执行一次，`RunCommandStarter` 丢弃返回值；关闭段以 `defer` 释放全局对象（命令返回错误时同样执行），但因 `ICommandContext.GetStarterApp()` 返回 `CommandStarter` 而非 `ApplicationStarter`，它不复用 `clearApplicationGlobals`，也不停止健康检查——CLI 的健康检查本就是一次性的，没有需要停止的后台 ticker；见[命令行指南](../guides/command-line.md) |
| 扩展运行位点与关闭链 | 已接入 | 实验性 | 公共 API | 应用可把自定义 manager 显式绑定到 server run 的 before/main location，以及 shutdown 的 before/main/after location；普通 manager 先加载，`GroupExtendReplace` manager 只替代同一 location 的默认逻辑 | `RunServer` 会收集运行与关闭管理器，核心运行结果无论成功、失败或 panic 都进入协调通道，且运行分支失败（监听绑定失败、运行 goroutine panic）时也会释放全局对象——该分支从不调用 `Shutdown`，此前是唯一没有回收的退出路径；信号触发 shutdown，Fiber/Gin 的运行链消费 before/main 位点，关闭链消费 before/main/after 位点，且两者都保证 after 位点在全局对象释放与日志器关闭**之前**执行，因此该位点的 provider（仓库内即 pprof 关闭提供者）仍可正常记录日志；尚无统一的 provider 关闭接口和跨组件资源所有权契约 | 单元/契约 | 专项测试覆盖正常返回、信号关闭、同位点替代、不同位点互不抑制、shutdown before/after 执行，以及 after 位点早于日志器关闭的顺序回归（经注入 `io.WriteCloser` 观测）；`ServerRunAfter` 仍未被默认实现消费，真实进程信号与外部资源组合关闭仍未进入 smoke；见[Web 启动生命周期](../concepts/startup-lifecycle.md) |

## 内部工具

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Dig 容器 | 已接入 | 实验性 | 内部工具 | `CmdContext` 持有单例容器，CLI ORM 示例显式用于启动和命令装配 | 创建、运行、失败返回和 reset 有路径；没有通用资源关闭编排 | 单元/契约 | Web 运行时不推荐使用，`ResetDigContainer` 非并发安全；见[组件目录](components.md)、[命令行指南](../guides/command-line.md) |
| jsonconvert | 已接入 | 实验性 | 内部工具 | recovery 错误处理按调用借用 wrapper，用后显式 `Release` | 创建、运行、失败回退、释放均有路径；单个 wrapper 不支持并发复用 | 单元/契约 | 主要把任意数据转换为 JSON 或字符串；见[组件目录](components.md)、[错误与恢复](../guides/errors-and-recovery.md) |
| mongodecimal | 已接入 | 实验性 | 内部工具 | 随 `dbmongo.NewClient` 构造自动注册到 BSON registry，不是独立服务 | 创建和运行随 Mongo client；失败与关闭也依附 client，没有独立生命周期 | 无专项测试 | 只服务 `decimal.Decimal` BSON 编解码，外部 Mongo live 行为未验证；见[组件目录](components.md)、[数据库指南](../guides/database.md) |
| logging writer 与框架 adaptor | 已接入 | 实验性 | 内部工具 | bootstrap 按配置选择同步/异步 writer；task logger adaptor 由示例任务显式装配；选择 Gin core 时自动安装 Gin 日志 adaptor | writer 和 adaptor 都有创建、运行与失败入口；Gin lease 有幂等 owner 停用与原始行为回退路径且不关闭框架日志器，稳定入口只在首次有效安装时写入，异步 writer 的生产者停止与关闭顺序仍未统一编排 | 单元/契约 + race | Gin adapter 的级别/字段、并发 release、稳定全局入口和回退已有测试，但使用进程级 hook 且多个 engine 共享 logger；异步 writer 仍可能丢日志，task adaptor 目前主要服务示例；见[组件目录](components.md)、[日志指南](../guides/logging.md)、[异步任务指南](../guides/background-tasks.md) |

## 判断依据

本页以 `default.go`、`boot.go`、具体 core starter、`component/`、`plugins/`、`response/pb` 和三个示例目录的当前调用路径为准：类型、目录或配置键不算接入；默认集合不算自动启用；缺失 provider、manager、配置或 global 时可能返回错误、记录 fatal 或 panic；后台 goroutine、连接池、缓存、writer 和 task worker 的资源所有权仍须由应用明确。`example_main`、`example_config` 与 `example_application` 只证明调用路径存在，不构成兼容性或成熟度承诺。

复核入口包括 `Default()`、`New()`、`RunServer()`、`RunApplicationStarter()`、默认 provider/manager 集合、预定义 location、Fiber/Gin/response/recovery 的启动实现，以及 GlobalManager、缓存、数据库、任务和 CLI 的构造与关闭方法。后续指南若与本页冲突，应先按当前源码复核调用路径，再更新本页。

## 稳定公共能力晋级条件

能力只有同时满足以下条件，才能从“实验性”晋级为“稳定公共能力”：

1. 创建、运行、失败、关闭四条路径都有明确契约。
2. 公开 API 有兼容承诺，默认与显式启用方式无歧义。**（v2.0.0 前不作为晋级门槛，见下方说明）**
3. 单元/契约测试进入 CI；并发能力通过 race，外部依赖通过可重复 live integration test。
4. 不依靠 panic 或 fatal 表达可恢复的装配或运行错误。
5. 文档示例确实执行该路径，并明确资源所有权和限制。

### 关于条件 2 的当前豁免

本仓库仍处在积极开发期，不承诺生产可用，并明确允许 breaking change。全面重构与
稳定版本将在 v2.0.0 交付，兼容承诺届时统一给出。因此在 v2.0.0 之前，**条件 2 不
作为晋级门槛**：一项能力满足条件 1、3、4、5 即可标注为“稳定公共能力”，其含义是
“该能力的生命周期契约、验证证据与文档已闭合”，而非“其导出符号已冻结”。

这条豁免只放宽兼容承诺，不放宽其余四条。晋级后仍可能发生 breaking rename 或签名
调整；此类调整须在 CHANGELOG 与本页同步记录。v2.0.0 起本豁免失效，条件 2 恢复为
硬性门槛。
