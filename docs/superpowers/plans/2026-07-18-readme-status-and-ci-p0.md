# README Status and CI P0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make README maturity claims reproducible, make feature status dimensions orthogonal, and turn the existing Go checks into real CI gates.

**Architecture:** Keep README as the stable project-level summary and `docs/reference/feature-status.md` as the single detailed capability matrix. Split GitHub Actions into hermetic quality, hermetic race, and service-backed HTTP smoke jobs so each failure has one meaning. Use `go vet` as the RED check for the behavior-preserving keyed-literal cleanup.

**Tech Stack:** Go 1.25, `go test`, `go vet`, Go race detector, GitHub Actions, Docker Official Images, Markdown.

## Global Constraints

- Scope is only P0 from `.codegraph-qa-out/readme-current-status-optimization-todo.md`; do not implement P1, P2, or P3 behavior.
- Do not change public Go APIs or runtime semantics.
- Do not promote any experimental capability to stable public capability.
- README is a stable summary; `docs/reference/feature-status.md` is the only detailed capability-status source.
- HTTP smoke proves example assembly and one HTTP path only; it is not a Redis, MongoDB, MySQL, task-worker, or keepalive live integration test.
- Coverage produces a profile and total line without enforcing a threshold.
- Use exactly `redis:8.2.7-bookworm`, `mongo:8.0.26-noble`, and `mysql:8.4.10-oraclelinux9`.
- Preserve the current user-authored `component/database/dbmongo/mongo.go` diff exactly; include it in Task 1 because committed CI otherwise still fails vet.
- Preserve the untracked `.codegraph-qa-out/readme-current-status-analysis-2026-07-17.md` without editing or committing it.

---

### Task 1: Make external struct literals vet-safe

**Files:**
- Preserve and commit: `component/database/dbmongo/mongo.go:144-148`
- Modify: `example_application/providers/exceptions/get_exceptions.go:9-19`
- Modify: `example_application/module/example-module/model/example_model.go:42-149`
- Test: repository-wide `go vet` and Go tests

**Interfaces:**
- Consumes: `exception.Exception`, whose fields are `Code`, `Msg`, and `Data` through `type Exception response.RespInfo`; `bson.E`, whose fields are `Key` and `Value`.
- Produces: the same exception values and BSON documents without positional external-struct literals.

- [ ] **Step 1: Confirm the RED vet baseline**

Run:

```bash
GOCACHE=/tmp/fiberhouse-p0-task1-cache go vet ./...
```

Expected: FAIL with 12 diagnostics: eight `exception.Exception struct literal uses unkeyed fields` diagnostics and four `bson.E struct literal uses unkeyed fields` diagnostics. The already-edited `mongo.go` ping literal must not appear.

- [ ] **Step 2: Replace the exception literals with named fields**

Replace the `exceptions` map literal with:

```go
var (
	exceptions = exception.ExceptionMap{
		"InputParamError": {
			Code: 400001,
			Msg:  "Invalid request parameters",
			Data: nil,
		},
		"InternalError": {
			Code: 500001,
			Msg:  constant.UnknownErrMsg,
			Data: "Unknown Internal error",
		},
		"UnknownError": {
			Code: constant.UnknownErrCode,
			Msg:  constant.UnknownErrMsg,
			Data: exception.ErrorData{"msg": "Unknown request error"},
		},
		"NotFoundDocument": {
			Code: 400002,
			Msg:  "No matching records found",
			Data: nil,
		},
		"IllegalRequest": {
			Code: 400003,
			Msg:  "Illegal request",
			Data: nil,
		},
		"NotNeedToUpdate": {
			Code: 200001,
			Msg:  "No records to update",
			Data: nil,
		},
		"NotNeedToDelete": {
			Code: 200002,
			Msg:  "No records to delete",
			Data: nil,
		},
		"SqlProxyExecError": {
			Code: 200003,
			Msg:  "Sql proxy execute error",
			Data: nil,
		},
	}
)
```

- [ ] **Step 3: Replace the four BSON literals with named fields**

Use these exact statements:

```go
filter := bson.D{{Key: "_id", Value: _id}}
```

```go
filter := bson.D{{Key: "_id", Value: upExample.ID}}
update := bson.D{{Key: "$set", Value: upExample}}
```

```go
filter := bson.D{{Key: "_id", Value: id}}
```

- [ ] **Step 4: Format and verify GREEN**

Run:

```bash
gofmt -w component/database/dbmongo/mongo.go example_application/providers/exceptions/get_exceptions.go example_application/module/example-module/model/example_model.go
GOCACHE=/tmp/fiberhouse-p0-task1-cache go vet ./...
GOCACHE=/tmp/fiberhouse-p0-task1-cache go test ./... -count=1
```

Expected: both Go commands return 0. `gofmt` may retain the user-authored import grouping change in `mongo.go`; do not alter any other behavior.

- [ ] **Step 5: Commit the vet cleanup**

```bash
git add component/database/dbmongo/mongo.go example_application/providers/exceptions/get_exceptions.go example_application/module/example-module/model/example_model.go
git commit -m "fix: use keyed external struct literals"
```

---

### Task 2: Split CI into quality, race, and smoke gates

**Files:**
- Modify: `.github/workflows/go1.yml:1-119`
- Test: workflow text checks, YAML parse, and local equivalents of quality/race commands

**Interfaces:**
- Consumes: Go 1.25 module, `example_main/main.go`, test environment variables, Redis on 6379, MongoDB on host 27037, and MySQL on 3306.
- Produces: GitHub check names `quality`, `race`, and `smoke`; `coverage.out` and `coverage-summary.txt` within the quality runner.

- [ ] **Step 1: Confirm the workflow RED state**

Run:

```bash
rg -n 'Fake temporary test|redis:latest|mongo:latest|mysql:latest|build-and-test:' .github/workflows/go1.yml
```

Expected: all five obsolete patterns are present.

- [ ] **Step 2: Replace the workflow with three focused jobs**

Replace `.github/workflows/go1.yml` with:

```yaml
name: build & test

on:
  push:
    branches: ['main']
  pull_request:
    branches: ['main']
  workflow_dispatch:

jobs:
  quality:
    name: quality
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - name: Checkout code
        uses: actions/checkout@v5

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Vet
        run: go vet ./...

      - name: Test with coverage
        run: go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic

      - name: Report total coverage
        run: |
          go tool cover -func=coverage.out | tee coverage-summary.txt
          grep '^total:' coverage-summary.txt

  race:
    name: race
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - name: Checkout code
        uses: actions/checkout@v5

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Test with race detector
        run: go test -race ./... -count=1

  smoke:
    name: smoke
    runs-on: ubuntu-latest
    timeout-minutes: 15
    env:
      TARGET_NAME: fhweb
      LDFLAGS: -ldflags="-X 'main.Version=v0.0.1'"
      APP_ENV_application_env: test
      APP_ENV_application_appType: web
    services:
      redis:
        image: redis:8.2.7-bookworm
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      mongodb:
        image: mongo:8.0.26-noble
        ports:
          - 27037:27017
        env:
          MONGO_INITDB_ROOT_USERNAME: admin
          MONGO_INITDB_ROOT_PASSWORD: admin
        options: >-
          --health-cmd "mongosh --eval 'db.runCommand({ ping: 1 })' --quiet"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      mysql:
        image: mysql:8.4.10-oraclelinux9
        ports:
          - 3306:3306
        env:
          MYSQL_ROOT_PASSWORD: root
          MYSQL_DATABASE: test
        options: >-
          --health-cmd "mysqladmin ping -h localhost -u root -proot"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - name: Checkout code
        uses: actions/checkout@v5

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Build example
        run: go build ${{ env.LDFLAGS }} -o "$RUNNER_TEMP/${{ env.TARGET_NAME }}" ./example_main/main.go

      - name: Start example
        run: |
          "$RUNNER_TEMP/${{ env.TARGET_NAME }}" >"$RUNNER_TEMP/${{ env.TARGET_NAME }}.log" 2>&1 &
          echo $! > "$RUNNER_TEMP/${{ env.TARGET_NAME }}.pid"

      - name: Check HTTP path
        run: curl --fail --retry 10 --retry-delay 1 --retry-connrefused http://127.0.0.1:8080/example/hello/world

      - name: Print example log on failure
        if: failure()
        run: cat "$RUNNER_TEMP/${{ env.TARGET_NAME }}.log"

      - name: Stop example
        if: always()
        run: |
          if [[ -f "$RUNNER_TEMP/${{ env.TARGET_NAME }}.pid" ]]; then
            kill "$(cat "$RUNNER_TEMP/${{ env.TARGET_NAME }}.pid")" || true
          fi
```

- [ ] **Step 3: Validate the workflow structure and obsolete-pattern removal**

Run:

```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/go1.yml", aliases: true); puts "yaml ok"'
if rg -n 'Fake temporary test|redis:latest|mongo:latest|mysql:latest|build-and-test:' .github/workflows/go1.yml; then exit 1; fi
rg -n '^  (quality|race|smoke):|timeout-minutes:|coverage.out|go test -race|redis:8.2.7-bookworm|mongo:8.0.26-noble|mysql:8.4.10-oraclelinux9' .github/workflows/go1.yml
```

Expected: YAML prints `yaml ok`; the obsolete-pattern command returns 0 because its guarded `rg` finds nothing; all three jobs, timeouts, coverage, race, and exact service tags are printed.

- [ ] **Step 4: Run the local equivalents**

Run:

```bash
GOCACHE=/tmp/fiberhouse-p0-task2-cache go vet ./...
GOCACHE=/tmp/fiberhouse-p0-task2-cache go test ./... -count=1 -coverprofile=/tmp/fiberhouse-p0-coverage.out -covermode=atomic
go tool cover -func=/tmp/fiberhouse-p0-coverage.out | tail -n 1
GOCACHE=/tmp/fiberhouse-p0-task2-race-cache go test -race ./... -count=1
```

Expected: all commands return 0 and the coverage command prints a `total:` line. This does not locally reproduce GitHub service containers; the smoke job is verified on GitHub Actions.

- [ ] **Step 5: Commit the workflow**

```bash
git add .github/workflows/go1.yml
git commit -m "ci: enforce quality race and smoke gates"
```

---

### Task 3: Separate maturity dimensions and verification claims

**Files:**
- Modify: `README.md:20-31,204-217`
- Modify: `docs/reference/feature-status.md:1-89`
- Test: exact text assertions and stale-claim absence

**Interfaces:**
- Consumes: the design spec, current source/test evidence, and Task 2 job names.
- Produces: a concise README summary and one detailed feature-status source with orthogonal implementation, support, and audience dimensions.

- [ ] **Step 1: Confirm stale documentation is present**

Run:

```bash
rg -n '已知失败集中|Release.*存储 nil|失败状态残留|内部工具.*不承诺稳定公共抽象' README.md docs/reference/feature-status.md
```

Expected: README prints the obsolete known-failure paragraph and the feature-status page prints the obsolete GlobalManager limitations.

- [ ] **Step 2: Replace README's current-status summary**

Replace the content below `## 当前状态` and before `## 核心能力` with:

```markdown
FiberHouse 仍在演进中。能力状态使用三个正交维度：实现阶段回答代码和调用链是否存在，支持级别回答兼容与成熟度承诺，API 受众回答能力面向业务还是主要供框架内部使用。“已接入”不等于“稳定公共能力”。

| 范围 | 实现阶段 | 支持级别 | API 受众 | 摘要 |
|---|---|---|---|---|
| Fiber HTTP、Provider 主链、配置与日志、JSON 响应、校验 | 已接入 | 实验性 | 公共 API | 已有明确入口和单元/契约测试，但项目级生命周期与兼容政策尚未全部达到稳定晋级门槛 |
| Gin、GlobalManager、L2 缓存、任务、CLI、数据库、二进制响应 | 已接入 | 实验性 | 公共 API | 已有可达实现，但错误传播、并发、关闭或外部依赖验证仍存在明确缺口 |
| bufferpool、Dig、writer、jsonconvert、mongodecimal | 已实现或已接入 | 实验性 | 内部工具 | 主要服务框架或示例；公开 Go package 路径不等于稳定公共 API 承诺 |
| plugins、RPC、MQ、通用 i18n、Go JSON codec | 占位 | 不适用 | 未承诺 | 没有完整创建、运行、失败和关闭链 |

启用方式、生命周期完整度、验证级别和逐项限制只在[功能状态](docs/reference/feature-status.md)维护。`example_main`、`example_config`、`example_application` 只展示调用路径，不是生产模板或稳定 API。
```

- [ ] **Step 3: Replace README's development-and-verification text**

Keep the existing command block, add `go test -race ./... -count=1`, and replace the two paragraphs after it with:

```markdown
在当前工作树执行 `go test ./... -count=1` 与 `go test -race ./... -count=1` 应通过，`go vet ./...` 作为独立静态检查门禁。该结论针对当前提交，不自动成为最新发布标签的追溯保证；发布版本以 Git tag 和对应发布说明为准。

GitHub Actions 将普通测试、coverage 与 vet 放在 `quality` job，将 race 放在独立 `race` job，并由 `smoke` job 使用固定版本的 Redis、MongoDB、MySQL 启动完整示例和检查一条 HTTP 路径。服务容器出现在 smoke 环境中只证明示例装配可启动，不代表数据库/Redis 读写、重建、关闭、task worker 或 keepalive 已通过 live integration test。是否阻止合并取决于仓库分支保护是否把 `quality`、`race`、`smoke` 配置为 required status checks。

`Makefile` 还提供 `build`、`lint` 和交叉构建目标，但使用前应先核对其目标路径与本机工具。
```

- [ ] **Step 4: Rewrite the feature-status model and tables**

Retain the current evidence and guide links, but make every row use this exact schema:

```markdown
| 能力 | 实现阶段 | 支持级别 | API 受众 | 启用方式 | 生命周期完整度 | 验证级别 | 限制与主指南 |
```

Use these exact state assignments:

```markdown
| Fiber HTTP 内核 | 已接入 | 实验性 | 公共 API |
| Provider / Manager / Location | 已接入 | 实验性 | 公共 API |
| bootstrap、配置与日志 | 已接入 | 实验性 | 公共 API |
| JSON 流量编解码与 JSON 响应 | 已接入 | 实验性 | 公共 API |
| panic recovery 与错误响应 | 已接入 | 实验性 | 公共 API |
| 本地缓存与 Redis 缓存 | 已接入 | 实验性 | 公共 API |
| 参数验证 | 已接入 | 实验性 | 公共 API |
| Gin HTTP 内核 | 已接入 | 实验性 | 公共 API |
| MsgPack / Protobuf 响应 | 已接入 | 实验性 | 公共 API |
| GlobalManager | 已接入 | 实验性 | 公共 API |
| L2 缓存与 Redis 保护机制 | 已接入 | 实验性 | 公共 API |
| 异步任务 | 已接入 | 实验性 | 公共 API |
| CLI | 已接入 | 实验性 | 公共 API |
| MySQL / MongoDB | 已接入 | 实验性 | 公共 API |
| 扩展运行位点与关闭链 | 已实现 | 实验性 | 公共 API |
| bufferpool 与对象池 | 已实现 | 实验性 | 内部工具 |
| Dig 容器 | 已接入 | 实验性 | 内部工具 |
| jsonconvert | 已接入 | 实验性 | 内部工具 |
| mongodecimal | 已接入 | 实验性 | 内部工具 |
| logging writer 与 task logger adaptor | 已接入 | 实验性 | 内部工具 |
| plugins | 占位 | 不适用 | 未承诺 |
| RPC | 占位 | 不适用 | 未承诺 |
| MQ | 占位 | 不适用 | 未承诺 |
| i18n | 占位 | 不适用 | 未承诺 |
| Go JSON codec | 占位 | 不适用 | 未承诺 |
| 空 component/middleware 目录说明 | 占位 | 不适用 | 未承诺 |
| 未消费的生命周期 hook | 占位 | 不适用 | 未承诺 |
```

For `启用方式`, translate the existing `默认注册` and `应用启用` evidence without changing facts. For `生命周期完整度`, explicitly name available and missing portions among 创建、运行、失败、关闭. For `验证级别`, use only the following vocabulary and never infer external coverage from smoke:

```markdown
单元/契约
单元/契约 + race
HTTP smoke
未验证外部 live integration
无专项测试
不适用
```

Replace the GlobalManager limitation cell with this exact text:

```markdown
`Get`/`Rebuild`/`Release`/`ClearAll` 的并发状态机、重建后旧实例关闭、`ClearAll` 不逐项 `Close`、owner/locator 责任和 keepalive 取消/等待退出仍未形成统一契约；见[GlobalManager](../guides/global-manager.md)
```

At the top of the page, define the dimensions exactly as:

```markdown
- **实现阶段**：`占位`表示没有完整运行链；`已实现`表示有可执行实现但未证明进入框架或示例主链；`已接入`表示存在明确入口和可达运行路径。
- **支持级别**：`实验性`表示兼容、生命周期、错误、并发或验证仍未达到晋级门槛；`稳定公共能力`必须满足本文末尾全部晋级条件。本页当前不把任何能力新增标记为稳定。
- **API 受众**：`公共 API`面向使用方；`内部工具`描述设计受众，不覆盖 Go package 实际可导入的兼容事实；占位能力使用`未承诺`。
```

Add this verification-baseline paragraph before the capability tables:

```markdown
当前工作树的 hermetic 基线是 `go test ./... -count=1`、`go vet ./...` 和 `go test -race ./... -count=1`。CI 的 `smoke` job 还构建并启动示例、检查一条 HTTP 路径，但当前没有针对 Redis、MongoDB、MySQL、task worker 或 keepalive 的可重复 live integration suite。表中必须显式保留这些验证空白。
```

- [ ] **Step 5: Verify stale claims are gone and dimensions are visible**

Run:

```bash
if rg -n '已知失败集中|Release.*存储 nil|失败状态残留' README.md docs/reference/feature-status.md; then exit 1; fi
rg -n '实现阶段|支持级别|API 受众|生命周期完整度|验证级别|已接入.*实验性|已实现.*内部工具|未验证外部 live integration|quality.*race.*smoke' README.md docs/reference/feature-status.md
git diff --check -- README.md docs/reference/feature-status.md
```

Expected: the guarded stale-claim scan finds nothing; the second command prints the orthogonal dimensions, required combinations, validation gaps, and CI job names; diff check returns 0.

- [ ] **Step 6: Commit the documentation model**

```bash
git add README.md docs/reference/feature-status.md
git commit -m "docs: clarify capability maturity and validation"
```

---

### Task 4: Record P0 execution status and run integration verification

**Files:**
- Modify and add to version control: `.codegraph-qa-out/readme-current-status-optimization-todo.md`
- Preserve untracked: `.codegraph-qa-out/readme-current-status-analysis-2026-07-17.md`
- Test: full repository verification and P0 acceptance scans

**Interfaces:**
- Consumes: Tasks 1-3 commits and their test evidence.
- Produces: a durable P0 execution record that distinguishes completed code work from the external branch-protection setting.

- [ ] **Step 1: Run the complete P0 verification suite**

Run:

```bash
GOCACHE=/tmp/fiberhouse-p0-final-cache go vet ./...
GOCACHE=/tmp/fiberhouse-p0-final-cache go test ./... -count=1 -coverprofile=/tmp/fiberhouse-p0-final-coverage.out -covermode=atomic
go tool cover -func=/tmp/fiberhouse-p0-final-coverage.out | tee /tmp/fiberhouse-p0-final-coverage.txt
grep '^total:' /tmp/fiberhouse-p0-final-coverage.txt
GOCACHE=/tmp/fiberhouse-p0-final-race-cache go test -race ./... -count=1
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/go1.yml", aliases: true); puts "yaml ok"'
```

Expected: vet, test, coverage generation, race, and YAML parse all return 0; coverage output contains a `total:` line.

- [ ] **Step 2: Run acceptance scans**

Run:

```bash
if rg -n 'Fake temporary test|redis:latest|mongo:latest|mysql:latest|已知失败集中|Release.*存储 nil|失败状态残留' .github/workflows/go1.yml README.md docs/reference/feature-status.md; then exit 1; fi
rg -n 'redis:8.2.7-bookworm|mongo:8.0.26-noble|mysql:8.4.10-oraclelinux9|go test -race|coverage.out' .github/workflows/go1.yml
git diff --check
```

Expected: no stale patterns; all exact pins and gates are present; diff check returns 0.

- [ ] **Step 3: Update the P0 checklist accurately**

In `.codegraph-qa-out/readme-current-status-optimization-todo.md`:

- Change the document status to `P0 已执行；P1–P3 待执行`.
- Check P0.1, P0.2, P0.3, the workflow/test/coverage items in P0.4, and all code-controlled items in P0.5.
- Leave `确认 CI 失败会阻止合并` unchecked unless repository branch protection was separately read and confirmed.
- Do not claim live integration coverage for Redis, MongoDB, MySQL, task worker, or keepalive.
- Before editing the record, capture `git rev-parse HEAD`; this is the verified implementation HEAD after Task 3.
- Add an `## P0 执行记录（2026-07-18）` section containing that verified implementation HEAD SHA, the commands from Step 1, their pass/fail results, the reported total coverage, the three CI job names, exact service image tags, and this sentence: `分支保护是否将 quality、race、smoke 设为 required status checks 需要在 GitHub 仓库设置中单独确认。`

- [ ] **Step 4: Commit only the todo execution record**

```bash
git add .codegraph-qa-out/readme-current-status-optimization-todo.md
git commit -m "docs: record README status P0 completion"
```

- [ ] **Step 5: Confirm the preserved analysis file remains untracked and untouched**

Run:

```bash
git status --short
git diff -- .codegraph-qa-out/readme-current-status-analysis-2026-07-17.md
```

Expected: the analysis file remains `??` and has no tracked diff; no other task-owned files remain modified.

---

## Final Review Inputs

- Design: `docs/superpowers/specs/2026-07-18-readme-status-and-ci-p0-design.md`
- Plan: `docs/superpowers/plans/2026-07-18-readme-status-and-ci-p0.md`
- User-authored baseline diff to preserve: `component/database/dbmongo/mongo.go`
- Final review must inspect the complete range from `b7a0abe` through Task 4 HEAD and separately confirm no uncommitted task-owned changes remain.
- The remaining untracked analysis file is expected and must not be reported as a task regression.
