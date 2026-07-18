# README“当前状态”与项目成熟度优化代办

> 创建日期：2026-07-18
> 状态：P0 已执行；P1 部分执行；P2–P3 待执行
> 背景分析：[README“当前状态”与代码库成熟度分析](readme-current-status-analysis-2026-07-17.md)
> 适用基线：`main@385eb2c`（`v1.0.5-19-g385eb2c`）；执行前重新确认 HEAD 与文档、测试现状。

## 目标

让 README 的“当前状态”能够准确回答以下问题，并让关键声明受到自动验证：

1. 能力是否已有可达实现；
2. 能力是实验性还是稳定公共能力；
3. 能力是自动启用、默认集合还是应用显式装配；
4. 创建、运行、失败、关闭四段生命周期完成到什么程度；
5. 通过了单元、竞态、外部集成或 smoke 中的哪些验证。

## 执行原则

- [ ] “已接入”只表示存在明确入口和可达运行路径，不等同于生产就绪。
- [ ] 状态调整必须同时核对源码调用链、测试、CI 和示例，不以目录、类型、配置键或单个测试作为依据。
- [ ] README 只保留稳定摘要；逐能力限制放在 `docs/reference/feature-status.md`，调查记录保存在 `.codegraph-qa-out/`。
- [ ] 每个实验性能力独立收敛；不在一次变更中混合生命周期、缓存语义、数据库 client 替换和公共 API 迁移。
- [ ] 只有达到本文“晋级门槛”的能力才可标记为稳定公共能力。

## 优先级总览

| 优先级 | 专题 | 工作量 | 主要收益 | 前置依赖 |
|---|---|---:|---|---|
| P0 | 修正文档事实与状态模型 | 小 | 恢复 README 可信度 | 无 |
| P0 | 将现有测试变成 CI 门禁 | 小到中 | 防止状态说明再次漂移 | 先修 vet 或分阶段启用 |
| P1 | 统一运行、错误与关闭生命周期 | 大 | 同时改善多数实验性能力 | 先完成生命周期设计决策 |
| P1 | 明确 v1 API 与内部工具政策 | 中到大 | 降低下游兼容风险 | 需要维护者决策 |
| P2 | Gin、缓存、任务、CLI、数据库、二进制响应专项 | 中 | 逐项从实验性晋级 | 依赖 P1 的公共生命周期契约 |
| P3 | 最小示例与占位治理 | 小到中 | 改善首次体验、收窄能力叙事 | 可与 P1/P2 并行 |

## P0：修正文档事实与状态模型

### P0.1 当前验证事实

- [x] 删除 README 中“`bootstrap` 与 writer 测试已知失败”的过时说明。
- [x] 改为记录可复现命令及验证边界：`go test ./...`、coverage、`go vet ./...`、race、外部服务测试是否进入 CI。
- [x] 明确区分最新发布标签与当前工作树，避免把未发布提交的测试结果写成发布版保证。
- [x] 避免在 README 固化容易过时的失败 case 数量；具体故障记录放入带日期的分析文档或 issue。

完成判据：

- [x] README 的测试说明与当前 HEAD 实测一致。
- [x] 文档没有把 hermetic 测试推导为数据库、Redis、真实监听器、task worker 或 keepalive 已通过集成验证。

### P0.2 状态维度

- [x] 将“实现阶段”和“支持级别”拆开：
  - 实现阶段：`占位` → `已实现` → `已接入`；
  - 支持级别：`实验性` → `稳定公共能力`；
  - `内部工具`作为 API 受众标记，不再与成熟度互斥。
- [x] 在详细功能状态中补充“启用方式”“生命周期完整度”“验证级别”。
- [x] README 使用精简摘要；避免复制详细状态表造成双份事实源。

完成判据：

- [x] 同一能力可以被准确表达为“已接入 + 实验性”或“已实现 + 内部工具”。
- [x] 读者无需从“已接入”猜测生产稳定性。

### P0.3 修正 GlobalManager 限制描述

- [x] 删除或改写已过时的“`Release` 存 nil、初始化失败状态残留”说明。
- [x] 保留实验性状态，并记录仍存在的真实边界：
  - `Get`/`Rebuild`/`Release`/`ClearAll` 并发状态机；
  - 重建时旧实例的关闭责任；
  - `ClearAll` 不逐项 `Close`；
  - GlobalManager 是 owner 还是 locator；
  - 自定义 `FrameStarter` 的 keepalive 取消与退出责任。

完成判据：

- [x] `docs/reference/feature-status.md` 的每条限制都能在当前源码或测试中复核。

## P0：质量门禁

### P0.4 启用真实测试步骤

- [x] 将 `.github/workflows/go1.yml` 中的 `Fake temporary test` 替换为 `go test ./... -count=1`。
- [x] 保留示例构建和 HTTP smoke，但不得用 smoke 替代单元/契约测试。
- [x] 生成 coverage profile，并至少输出总覆盖率；是否设置阈值另行决策。
- [ ] 确认 CI 失败会阻止合并。

### P0.5 vet、race 与服务版本

- [x] 修复当前 `go vet ./...` 的 unkeyed struct literal 问题。
- [x] 将 `go vet ./...` 纳入 CI。
- [x] 将 `go test -race ./... -count=1` 作为独立 job 或定期 job，避免模糊普通测试失败原因。
- [x] 固定 Redis、MongoDB、MySQL CI 镜像版本，避免长期使用 `latest`。

完成判据：

- [x] 普通测试与 vet 在干净环境通过并由 CI 执行。
- [x] race 和外部服务 job 的触发条件、超时和失败政策有明确说明。

## P0 执行记录（2026-07-18）

- 已验证实现 HEAD：`c6ac37801afe643e3b63762a0d0f51861e17e268`。该 SHA 指向 Task 1–3 实现，不指向本执行记录自身的提交。
- `GOCACHE=/tmp/fiberhouse-p0-final-cache go vet ./...`：通过。
- `GOCACHE=/tmp/fiberhouse-p0-final-cache go test ./... -count=1 -coverprofile=/tmp/fiberhouse-p0-final-coverage.out -covermode=atomic`：通过。
- `go tool cover -func=/tmp/fiberhouse-p0-final-coverage.out | tee /tmp/fiberhouse-p0-final-coverage.txt`：失败，原因是沙箱默认 Go 缓存只读（`read-only file system`）。使用生成 coverage profile 时的同一可写缓存重试：`GOCACHE=/tmp/fiberhouse-p0-final-cache go tool cover -func=/tmp/fiberhouse-p0-final-coverage.out | tee /tmp/fiberhouse-p0-final-coverage.txt`，通过；总语句覆盖率为 `46.5%`。
- `grep '^total:' /tmp/fiberhouse-p0-final-coverage.txt`：通过，输出 `total: (statements) 46.5%`。
- `GOCACHE=/tmp/fiberhouse-p0-final-race-cache go test -race ./... -count=1`：通过。
- workflow YAML 语法检查：通过。当前环境未安装 Ruby，因此原 `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/go1.yml", aliases: true); puts "yaml ok"'` 命令未执行成功（`zsh: command not found: ruby`）；使用只读的 PyYAML `yaml.safe_load` 等价解析，输出 `yaml ok (PyYAML)`。
- P0 验收扫描：过时模式扫描通过；精确镜像与门禁扫描通过；`git diff --check` 返回 0（仅输出现有工作树的 LF/CRLF 转换警告）。
- CI job 名称：`quality`、`race`、`smoke`。
- 服务镜像：`redis:8.2.7-bookworm`、`mongo:8.0.26-noble`、`mysql:8.4.10-oraclelinux9`。
- 当前本地验证与 `smoke` workflow 文本检查不构成 Redis、MongoDB、MySQL、task worker 或 keepalive 的 live integration coverage。

分支保护是否将 quality、race、smoke 设为 required status checks 需要在 GitHub 仓库设置中单独确认。

## P1：统一运行、错误与关闭生命周期

### P1.1 先确定公共契约

- [ ] 决定 Web/CLI 新运行入口是否采用 `Run(ctx context.Context) error` 或等价形式。
- [ ] 列出哪些错误可返回、哪些属于不可恢复编程错误；减少可恢复路径上的 `panic`/`fatal`。
- [ ] 保持 v1 兼容：优先新增入口并迁移内部调用，不静默改变已有函数语义。

### P1.2 有序关闭

- [ ] 设计 shutdown registry/closer stack 和所有权规则。
- [ ] 固定关闭顺序：停止接收流量 → 停止生产者 → 停止 worker/keepalive → 关闭 cache/client/writer → 清理 locator/container。
- [ ] 为每一步定义超时、错误聚合和重复关闭语义。
- [ ] 消费 `ServerShutdownBefore`/`ServerShutdownAfter` location；若不采用则删除未兑现的承诺。
- [ ] Fiber 与 Gin 使用同一套生命周期契约，仅保留引擎差异。

### P1.3 GlobalManager 所有权与并发状态机

- [ ] 明确 GlobalManager 是资源 owner、locator，或拆成两个抽象。
- [ ] 定义并测试 `Register`、`Get`、`Rebuild`、`Release`、`Unregister`、`ClearAll` 的合法状态转移。
- [ ] 定义 Rebuild 成功后旧实例的关闭行为；避免无主 client/连接池泄漏。
- [x] 禁止在已使用的 `sync.Map` 上通过结构赋值实现并发清空，采用可证明安全的清理方式。
- [x] 为 keepalive 增加 context/cancel、等待退出和重复停止语义。

完成判据：

- [ ] 创建、运行、启动失败、运行失败、正常关闭、超时关闭均有契约测试。
- [ ] 相关并发测试通过 race。
- [ ] Fiber、Gin、CLI 不再各自复制不一致的资源回收逻辑。

## P1-A 执行记录（2026-07-18）

- 已验证实现 HEAD：`00ed776`。该 SHA 指向 Task 1–3 实现与测试，不指向本执行记录自身的提交。
- `GlobalManager.ClearAll(true)` 已改为在原 `sync.Map` 上调用 `Clear`，仍只删除条目，不调用 `Release`、`ReleaseAll` 或资源 `Close`。
- 默认 `FrameApplication` keepalive 已具备取消、等待退出和重复停止语义；内置 Fiber/Gin 在 deletion-only 清空前停止并等待它。自定义 `FrameStarter` 的 keepalive 生命周期仍由自定义实现负责。
- `Get`/`Rebuild`/`Release` 并发状态机、`Rebuild` 旧实例退役、GlobalManager owner/locator 边界、共享 alias/组合资源所有权和 task lifecycle 均未在 P1-A 解决。
- `GOCACHE=/tmp/fiberhouse-p1a-task4-audit-task1 go test -race ./globalmanager -count=1`：通过。
- `GOCACHE=/tmp/fiberhouse-p1a-task4-audit-task2 go test -race . -run 'TestFrameApplication_' -count=1`：通过。
- `GOCACHE=/tmp/fiberhouse-p1a-task4-audit-task3 go test -race . -run 'Test(ClearApplicationGlobals|StopFrameHealthCheck)' -count=1`：通过。
- `GOCACHE=/tmp/fiberhouse-p1a-task3-branchfix go test . -count=1`：通过。
- `GOCACHE=/tmp/fiberhouse-p1a-task3-branchfix-race go test -race . -count=1`：通过。
- `GOCACHE=/tmp/fiberhouse-p1a-task3-branchfix go vet .`：通过。
- `ast-grep run --pattern '$_CTX.GetContainer().ClearAll(true)' --lang go core_fiber_starter_impl.go core_gin_starter_impl.go`：无匹配，按 ast-grep 的无匹配语义退出 1。
- `ast-grep run --pattern 'clearApplicationGlobals($_CTX)' --lang go core_fiber_starter_impl.go core_gin_starter_impl.go`：通过，Fiber/Gin 各匹配一次。
- `ast-grep run --pattern 'clearApplicationGlobals($_CTX)' --lang go --context 2 core_fiber_starter_impl.go`：通过；上下文显示 Fiber 按清理全局对象、记录最终 shutdown 日志、关闭 logger 的顺序执行。

本记录不预先声明最终全仓 `go vet ./...`、`go test ./... -count=1` 或 `go test -race ./... -count=1` 已通过；这些命令由隔离分支的最终验收 fresh 执行。

## P1-B 执行记录（2026-07-18）

- 已验证实现 HEAD：`945d702`。该 SHA 指向 entry 级维护门禁及其契约测试，不指向本执行记录自身的提交。
- 同一已注册 entry generation 内，`Rebuild` 与 `Release` 共享 fail-fast 维护门禁；任一维护回调运行时，同 entry 的冲突维护调用返回 busy error，不等待也不进入第二个回调。删除后以同名重新注册会创建新的 entry generation，不属于该门禁的协调范围。
- busy error 仍是普通的实验性错误；其 private sentinel 只供当前包内实现和测试识别，不是稳定导出的 retry 分类契约。
- `Unregister` 或 `ClearAll(true)` 只影响后续查找，不取消已经取得 entry 并开始执行的 `Get` initializer；`ClearAll` 仍是 deletion-only，不逐项 `Close`。
- 本轮没有定义调用方已取得引用的存活期，也没有解决 `Rebuild` 旧实例退役。当前数据库与缓存 wrapper 会在 rebuild callback 中原地替换内部 client；管理器若自动关闭所谓“旧实例”，可能通过同一 wrapper 关闭刚替换的新 client，因此在明确 ownership 与替换协议前并不安全。
- GlobalManager 的 owner/locator 边界、共享 alias/组合资源所有权、task lifecycle、自定义 `FrameStarter` 的 keepalive 责任和统一关闭链仍未解决；P1.3 的完整合法状态转换与旧实例关闭项保持未完成。
- `GOCACHE=/tmp/fiberhouse-p1b-task1 go test ./globalmanager -run '^TestGetInitialization_RemovalOnlyAffectsFutureLookups$' -count=50`：通过。
- `GOCACHE=/tmp/fiberhouse-p1b-task1-race go test -race ./globalmanager -run '^TestGetInitialization_RemovalOnlyAffectsFutureLookups$' -count=10`：通过。
- `GOCACHE=/tmp/fiberhouse-p1b-task2-final-focused go test ./globalmanager -run 'Test(Rebuild_ConcurrentMaintenance|Release_ConcurrentMaintenance|Maintenance_(RebuildAndReleaseConflict|SameEntryReentry|DifferentKeys)|Entry_BeginMaintenance)' -count=50`：通过。
- `GOCACHE=/tmp/fiberhouse-p1b-task2-final-race go test -race ./globalmanager -count=1`：通过。
- `GOCACHE=/tmp/fiberhouse-p1b-task3 go test ./globalmanager -run '^TestMaintenanceGate_ReleasesAfter' -count=50`：通过。
- `GOCACHE=/tmp/fiberhouse-p1b-task3-race go test -race ./globalmanager -run '^TestMaintenanceGate_ReleasesAfter' -count=20`：通过。

本记录只确认 P1-B 的 entry generation 级 fail-fast 门禁、错误/恐慌后的门禁释放，以及删除期间已开始初始化的行为；不预先声明隔离分支的最终全仓验收结果。

## P1：v1 API 治理

- [ ] 发布明确的 v1 兼容政策：稳定 API、实验性 API、弃用周期、允许的破坏性变更范围。
- [ ] 盘点 bufferpool、Dig、writer、converter 等公开 package 的真实下游定位。
- [ ] 对内部工具选择并记录迁移方案：
  - 正式纳入公共 API；或
  - 先弃用并提供替代，再在下一主版本迁入 `internal/`。
- [ ] 解决 `Default()` 与 `DefaultProviders()`/`DefaultPManagers()` 的语义落差；v1 优先新增含完整默认装配的明确入口。

完成判据：

- [ ] README、Go doc 和功能状态对公共/实验性/内部 API 的承诺一致。
- [ ] 不再仅靠“内部工具”文案规避公开 Go package 的兼容问题。

## P2：分能力专题

### Gin

- [ ] 删除证书路径有效时的 panic。
- [ ] 使用正确的 TLS serve 路径，并覆盖证书成功、证书失败、监听失败、shutdown。
- [ ] 评估 Gin package 级 JSON codec 的进程级副作用。

### 数据库与远端缓存

- [ ] 为 MySQL、MongoDB、Redis 增加容器化 live integration test。
- [ ] 覆盖连接失败、健康检查、重建、旧 client 关闭、并发读取与替换。
- [ ] 明确启动强制初始化、重试、降级与失败传播策略。

### L2 缓存

- [ ] 先定义 loader、singleflight、Bloom filter、circuit breaker、miss 和 fallback 的组合语义。
- [ ] 覆盖同步/异步写、并发 miss、关闭中请求、部分依赖失败。
- [ ] 明确内部 goroutine/pool 的创建、等待和关闭责任。

### 异步任务与 CLI

- [ ] task worker/dispatcher 的启动错误返回调用方。
- [ ] 将 worker、dispatcher、Redis client 纳入统一关闭链。
- [ ] CLI 传播 `AppCoreRun` 返回值，不再丢弃错误。
- [ ] CLI 命令结束时执行资源回收；明确健康检查是一次检查还是持续 keepalive。

### MsgPack / Protobuf HTTP 响应

- [ ] 明确 `Accept` 与 `Content-Type` 优先级、多值、q-value、未知媒体类型及 fallback。
- [ ] 明确其为 HTTP body 编码能力，不使用 RPC 表述。
- [ ] 若不实现完整内容协商，在 API 命名和文档中收窄承诺。

每个专题的完成判据：

- [ ] 独立设计说明、失败路径测试、并发/关闭测试、文档同步均完成。
- [ ] 不因单个 happy-path 测试通过就调整成熟度。

## P3：示例与占位治理

- [ ] 增加不依赖 MySQL、MongoDB、Redis 的最小可运行 Fiber 示例。
- [ ] 将“核心启动体验”和“完整外部集成演示”拆成两个入口。
- [ ] 为示例固定凭据、debug 配置和关闭缺口添加醒目标记。
- [ ] 为 plugins、RPC、MQ、通用 i18n 决定：进入带 owner/里程碑的 roadmap，或退出 README 主能力叙事。
- [ ] 不因存在接口、配置键、生成类型或空目录而提升占位能力状态。

## 能力晋级门槛

只有同时满足以下条件，才把能力从“实验性”调整为“稳定公共能力”：

- [ ] 创建、运行、失败、关闭四条路径均有明确契约。
- [ ] 公开 API、默认行为、显式启用方式和兼容承诺无歧义。
- [ ] 单元/契约测试进入 CI。
- [ ] 涉及并发的路径通过 race；涉及外部依赖的路径通过可重复集成测试。
- [ ] 可恢复错误不依赖 `panic`/`fatal` 表达。
- [ ] 示例实际执行该能力，并说明资源所有权和限制。
- [ ] README、功能状态页、专题指南和 Go doc 已同步。

## 推荐执行顺序

1. [x] P0.1–P0.3：先修正文档事实和状态模型。
2. [x] P0.4–P0.5：把当前测试、vet 和基础 smoke 变成真实门禁。
3. [ ] P1.1：形成生命周期与错误模型设计决策。
4. [ ] P1.2–P1.3：实现统一关闭链与 GlobalManager 状态机。
5. [ ] 完成 v1 API 治理决策，确定兼容迁移方式。
6. [ ] Gin、数据库/缓存、任务/CLI、二进制响应分别立项执行。
7. [ ] 按晋级门槛逐项更新状态，最后回写 README 摘要。
