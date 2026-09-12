# 功能状态

本文按当前源码记录 FiberHouse 的能力边界，并作为逐能力状态、启用方式、生命周期、验证证据和限制的唯一详细事实源。这里的“默认”分为两层：`Default()` 只生成默认 `BootConfig` 并调用 `New()`；`DefaultProviders()` 与 `DefaultPManagers(ctx)` 只是提供默认集合，应用仍须通过 `WithProviders`、`WithPManagers` 把集合交给 `FiberHouse`。因此“源码已实现”不等于“调用 `Default()` 后自动启用”。

## 如何理解状态

- **实现阶段**：`已实现`表示有可执行实现但未证明进入框架或示例主链；`已接入`表示存在明确入口和可达运行路径。
- **支持级别**：`实验性`表示兼容、生命周期、错误、并发或验证仍未达到晋级要求；`稳定公共能力`必须满足本文末尾全部晋级条件（条件 2 在 v2.0.0 前豁免，见文末说明）。表中共 15 项公共 API 能力（另有 4 项内部工具不参与晋级），**15 项全部已完成晋级**，零 `deferred-v2.0.0`。
- **API 受众**：`公共 API`面向使用方；`内部工具`描述设计受众，不覆盖 Go package 实际可导入的兼容事实。
- **关于已移除的 `deferred-v2.0.0` 标记**：本页曾用该标记把 8 项能力的晋级推迟到 v2.0.0，理由是三个共同阻塞项——`AppState` 语义重构、统一 provider 关闭契约、无替代入口的装配期 panic。**三者现已全部交付**：`AppPhase` 有序状态机取代布尔 `AppState`（`RegisterAppState`/`GetAppState` 降为 `Deprecated` 转接层）、`ClosableProvider` + `CloseProviders` 形成统一关闭契约并由关闭链消费、53 处 C 类装配 panic 逐条裁决完毕（见[panic 裁决表](panic-adjudication.md)）。标记连同「后续复核直接跳过这些行」的规则一并移除——那条规则正是它一度让过期结论自我延续的原因。**本页不再有任何以版本号为由推迟的晋级判定；每一行的支持级别都必须由当前源码与测试证据直接支撑。**

表中的“默认集合”表示对象是否由 `DefaultProviders()` 或 `DefaultPManagers(ctx)` 收集，不表示 `Default()` 自动装配。状态以完整能力为单位，而不是以单个导出符号为单位；阅读每一行时必须同时检查启用方式，以及创建、运行、失败、关闭四段生命周期。

当前工作树的 hermetic 基线是 `go test ./... -count=1`、`go vet ./...` 和 `go test -race ./... -count=1`。

CI 的 `smoke` job 还构建并启动示例、检查一条 HTTP 路径，并定向执行带 `liveintegration` build tag 的回归测试（`go test -tags=liveintegration ./... -run 'TestLive_' -v -count=1`），覆盖 Redis cache、asynq、MySQL、MongoDB 各自的创建-读写-关闭路径。

live 测试现已覆盖：三个封装各自的并发读写与并发健康检查同时进行（N 个读写 goroutine 叠加持续 `IsHealthy`，断言无错误、无丢行/丢文档、健康检查确实执行且未误报）、MySQL 与 MongoDB 的 `Close` 幂等，以及就绪探针对真实依赖故障的端到端反应（依赖可达 200 → TCP 层切断 503 并点名不健康 key → 恢复后重新 200，全程框架不做任何重建）。上述用例均在 `-race` 下通过。

asynq 任务的 live 覆盖包含并发入队-消费（断言不丢任务、不重复消费）、handler 持续失败的
重试-归档路径（断言 `Retried`、`LastErr` 与 `Config.ErrorHandler` 上报）以及 `SkipRetry`
立即归档；`TaskDispatcher.Close` 与 `TaskWorker.Shutdown` 已补齐，统一关闭与 dispatcher
回收经容器 `ReleaseAll` 闭合。L2 缓存另有 Redis live 覆盖：异步双写往返、关闭前排空在途
写入、关闭后拒绝操作与重复关闭幂等。
关闭路径的资源释放现已覆盖四条：Fiber 与 Gin 走 `clearApplicationGlobals`，CLI（`RunCommandStarter`）与 `example_application/hertzcore` 因上下文类型与匯出边界不同，各自直接调用容器已匯出的 `ReleaseAllWithObserver` + `ClearAll`。四条路径的释放失败均按 key 上报且不中断回收。

## 稳定公共能力

以下能力满足文末全部晋级条件（条件 2 在 v2.0.0 前豁免）：四段生命周期契约明确、
测试与 race 进入 CI、外部依赖有可重复 live integration 覆盖、调用方本可处理的错误
不以 panic 表达（启动期不可恢复的装配错误允许 panic，但须有返回 error 的替代入口，
见文末条件 4 的适用边界）、示例确实执行对应路径。晋级不冻结导出符号，见文末豁免说明。

晋级过程与逐条证据见
[GlobalManager 稳定性晋级设计](../superpowers/specs/2026-09-08-globalmanager-stability-promotion-design.md)。

JSON 流量编解码逐条对照：条件 1，四条路径有契约，且「没有独立关闭资源」是完整契约——
`JsonWrapper` 只暴露 `Marshal`/`Unmarshal`，包内无任何 `Close`/`Shutdown`；条件 3，
`component/codec/json` 覆盖率 100%，随 `go test ./...` 与独立 race job 进入 CI，且不依赖外部
服务，无需 live integration；条件 4，全包不含 `panic`/`fatal`，错误一律以 error 返回；条件 5，
[响应与序列化](../guides/response-and-serialization.md)描述该路径，示例经 `resolveJSONCodec`
实际执行。条件 2 依 v2.0.0 前豁免不作门槛。

参数验证逐条对照：条件 1，创建、运行、失败、关闭四条路径有契约，且「没有独立关闭资源」是完整
契约——包内不存在任何 `Close`/`Shutdown`；条件 3，单元/契约测试随 `go test ./...` 与独立 race job
进入 CI，含「失败 initializer 被跳过而非 panic」的红线回归，且不依赖外部服务故无需 live；条件 4，
`ValidateInitializer` 由 `func() ValidateRegister` 改为 `func() (ValidateRegister, error)`，
框架三个语言包与示例的日语、韩语验证器均以 error 返回翻译注册失败，全包已无 `panic`/`fatal`；
条件 5，[验证指南](../guides/validation.md)描述该路径，示例经 `ConfigCustomValidateInitializers`
实际执行。条件 2 依 v2.0.0 前豁免不作门槛。**该晋级含 breaking change**：`ValidateInitializer`
签名变更，自定义语言验证器需同步改为返回 `(ValidateRegister, error)`。

MsgPack / Protobuf 响应逐条对照：条件 1，两种实现的创建、运行、失败回退有契约，对象池所有权
已对称（四个发送方法均以 `defer r.Release()` 开头），由 `response_binary_ownership_test.go` 钉住；
条件 3，Fiber 与 Gin 真实内核的协商往返测试解码断言 body，随 `go test ./...` 与 race job 进入 CI，
无外部依赖故不需 live；条件 4，`NewRespInfoPManagerOnceWithError` 提供返回 error 的替代入口，
原入口保留 panic 属 `Must*` 惯例；条件 5，示例新增 `GET /examples/binary`（Hertz，example_main
当前默认核心）实际发送二进制响应，CI smoke 断言三条路径的 Content-Type 与 `data` 完整性。

Fiber / Gin HTTP 内核逐条对照：条件 1，创建、中间件与监听、运行错误传递、信号关闭四条路径有
契约，跨组件资源所有权由 `ClosableProvider` + `CloseProviders` 统一；条件 3，两者均有单元/契约
测试进 CI，Gin 另有 race 与 loopback listener 驱动的真实 TLS 握手/`Shutdown` 回归；条件 4，
`CoreWithFiber` 新建 `initErr` 状态机（8 处守卫），`AppCoreRun`/`Shutdown` 返回该错误，
fail-fast 行为保留在 `InitCoreAppMust`，Gin 的两处绕开 `initErr` 的 panic 已对齐其自身机制；
条件 5，`example_main` 实际装配两者之一并执行启动、路由与关闭链。**已知限制**：多个 Gin engine
共享一个框架日志器，逐 engine 原生诊断隔离不受支持。

Provider / Manager / Location 与扩展运行位点逐条对照：条件 1，`ClosableProvider` opt-in 契约 +
`IProviderManager.CloseProviders` 形成统一关闭入口，`List()` 按注册序返回以保证逆序关闭确定
（不依赖 map 遍历），重复绑定经 `SetOrBindToLocationWithError` 显式返回错误；条件 3，单元/契约
测试覆盖位点顺序、同位点替代、shutdown before/after 执行与 after 早于日志器关闭的顺序回归；
条件 4，装配期 panic 均有 `*WithError` 姊妹（见[panic 裁决表](panic-adjudication.md)）；
条件 5 见下表该行——**该位点的示例证据仅在 test profile 下执行**，已在「限制」列明确记载。

bootstrap、配置与日志逐条对照：条件 1，关闭顺序已定义——两个异步 writer 各自以 CAS + `closeDone`
幂等，且刻意不在 `Close` 内 `Flush`，由 consume goroutine 排空后完成最终 `Flush` 才退出，避免
并发写 `bufio.Writer`；`LoggerWrap.Close` 具 `sync.Once` 守卫并缓存首次结果（移除守卫后底层关闭
次数变为 5/2/16，变异验证有效）；条件 3，bootstrap 与 logging 的单元/契约测试进 CI 并过 race；
条件 4，`NewWriterAsyncWithError`、`NewConfigOnceWithError`、`LoadDefaultWithError`/`LoadYamlWithError`
提供 error 入口；条件 5，[配置指南](../guides/configuration.md)与[日志指南](../guides/logging.md)
描述该路径，示例经 `./example_config`、`./example_main/logs` 实际执行。

panic recovery 与错误响应逐条对照：条件 1，两种 recovery 与核心错误中间件的创建、运行、失败响应
有契约，「没有独立关闭资源」是完整契约；条件 3，单元/契约测试进 CI；条件 4，
`RecoverMiddlewareWithError` 并入 `IErrorHandler` 介面提供 error 入口，且恢复路径上的运行期 panic
（`TraceID` 上下文类型不符、Gin 错误处理器自身失败）已改为降级而非再抛——运行期 panic 始终不满足
条件 4；条件 5，[错误与恢复](../guides/errors-and-recovery.md)描述该路径，示例实际执行。

异步任务逐条对照：条件 1，`TaskDispatcher.Close` 以 atomic 幂等守卫委派 `asynq.Client.Close()`、
关闭后 `Enqueue` 返回错误且刻意不持有 Redis 连接所有权，`TaskWorker.Shutdown` 委派
`asynq.Server.Shutdown()`，两者实作 `globalmanager.Closable` 并经容器 `ReleaseAll` 回收，
`RunAsync` 不再恒返回 nil；条件 3，live integration 覆盖入队-消费-优雅关闭、并发入队消费不丢不重、
handler 持续失败的重试-归档与 `Config.ErrorHandler` 上报、`SkipRetry` 立即归档；条件 4，装配期
panic 已有 error 入口；条件 5，示例模块注册 task handler 并经 `application.task.enableServer` 执行。

L2 缓存与 Redis 保护机制逐条对照：条件 1，关闭具原子幂等、关闭后拒绝操作、子缓存关闭与错误聚合，
实作 `CompositeClosable` 与 `CompositeOwnershipInspector` 使 `ReleaseAll` 先释放 L2 再释放子缓存；
异步写入所有权完整——`admit()` 在 `Submit` 前于锁内递增 `pending` 并在 0→1 时建立 `drain`，任务体
以 `defer complete()` 收尾且提交失败补偿递减，故 `Flush`/`Wait` 覆盖「排队中但尚未执行」的任务
（`level2_wait_contract_test.go` 以 `Running()==1` vs `pending==2` 对照钉住）；条件 3，单元/契约 +
race + Redis live integration（异步双写往返、关闭前排空在途写、关闭后拒绝操作与重复关闭幂等）；
条件 4，`NewLevel2CacheWithError` 返回构造错误；条件 5，[缓存指南](../guides/cache.md)描述该路径，
示例把 L2 列入 `ConfigRequiredGlobalKeys` 使组合对象关闭排序进入示例路径。

CLI 逐条对照：条件 1，创建、命令注册、运行与资源关闭四条路径有契约——`AppCoreRun` 的错误经
`RunCommandStarterWithError` 返回给调用方，`RunCommandStarter` 保持 void 签名但把错误写入 CLI
日志器（不再静默丢弃），关闭段以 `defer` 释放全局对象；健康检查是一次性的，「没有需要停止的
东西」本身即完整契约；条件 3，单元/契约测试进 CI；条件 4，三个 `Register*` 与
`NewFrameCmdApplication` 均有 `*WithError` 姊妹；条件 5，[命令行指南](../guides/command-line.md)
描述该路径，`example_application/command` 实际执行。

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| GlobalManager | 已接入 | 稳定公共能力 | 公共 API | `New()` 获取进程级单例；应用显式注册具体 initializer，且应在启动期完成 | 注册、懒初始化、健康检查、重建、释放、清空四段路径均有明确契约；`Rebuild`/`Release` 在同一 entry 上 fail-fast 互斥，冲突返回包装 `ErrMaintenanceInProgress` 的错误，可 `errors.Is` 分类重试；`Release` 对已初始化对象一律重置（非 `Closable` 也生效），`Close` 失败保留实例并返回错误，未初始化 entry 为幂等空操作；`ReleaseAll` 按嵌套深度由外向内释放组合对象（`CompositeClosable` / `CompositeOwnershipInspector`），失败经 `ReleaseAllWithObserver` 逐 key 上报；`ClearAll` 只删除条目、不关闭资源，须在 `ReleaseAll` 之后调用；初始化失败不会发布 typed-nil 实例；删除不取消已开始的 `Get` 初始化；健康检查具备取消、等待退出与重复停止语义 | 单元/契约 + race（含 200/300 轮关闭顺序、嵌套排序、持有成环终止、包外 sentinel 分类） | `Get`/`Rebuild`/`Release` 的完整并发状态机与调用方已取得引用的存活期契约仍未闭合，`Rebuild` 不会安全退役旧实例；持有关系成环时只保证终止、不保证顺序；`ReleaseAll` 现由四条关闭路径调用——Fiber/Gin 经 `clearApplicationGlobals`，CLI 与 hertzcore 因上下文类型与匯出边界不同而直接调用 `ReleaseAllWithObserver` + `ClearAll`；自定义 `FrameStarter` 的健康检查停止由自定义实现负责；见[GlobalManager](../guides/global-manager.md) |
| MySQL / MongoDB | 已接入 | 稳定公共能力 | 公共 API | 不默认创建；由应用 initializer 显式注册 GORM/MySQL、MongoDB v2 client，并决定是否在启动期强制初始化 | client/连接池/模型 locator 的创建、运行、失败/健康检查、关闭四条路径均有明确契约；两者均刻意不实现 `Rebuilder`，`Client` 构造后不再被替换，读侧无需加锁，连接失效恢复交由 database/sql 驱动层与 mongo-driver SDAM 负责，配合 `connMaxLifetime`/`connMaxIdleTime`；`Close` 幂等——Mongo 自带 `closed` 原子标志吸收 `ErrClientDisconnected`，MySQL 依赖 `sql.DB.Close` 的幂等语义（已由 live 测试实测）；模型构造提供 `NewMysqlModelWithError`/`NewMongoModelWithError`，把 key 未注册与类型不符作为可恢复错误返回并保留错误链 | 单元/契约 + race + live integration（建表/collection-写入-读取-清理、并发读写叠加健康检查、`Close` 幂等） | Mongo decimal codec 随 client 构造；连接失败会使需要资源的装配失败；`NewMysqlModel`/`NewMongoModel` 仍为 panic 变体，供 wire 生成的装配链使用，能处理错误的调用方应改用 `*WithError`；关闭在 Fiber/Gin/CLI/hertzcore 四条路径上均生效；见[数据库指南](../guides/database.md) |
| JSON 流量编解码与 JSON 响应 | 已接入 | 稳定公共能力 | 公共 API | Fiber/Gin 的 Std/Sonic provider 与 JSON manager 在默认集合中但需显式装配；`CoreType`、`TrafficCodec` 和 default/fast global key 必须按消费者匹配 | codec 与统一 `RespInfo` JSON 的创建、运行、失败回退四条路径均有明确契约；**没有独立关闭资源**是完整契约而非缺口——`JsonWrapper` 只有 `Marshal`/`Unmarshal`，包内不存在任何 `Close`/`Shutdown`，故无资源所有权需要交接 | 单元/契约 + race（`component/codec/json` 覆盖率 100%，随 `go test ./...` 与 race job 进入 CI） | 全包无 `panic`/`fatal`，装配与运行错误一律以 error 返回；示例注册两个 Sonic 实例并选择 `sonic_json_codec`，`resolveJSONCodec` 是实际执行路径；基础响应、缓存、task payload 与 recovery stack 使用的 codec key 不是统一前置；`gojson.go` 仅有包声明、不是可运行实现（`go-json` 未接线）；见[响应与序列化](../guides/response-and-serialization.md) |
| 参数验证 | 已接入 | 稳定公共能力 | 公共 API | Web `AppContext` 自动调用 `validate.NewWrap(cfg)`；需要感知语言包初始化失败的调用方改用 `NewWrapWithErrors(cfg)`；CLI 必须自行构造、注册并持有 wrapper | en、zh-cn、zh-tw、错误映射及自定义 tag/translator 的创建、运行、失败映射四条路径均有明确契约；`ValidateInitializer` 签名为 `func() (ValidateRegister, error)`，语言包翻译注册失败作为可恢复错误返回，FrameStarter 记录日志并跳过该语言、不中断启动；`NewWrapWithErrors` 始终返回非 nil `*Wrap`，失败语言缺席但不影响其余语言与英文回退；**没有独立关闭资源**是完整契约——包内不存在任何 `Close`/`Shutdown`，可变注册只适合启动期 | 单元/契约 + race（含失败 initializer 被跳过而非 panic 的红线回归） | 全包无 `panic`/`fatal`；Web 未配置语言时只注册 en，`CmdContext.GetValidateWrap()` 固定返回 nil；示例还追加日语、韩语和自定义 tag，两者同样以 error 返回失败；见[验证指南](../guides/validation.md) |
| 本地缓存与 Redis 缓存 | 已接入 | 稳定公共能力 | 公共 API | 不在默认集合；应用通过 GlobalManager 显式注册实例，Redis 还需服务、配置和 `CacheOption` | `cachelocal`、`cacheremote` 的创建、TTL/序列化运行、失败/健康检查和关闭四条路径均有明确契约；Redis 刻意不实现 `Rebuilder`，连接失效恢复交由 go-redis 内建连接池与重试负责，读侧通过 `clientSnapshot()` 加锁访问；`Close` 幂等（`closed` 原子标志），关闭后操作返回 `ErrCacheClosed`；应用关闭时由 `clearApplicationGlobals` 先释放再清空 | 单元/契约 + race + Redis live integration（创建-读写-关闭、并发读写叠加健康检查、就绪探针故障与恢复端到端） | 示例把 Redis、本地缓存、远端缓存别名与 L2 缓存一并列入 `ConfigRequiredGlobalKeys`，`InitializeGlobalRequired` 会在启动期逐个 `Get` 从而真正构造它们，使组合对象关闭排序进入示例路径（注意该初始化失败只记录日志、不中断启动，见 `frame_starter_impl.go:257-263`）；资源释放在 Fiber/Gin/CLI/hertzcore 四条路径上均生效（后两者直接调用容器已匯出的释放入口，不经 `clearApplicationGlobals`）；见[缓存指南](../guides/cache.md) |

## 已接入的核心能力

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Fiber HTTP 内核 | 已接入 | 稳定公共能力 | 公共 API | Fiber core provider 在默认集合中，`Default()` 的 `CoreType` 也选择 Fiber，但集合仍需显式装配；应用还需注册 `ApplicationRegister`、`ModuleRegister` 和监听配置 | `CoreWithFiber` 的创建、中间件/监听、运行错误传递和信号关闭均有路径；运行链消费 before/main 位点，关闭链消费 before/main/after 位点；跨组件资源所有权已由 `ClosableProvider` + `IProviderManager.CloseProviders` 形成统一契约，关闭链在 `ServerShutdownAfter` 之后、全局对象释放之前按注册逆序回收（`provider_close_chain.go`） | 单元/契约 + HTTP smoke | Fiber 仍是 `Default()` 的 `CoreType`，但 `example_main` 当前选择的是 Hertz，因此 CI smoke 启动的是 Hertz 内核，Fiber 路径不被 smoke 覆盖（smoke 探测 `/health/livez` 与 `/examples/binary`，两者均由 Hertz 模块提供）；专项测试覆盖运行返回与关闭位点顺序；见[Web 运行时](../guides/web-runtime.md) |
| Provider / Manager / Location | 已接入 | 稳定公共能力 | 公共 API | 默认集合与预定义 location 需显式传给 `WithProviders`、`WithPManagers`；`DefaultProviders()`/`DefaultPManagers(ctx)` 集合是进程级单例，`Add`/`Except` 只应在启动装配期修改；自定义能力还需匹配 type、target、manager/location 和初始化输入 | type、manager 与 location 驱动创建、运行和失败分发；Provider 使用不可变状态值，Manager 缓存初始化结果或错误、避免重复初始化，`GroupExtendReplace` 只替代同一 location 的默认逻辑；`ClosableProvider` opt-in 契约与 `CloseProviders` 已形成统一的 provider 关闭入口，`List()` 按注册序返回（不依赖 map 遍历）以保证逆序关闭确定，重复绑定经 `SetOrBindToLocationWithError` 显式返回错误而非静默吞掉 | 单元/契约 | 未匹配 provider 会交给默认 manager；`example_main` 展示集合合并而非自动发现；状态 API 近期存在不兼容调整，见[Provider 系统](../concepts/provider-system.md) |
| bootstrap、配置与日志 | 已接入 | 稳定公共能力 | 公共 API | `New()` 自动初始化配置与日志单例，不经过 provider 集合；应用需提供可读配置目录，异步日志由配置选择 | 文件/环境配置和 console/轮转文件、同步/异步 writer 的创建、运行、失败有路径；关闭顺序已明确定义：两个异步 writer 各自以 CAS + `closeDone` 幂等，且刻意不在 `Close` 内 `Flush`——由 consume goroutine 排空 channel/diode 后完成最终 `Flush` 才退出，避免并发写 `bufio.Writer`；`LoggerWrap.Close` 具备 `sync.Once` 幂等守卫并缓存首次结果 | 单元/契约 | `Default()` 使用 `./config`、`./logs`，示例改用 `./example_config`、`./example_main/logs`；见[配置指南](../guides/configuration.md)、[日志指南](../guides/logging.md) |
| panic recovery 与错误响应 | 已接入 | 稳定公共能力 | 公共 API | Fiber/Gin recovery provider 与 manager 在默认集合中，需随所选内核显式装配 | 两种 recovery 和核心错误中间件的创建、运行、失败响应有路径；**没有独立关闭资源**是完整契约而非缺口——recovery 不持有需要交接的资源；装配失败已有返回 error 的替代入口（`RecoverMiddlewareWithError`，并入 `IErrorHandler` 介面），原入口保留 panic 属 `Must*` 惯例 | 单元/契约 | 调试信息受 recovery 配置控制，生产环境应关闭详细输出；示例的 `debugMode` 只适合本地演示；见[错误与恢复](../guides/errors-and-recovery.md) |

## 实验性或存在明显限制的公共能力

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Gin HTTP 内核 | 已接入 | 稳定公共能力 | 公共 API | Gin core provider 在默认集合中但 `Default()` 仍选择 Fiber；启用时设置 `CoreType` 为 `gin` 并显式装配 Gin codec、recovery、中间件和路由 provider/manager；原生诊断自动接入框架日志器 | `CoreWithGin` 的创建、运行错误传递和信号关闭均有路径；路由 location 已处理时不会再次执行模块默认注册；日志 bridge 在引擎创建前固定稳定转发入口并取得独占 lease，初始化失败、server 返回或 shutdown 时幂等停用 owner，无 owner 时按行为回退到首次捕获的 Gin 输出；有效证书可填充 `TLSConfig` 并选择 TLS serve，缺失路径仍保留 HTTP 路径 | 单元/契约 + race | adapter 与 core 测试覆盖级别/字段、稳定入口与回退、安装冲突、并发 release、mode fallback、server error logger、重复路由防护、单条访问记录及各退出路径，另有 loopback listener 驱动的真实 TLS 握手与 `Shutdown` 回归；运行期不写回 Gin 全局变量以避免与无同步读取竞争，多 Gin engine 仍共享一个框架日志器，逐 engine 原生诊断隔离不受支持，Gin 保持实验性；见[Web 运行时](../guides/web-runtime.md) |
| MsgPack / Protobuf 响应 | 已接入 | 稳定公共能力 | 公共 API | 两种 MIME provider 与响应 manager 在默认集合中但需显式装配；还需启用 `EnableBinaryProtocolSupport` 并命中 `application/msgpack` 或 `application/x-protobuf` | 两种 HTTP body 实现的创建、运行、失败回退有路径；没有独立关闭资源；两种实现的对象池所有权已对称——`SendWithCtx` 与 `JsonWithCtx` 四个方法都以 `defer r.Release()` 开头，由 `response_binary_ownership_test.go` 的对称性用例钉住 | 单元/契约（含 Fiber 与 Gin 真实内核的协商往返：首个 Accept 值优先、`Content-Type` 优先于 `Accept`、未知 MIME 与通配符回退 JSON，均解码断言 body）+ CI smoke（`GET /examples/binary` 三条路径断言 Content-Type 与可解码 body） | 四项晋级条件均已闭合。条件 4：`NewRespInfoPManagerOnceWithError` 提供返回 error 的替代入口，`NewRespInfoPManagerOnce` 的 panic 因此符合 `Must*`/`*WithError` 惯例；失败结果同样被 `sync.Once` 缓存。条件 3：**无外部依赖故不需要 live integration**，协商已由 Fiber/Gin 真实内核往返测试覆盖。条件 5：示例模块新增 `GET /examples/binary`（Hertz，example_main 当前默认核心）实际发送二进制响应，CI smoke job 断言 MsgPack、Protobuf 与 JSON 回退三条路径的 Content-Type 与 data 完整性。**已知限制**：Protobuf 分支经 `structpb.NewValue` 转换 data，该函式只接受 nil、bool、数值、string、`[]byte`、`map[string]interface{}` 与 `[]interface{}`；`RespInfoPB.Reset` 又吞掉转换错误（两处 `if err == nil` 均无 else），故传入自定义 struct 会使 data 静默丢失。示例 handler 因此以 map 构造 payload。未命中或加载失败时回退 JSON，协商只取首个媒体类型；这是 HTTP body 编码而非通用 RPC；见[响应与序列化](../guides/response-and-serialization.md) |
| L2 缓存与 Redis 保护机制 | 已接入 | 稳定公共能力 | 公共 API | 不默认创建；应用显式构造 local、Redis、L2 并选择回填、同步/异步写、singleflight、Bloom filter 和 circuit breaker | 创建、组合运行和失败保护有代码路径；关闭已具备原子幂等、关闭后拒绝操作、子缓存关闭与错误聚合；实现 `globalmanager.CompositeClosable` 与 `CompositeOwnershipInspector`，向容器声明它持有 local/remote 的关闭责任，使 `ReleaseAll` 先释放它、再释放子缓存各自的 key（先 `<-drain` 排空再关子缓存，避免排空期间打在已关闭的 Redis 上）；异步写入所有权完整——`admit()` 在 `Submit` 之前于锁内递增 `pending` 并在 0→1 时建立 `drain`，任务体以 `defer complete()` 收尾且提交失败补偿递减，故 `Flush`/`Wait` 覆盖「排队中但尚未执行」的任务 | 单元/契约 + race + Redis live integration（异步双写往返、关闭前排空在途写、关闭后拒绝操作与重复关闭幂等） | Bloom/breaker miss 语义不一致；singleflight **不在 L2 内**——`singleflight.Group` 属 `cache.Coordinator`（`cache_utils.go:79`），与 L2 是两套可独立使用的机制，四件组的代码级定位见 `.codegraph-qa-out/l2-cache-component-map.md`；见[缓存指南](../guides/cache.md) |
| 异步任务 | 已接入 | 稳定公共能力 | 公共 API | 无默认 task register；应用需提供 Redis、initializer、handler、`TaskRegister` 并启用 `application.task.enableServer` | asynq `TaskWorker`/`TaskDispatcher` 的创建、同步/异步运行、失败记录与关闭四条路径均有明确契约；`TaskDispatcher.Close` 以 atomic 幂等守卫委派 `asynq.Client.Close()`、关闭后 `Enqueue` 返回错误且**刻意不持有 Redis 连接的所有权**，`TaskWorker.Shutdown` 委派 `asynq.Server.Shutdown()`，两者均实作 `globalmanager.Closable` 并经容器 `ReleaseAll` 回收；`RunAsync` 不再恒返回 nil | 单元/契约 + live integration（入队-消费-优雅关闭；并发入队消费不丢不重；handler 持续失败的重试-归档与 `Config.ErrorHandler` 上报；`SkipRetry` 立即归档不消耗重试） | 四项晋级条件均已闭合。条件 1 由 `Close`/`Shutdown` 契约补齐；条件 3 的 live 验证已覆盖入队-消费-优雅关闭、并发入队消费不丢不重、handler 持续失败的重试-归档与 `SkipRetry` 立即归档。示例依赖外部 Redis（live 用例需 Redis 容器）。**已知限制：启用 `enableServer` 时进程关闭必然 panic**——`NewTaskWorker` 经 `asynq.NewServerFromRedisClient` 构造，asynq 据此把 `sharedConnection` 置为 true 而不关闭 broker；但 `RunServer` 最终走 `asynq.Server.Run`，其内建 `waitForSignals` 与框架关闭链并行处理 SIGTERM/SIGINT，Redis 客户端被框架回收后 subscriber 仍在 `select`，`subscriber.go:83` 对 nil `*redis.Message` 取 `.Payload` 触发 SIGSEGV（故障地址 `0x20` 与该字段偏移一致），进程以 exit status 2 结束。实测 SIGTERM 3/3、SIGINT 1/1 必现；框架自身关闭链正确，全部组件关闭日志先于 panic 输出。修复方向是改用 `asynq.Server.Start` 使 asynq 不注册信号处理，关闭交由已有的 `TaskWorker.Close`，但这会变更 `task.go` 的公共行为语义，留待单独裁决；见[异步任务指南](../guides/background-tasks.md) |
| CLI | 已接入 | 稳定公共能力 | 公共 API | 不属于 Web 默认集合；应用单独创建 `CmdContext`、应用注册器和基于 urfave/cli 的 `CMDLineApplication` | 创建、命令注册、运行与资源关闭四条路径均有明确契约；`AppCoreRun` 的错误经 `RunCommandStarterWithError` 返回给调用方，`RunCommandStarter` 保持 void 签名但把错误写入 CLI 日志器（不再静默丢弃）；健康检查是一次性的，**「没有需要停止的东西」本身即完整契约**，与 JSON 编解码的「没有独立关闭资源」同例 | 单元/契约 | 健康检查只执行一次，`RunCommandStarter` 丢弃返回值；关闭段以 `defer` 释放全局对象（命令返回错误时同样执行），但因 `ICommandContext.GetStarterApp()` 返回 `CommandStarter` 而非 `ApplicationStarter`，它不复用 `clearApplicationGlobals`，也不停止健康检查——CLI 的健康检查本就是一次性的，没有需要停止的后台 ticker；见[命令行指南](../guides/command-line.md) |
| 扩展运行位点与关闭链 | 已接入 | 稳定公共能力 | 公共 API | 应用可把自定义 manager 显式绑定到 server run 的 before/main location，以及 shutdown 的 before/main/after location；普通 manager 先加载，`GroupExtendReplace` manager 只替代同一 location 的默认逻辑 | `RunServer` 会收集运行与关闭管理器，核心运行结果无论成功、失败或 panic 都进入协调通道，且运行分支失败（监听绑定失败、运行 goroutine panic）时也会释放全局对象——该分支从不调用 `Shutdown`，此前是唯一没有回收的退出路径；信号触发 shutdown，Fiber/Gin 的运行链消费 before/main 位点，关闭链消费 before/main/after 位点，且两者都保证 after 位点在全局对象释放与日志器关闭**之前**执行，因此该位点的 provider（仓库内即 pprof 关闭提供者）仍可正常记录日志；provider 关闭接口与跨组件资源所有权已由 `ClosableProvider` + `CloseProviders` 统一，关闭链按注册逆序回收并逐个上报错误而不中断 | 单元/契约 | 专项测试覆盖正常返回、信号关闭、同位点替代、不同位点互不抑制、shutdown before/after 执行，以及 after 位点早于日志器关闭的顺序回归（经注入 `io.WriteCloser` 观测）；**条件 5 的示例证据仅在 test profile 下执行**：该位点在仓库内唯一的默认消费者是 pprof 关闭提供者，而 `application_dev.yml` 与 `application_prod.yml` 的 `pprof.enable` 均为 `false`，只有 CI smoke 使用的 `application_test.yml` 为 `true`。即示例的 dev/prod 默认路径不执行该关闭位点，应用若需该位点的生产证据须自行绑定 manager。`ServerRunAfter` 按设计不被框架默认实现消费（`provider_location.go:139` 已声明），它是应用专用扩展点而非缺口；真实进程信号与外部资源组合关闭仍未进入 smoke；见[Web 启动生命周期](../concepts/startup-lifecycle.md) |

## 内部工具

**本节四项均不参与晋级**，不计入 §2 的公共能力门槛。它们是框架内部的实现工具，
而非对外承诺的能力：`API 受众` 一列标为「内部工具」即此意。因此本节各行的
「晋级阻塞」不是待办事项——不存在要晋级的目标。

逐项理由：

- **Dig 容器**：全库**零个非测试** `Provide`/`Invoke` 调用者（已复核），
  仅 `CmdContext` 持有容器字段、CLI 示例只调用 `NewCmdContextOnce`。
  没有任何执行路径，条件 5 在结构上不可满足。是否移除该字段留待用户决定。
- **jsonconvert**：只服务 recovery 错误处理的内部转换，不在公共 API 上暴露。
  当前只有 bench 测试（`convert_vs_opt_bench_test.go`），无正确性测试——
  若将来要晋级，这是必须先补的一项。
- **mongodecimal**：位于 `component/database/dbmongo/internal/mongodecimal/`。
  Go 的 `internal/` 包**无法被仓库外的示例导入**，因此条件 5「文档示例确实执行
  该路径」在结构上不可满足——一个 `internal/` 包本质上不可能是公共能力。
  它的行为随 `dbmongo.NewClient` 构造自动注册，应视为 MySQL/MongoDB 行的实现
  细节而非独立能力。
- **logging writer 与框架 adaptor**：Gin adapter 使用进程级 hook，多个 engine
  共享同一个框架日志器——这是架构性限制而非可修的缺陷（见
  [Web 运行时](../guides/web-runtime.md)、[日志指南](../guides/logging.md)）。
  异步 writer 本身的关闭顺序已定义且正确（两个 writer 各自 CAS 幂等，
  最终 Flush 由 consume goroutine 完成以避免并发写 `bufio.Writer`），
  `LoggerWrap.Close` 也已具备幂等守卫。

| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
|---|---|---|---|---|---|---|---|
| Dig 容器 | 已实现 | 实验性 | 内部工具 | `CmdContext` 持有单例容器字段；**仓库内无任何 `Provide`/`Invoke` 调用者**，CLI 示例只调用 `NewCmdContextOnce` | 创建、运行、失败返回和 reset 有路径；没有通用资源关闭编排 | 单元/契约（41.7%） | **晋级阻塞：条件 5**——没有文档示例实际执行该路径。Web 运行时不推荐使用，`ResetDigContainer` 非并发安全；见[组件目录](components.md)、[命令行指南](../guides/command-line.md) |
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
4. 不以 panic 或 fatal 表达调用方本可处理的错误。启动期检测到的不可恢复装配错误允许
   panic，**前提是存在文件化的、返回 error 的替代入口**（见下方说明）。
5. 文档示例确实执行该路径，并明确资源所有权和限制。

### 关于条件 4 的适用边界

本条原文为「不依靠 panic 或 fatal 表达可恢复的装配或运行错误」。该措辞与本页实际
采用的判据不一致，已按实际判据改写，理由如下。

本条判据的来历：已晋级的 MySQL / MongoDB 含两处装配期 `panic(err)`
（`mysql_model_impl.go:33`、`mongo_model_impl.go:36`），而当时被判定卡住条件 4 的
MsgPack / Protobuf，其 `response_facade.go:33,36` 在结构上与之相同。两者之所以
结论不同，真正的区别是**前者有文件化的 `NewMysqlModelWithError` /
`NewMongoModelWithError` 可供调用方改用，后者没有任何返回 error 的替代入口**。
这个区别是成立的，但原文字从未表达它，因而对同类代码给出了不同结论。

该差异现已消除：MsgPack / Protobuf 补上 `NewRespInfoPManagerOnceWithError` 后满足
本条，已完成晋级（见上表）。此处保留这段推导，是因为它正是本条判据的由来，
后续同类判定仍应据此执行——而不表示这两项能力仍卡在条件 4。

因此本条的判据是：

- **允许**：不可恢复的装配错误在启动期 panic，且同时提供返回 error 的替代入口
  （`Must*` / `*WithError` 成对出现的惯例，与 stdlib 的 `regexp.MustCompile`、
  `template.Must` 一致）。调用方若需处理错误，永远有一条不 panic 的路径。
- **不允许**：调用方可以处理、却只能以 panic 得知的错误；或唯一入口即 panic，
  调用方无从选择。改写前的参数验证即属此类——语言包注册失败时只有 panic 一条路，
  故 T6 将其改为返回 error。

仅放宽「有替代入口」的情形，不放宽其余。以 panic 表达运行期（非装配期）错误始终
不满足本条。

### 关于条件 2 的当前豁免

**条件 2（兼容承诺）是本页唯一仍被豁免的条件，且只豁免它一条。** 一项能力满足
条件 1、3、4、5 即可标注为“稳定公共能力”，其含义是“该能力的创建、运行、失败、
关闭四段生命周期契约明确，验证证据与文档已闭合”，而非“其导出符号已冻结”。

要点在于区分两件常被混为一谈的事：**生命周期与验证的成熟度**，和**导出符号的
冻结承诺**。前者是本页 15 项能力当前已全部达到的标准——四段契约、CI 内的单元/
契约与 race、外部依赖的可重复 live integration、不以 panic 表达调用方本可处理的
错误、示例确实执行该路径。后者留待 v2.0.0 统一给出，在此之前仍可能发生 breaking
rename 或签名调整；此类调整须在 CHANGELOG 与本页同步记录。

因此「尚未 v2.0.0」**不构成**任何能力推迟晋级的理由：条件 1、3、4、5 都必须在
当前版本上逐条兑现，不得以版本号代替证据。v2.0.0 起本豁免失效，条件 2 恢复为
硬性门槛。
