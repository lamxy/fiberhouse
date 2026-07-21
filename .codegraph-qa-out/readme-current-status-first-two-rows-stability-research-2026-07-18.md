# README“当前状态”前两行解释与快速稳定性研究

> 研究日期：2026-07-18
> 研究基线：`main@613af8f`
> 研究范围：根目录 README“当前状态”前两行，以及它们对应的主链稳定性、并发、关闭和真实外部依赖验证。
> 性质：研究与候选清单，不构成功能代码修改授权。
> 硬约束：不新增 `Run(ctx context.Context) error` 或等价入口；不修改公共 API；不建立统一生命周期/关闭架构；不拆分 GlobalManager owner/locator；不调查或修改 MsgPack/Protobuf 内容协商；一次只处理一个可复现的局部问题。

## 一、用人话解释 README 前两行

README 当前写法是：

| 范围 | 实现阶段 | 支持级别 | API 受众 | 摘要 |
|---|---|---|---|---|
| Fiber HTTP、Provider 主链、配置与日志、JSON 响应、校验 | 已接入 | 实验性 | 公共 API | 已有明确入口，并按能力具有单元/契约或 HTTP smoke 证据；部分错误与关闭路径仍有明确限制 |
| Gin、GlobalManager、L2 缓存、任务、CLI、数据库、二进制响应 | 已接入 | 实验性 | 公共 API | 已有可达实现，但错误传播、并发、关闭或外部依赖验证仍存在明确缺口 |

更容易理解的说法是：

1. **第一行**：以 Fiber 为默认选择的 Web 主流程已经能够跑通。应用可以加载配置和日志，装配 Provider，启动 HTTP 服务，校验参数并返回统一 JSON。仓库有单元/契约测试，示例还会实际启动 Fiber 并请求一个接口。但是默认组件仍要显式装配，部分错误处理和资源关闭还没有形成稳定承诺。
2. **第二行**：Gin、全局对象管理、二级缓存、后台任务、CLI、数据库和二进制响应都不是空壳，已经存在真实代码和调用入口。但是不同能力分别存在错误被忽略、并发边界不完整、关闭不彻底，或者没有连接真实 Redis/MySQL/MongoDB 进行验证的问题。

三个状态词分别回答不同问题：

- `已接入`：存在明确入口，并能走到实际运行逻辑。
- `实验性`：兼容性、错误、并发、关闭或验证还没有达到稳定门槛。
- `公共 API`：这些入口面向业务使用，不代表已经承诺长期兼容。

因此：

```text
已接入 + 实验性 + 公共 API
=
业务可以调用，代码也确实能够运行；
但不能据此推导为自动启用、生产就绪或稳定兼容。
```

第一行和第二行也不是严格的“核心/非核心”分类。GlobalManager 本身就是基础设施。两行的主要差别是当前证据强度和缺口类型：第一行以主 Web 正常路径为主，第二行更多涉及替代内核、共享状态、异步执行和真实外部服务。

## 二、为什么不能把整行直接改成“稳定”

整行晋级会把风险不同的能力捆绑在一起：

- validation、JSON 响应等纯 Go 能力可以通过局部失败路径测试快速收敛；
- Fiber/Gin 监听和日志 writer 涉及 goroutine、信号与关闭竞争；
- GlobalManager、L2、task 和数据库涉及资源所有权、并发引用和外部进程；
- Redis、MySQL、MongoDB 目前虽然已由 CI 启动，但 CI 成功条件没有调用仓库中的这些 client 执行真实读写。

更稳妥的做法是按单能力逐项增加可靠性和证据，达到条件后再单独评估支持级别。短期目标是“减少明确失败和补足验证”，不是为了改标签而改标签。

## 三、调研方法与证据

本次使用以下方式交叉核对：

- CodeGraph：追踪 `Default`/`New`、Fiber/Gin core、Provider/Manager、GlobalManager、L2、task、CLI、数据库和响应调用链；
- ast-grep：确认 CLI 和 task 中显式丢弃错误的 AST 位置，并检查缓存关闭状态模式；
- 源码复核：检查具体函数中的状态更新、返回值、资源关闭和对象池数据重置；
- 测试与 CI：检查现有 focused/race 测试，以及 `.github/workflows/go1.yml` 的 quality、race、smoke job。

关键现状：

- `quality` 已执行 `go vet ./...` 和全仓单元/契约测试；`race` 已执行全仓 race。
- `smoke` 已启动固定版本的 Redis、MongoDB、MySQL，也构建并启动示例。
- `smoke` 当前唯一成功断言是访问 Fiber 的 `/example/hello/world`；没有通过仓库 Redis/MySQL/MongoDB client 执行读写。
- Gin TLS 当前测试只验证配置和证书加载，没有真实 loopback listener/TLS handshake。
- task 测试使用不可达 Redis client 完成构造，并直接调用 mux，没有真实 enqueue/worker/关闭链。

## 四、三种推进方式

### 方案 A：证据驱动的单点补丁队列（推荐）

每次只处理一个已经能定位到具体函数的缺陷：先补失败测试，再修改原函数，保持公共签名和现有调用结构不变。外部服务验证作为独立测试/CI 补丁推进。

优点：

- 改动小，审核和回滚简单；
- 不需要重新定义全局架构；
- 每个补丁都能给 README 状态增加一条直接证据；
- 可以在发现需要扩大范围时立即停止。

缺点：

- 不会一次性解决所有生命周期问题；
- “稳定公共能力”仍需按能力逐项评估。

### 方案 B：先只补测试和 CI

暂不改生产代码，先增加 Gin loopback 握手和 Redis/MySQL/MongoDB/task live integration；测试暴露问题后再决定局部补丁。

优点：副作用最低，能快速提高文档可信度。
缺点：已经确认的并发关闭和数据残留问题仍会继续存在；真实服务测试也可能一次暴露多个问题，导致后续范围不好控制。

### 方案 C：按子系统集中收敛

一次性处理日志、HTTP、缓存、任务、数据库的生命周期和错误传播。

优点：理论上能统一行为。
缺点：必然跨组件、耗时长、容易修改公共语义，与当前“快速、局部、非重构”要求冲突。因此不采用。

## 五、已经复核成立的快速生产代码候选

以下候选均保持现有公共签名。它们只是候选，执行时仍需一项一审核，并在独立 Git 工作树中修改。

### P0-1：AsyncChannelWriter 并发 Write/Close 和重复 Close

证据：`component/logging/writer/async_channel_writer.go` 中，`Write` 先检查 `closed`，随后向 `logChan` 发送；`Close` 直接写入关闭标志并 `close(logChan)`。

当前可能出现：

- 两个 goroutine 同时 `Close`，重复关闭 channel 并 panic；
- `Write` 已通过 closed 检查后，另一个 goroutine关闭 channel，随后发送导致 panic。

最小方向：只在该 writer 内增加关闭幂等和 Write/Close 协调，参考同目录已经具备并发关闭测试的 diode writer，不改变 `Write`/`Close` 签名和日志架构。

最小验证：

```bash
go test -race ./component/logging/writer -run AsyncChannelWriter -count=1
```

优先原因：这是进程级 panic 风险，范围集中在一个实现文件和一个测试文件。

### P0-2：Fiber 自定义 CoreCfg 路径留下 nil JSON codec

证据：`CoreWithFiber.InitCoreApp` 在 `CoreCfg != nil` 时创建 `fiber.App` 后立即返回，没有给 `cf.json` 赋值；后续 `RegisterAppMiddleware` 无条件调用 `cf.json.Marshal` 配置 recovery。

当前结果：使用公开的自定义 CoreCfg 选项并继续标准启动链时，可能出现 nil pointer panic。

最小方向：保持自定义 Fiber 配置不变，只保证 codec 选择和 `cf.json` 初始化不被提前返回跳过；扩展现有 CoreCfg 测试覆盖中间件注册。

最小验证：

```bash
go test . -run 'TestCoreWithFiber_WithCoreCfg' -count=1
```

### P0-3：RespInfo 链式复用残留旧 Data

证据：`response/response_impl.go` 的方法 `SuccessWithData()` 在没有参数时不清空 `Data`，`ErrorCustom()` 也只改 `Code`/`Msg`。同一对象先带成功数据、再转错误响应时，旧数据可能进入错误响应。

最小方向：无参成功和错误状态都显式清空 `Data`；只改现有方法和回归测试。

最小验证：

```bash
go test ./response -run 'TestRespInfo_.*ClearsData' -count=1
```

### P0-4：validation 英文 fallback 可能为 nil

> 2026-07-20 已完成：`component/validate/validate_wrapper.go` 的 `NewWrap` 现在无条件注册英文验证器/翻译器，`langFlags` 仅包含非英语或全部未识别值时，`GetValidate()`/`GetTranslator()` 不再返回 nil。回归测试见 `component/validate/validate_wrapper_test.go` 的 `TestValidate_EnglishFallbackAlwaysAvailableWhenLangFlagsOmitsEnglish` 与 `TestValidate_EnglishFallbackAlwaysAvailableWhenLangFlagsAllUnrecognized`。

证据：`validate.NewWrap` 只有在未配置语言或明确包含 `en` 时才注册英文验证器；`GetValidate`/`GetTranslator` 对未知语言固定回退 `en`。配置只包含中文或全部是未知语言时，fallback 可能返回 nil。

最小方向：构造 Wrap 时始终保证默认英文验证器和 translator 存在；不改变现有语言选择 API。

最小验证：

```bash
go test ./component/validate -run 'TestValidate_.*Fallback' -count=1
```

### P0-5：JSON codec provider 重复初始化返回 nil

> 2026-07-21 已完成：`json_fiber_provider.go`、`json_gin_provider.go`、`json_sonic_fiber_provider.go`、`json_sonic_gin_provider.go` 的四个 provider 新增未导出字段缓存首次初始化构造出的 codec 实例，`Initialize()` 的 `StateLoaded` 分支现在返回该缓存值，不再返回 `nil`；首次初始化路径的构造逻辑与副作用赋值（如 `ginJson.API = jcodec`）保持原位置不变。回归测试见 `json_codec_providers_test.go` 的 `TestJsonJCodecFiberProvider_RepeatedInitializeReturnsSameCodec`、`TestJsonJCodecGinProvider_RepeatedInitializeReturnsSameCodec`、`TestSonicJCodecFiberProvider_RepeatedInitializeReturnsSameCodec`、`TestSonicJCodecGinProvider_RepeatedInitializeReturnsSameCodec`。

证据：Fiber/Gin 的 Std/Sonic 四个 codec provider 在 `StateLoaded` 时返回 `(nil, nil)`；默认 Provider 集合是进程级单例，manager 再次加载同一 provider 时会拿到 nil，Core 随后可能在类型断言处 panic。

最小方向：provider 重入时返回已有/可重新取得的 codec，而不是成功加 nil；不改变 manager 或 Provider 接口。

最小验证：

```bash
go test . -run 'TestJSONCodecProviders_RepeatedInitialize' -count=1
```

### P1-1：LocalCache/RedisDb 并发 Close 双关

> 2026-07-21 已完成：`component/cache/cachelocal/local_cache.go`、`component/cache/cacheremote/redis_cache.go` 的 `Close()` 改用 `atomic.Bool.CompareAndSwap` 原子选出唯一关闭者，避免并发重入执行关闭副作用；`LocalCache` 第二次 Close 返回 `nil`、`RedisDb` 第二次 Close 返回 `cache.ErrCacheClosed` 的既有语义保持不变。回归测试见 `component/cache/cachelocal/local_cache_test.go` 的 `TestLocalCache_ConcurrentCloseIsIdempotentAndPanicFree`、`component/cache/cacheremote/redis_cache_test.go` 的 `TestRedisDb_CloseAndPostCloseErrors`、`TestRedisDb_ConcurrentCloseIsIdempotentAndPanicFree`。

证据：两个 `Close` 都使用“Load closed → 关闭 client → Store closed”或“Load → Store → Close”的非原子组合。并发调用可能让多个调用者同时进入底层 Close。

最小方向：使用现有 `atomic.Bool.CompareAndSwap` 选出唯一关闭者；保持 LocalCache 第二次 Close 返回 nil、RedisDb 第二次 Close 返回 `ErrCacheClosed` 的既有语义。

边界：该补丁只解决 Close/Close 幂等竞争，不声称解决所有操作与 Close 并发的完整生命周期。

最小验证：

```bash
go test -race ./component/cache/cachelocal ./component/cache/cacheremote -count=1
```

### P1-2：MySQL 初次 Ping 失败泄漏已创建连接池

> 2026-07-21 已完成：`component/database/dbmysql/mysql.go` 的 `NewClient` 在 `PingContext` 失败路径新增 `sqlDb.Close()` 清理已创建的连接池，`Close()` 自身的错误仅记录日志、不覆盖原始 ping 错误，`gorm.Open`/`db.DB()` 失败路径（尚无可关闭的连接池句柄）与 ping 成功路径行为不变。回归测试见 `component/database/dbmysql/mysql_test.go` 的 `TestNewClient_PingFailureReturnsError`。

证据：`dbmysql.NewClient` 已经取得 `sql.DB`，但 `PingContext` 失败后直接返回，没有关闭该连接池。

最小方向：仅在初次 Ping 失败路径关闭已经创建的 `sql.DB`，并保留原始连接错误作为主要返回错误。

最小验证：增加一个失败连接测试，随后运行：

```bash
go test ./component/database/dbmysql -count=1
```

## 六、只改测试/CI 的完整性补丁

### P1-3：Gin loopback HTTP/TLS 握手

> 2026-07-21 已完成：`core_starter_init_test.go` 新增 `TestCoreInit_GinLoopbackTLSHandshake`，复用既有临时自签名证书生成逻辑（提取为 `generateTask4SelfSignedCert` 辅助函数，`TestCoreInit_GinTLSLoadsConfiguredCertificate` 同步改用该辅助函数，行为不变），以 `127.0.0.1:0` 建立 loopback listener，通过 `http.Server.ServeTLS` 驱动一次真实 TLS 握手与 HTTP 请求-响应，并验证 `Shutdown` 在 3 秒内正常完成、服务 goroutine 以 `http.ErrServerClosed` 正常退出。未新建文件，未修改任何生产代码。回归测试见 `core_starter_init_test.go` 的 `TestCoreInit_GinLoopbackTLSHandshake`。

复用现有临时证书生成逻辑，以 `127.0.0.1:0` 创建 listener，实际执行一次 HTTP/TLS 请求，并在 3 秒内 Shutdown。该测试只验证 Gin server 配置确实可以完成真实握手，不修改生产启动入口，也不把 `tls.enable=true` 解释成用户必须使用 HTTPS。

### P1-4：复用 smoke 服务的 live integration

推荐给 live tests 增加显式 `liveintegration` build tag，并只在 smoke job 中定向运行。普通 quality/race 不连接外部服务。

推荐拆成小批次：

1. Redis cache：Ping、SET、GET、DEL、Close 后失败；使用唯一 key。
2. asynq：使用 Redis DB 15，入队一个唯一 task，确认 worker 消费，并设置 10 秒总等待和 3 秒 Shutdown。
3. MySQL：创建唯一临时表，写入/读取后由 `t.Cleanup` 删除并关闭连接。
4. MongoDB：创建唯一临时 collection，写入/读取后删除并关闭 client。

CI 不需要新增容器。可以在现有 smoke job 增加类似的定向步骤，并给整个 live test 命令设置约 90 秒上限。所有测试必须使用唯一资源名、短 context 和 `t.Cleanup`，避免并发 PR 或失败重跑相互污染。

不推荐：

- 只运行 `redis-cli`、`mysql` 或 `mongosh` 探针，因为它只能证明容器正常，不能证明仓库 client 配置和读写路径正常；
- 为测试增加业务示例 API，因为这会把组件验证耦合到示例路由；
- 第一批同时覆盖所有重建、故障注入和高并发场景，因为这会把快速验证扩成长期集成项目。

## 七、需要单独设计批准的有限改动

### L2 Wait 完整 flush

当前 `Level2Cache.Wait` 只等待 local/remote 子缓存，没有等待已经提交到 ants pool 的任务；`Close` 轮询 `Running()` 也不能可靠表示队列已排空。

若以后批准，应只在 Level2Cache 内部增加：

- 提交闸门：关闭开始后拒绝新任务；
- 内部任务计数：提交成功后计数，任务结束时递减；
- `Wait`：先等已提交任务，再调用子缓存 `Wait`；
- `Close`：停止提交、有限等待、释放池、关闭子缓存并聚合错误。

该方向虽然不需要改公共签名，但涉及提交失败、Wait/Close 竞争和超时语义，超过“一个条件分支”的普通补丁，因此继续保留为待单独审核的有限重构，不自动执行。

## 八、继续暂缓的方向

以下问题不能在当前约束下安全快速闭合：

- 新增 Web/CLI `Run(ctx) error` 或修改 `RunCommandStarter` 返回签名；
- task worker、dispatcher、CLI 的统一关闭编排；
- GlobalManager owner/locator 拆分、完整状态机和调用方引用存活期；
- GlobalManager 自动关闭 Rebuild 的旧实例；当前 wrapper 可能原地替换 client，自动关闭“旧实例”可能关闭刚替换的新 client；
- MySQL/MongoDB/Redis 重建时旧 client 的并发替换和读侧锁；
- 跨 Fiber/Gin/task/cache/database/writer 的统一 shutdown registry；
- MsgPack/Protobuf 内容协商。

这些内容继续作为已知限制，不进入快速实施队列。

## 九、推荐执行顺序

每一项都必须独立取证、独立工作树、独立审核，不能把下列清单合成一个大分支：

1. `AsyncChannelWriter` 并发 Write/Close 和重复 Close。
2. Fiber 自定义 `CoreCfg` 的 nil codec 路径。
3. `RespInfo` 旧 Data 残留。
4. validation 默认英文 fallback（已完成）。
5. JSON codec provider 重复初始化返回 nil（已完成）。
6. LocalCache/RedisDb Close 的 CAS 幂等补丁（已完成）。
7. MySQL 初次 Ping 失败关闭连接池（已完成）。
8. Gin loopback HTTP/TLS 握手测试（已完成）。
9. Redis cache + asynq live integration。
10. MySQL、MongoDB live integration，各自独立提交。
11. 重新核对逐能力状态；只有某项能力满足创建、运行、失败、关闭、验证和兼容承诺后，才单独讨论从实验性晋级。
12. L2 Wait flush 仅在单独设计审核通过后进入实现。

## 十、每个快速补丁的准入和停止条件

准入条件：

- 有当前源码或可复现测试直接证明问题；
- 保持公共方法签名、现有抽象和调用结构；
- 生产代码集中在一个文件或一条短调用路径；
- 可以用一个 focused test 或 race test 验证；
- 功能代码在独立 Git 工作树中由子代理驱动，审核前不合入 main。

停止条件：

- 需要新增公共运行入口；
- 需要统一多个组件的生命周期；
- 需要定义新的跨组件资源所有权；
- 需要迁移 API、移动目录或重命名抽象；
- focused test 无法描述预期行为，必须先做更大的语义设计。

满足停止条件时，只更新研究结论，不继续扩大代码修改。
