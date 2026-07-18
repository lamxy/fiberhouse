# README“当前状态”与项目成熟度优化代办

> 创建日期：2026-07-18
> 状态：P0、P1-A、P1-B、Gin TLS、CLI 日志局部补丁与状态文档同步已执行；当前无自动承接的局部代码任务
> 背景分析：[README“当前状态”与代码库成熟度分析](readme-current-status-analysis-2026-07-17.md)
> 最新收口基线：`main@5faeabf`；执行前重新确认 HEAD 与文档、测试现状。
> 后续优化约束（2026-07-18）：禁止重构和大范围修改。历史分析、spec、plan 与执行记录只作为审计材料，不构成后续实施授权；未来任务以本文当前活动项为准。

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
- [ ] 只有当前源码、测试、CI 和文档证据一致时，才调整能力状态。

## 无重构硬约束（2026-07-18）

- 不新增 `Run(ctx context.Context) error` 或等价入口，不修改现有公共接口、方法签名和调用链。
- 不新增 shutdown registry、closer stack、生命周期协调器或其他统一关闭基础设施。
- 不拆分 GlobalManager owner/locator，不实施完整状态机重写，不自动扩大旧实例退役语义。
- 不迁移公开 package 到 `internal/`，不新增替代性的默认装配 API。
- 不顺带抽象、改名、移动文件、统一相似实现或清理邻近代码。
- 只能处理有明确当前证据、位于一个函数或一条短调用路径、能在原接口内修复的单一缺陷。
- 无法局部修复的问题保留为限制，不进入实施计划。

## 优先级总览

| 优先级 | 专题 | 工作量 | 主要收益 | 前置依赖 |
|---|---|---:|---|---|
| P0 | 修正文档事实与状态模型 | 小 | 恢复 README 可信度 | 无 |
| P0 | 将现有测试变成 CI 门禁 | 小到中 | 防止状态说明再次漂移 | 先修 vet 或分阶段启用 |
| P1 | README“当前状态”快速事实修正 | 小 | 及时修正失实、过时或过宽表述 | 当前源码、测试和 CI 证据 |
| P2 | 已证实的单点代码缺陷 | 小 | 在原实现上修复当前错误行为 | 必须先有复现或静态证据 |
| P3 | 单能力局部验证与文档补丁 | 小 | 补足一个明确证据缺口 | 不依赖跨组件修改 |

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

## P1：README“当前状态”快速修正与局部补丁

### 已撤销方向

以下方向不再是活动任务，也不替换为另一套架构方案：新增 Web/CLI 运行入口、统一关闭链或生命周期协调器、Fiber/Gin/CLI 生命周期统一、GlobalManager owner/locator 拆分、完整状态机重写、公开 package 迁移、默认装配 API 扩展，以及 MsgPack/Protobuf 内容协商调查或修改。

### 快速任务准入

- 优先检查仓库根目录 `README.md` 的“当前状态”是否与当前源码、测试、CI 和 `docs/reference/feature-status.md` 一致。
- 文档失实或过时：直接做最小文字补丁，不扩大到相邻章节重写。
- 缺少证据：只补一个明确的测试、命令或引用，不建立新的验证框架。
- 真实代码缺陷：必须指向一个具体函数或一条短调用路径，并能保持公开签名、现有抽象和调用结构不变。
- 一次只处理一个问题；开始前列出预期修改文件，不能夹带整理、抽象、迁移或相邻修复。
- 无法在原实现内局部修复时停止，只在功能状态页记录限制。

### 当前保留限制

- 某些现有入口的错误记录、panic/fatal 和返回行为并不完全一致。
- 部分资源缺少关闭验证或可重复外部集成测试。
- GlobalManager 的引用存活期、旧实例退役和 deletion-only 清理边界仍有限制。
- 这些事实只说明当前边界，不授权公共接口或跨组件改造。

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

## Gin TLS 与状态文档执行记录（2026-07-18）

- `b6648d6`：有效证书可填充 `TLSConfig`，运行阶段按现有字段选择 `ListenAndServeTLS("", "")` 或 `ListenAndServe()`。
- `e75f425`：无效非空证书保持 fail-stop；缺失路径保持记录错误并保留 HTTP 路径的既有行为。
- `cf67d58`：README、功能状态、Web runtime、入门、示例、启动生命周期和 CodeGraph QA 文档同步到当前 TLS 事实。
- Gin 仍维持实验性：尚无真实 listener/握手集成验证，且缺失证书路径仍可能保留 HTTP 路径。

## CLI 日志补丁与后续清单收口（2026-07-18）

- `5faeabf`：CLI 健康检查在重建失败后继续遍历，但不再为同一条目记录矛盾的 rebuild success；回归测试按 RED→GREEN 完成，合入后的 `go test ./commandstarter -count=1` 通过。
- L2 local metrics 已确认只调用内置 `*cachelocal.LocalCache` 的具体能力，不属于通用 `cache.Cache` 契约；保留具体类型断言，不新增代码补丁。
- `main@5faeabf` 的 README 与功能状态重新核对后，能力分级、启用方式和验证边界仍准确；仅需对齐 Fiber/Gin 状态维度及缓存指南的 metrics 边界。
- 当前没有自动承接的局部代码任务；L2 `Wait` 完整 flush 仍只是待单独审核的有限重构建议。

## 快速调查候选

以下内容只允许快速核对，不自动授权代码修改：

1. `README.md`“当前状态”中的能力、成熟度、启用方式和验证说明是否仍准确。
2. README 与 `docs/reference/feature-status.md` 是否出现同一能力的事实冲突。
3. README 记录的测试、vet、race、CI 或外部服务验证范围是否过时或表述过宽。
4. 某条状态限制是否对应一个能在原函数内修复的单点缺陷。

调查结果优先选择“不修改”或文档补丁。只有确认文档无法准确描述、且代码存在可复现的局部错误时，才单独提出最小代码补丁并重新取得批准。

## 快速补丁判据

- 文档补丁：只改失实句子、状态标签、证据链接或验证边界。
- 测试补丁：只补一个缺失行为，不建立新测试框架。
- 代码补丁：保持现有接口和结构，只改一个明确缺陷。
- 修改后运行与变更直接相关的最小检查；不因纯文档修改运行全仓测试。
- 审核发现需要重构、大范围修改或跨组件协调时，立即停止。

## 推荐执行顺序

1. [x] P0：修正文档事实、状态模型和质量门禁。
2. [x] P1-A/P1-B：完成已审核的局部生命周期与 maintenance gate 修复。
3. [x] 快速核对根目录 README“当前状态”，并完成 Gin TLS 事实同步。
4. [ ] 对问题优先做最小文档补丁；必要时提出一个原实现内的最小测试或代码补丁。
5. [ ] 修改代码前单独取得批准；修改后只运行直接相关检查并统一查看清单。

不存在自动承接的架构任务；每个代码补丁都必须重新确认范围和批准。
