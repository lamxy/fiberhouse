# 组件目录

`component/` 既包含已进入框架调用链的实现，也包含内部辅助代码和空目录意图。下表按当前主调用者与生命周期归类；“内部工具”不表示对外稳定 API，也不建议业务代码仅因其导出而直接依赖。

目录名不是成熟度信号。判断一个组件是否可用，需要同时找到构造入口、实际调用者、错误出口和资源结束方式。

| 组件 / package | 用途 | 主要调用者 | 生命周期、错误与并发边界 | 状态 | 相关指南 |
|---|---|---|---|---|---|
| `component/bufferpool` | 分片 `bytes.Buffer` 池与泛型 `sync.Pool` | 当前仓库没有运行时调用者，`createStringBuilderPool` 仅为包内示例 | pool 可并发获取/归还；借出的对象仍由单个调用方独占，归还后不得继续引用；超出容量范围的 buffer 不回收 | 内部工具 | 《组件目录》 |
| `component` 的 `DigContainer` | 包装 uber/dig，收集 `Provide` 错误并支持泛型 `Invoke` | `CmdContext`，CLI `test-orm` 与其 service 装配 | 单例从命令上下文创建；先 `Provide` 再 `Invoke`，错误由 `GetProvideErrs` 或 `Invoke` 返回；错误切片写入和 `ResetDigContainer` 不适合并发运行期 | 内部工具 | 《命令行指南》 |
| `component/jsoncodec` | Std JSON 与 Sonic 的 `JsonWrapper`/Gin codec 实现 | JSON provider、HTTP core、task payload；示例注册 Sonic 实例 | 实例通常在启动期构造后只读；Sonic 解码失败回退标准库并返回最终错误；`gojson.go` 无实现 | 已接入（Std/Sonic）；预留/占位（Go JSON） | 《响应与编解码指南》 |
| `component/jsonconvert` | 把 recovery 数据分类为 JSON、标量字符串或不可序列化值 | Gin recovery 与统一错误处理器 | `DataWrap` 来自 `sync.Pool`，调用后必须 `Release`；单个实例明确用于非并发场景；编码错误由 `GetJson` 返回 | 内部工具 | 《错误处理指南》 |
| `component/mongodecimal` | 在 `decimal.Decimal` 与 BSON Decimal128 间转换 | `dbmongo.NewClient` 的 BSON registry | codec 随 Mongo client registry 存活且无可变状态；类型不符、解析或读写失败均返回错误 | 内部工具 | 《数据库指南》 |
| `component/writer` | lumberjack 同步 writer、channel/diode 异步 writer | `bootstrap.NewLoggerOnce` 的文件输出装配 | 异步实现各自启动后台 goroutine；channel 满或 diode 覆盖会计数丢日志；应停止生产者后只调用一次 `Close`，等待排空和 flush，不能承诺无损 | 内部工具（异步路径有明显限制） | 《日志指南》 |
| `component/tasklog` | 把 asynq `Logger` 转到 FiberHouse 日志来源 | `example_application` 的 `TaskAsync` | 与 TaskWorker/应用上下文同寿命；只读取上下文；`Fatal` 沿用全局日志器的 fatal 语义，当前框架默认任务链不自动安装该 adapter | 内部工具（示例装配） | 《异步任务指南》《示例目录》 |
| `component/validate` | 多语言 validator、translator、自定义 tag 和错误响应映射 | `AppContext`/`CmdContext`、`FrameStarter`、请求 DTO | context 创建时注册 en/zh-cn/zh-tw，应用可在启动期追加；注册失败返回错误集合；内部 map 不支持运行期并发读写，服务开始后只读 | 已接入 | 《验证指南》 |
| `database/dbmysql` | GORM/MySQL client、连接池、健康检查及 model locator | 示例 Web/CLI 的 GlobalManager initializer 与 MySQL model/service | 应用持有并负责 `Close`；初始化会校验 DSN、连接并 ping；`Rebuild` 替换 client 但不关闭旧连接，读侧未与替换锁配套 | 实验性 | 《数据库指南》《全局管理器指南》 |
| `database/dbmongo` | MongoDB v2 client、连接选项、健康检查及 model locator | 示例 Web/CLI initializer 与 Mongo model；内部注册 `mongodecimal` | 应用持有并负责 `Disconnect`；连接/命令错误向上传递；`Rebuild` 同样不关闭旧 client，读侧未与替换锁配套 | 实验性 | 《数据库指南》《全局管理器指南》 |
| `component/i18n` | 通用国际化的目录意图 | 无 Go 调用者 | 无初始化、错误、并发或关闭语义；validate 翻译不等于通用 i18n | 预留/占位 | 《功能状态》《验证指南》 |
| `component/mq` | 消息队列的目录意图 | 无 Go 调用者 | 只有 RabbitMQ 方向说明，没有 client、consumer 或生命周期 | 预留/占位 | 《功能状态》 |
| `component/rpc` | RPC 的目录意图 | 无 Go 调用者 | 无 RPC client/server；根目录生成的响应 proto 也不提供 RPC 生命周期 | 预留/占位 | 《功能状态》《响应与编解码指南》 |

组件装配应在服务进入并发处理前完成。对象池条目、Dig 容器、验证器注册表和异步 writer 的可变状态具有不同并发语义，不能把 package 级单例等同于任意时刻都可安全重配。数据库、日志与任务组件的关闭错误也不会由目录结构自动汇总；应用需要为资源建立明确的创建者、停止顺序和错误出口。

## 如何使用这张表

“主要调用者”只记录当前仓库内可达的主路径。导出符号即使没有仓库内调用者，也可能被外部使用，因此这里的结论用于文档分级，不用于自动删除代码。

“生命周期”同时描述创建和结束。没有 `Close` 的纯转换器通常随调用或 registry 存活；拥有 goroutine、连接池或底层文件的组件则必须有明确所有者。

“状态”描述当前组合能力。一个 package 可以同时包含已接入实现和占位文件，例如 `jsoncodec` 的 Std/Sonic 可用，而 Go JSON 文件仍为空。

“相关指南”是主解释位置的纯文本名称。目标页面齐备后再统一转换为仓库相对链接，当前页不制造悬空链接。

## 启动期装配组件

Dig 容器适合命令应用或启动阶段组装对象。`Provide` 把错误追加到容器的错误切片，调用方必须在 `Invoke` 前检查；重复向进程级单例注册相同构造关系也应视为应用装配错误。

`ResetDigContainer` 会清空内部容器并替换 `sync.Once`，源码明确标注非并发安全。它不应在 Web 请求、任务 handler 或其他并发调用仍在进行时使用。

验证包装器由应用上下文创建。内建语言在构造阶段注册，应用自定义 initializer 和 tag 由 FrameStarter 在启动阶段追加；注册函数返回的错误需要在启动阶段处理。

验证器、translator、语言切片和内部 map 没有为运行期写入提供同步保护。稳定用法是启动期写、服务期读，不在请求处理中动态注册规则。

JSON codec 实例本身主要用于只读调用。Sonic 配置经过冻结后供 provider、HTTP core 和任务 payload 使用；选择 codec 仍依赖应用注册的实例 key 与 provider/manager 装配。

空的 Go JSON 文件不提供 marshal、unmarshal 或 provider，因此 `TrafficCodecWithGoJson` 名称不能单独证明可选择该实现。

## 池化与转换辅助

`bufferpool.BufferPool` 只把容量命中既有 shard 的 buffer 放回池。调用者应把 `Get`/`Put` 视为所有权转移，不能在 `Put` 后保留切片或 builder 的可变引用。

泛型 `Pool[T]` 会在归还时调用 `resetFn`。重置函数由创建者负责，若它没有清理敏感或大型引用，池会延长这些对象的生命周期。

当前仓库没有生产路径使用 bufferpool，因此它被归为内部工具。这里记录行为边界，不把它推荐为业务层的通用内存优化方案。

`jsonconvert.DataWrap` 也是池化对象。`NewDataWrap` 判定复杂值是否尝试 JSON 编码，标量走字符串转换，运行时错误转成错误文本。

`GetJson` 会把 encoder 错误返回给调用方；`Release` 负责清空引用并归还池。单个 wrapper 不能在多个 goroutine 间共享，也不能在释放后继续使用。

`mongodecimal.MongoDecimal` 没有可变字段，由 MongoDB client 构造时注册到 BSON registry。它的错误语义是返回类型、Decimal128 解析或 reader/writer 错误，不负责重试或替代数据。

## 日志与任务适配

同步 writer 直接把 `Write` 与 `Close` 交给 lumberjack。轮转参数来自应用配置，文件路径和关闭时机由 bootstrap 日志生命周期决定。

channel writer 会复制输入字节，并在通道持续满一秒后丢弃该条日志；diode writer 在容量压力下覆盖并累计 missed 数量。两者都不能描述为无损输出。

两种异步 writer 都启动后台 goroutine，并由该 goroutine执行最终 flush。安全关闭顺序是先停止所有日志生产者，再调用一次 `Close` 并等待返回。

`tasklog.TaskLoggerAdapter` 只是协议适配器，不创建 worker，也不拥有 asynq server。它读取上下文中的日志器和来源字段，寿命应由 TaskWorker 装配方控制。

adapter 的 `Fatal` 继承底层全局日志器行为。业务任务不应把 fatal 当作普通可恢复错误；handler 的业务失败仍应通过 handler 返回值交给 asynq。

## 数据库辅助

`dbmysql` 和 `dbmongo` 位于 `database/`，但与 component 内部 codec、日志适配器和 GlobalManager 生命周期紧密相关，因此在同一目录表中列出。

两种 client 都读取配置并返回初始化错误。示例把它们注册为 GlobalManager initializer，并选择在启动期强制 `Get`，所以连接失败会阻断示例完整装配。

`Close`/`Disconnect` 由资源所有者调用。当前 `Rebuild` 替换 client 时没有先关闭旧 client，应用不应把重建理解为自动完成连接迁移和回收。

model locator 保存 context、实例 key、库名和表名等定位信息。它是访问已注册 client 的辅助层，不负责创建缺失的数据库服务。

## 占位目录

`component/i18n`、`component/mq` 与 `component/rpc` 没有 Go 运行实现。它们没有默认注册、应用 opt-in 入口、错误语义、并发模型或关闭流程。

验证组件中的 translator 只服务于校验错误消息，不构成通用 i18n；响应 proto 只服务于 HTTP 响应结构，不构成 RPC server。

当这些目录将来出现实现时，状态升级至少要确认构造、默认或手动注册、应用启用、运行、错误传播和关闭六个环节。

## 应用侧检查清单

- 在启动期完成 Dig、validator、codec 和 registry 写入。
- 为 writer、数据库 client、缓存和 task worker 指定唯一资源所有者。
- 让初始化、编码、连接和关闭错误到达可观察的应用错误出口。
- 在对象归还池或资源关闭后，不再保留或并发使用旧引用。
- 不把示例调用者、导出符号或配置键当作稳定公共契约。
- 不为 i18n、MQ、RPC 等占位目录宣称不存在的运行能力。
