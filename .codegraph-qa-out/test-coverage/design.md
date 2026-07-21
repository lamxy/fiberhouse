# FiberHouse 关键测试覆盖提升设计

日期：2026-07-17

基线提交：`8c10a025fdbc046b55066d1a46e1456b5644508f`

工作分支：`test/meaningful-coverage-20260717`

## 目标

在不追求 100% 覆盖率的前提下，建立一套默认可离线、可重复、可在竞态检测下运行的测试集，覆盖 FiberHouse 作为可切换 HTTP 核心 Web 框架的关键契约，并重写已经与实现脱节或依赖固定睡眠的旧测试。

成功标准：

1. `go test ./... -count=1` 通过，并连续多次运行不再出现当前 bootstrap/writer flaky。
2. `go test -race ./... -count=1` 通过，或将不可在本任务内安全修复的既有竞态以精确复现和边界记录下来。
3. 全仓 statement coverage 从基线 6.8% 显著提高；默认无外部服务的 hermetic 范围从 10.0% 提升到至少 55%，并优先保证下面列出的关键契约，而不是为了百分比测试占位代码。
4. Provider/Manager/Location、启动顺序、Fiber/Gin core 选择、请求上下文、错误/recover/response、GlobalManager 生命周期和关键基础组件均有正常、错误和边界用例。
5. 默认测试不监听真实端口，不访问 Redis/MySQL/Mongo，不依赖固定 `time.Sleep` 猜测异步完成。

覆盖口径：

- 全仓：覆盖 profile 中全部 5,208 个 statement。
- library scope：排除 `example_application/**`、`example_main/**`、生成的 `response/pb/**` 和尚为占位的 `plugins/**`；基线 352/3,898（9.0%）。
- hermetic scope：在 library scope 上再排除真实数据库 wrapper 和 `component/cache/cacheremote/**`；基线 352/3,533（10.0%）。

## 调研依据

- CodeGraph MCP 用于启动链、动态接口分派、双核心调用路径和 blast radius。
- ast-grep 统计到 289 个包级生产函数、762 个方法匹配、82 个 `Test*`，并列出仅 11 个现有 `_test.go` 文件。
- 初始全量测试复现：
  - `bootstrap.Test_Config_EnvOverrideAndSingleton` 加载不存在的 `application_dev.yml`；测试却写了 `application_web_dev.yml`。
  - async diode writer 的写入/多写/并发/关闭测试在 100ms、200ms 或恰好 1s 后读盘，与 1s flush ticker 竞争。
  - `WriteAfterClose` 仍断言成功，和当前明确返回 `(0, error)` 的契约相反。
  - `InvalidFilePath` 在 POSIX 上把 `D:/...` 当相对路径，污染仓库且没有有效断言。
- 三个只读子代理分别审计了生命周期、双 HTTP 核心和组件；其关键发现已汇总到本设计的风险与任务边界中。

## 方案比较

### 方案 A：按覆盖率排名广撒网

从 statement 数量最大的未覆盖文件开始，为示例 CRUD、数据库 wrapper、占位插件和简单 getter 大量补测试。数字上升最快，但会产生大量低价值断言，还可能迫使默认测试依赖外部服务。否决。

### 方案 B：核心契约优先、hermetic 分层（采用）

先修旧测试，再覆盖框架装配与双核心一致性；通过内存 fake、Fiber `app.Test`、Gin `httptest.ResponseRecorder`、临时目录和进程内组件测试关键路径。测试发现确定性缺陷时，先建立失败回归测试，再只做使契约成立的最小生产修复。该方案兼顾价值、速度和长期稳定性。

### 方案 C：容器化端到端集成

启动 Redis、MySQL、Mongo 和真实 HTTP listener，覆盖完整示例应用。真实性最高，但测试慢、环境要求高，且会把当前主要问题（框架内部契约缺失）埋在外部基础设施噪声里。本轮只记录未来 integration test 范围，不作为默认门禁。

## 测试架构

### 第一层：纯函数与本地状态

覆盖 type/location registry、Provider/Manager 状态机、BootConfig options、DefaultStorage、JSON codec、buffer pool、utils、exception、response 编解码、validator 注册和 CacheOption。用表驱动测试覆盖正常值、零值、重复、错误类型和 panic 边界。

### 第二层：内存 fake 的组件协作

使用 recording provider/manager/starter/cache/logger/context，验证：

- `RunApplicationStarter` 的严格顺序与 manager 透传；
- FrameApplication 的 app-state guard、注册器和全局 initializer 行为；
- GlobalManager 的失败重试、Release/Rebuild/health/clear 生命周期；
- Level2Cache 的本地/远端顺序、关闭与等待语义；
- PayloadBase、task mux context 注入和命中/回退分支。

Fake 只模拟真实接口契约，不断言 fake 自身调用以替代可观察结果；当调用顺序本身就是框架公开契约时，使用 recording fake 是必要且允许的。

### 第三层：无监听端口的双核心 HTTP 合约

- Fiber 使用 `app.Test(httptest.NewRequest(...), -1)`。
- Gin 使用 `httptest.NewRecorder` 与 `engine.ServeHTTP`。
- 同一张用例矩阵验证 header、JSON/bytes、错误传播、业务异常、recover 和响应 envelope。
- Gin 的全局 JSON API 或 recover 配置若被修改，测试必须串行且 cleanup 恢复原值。

### 第四层：隔离的外部集成（本轮只记录）

Redis/MySQL/Mongo 网络、认证、连接池、rebuild 与真实 TTL 后续以 build tag 或显式环境变量 gate；默认 `go test ./...` 不执行。

## 生产代码修改边界

测试是本任务的主要交付物。允许修改生产代码的条件同时满足：

1. 关键契约测试先以预期原因失败；
2. 缺陷可确定复现，而不是为了提高覆盖率臆造行为；
3. 修复局部、兼容公开 API，并有回归测试；
4. 不借机重构真实 server signal、数据库 connector 或整个生命周期架构。

优先候选包括：ProviderManager `Unregister` 空实现、DefaultPManager 聚合 nil error、同一 Location 误拒绝不同 Manager、Gin starter provider 误返回 Fiber、GlobalManager Release 的 `atomic.Value.Store(nil)` panic、异常 Throw 选错 key、Level2Cache Close 状态、writer Close 幂等/并发关闭。完整 `RunServer` 可取消 coordinator、真实 TLS/readiness 和外部连接重建属于较大架构变更，本轮以测试档案和后续建议为主，除非可以在不扩大接口的前提下安全修复。

## 旧测试处置

- `bootstrap/bootstrap_test.go`：保留包内 singleton reset helper，fixture 统一使用实际支持的 `application_<env>.yml`；删除不存在的 `appType` 文件选择契约；明确 `APP_ENV -> YAML -> 回写 env -> APP_CONF` 覆盖顺序。
- `async_diode_writer_test.go`：重写为 `Close` 作为完成屏障；删除固定睡眠与伪无效 Windows 路径；关闭后写入断言 `(0, error)`；增加输入 slice 拷贝和幂等关闭测试。
- 现有测试中只 `Logf` 不失败、断言 mock/对象池身份、释放后继续读取池对象的用例，按实际公开行为重写。

## 任务分解

1. 稳定 bootstrap 与 logging writer 旧测试，并用重复运行验证。
2. 覆盖 Provider/Manager/Location、Context/Storage、BootConfig/default/options；对确定性状态机缺陷按红绿循环最小修复。
3. 覆盖 GlobalManager 生命周期、exception/response/codec/utils/bufferpool；修复由回归测试证明的资源与异常映射缺陷。
4. 覆盖 Fiber/Gin context/error adaptor、core provider 选择和无端口 core 初始化合约。
5. 覆盖 recover、response facade 和双核心 HTTP 合约；避免依赖真实 listener。
6. 覆盖 Local/Level2 cache、validate、task/payload/log adaptor 等 hermetic 组件。
7. 覆盖 Frame/command 启动顺序与关键 manager/provider；运行覆盖率缺口分析并补最后一轮高价值用例。
8. 全量、重复、race、覆盖率验证和整分支审查。

每个实现任务由新的子代理完成并提交；随后由独立审查子代理检查规格符合性与代码质量。多个只读审计可并行，多个写共享工作树的实现任务不得并行。

## 风险与缓解

- **进程级 singleton 污染**：同包测试提供严格 cleanup，相关用例串行；能使用 fresh struct/manager 时不走默认 singleton。
- **对象池所有权**：发送/Release 后不再读取对象；不假设 `sync.Pool` 返回同一地址。
- **异步不稳定**：使用 Close、WaitGroup、channel 或带截止时间的条件等待；禁止任意 sleep 作为成功条件。
- **全局 Gin/Fiber 状态**：修改后 cleanup 恢复；不并行。
- **CodeGraph worktree 提示**：索引来自主 worktree，分支开始时源码相同；一旦文件编辑，编辑过的文件以当前 worktree 读取和测试输出为准，不擅自初始化新索引。
- **覆盖率诱导低价值测试**：达标判断同时检查关键契约清单和 diff 质量，百分比只作为辅助退出条件。

## 自审结论

- 无 `TBD`/`TODO` 占位。
- 范围与“默认无外部服务、核心优先”的架构一致。
- 对生产修复授权边界、排除项、覆盖口径和验收命令均已明确。
- 用户已明确要求无需中途干预，因此本次以该授权替代 brainstorming 的交互审批与书面 spec 复审等待点。
