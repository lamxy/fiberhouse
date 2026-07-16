# Component Hierarchical Namespaces Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 JSON codec、task logger adaptor 和日志 writer 迁入已确认的 component 分层领域命名空间，同时保持导出 API、运行行为、配置和既有测试内容不变。

**Architecture:** `component/codec`、`component/task`、`component/logging` 是不包含 Go 文件的纯领域命名空间，API 分别位于 `codec/json`、`task/logadaptor`、`logging/writer` 叶子 package。三个 import path 迁移分成独立提交，当前文档和最终验证集中在第四个提交；root 只导入不反向依赖 root 的 jsoncodec，task logadaptor 继续由应用装配层接入。

**Tech Stack:** Go modules、Fiber、Gin、Sonic、asynq、zerolog、lumberjack、go-diodes、Git rename detection、CodeGraph。

**Design spec:** `docs/superpowers/specs/2026-07-17-component-hierarchical-namespaces-design.md`

## Global Constraints

- 从包含设计提交 `4853332` 的 `main` 创建独立 worktree；执行时先使用 `superpowers:using-git-worktrees`，建议分支名 `refactor/component-hierarchical-namespaces`。
- 开始前阅读 `AGENTS.md`、`CLAUDE.md` 和设计规格；需要定位代码时先使用 CodeGraph。
- 旧 import path 直接失效，不创建兼容 shim、facade、类型别名、转发函数、registry 或自动注册。
- 保留全部现有导出类型、函数、方法、字段和方法签名；特别是不重命名 `SonicJsonFastest`、`StdJsonDefault`、`TaskLoggerAdapter` 和 writer 构造器。
- 不修改 JSON fallback、Gin/Fiber codec 装配、task 日志字段、writer 并发、丢弃、flush、关闭或错误行为。
- 不修复五个 writer 测试问题或 bootstrap 测试问题，不新增功能测试；writer 测试文件只移动。
- 不修改 `go.mod`、`go.sum`、YAML 配置、常量值、GlobalManager key、日志 Origin 或生成源码。
- 不手工修改 `.codegraph/` 生成数据，不机械改写历史设计、计划和时间点审计记录。
- 使用 `git mv` 保留历史；只移动八个跟踪文件，不携带 `D:/invalid/path/test.log` 测试伪影。
- 不执行全仓 `gofmt` 或换行规范化。仓库策略是 LF；暂存后用 rename-aware diff、`--ignore-cr-at-eol` 和 `--check` 审核真实内容。
- 所有 Go 命令使用 `/tmp` 下独立 `GOCACHE`，避免只读默认缓存影响验证。
- 不 push 远程。

## File Map

### Move without content changes

- `component/jsoncodec/gojson.go` → `component/codec/json/gojson.go`
- `component/jsoncodec/sonicjson.go` → `component/codec/json/sonicjson.go`
- `component/jsoncodec/stdjson.go` → `component/codec/json/stdjson.go`
- `component/writer/async_channel_writer.go` → `component/logging/writer/async_channel_writer.go`
- `component/writer/async_diode_writer.go` → `component/logging/writer/async_diode_writer.go`
- `component/writer/async_diode_writer_test.go` → `component/logging/writer/async_diode_writer_test.go`
- `component/writer/sync_lumberjack_writer.go` → `component/logging/writer/sync_lumberjack_writer.go`

### Move with one package-clause change

- `component/tasklog/logger_adapter.go` → `component/task/logadaptor/logger_adapter.go`; `package tasklog` → `package logadaptor`

### Production import updates

- `json_fiber_provider.go`
- `json_gin_provider.go`
- `task.go`
- `example_application/application_impl.go`
- `example_application/command/application/application.go`
- `example_application/module/task_impl.go`
- `bootstrap/bootstrap.go`

### Current documentation and durable analysis

- `README.md`
- `docs/reference/components.md`
- `docs/reference/examples.md`
- `docs/reference/feature-status.md`
- `docs/reference/known-test-failures.md`
- `docs/guides/web-runtime.md`
- `docs/guides/background-tasks.md`
- `docs/guides/logging.md`
- `.codegraph-qa-out/todo.md`
- Create `.codegraph-qa-out/component-hierarchical-namespaces.md`

---

### Task 1: Move JSON codecs into the codec namespace

**Files:**
- Move: `component/jsoncodec/gojson.go` → `component/codec/json/gojson.go`
- Move: `component/jsoncodec/sonicjson.go` → `component/codec/json/sonicjson.go`
- Move: `component/jsoncodec/stdjson.go` → `component/codec/json/stdjson.go`
- Modify: `json_fiber_provider.go:4`
- Modify: `json_gin_provider.go:5`
- Modify: `task.go:13`
- Modify: `example_application/application_impl.go:12`
- Modify: `example_application/command/application/application.go:11`

**Interfaces:**
- Consumes: existing `fiberhouse.JsonWrapper` contract and Gin `codec/json.Core` method set.
- Produces: import path `github.com/lamxy/fiberhouse/component/codec/json`, declared package `jsoncodec`, with all existing `StdJSON`, `SonicJSON`, constructors and methods unchanged.

- [ ] **Step 1: Establish the old-package compile baseline**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy-baseline go test ./component/jsoncodec -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy-baseline go list -f '{{.ImportPath}} package={{.Name}}' ./component/jsoncodec
```

Expected: both commands exit 0; `go list` prints `github.com/lamxy/fiberhouse/component/jsoncodec package=jsoncodec`.

- [ ] **Step 2: Move the three tracked codec files**

Run:

```bash
mkdir -p component/codec/json
git mv component/jsoncodec/gojson.go component/codec/json/gojson.go
git mv component/jsoncodec/sonicjson.go component/codec/json/sonicjson.go
git mv component/jsoncodec/stdjson.go component/codec/json/stdjson.go
rmdir component/jsoncodec
```

Do not change any package clause or codec implementation. The three destination files must still declare:

```go
package jsoncodec
```

- [ ] **Step 3: Update the five direct import paths**

In all five production files listed above, apply exactly this replacement and leave selectors unchanged:

```diff
- "github.com/lamxy/fiberhouse/component/jsoncodec"
+ "github.com/lamxy/fiberhouse/component/codec/json"
```

Calls must continue to use the existing package identifier, for example:

```go
jcodec := jsoncodec.StdJsonDefault()
return jsoncodec.SonicJsonFastest()
```

- [ ] **Step 4: Verify the new package identity and all direct consumers**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy go list -f '{{.ImportPath}} package={{.Name}}' ./component/codec/json
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./component/codec/json -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go test . -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./example_application/... -run '^$'
rg -n 'github.com/lamxy/fiberhouse/component/jsoncodec' --glob '*.go' .
```

Expected:

- `go list` prints `github.com/lamxy/fiberhouse/component/codec/json package=jsoncodec`.
- All compile checks exit 0.
- `rg` exits 1 with no matches.

- [ ] **Step 5: Review and commit the codec migration**

Stage only the three moves and five import consumers:

```bash
git add -A -- component/jsoncodec component/codec/json
git add json_fiber_provider.go json_gin_provider.go task.go example_application/application_impl.go example_application/command/application/application.go
git diff --cached --check
git diff --cached --find-renames --summary
git diff --cached --find-renames --ignore-cr-at-eol -- component/jsoncodec component/codec/json json_fiber_provider.go json_gin_provider.go task.go example_application/application_impl.go example_application/command/application/application.go
git commit -m "refactor: move json codecs into codec namespace"
```

Expected: three renames plus five one-line import-path changes; no package, API or implementation change.

---

### Task 2: Move the task logger adaptor into the task namespace

**Files:**
- Move and modify: `component/tasklog/logger_adapter.go` → `component/task/logadaptor/logger_adapter.go`
- Modify: `example_application/module/task_impl.go:9,89`

**Interfaces:**
- Consumes: `fiberhouse.IContext` and the structural asynq logger method set `Debug/Info/Warn/Error/Fatal(args ...interface{})`.
- Produces: import path `github.com/lamxy/fiberhouse/component/task/logadaptor`, package `logadaptor`, while retaining `TaskLoggerAdapter`, public field `Ctx`, `NewTaskLoggerAdapter` and all five methods.

- [ ] **Step 1: Establish the old-package compile baseline**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy-baseline go test ./component/tasklog -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy-baseline go list -f '{{.ImportPath}} package={{.Name}}' ./component/tasklog
```

Expected: both commands exit 0; `go list` prints `github.com/lamxy/fiberhouse/component/tasklog package=tasklog`.

- [ ] **Step 2: Move the adapter file and change only its package clause**

Run:

```bash
mkdir -p component/task/logadaptor
git mv component/tasklog/logger_adapter.go component/task/logadaptor/logger_adapter.go
rmdir component/tasklog
```

Apply this exact source change at the top of the moved file:

```diff
-package tasklog
+package logadaptor
```

Do not change `TaskLoggerAdapter`, `NewTaskLoggerAdapter`, `Ctx`, any method body, log Origin, `Component=Asynq`, or `Fatal` behavior.

- [ ] **Step 3: Update the application assembly import and selector**

In `example_application/module/task_impl.go`, make exactly these changes:

```diff
- "github.com/lamxy/fiberhouse/component/tasklog"
+ "github.com/lamxy/fiberhouse/component/task/logadaptor"
```

```diff
-Logger:   tasklog.NewTaskLoggerAdapter(ta.Ctx),
+Logger:   logadaptor.NewTaskLoggerAdapter(ta.Ctx),
```

Keep the existing Chinese comment on the `Logger` field.

- [ ] **Step 4: Verify the package name, assembly, and dependency direction**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy go list -f '{{.ImportPath}} package={{.Name}} imports={{join .Imports ","}}' ./component/task/logadaptor
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./component/task/logadaptor -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./example_application/module/... -run '^$'
rg -n 'github.com/lamxy/fiberhouse/component/tasklog|\btasklog\.' --glob '*.go' .
```

Expected:

- `go list` prints the new path with `package=logadaptor` and includes `github.com/lamxy/fiberhouse` in its imports.
- Compile checks exit 0 without an import cycle.
- `rg` exits 1 with no matches.

- [ ] **Step 5: Review and commit the task adaptor migration**

```bash
git add -A -- component/tasklog component/task/logadaptor
git add example_application/module/task_impl.go
git diff --cached --check
git diff --cached --find-renames --summary
git diff --cached --find-renames --ignore-cr-at-eol -- component/tasklog component/task/logadaptor example_application/module/task_impl.go
git commit -m "refactor: move task logger adaptor into task namespace"
```

Expected: one rename with the package-clause change and two consumer-line changes; no method-body or behavior change.

---

### Task 3: Move log writers into the logging namespace

**Files:**
- Move: `component/writer/async_channel_writer.go` → `component/logging/writer/async_channel_writer.go`
- Move: `component/writer/async_diode_writer.go` → `component/logging/writer/async_diode_writer.go`
- Move: `component/writer/async_diode_writer_test.go` → `component/logging/writer/async_diode_writer_test.go`
- Move: `component/writer/sync_lumberjack_writer.go` → `component/logging/writer/sync_lumberjack_writer.go`
- Modify: `bootstrap/bootstrap.go:22`

**Interfaces:**
- Consumes: `appconfig.IAppConfig`, lumberjack and go-diodes.
- Produces: import path `github.com/lamxy/fiberhouse/component/logging/writer`, package `writer`, retaining the three concrete writer types, constructors, `Write`, `Close` and `DroppedLogs` methods.

- [ ] **Step 1: Verify the old directory contains only the four expected tracked files**

Run:

```bash
git status --short --untracked-files=all -- component/writer
git ls-files component/writer
GOCACHE=/tmp/fiberhouse-component-hierarchy-baseline go test ./component/writer -run '^$'
```

Expected:

- Status output is empty.
- `git ls-files` prints exactly the four files listed in this task.
- Compile-only test exits 0.

If the exact known test artifact `component/writer/D:/invalid/path/test.log` exists, remove only that generated file and its now-empty `D:/invalid/path` directory chain before moving files. Stop for user direction if any other untracked file exists.

For that exact artifact only, run:

```bash
rm -f 'component/writer/D:/invalid/path/test.log'
rmdir 'component/writer/D:/invalid/path'
rmdir 'component/writer/D:/invalid'
rmdir 'component/writer/D:'
```

- [ ] **Step 2: Move the four tracked writer files**

Run:

```bash
mkdir -p component/logging/writer
git mv component/writer/async_channel_writer.go component/logging/writer/async_channel_writer.go
git mv component/writer/async_diode_writer.go component/logging/writer/async_diode_writer.go
git mv component/writer/async_diode_writer_test.go component/logging/writer/async_diode_writer_test.go
git mv component/writer/sync_lumberjack_writer.go component/logging/writer/sync_lumberjack_writer.go
rmdir component/writer
```

All four destination files must continue to declare:

```go
package writer
```

Do not edit production or test logic.

- [ ] **Step 3: Update bootstrap's import path**

In `bootstrap/bootstrap.go`, make exactly this change and leave all `writer.New...` calls untouched:

```diff
- "github.com/lamxy/fiberhouse/component/writer"
+ "github.com/lamxy/fiberhouse/component/logging/writer"
```

- [ ] **Step 4: Verify compile-only behavior and old import removal**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy go list -f '{{.ImportPath}} package={{.Name}}' ./component/logging/writer
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./component/logging/writer -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./bootstrap -run '^$'
rg -n 'github.com/lamxy/fiberhouse/component/writer' --glob '*.go' .
```

Expected:

- `go list` prints `github.com/lamxy/fiberhouse/component/logging/writer package=writer`.
- Both compile-only checks exit 0.
- `rg` exits 1 with no matches.

Do not run or fix the writer functional tests in this task; the full-suite comparison happens after documentation paths are current.

- [ ] **Step 5: Review and commit the writer migration**

```bash
git add -A -- component/writer component/logging/writer
git add bootstrap/bootstrap.go
git diff --cached --check
git diff --cached --find-renames --summary
git diff --cached --find-renames --ignore-cr-at-eol -- component/writer component/logging/writer bootstrap/bootstrap.go
git commit -m "refactor: move log writers into logging namespace"
```

Expected: four renames and one import-path change. The test file and all three implementation files must be byte-equivalent modulo Git's configured EOL normalization.

---

### Task 4: Document the hierarchy and run repository-wide verification

**Files:**
- Modify: `README.md:215`
- Modify: `docs/reference/components.md:3-7,19,21-22,39,55-63`
- Modify: `docs/reference/examples.md:82`
- Modify: `docs/reference/feature-status.md:66`
- Modify: `docs/reference/known-test-failures.md:3-5,12-16,20-23`
- Modify: `docs/guides/web-runtime.md:59,94`
- Modify: `docs/guides/background-tasks.md:95`
- Modify: `docs/guides/logging.md:96,101`
- Modify: `.codegraph-qa-out/todo.md:8,11`
- Create: `.codegraph-qa-out/component-hierarchical-namespaces.md`

**Interfaces:**
- Consumes: the three committed package migrations and the six-test known-failure baseline.
- Produces: current documentation that defines pure domain namespaces, points to all three new paths, and preserves the historical meaning of prior specs and failure records.

- [ ] **Step 1: Update the component hierarchy definition and table**

Replace the flat first-level rule in `docs/reference/components.md` with this meaning:

```markdown
`component/` 是 FiberHouse 提供的内置、可选装配、可复用能力的命名空间，不是严格的底层依赖层。根目录不提供 Go API。一级目录既可以是可直接导入的能力 package，也可以是不包含 `.go` 文件的纯领域命名空间；可装配 API 位于叶子 package。

子组件可以依赖 FiberHouse 核心接口，但依赖 root package 的组件必须由应用注册器或 Provider 装配，root package 和父级命名空间不得反向聚合导入。纯领域命名空间不创建 facade 或 re-export package；仅服务于单个组件的辅助实现放入该组件的 `internal/` 子树。
```

Update the three table paths without changing their lifecycle/status descriptions:

```text
component/jsoncodec  -> component/codec/json
component/tasklog    -> component/task/logadaptor
component/writer     -> component/logging/writer
```

Keep `component/jsonconvert` at the top level. Update nearby prose so `jsoncodec` means the package name and the table/path uses `component/codec/json`.

- [ ] **Step 2: Update all current guide, reference, README, and TODO paths**

Apply these exact path mappings to the current files listed in this task:

```text
component/jsoncodec/gojson.go -> component/codec/json/gojson.go
component/jsoncodec/          -> component/codec/json/
component/jsoncodec           -> component/codec/json
component/tasklog/            -> component/task/logadaptor/
component/tasklog             -> component/task/logadaptor
component/writer/             -> component/logging/writer/
component/writer              -> component/logging/writer
component/writer.Test         -> component/logging/writer.Test
```

Replace the current README status sentence with:

```markdown
当前 `go test ./...` 的已知失败集中在 `bootstrap` 与 `component/logging/writer` 两个 package；分层命名空间迁移只移动 writer package，没有修改或修复这些测试。writer 的失败 case 数量会受异步写入时序影响，不宜作为固定基线。
```

In `docs/reference/known-test-failures.md`, do not claim that this migration left the writer path untouched. Use wording with this precise meaning:

```markdown
分层命名空间迁移只把 `component/writer/` 原样移动到 `component/logging/writer/`，没有修改 writer 生产逻辑或测试逻辑。以下问题的名称和归因保持不变，仅 package 路径随迁移更新。
```

Change its five writer test rows to `component/logging/writer.Test...` and change the verification bullet to state that writer production/test content is unchanged apart from path and package identity. Preserve the original 2026-07-16 baseline commit information and the `ConcurrentWrites` flaky qualification.

- [ ] **Step 3: Add the focused CodeGraph analysis record**

Create `.codegraph-qa-out/component-hierarchical-namespaces.md` with this complete content:

```markdown
# Component 分层命名空间

## 决策

`component/codec`、`component/task`、`component/logging` 是不包含 Go 文件的纯领域命名空间。可导入 API 位于叶子 package；父目录不提供 facade、registry 或 re-export。

| 旧路径 | 当前路径 | package | 调用 selector |
|---|---|---|---|
| `component/jsoncodec` | `component/codec/json` | `jsoncodec` | `jsoncodec` |
| `component/tasklog` | `component/task/logadaptor` | `logadaptor` | `logadaptor` |
| `component/writer` | `component/logging/writer` | `writer` | `writer` |

JSON 路径末段与 package 名有意不同，用于避免与标准库及 Gin 的 `json` package 混淆。`logadaptor` 沿用仓库现有 `adaptor/` 目录词汇；writer 使用 Go 惯用的单数名称。

## 依赖边界

- FiberHouse root 可以导入不反向依赖 root 的 `component/codec/json`。
- `bootstrap` 导入 `component/logging/writer`；writer 依赖 `appconfig`。
- `component/task/logadaptor` 依赖 FiberHouse root，因此只能由应用装配层接入，root `task.go` 不得反向导入它。
- `component/jsonconvert` 不属于 codec backend，本次保持原位。

## 迁移边界

迁移改变公开 import path 和 Go package identity，但不重命名导出 API、不保留兼容 shim，也不修改运行行为、配置、依赖版本或既有测试逻辑。完整测试仍按 `docs/reference/known-test-failures.md` 中记录的六项既有问题分类。
```

Do not modify generated `.codegraph/` files or historical provider/component migration reports.

- [ ] **Step 4: Prove current code and current docs contain no old paths**

Run:

```bash
rg -n 'github.com/lamxy/fiberhouse/component/(jsoncodec|tasklog|writer)' --glob '*.go' .
rg -n 'component/(jsoncodec|tasklog|writer)' README.md docs/reference docs/guides
rg -n '^component/(codec|task|logging)/[^/]+\.go$' < <(rg --files component/codec component/task component/logging)
```

Expected: all three commands exit 1 with no matches. Historical files under `docs/superpowers/` and historical `.codegraph-qa-out` reports are intentionally outside this scan.

- [ ] **Step 5: Verify exact package paths, names, and the absence of parent packages**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy go list -f '{{.ImportPath}} package={{.Name}}' ./component/codec/json ./component/task/logadaptor ./component/logging/writer
GOCACHE=/tmp/fiberhouse-component-hierarchy go list ./component/...
```

Expected first-command output:

```text
github.com/lamxy/fiberhouse/component/codec/json package=jsoncodec
github.com/lamxy/fiberhouse/component/task/logadaptor package=logadaptor
github.com/lamxy/fiberhouse/component/logging/writer package=writer
```

The recursive list must not contain standalone `.../component/codec`, `.../component/task`, or `.../component/logging` packages.

- [ ] **Step 6: Run focused compile checks and the full build**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./component/codec/json -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./component/task/logadaptor -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./component/logging/writer -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./... -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy go build ./...
```

Expected: every command exits 0; no package-not-found, undefined selector or import-cycle error.

- [ ] **Step 7: Run the full suite and classify every failure**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy go test ./... -count=1
```

Expected: the command may exit nonzero. Every failure must be a member of this known set, with the writer package shown at its new path:

```text
bootstrap.Test_Config_EnvOverrideAndSingleton
component/logging/writer.TestAsyncDiodeWriter_Write
component/logging/writer.TestAsyncDiodeWriter_MultipleWrites
component/logging/writer.TestAsyncDiodeWriter_ConcurrentWrites
component/logging/writer.TestAsyncDiodeWriter_Close
component/logging/writer.TestAsyncDiodeWriter_WriteAfterClose
```

`ConcurrentWrites` may pass or fail in a given run. Stop and diagnose before proceeding if any other test or package fails.

- [ ] **Step 8: Remove only the generated writer test artifact and verify status**

If the full suite created this exact file, remove it:

```text
component/logging/writer/D:/invalid/path/test.log
```

For that exact artifact only, run:

```bash
rm -f 'component/logging/writer/D:/invalid/path/test.log'
rmdir 'component/logging/writer/D:/invalid/path'
rmdir 'component/logging/writer/D:/invalid'
rmdir 'component/logging/writer/D:'
```

Do not use a repository-wide clean command and do not delete any other untracked path. Then run:

```bash
git status --short --untracked-files=all
```

Expected: only the documentation and `.codegraph-qa-out` changes from this task are present.

- [ ] **Step 9: Review and commit the documentation boundary**

```bash
git add README.md docs/reference/components.md docs/reference/examples.md docs/reference/feature-status.md docs/reference/known-test-failures.md docs/guides/web-runtime.md docs/guides/background-tasks.md docs/guides/logging.md .codegraph-qa-out/todo.md .codegraph-qa-out/component-hierarchical-namespaces.md
git diff --cached --check
git diff --cached --stat
git diff --cached --ignore-cr-at-eol
git commit -m "docs: define hierarchical component namespaces"
```

Expected: only current documentation and the focused durable analysis change; no historical specs, generated CodeGraph data, Go source, dependencies or configuration appear in this commit.

---

### Task 5: Perform the final migration audit

**Files:**
- Verify only; no file changes expected.

**Interfaces:**
- Consumes: the four implementation commits.
- Produces: evidence that the branch satisfies the design and is ready for review without remote publication.

- [ ] **Step 1: Verify the four-commit history after the design commit**

Run:

```bash
git log --oneline 4853332..HEAD
```

Expected subjects, newest first:

```text
docs: define hierarchical component namespaces
refactor: move log writers into logging namespace
refactor: move task logger adaptor into task namespace
refactor: move json codecs into codec namespace
```

- [ ] **Step 2: Audit rename and protected-file scope across the whole range**

Run:

```bash
git diff --find-renames --summary 4853332..HEAD
git diff --name-only 4853332..HEAD -- go.mod go.sum example_config .codegraph
git diff --check 4853332..HEAD
```

Expected:

- The range contains seven content-preserving renames and one rename with the task package-clause change.
- The protected-file command prints nothing.
- `git diff --check` exits 0.

- [ ] **Step 3: Repeat the final path and build gates from a clean process cache**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-hierarchy-final go list ./component/...
GOCACHE=/tmp/fiberhouse-component-hierarchy-final go test ./... -run '^$'
GOCACHE=/tmp/fiberhouse-component-hierarchy-final go build ./...
rg -n 'github.com/lamxy/fiberhouse/component/(jsoncodec|tasklog|writer)' --glob '*.go' .
rg -n 'component/(jsoncodec|tasklog|writer)' README.md docs/reference docs/guides
```

Expected: list, compile and build commands exit 0; both `rg` commands exit 1 with no matches.

- [ ] **Step 4: Confirm the worktree is clean and stop before publication**

Run:

```bash
git status --short
```

Expected: no output. Do not push the branch or open a pull request without a new explicit user request.
