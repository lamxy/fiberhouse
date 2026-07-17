# README 状态与 CI P0 优化设计

## 背景

当前 `main@385eb2c` 的 README 测试说明、功能状态模型和 CI 门禁已经与代码事实发生漂移：`go test ./... -count=1` 与 `go test -race ./... -count=1` 在当前 HEAD 通过，但 README 仍声明 `bootstrap` 与 logging writer 存在已知失败；`go vet ./...` 因 13 个未命名字段的结构体字面量失败；GitHub Actions 的测试步骤仍为占位输出；外部服务镜像使用 `latest`。

本设计只交付 `.codegraph-qa-out/readme-current-status-optimization-todo.md` 的 P0。生命周期统一、GlobalManager 状态机、v1 API 治理以及 Gin、缓存、任务、CLI、数据库、二进制响应等专题留给后续独立设计，避免把文档可信度修复与公共行为变更混在同一批次。

## 目标

1. README 的“当前状态”和“开发与验证”准确表达当前 HEAD 的可复现事实及验证边界。
2. 功能状态页使用正交维度表达实现阶段、支持级别和 API 受众，不再让“已接入”暗示生产稳定。
3. GlobalManager 的限制描述删除已修复结论，仅保留能由当前源码或测试复核的边界。
4. CI 用独立 job 区分普通质量门禁、竞态门禁和外部服务参与的 HTTP smoke。
5. `go vet ./...`、普通测试和竞态测试在当前工作树通过。

## 非目标

- 不新增 coverage 阈值。
- 不把现有 HTTP smoke 描述为数据库、Redis、MongoDB、task worker 或 keepalive 的 live integration test。
- 不修改 GlobalManager、启动链、关闭链或其他公共 API 行为。
- 不让任何实验性能力晋级为稳定公共能力。
- 不实现 P1、P2 或 P3 项目。

## 方案

采用三类 CI 门禁分离的方案：

1. `quality` 不启动外部服务，执行依赖下载、`go vet ./...`、`go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic`，并通过 `go tool cover -func=coverage.out` 输出总覆盖率。
2. `race` 不启动外部服务，独立执行 `go test -race ./... -count=1`。它在 push、pull request 与手动触发时作为普通必过 job，失败不会被普通测试输出掩盖。
3. `smoke` 启动固定版本的 Redis、MongoDB、MySQL，构建示例二进制、后台启动、执行 HTTP health check，并在 `always()` 清理进程。该 job 只证明示例装配和指定 HTTP 路径可运行，不宣称覆盖各外部服务客户端的读写、重建或关闭。

固定服务镜像为：

- `redis:8.2.7-bookworm`
- `mongo:8.0.26-noble`
- `mysql:8.4.10-oraclelinux9`

这些标签是设计日期可用的 Docker Official Image 明确版本标签。固定完整补丁与基础系统变体，避免 `latest`、浮动主版本或浮动次版本在无代码变更时改变 CI 环境。

## 文档模型

### README

README 只保留稳定摘要：

- 当前发布标签与当前工作树是两个不同保证范围。
- 当前 HEAD 的本地复现命令和结果边界。
- CI 执行哪些门禁。
- 外部服务出现于 smoke 环境不等于已完成 live integration 验证。
- 详细能力状态链接到 `docs/reference/feature-status.md`，不复制完整矩阵。

删除具体失败 case 数量及已过时的 package 失败说明，因为这类短期事实应进入带日期的调查记录或 issue。

### 功能状态页

状态采用三个正交概念：

- 实现阶段：`占位`、`已实现`、`已接入`。
- 支持级别：`实验性`、`稳定公共能力`。
- API 受众：`公共 API`、`内部工具`。

详细表格继续按能力分组，但每项必须能回答：

- 如何启用：自动、默认集合、显式装配或不适用。
- 生命周期：创建、运行、失败、关闭四段中哪些已有明确路径。
- 验证级别：单元/契约、race、HTTP smoke、外部 live integration 中哪些实际存在。

没有证据的单元格必须明确写“未验证”或具体缺口，不能由目录、配置键、默认集合或单个 happy-path 测试推导成熟度。

### GlobalManager

GlobalManager 表达为“已接入 + 实验性 + 公共 API”。删除“`Release` 存储 nil”和“初始化失败状态残留”两项已修复限制，保留：

- `Get`、`Rebuild`、`Release`、`ClearAll` 的并发状态机仍需独立设计与验证。
- `Rebuild` 后旧实例的关闭责任未形成统一契约。
- `ClearAll` 不逐项调用资源 `Close`。
- GlobalManager 是 owner 还是 locator 尚未形成公共所有权契约。
- keepalive 缺少完整的取消、等待退出和重复停止语义。

本批次只修正文档，不改变上述行为。

## Vet 修复

`go vet ./...` 当前报告 13 个 unkeyed struct literals：8 个 `exception.Exception`、1 个 `component/database/dbmongo` 中的 `bson.E`，以及示例 model 中 4 个 `bson.E`。

修复只把位置字段改为命名字段：

- `exception.Exception` 使用其声明中的字段名。
- `bson.E` 使用 `Key` 与 `Value`。

不改变值、顺序、控制流或公共接口。`go vet ./...` 是该机械修复的 RED 验证；修复后由 vet、普通测试和 race 共同验证没有行为回归。

## 文件职责

- `README.md`：项目级状态与验证摘要。
- `docs/reference/feature-status.md`：逐能力状态、启用方式、生命周期和验证证据的唯一详细事实源。
- `.github/workflows/go1.yml`：`quality`、`race`、`smoke` 三类门禁及固定服务版本。
- `example_application/providers/exceptions/get_exceptions.go`：将 `exception.Exception` 改为命名字段字面量。
- `component/database/dbmongo/mongo.go`：将 `bson.E` 改为命名字段字面量。
- `example_application/module/example-module/model/example_model.go`：将 `bson.E` 改为命名字段字面量。
- `.codegraph-qa-out/readme-current-status-optimization-todo.md`：完成后更新 P0 勾选状态和实测记录；保留 P1–P3 为后续工作。

## 验证与验收

实现完成后必须满足：

1. `go vet ./...` 返回 0。
2. `go test ./... -count=1` 返回 0。
3. `go test -race ./... -count=1` 返回 0。
4. coverage profile 可生成，`go tool cover -func=coverage.out` 能输出 `total` 行；本轮不设阈值。
5. workflow 中不存在 `Fake temporary test` 和 `redis:latest`、`mongo:latest`、`mysql:latest`。
6. README 不再声称当前测试有已知失败，也不把未发布 HEAD 的结果表述成最新发布版保证。
7. 功能状态页可表达“已接入 + 实验性”和“已实现 + 内部工具”，并明确验证空白。
8. GlobalManager 每条已知限制都能由当前源码或测试复核。
9. 变更不修改对外 Go API 或运行时语义。

## 后续任务边界

本批次完成后按独立设计推进：

1. P1 生命周期和错误模型公共契约。
2. GlobalManager owner/locator 决策与并发状态机。
3. v1 API 兼容和内部工具政策。
4. Gin、数据库/远端缓存、L2、task/CLI、二进制响应分别立项。
5. 最小无外部依赖示例与占位能力治理。
