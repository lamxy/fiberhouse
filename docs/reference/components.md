# 组件目录

`component/` 既包含已进入框架调用链的实现，也包含内部辅助代码和空目录意图。下表按当前主调用者与生命周期归类；“内部工具”不表示对外稳定 API，也不建议业务代码仅因其导出而直接依赖。

`component/` 是 FiberHouse 提供的内置、可选装配、可复用能力的命名空间，不是严格的底层依赖层。根目录不提供 Go API；每个一级子目录代表可独立理解和装配的能力。

子组件可以依赖 FiberHouse 核心接口，但依赖 root package 的组件必须由应用注册器或 Provider 装配，root package 和父级命名空间不得反向聚合导入。仅服务于单个组件的辅助实现放入该组件的 `internal/` 子树。

目录名不是成熟度信号。判断一个组件是否可用，需要同时找到构造入口、实际调用者、错误出口和资源结束方式。

| 组件 / package | 用途 | 主要调用者 | 生命周期、错误与并发边界 | 状态 | 相关指南 |
|---|---|---|---|---|---|
| `component/bufferpool` | 分片 `bytes.Buffer` 池与泛型 `sync.Pool` | 当前仓库没有运行时调用者，`createStringBuilderPool` 仅为包内示例 | pool 可并发获取/归还；借出的对象仍由单个调用方独占，归还后不得继续引用；超出容量范围的 buffer 不回收 | 内部工具 | 本页 |
| `component/container` | 基于 Uber Dig 的启动期依赖注入容器与泛型解析包装器 | `CmdContext`、CLI `test-orm` 与其 service 装配 | 单例容器只用于启动装配；`Provide` 错误显式收集，`ResetDigContainer` 不支持并发运行期调用 | 内部工具 | [命令行指南](../guides/command-line.md) |
| `component/cache` | 通用 `Cache` 接口、选项池、缓存读取工具与保护机制 | 示例 service、任务注册器和应用全局 initializer | 实例由应用显式注册；目录不统一接管 local/remote/L2 的创建、等待和关闭 | 实验性 | [缓存指南](../guides/cache.md)、[GlobalManager](../guides/global-manager.md) |
| `component/cache/cachelocal` | 基于 Ristretto 的本地缓存 | Web/CLI 应用 initializer 与 L2 cache | 应用持有实例；异步写入后可调用 `Wait`，关闭后操作返回缓存关闭错误 | 实验性 | [缓存指南](../guides/cache.md) |
| `component/cache/cacheremote` | 基于 go-redis 的远程缓存、Redis client 与缓存定位辅助 | Web/CLI initializer、任务系统与 L2 cache | 应用持有 Redis client；连接、重建、熔断及关闭语义保持由实现暴露 | 实验性 | [缓存指南](../guides/cache.md) |
| `component/cache/cache2` | 组合 local/remote 的二级缓存和异步同步策略 | Web 应用的 GlobalManager initializer | 持有两个 ants pool；应用负责创建依赖 cache 并在停止阶段关闭 | 实验性 | [缓存指南](../guides/cache.md) |
| `component/jsoncodec` | Std JSON 与 Sonic 的 `JsonWrapper`/Gin codec 实现 | JSON provider、HTTP core、task payload；示例注册 Sonic 实例 | 实例通常在启动期构造后只读；Sonic 解码失败回退标准库并返回最终错误；`gojson.go` 无实现 | 已接入（Std/Sonic）；预留/占位（Go JSON） | [响应与序列化](../guides/response-and-serialization.md) |
| `component/jsonconvert` | 把 recovery 数据分类为 JSON、标量字符串或不可序列化值 | Gin recovery 与统一错误处理器 | `DataWrap` 来自 `sync.Pool`，调用后必须 `Release`；单个实例明确用于非并发场景；编码错误由 `GetJson` 返回 | 内部工具 | [错误与恢复](../guides/errors-and-recovery.md) |
| `component/writer` | lumberjack 同步 writer、channel/diode 异步 writer | `bootstrap.NewLoggerOnce` 的文件输出装配 | 异步实现各自启动后台 goroutine；channel 满或 diode 覆盖会计数丢日志；应停止生产者后只调用一次 `Close`，等待排空和 flush，不能承诺无损 | 内部工具（异步路径有明显限制） | [日志指南](../guides/logging.md) |
| `component/tasklog` | 把 asynq `Logger` 转到 FiberHouse 日志来源 | `example_application` 的 `TaskAsync` | 与 TaskWorker/应用上下文同寿命；只读取上下文；`Fatal` 沿用全局日志器的 fatal 语义，当前框架默认任务链不自动安装该 adapter | 内部工具（示例装配） | [异步任务指南](../guides/background-tasks.md)、[示例目录](examples.md) |
| `component/validate` | 多语言 validator、translator、自定义 tag 和错误响应映射 | Web `AppContext`/`FrameStarter`、请求 DTO；CLI 自建 wrapper 时按需使用 | Web `AppContext` 创建时按 `application.validate.langFlags` 注册 en/zh-cn/zh-tw 中被选中的语言，未配置时仅注册 en；`CmdContext.GetValidateWrap()` 固定返回 nil，CLI 需自行构造和持有；内部 map 不支持运行期并发读写，服务开始后只读 | 已接入 | [验证指南](../guides/validation.md) |
| `component/database/dbmysql` | GORM/MySQL client、连接池、健康检查及 model locator | 示例 Web/CLI 的 GlobalManager initializer 与 MySQL model/service | 应用持有并负责 `Close`；初始化会校验 DSN、连接并 ping；`Rebuild` 替换 client 但不关闭旧连接，读侧未与替换锁配套 | 实验性 | [数据库指南](../guides/database.md)、[GlobalManager](../guides/global-manager.md) |
| `component/database/dbmongo` | MongoDB v2 client、连接选项、健康检查及 model locator | 示例 Web/CLI initializer 与 Mongo model | 应用持有并负责 `Disconnect`；连接/命令错误向上传递；`Rebuild` 同样不关闭旧 client，读侧未与替换锁配套 | 实验性 | [数据库指南](../guides/database.md)、[GlobalManager](../guides/global-manager.md) |
| `component/database/dbmongo/internal/mongodecimal` | 在 `decimal.Decimal` 与 BSON Decimal128 间转换 | 仅 `dbmongo.NewClient` 的 BSON registry | dbmongo 私有无状态 codec；类型不符、解析或读写失败均返回错误 | 内部实现 | [数据库指南](../guides/database.md) |
| `component/i18n` | 通用国际化的目录意图 | 无 Go 调用者 | 无初始化、错误、并发或关闭语义；validate 翻译不等于通用 i18n | 预留/占位 | [功能状态](feature-status.md)、[验证指南](../guides/validation.md) |
| `component/mq` | 消息队列的目录意图 | 无 Go 调用者 | 只有 RabbitMQ 方向说明，没有 client、consumer 或生命周期 | 预留/占位 | [功能状态](feature-status.md) |
| `component/rpc` | RPC 的目录意图 | 无 Go 调用者 | 无 RPC client/server；`response/pb` 仅提供 HTTP 统一响应的 Protobuf 数据契约，不提供 RPC 生命周期 | 预留/占位 | [功能状态](feature-status.md)、[响应与序列化](../guides/response-and-serialization.md) |

组件装配应在服务进入并发处理前完成。对象池条目、Dig 容器、验证器注册表和异步 writer 的可变状态具有不同并发语义，不能把 package 级单例等同于任意时刻都可安全重配。数据库、日志与任务组件的关闭错误也不会由目录结构自动汇总；应用需要为资源建立明确的创建者、停止顺序和错误出口。

## 如何使用这张表

“主要调用者”只记录当前仓库内可达的主路径。导出符号即使没有仓库内调用者，也可能被外部使用，因此这里的结论用于文档分级，不用于自动删除代码。

“生命周期”同时描述创建和结束。没有 `Close` 的纯转换器通常随调用或 registry 存活；拥有 goroutine、连接池或底层文件的组件则必须有明确所有者。

“状态”描述当前组合能力。一个 package 可以同时包含已接入实现和占位文件，例如 `jsoncodec` 的 Std/Sonic 可用，而 Go JSON 文件仍为空。

“相关指南”指向主解释位置；本页只保留跨组件索引和其他指南未覆盖的实现细节。

## 池化与转换辅助

`bufferpool.BufferPool` 只把容量命中既有 shard 的 buffer 放回池。调用者应把 `Get`/`Put` 视为所有权转移，不能在 `Put` 后保留切片或 builder 的可变引用。

泛型 `Pool[T]` 会在归还时调用 `resetFn`。重置函数由创建者负责，若它没有清理敏感或大型引用，池会延长这些对象的生命周期。

`jsonconvert.DataWrap` 也是池化对象。`NewDataWrap` 判定复杂值是否尝试 JSON 编码，标量走字符串转换，运行时错误转成错误文本。

`GetJson` 会把 encoder 错误返回给调用方；`Release` 负责清空引用并归还池。单个 wrapper 不能在多个 goroutine 间共享，也不能在释放后继续使用。

`mongodecimal.MongoDecimal` 没有可变字段，由 MongoDB client 构造时注册到 BSON registry。它的错误语义是返回类型、Decimal128 解析或 reader/writer 错误，不负责重试或替代数据。

## 日志与任务适配

两种异步 writer 都启动后台 goroutine，并由该 goroutine 执行最终 flush。安全关闭顺序是先停止所有日志生产者，再调用一次 `Close` 并等待返回。

adapter 的 `Fatal` 继承底层全局日志器行为。业务任务不应把 fatal 当作普通可恢复错误；handler 的业务失败仍应通过 handler 返回值交给 asynq。

## 数据库辅助

`dbmysql` 和 `dbmongo` 位于 `component/database/`，但与 component 内部 codec、日志适配器和 GlobalManager 生命周期紧密相关，因此在同一目录表中列出。

model locator 保存 context、实例 key、库名和表名等定位信息。它是访问已注册 client 的辅助层，不负责创建缺失的数据库服务。

## 应用侧检查清单

- 在启动期完成 Dig、validator、codec 和 registry 写入。
- 为 writer、数据库 client、缓存和 task worker 指定唯一资源所有者。
- 让初始化、编码、连接和关闭错误到达可观察的应用错误出口。
- 在对象归还池或资源关闭后，不再保留或并发使用旧引用。
- 不把示例调用者、导出符号或配置键当作稳定公共契约。
- 不为 i18n、MQ、RPC 等占位目录宣称不存在的运行能力。
