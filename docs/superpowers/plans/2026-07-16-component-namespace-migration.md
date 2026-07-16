# Component Namespace Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert `component/` into a pure capability namespace, move the Dig container to `component/container`, move cache and database packages under `component/`, and make the Mongo decimal BSON codec private to dbmongo without changing runtime behavior.

**Architecture:** `component/` has no root Go package. Root-safe leaf components such as `component/container` may be imported by `fiberhouse`; components that depend on `fiberhouse`, including cache and database, remain application-assembled and are never re-exported through a component facade. Mongo's decimal codec lives at `component/database/dbmongo/internal/mongodecimal` so its ownership and Go visibility match its only consumer.

**Tech Stack:** Go, Uber Dig, Ristretto, go-redis, GORM/MySQL, MongoDB Go Driver v2, CodeGraph, Git.

## Global Constraints

- The migration is intentionally breaking; do not create compatibility shims for any old package path.
- Keep package names and exported cache/database/container symbols unchanged except for the package selector change from `component` to `container`.
- Keep YAML keys, GlobalManager keys, log origins, constructors, lifecycle, errors, and runtime behavior unchanged.
- Do not add `init()`, automatic registration, automatic connection, automatic shutdown, or a parent `component` facade.
- Keep `cachelocal`, `cacheremote`, `cache2`, `dbmysql`, and `dbmongo` names; do not normalize them to shorter names.
- Keep `mongodecimal` as an independent package under `component/database/dbmongo/internal/`.
- Use `git mv` for every relocation so Git can retain file history.
- Format only Go files touched by a task. Inspect line-ending changes after every formatting pass and invoke `normalizing-git-line-endings` if a semantic edit becomes a whole-file LF/CRLF diff.
- Do not edit generated `.codegraph/codegraph.db*`, daemon pid, or daemon log files.
- Preserve all pre-existing response/Protobuf and documentation work in the original dirty worktree; implementation starts in an isolated worktree from the approved design/plan commit.
- Do not add dependencies or change `go.mod`/`go.sum`.

## File Structure Map

### Relocations

- `component/dig_container.go` -> `component/container/dig_container.go`
- `cache/cache2/level2_cache.go` -> `component/cache/cache2/level2_cache.go`
- `cache/cache_errors.go` -> `component/cache/cache_errors.go`
- `cache/cache_interface.go` -> `component/cache/cache_interface.go`
- `cache/cache_option.go` -> `component/cache/cache_option.go`
- `cache/cache_option_test.go` -> `component/cache/cache_option_test.go`
- `cache/cache_utility.go` -> `component/cache/cache_utility.go`
- `cache/cachelocal/local_cache.go` -> `component/cache/cachelocal/local_cache.go`
- `cache/cachelocal/type.go` -> `component/cache/cachelocal/type.go`
- `cache/cacheremote/cache_model.go` -> `component/cache/cacheremote/cache_model.go`
- `cache/cacheremote/redis_cache.go` -> `component/cache/cacheremote/redis_cache.go`
- `cache/helper.go` -> `component/cache/helper.go`
- `cache/helper_test.go` -> `component/cache/helper_test.go`
- `database/dbmongo/interface.go` -> `component/database/dbmongo/interface.go`
- `database/dbmongo/mongo.go` -> `component/database/dbmongo/mongo.go`
- `database/dbmongo/mongo_model_impl.go` -> `component/database/dbmongo/mongo_model_impl.go`
- `database/dbmysql/interface.go` -> `component/database/dbmysql/interface.go`
- `database/dbmysql/mysql.go` -> `component/database/dbmysql/mysql.go`
- `database/dbmysql/mysql_model_impl.go` -> `component/database/dbmysql/mysql_model_impl.go`
- `component/mongodecimal/mongo_decimal.go` -> `component/database/dbmongo/internal/mongodecimal/mongo_decimal.go`

### New test

- `component/container/dig_container_test.go`: verifies constructor injection and the generic wrapper after the package move.

### Consumer updates

- `context_impl.go`: import `component/container`; update the stored Dig container type, constructor, and getter.
- `context_interface.go`: expose `*container.DigContainer` from `ICommandContext`.
- `example_application/command/application/commands/test_orm_command.go`: use `container.NewWrap` and `container.Invoke`.
- `example_application/module/command-module/service/example_mysql_service.go`: use `container.NewWrap` and `container.Invoke`.
- `example_application/application_impl.go`: import the new cache and database paths.
- `example_application/command/application/application.go`: import the new cache and database paths.
- `example_application/module/task_impl.go`: import `component/cache`.
- `example_application/module/example-module/service/example_service.go`: import `component/cache`.
- `example_application/module/example-module/model/example_model.go`: import the new dbmongo path.
- `example_application/module/command-module/model/mongodb_model.go`: import the new dbmongo path.
- `example_application/module/command-module/model/mysql_model.go`: import the new dbmysql path.

### Documentation and current analysis

- `docs/reference/components.md`: define the component namespace rules and update container/cache/database/mongodecimal entries.
- `docs/guides/cache.md`: update source paths and links.
- `docs/guides/database.md`: update source paths and links.
- `docs/reference/feature-status.md`: treat cache/database as component subtrees rather than root directories.
- `.codegraph-qa-out/codebase_summary.md`: update the current directory index; preserve unrelated response/Protobuf content from its branch baseline.

---

### Task 1: Move the Dig Container into `component/container`

**Files:**
- Move: `component/dig_container.go` -> `component/container/dig_container.go`
- Create: `component/container/dig_container_test.go`
- Modify: `context_impl.go:13-15,173-184,233-236`
- Modify: `context_interface.go:9-14,54-58`
- Modify: `example_application/command/application/commands/test_orm_command.go:3-9,48-74`
- Modify: `example_application/module/command-module/service/example_mysql_service.go:3-10,34-50`

**Interfaces:**
- Consumes: Uber Dig's `dig.Container`, existing `DigContainer`, `Wrap[T]`, `NewDigContainer`, `NewDigContainerOnce`, `Container`, `Invoke[T]`, and `ResetDigContainer` behavior.
- Produces: package `github.com/lamxy/fiberhouse/component/container` with the same exported names; `ICommandContext.GetDigContainer() *container.DigContainer`.

- [ ] **Step 1: Create an isolated execution worktree and capture the baseline**

Invoke `superpowers:using-git-worktrees` before changing files. Create a feature branch named `refactor/component-namespace` from the commit containing this plan. In the new worktree run:

```bash
git status --short
git ls-files --eol component/dig_container.go context_impl.go context_interface.go
GOCACHE=/tmp/fiberhouse-component-baseline go build ./...
GOCACHE=/tmp/fiberhouse-component-baseline go test ./...
```

Expected:

- `git status --short` prints nothing in the new worktree.
- `go build ./...` exits 0.
- Record the exact `go test ./...` baseline. The repository's documented baseline may include `bootstrap.Test_Config_EnvOverrideAndSingleton` and `component/writer` failures; no later task may add a new failing package or test.

- [ ] **Step 2: Write the container tests before moving the implementation**

Create `component/container/dig_container_test.go` with:

```go
package container

import "testing"

type testDependency struct {
	value string
}

func TestNewDigContainerProvideAndInvoke(t *testing.T) {
	dc := NewDigContainer().Provide(func() *testDependency {
		return &testDependency{value: "resolved"}
	})
	if got := dc.GetErrorCount(); got != 0 {
		t.Fatalf("GetErrorCount() = %d, want 0; errors: %v", got, dc.GetProvideErrs())
	}

	var resolved *testDependency
	if err := dc.Invoke(func(dep *testDependency) {
		resolved = dep
	}); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if resolved == nil || resolved.value != "resolved" {
		t.Fatalf("resolved dependency = %#v, want value %q", resolved, "resolved")
	}
}

func TestWrapSetGet(t *testing.T) {
	wrap := NewWrap[*testDependency]()
	if got := wrap.Get(); got != nil {
		t.Fatalf("new Wrap.Get() = %#v, want nil", got)
	}

	want := &testDependency{value: "wrapped"}
	wrap.Set(want)
	if got := wrap.Get(); got != want {
		t.Fatalf("Wrap.Get() = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 3: Run the new test to verify the red state**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-task1 go test ./component/container
```

Expected: non-zero exit with undefined `NewDigContainer` and `NewWrap`, because the implementation has not moved yet.

- [ ] **Step 4: Move the implementation and change its package identity**

Run:

```bash
git mv component/dig_container.go component/container/dig_container.go
```

In `component/container/dig_container.go`, make these exact semantic changes:

```go
package container
```

Update these exact documentation-example tokens in the same file:

```text
component.NewWrap -> container.NewWrap
component.Container -> container.Container
component.Invoke -> container.Invoke
```

Do not change any exported declaration or implementation body.

- [ ] **Step 5: Update the four consumer files**

Use this import in all four consumers:

```go
"github.com/lamxy/fiberhouse/component/container"
```

Apply these exact selector/type replacements:

```go
// context_impl.go
digContainer *container.DigContainer
digContainer: container.NewDigContainerOnce(),
func (c *CmdContext) GetDigContainer() *container.DigContainer

// context_interface.go
GetDigContainer() *container.DigContainer

// test_orm_command.go and example_mysql_service.go
container.NewWrap
container.Invoke
```

Remove only the old exact import:

```go
"github.com/lamxy/fiberhouse/component"
```

- [ ] **Step 6: Format and inspect semantic scope**

Run:

```bash
gofmt -w component/container/dig_container.go component/container/dig_container_test.go context_impl.go context_interface.go example_application/command/application/commands/test_orm_command.go example_application/module/command-module/service/example_mysql_service.go
git diff --check
git diff --stat
git diff --ignore-space-at-eol -- component/container context_impl.go context_interface.go example_application/command/application/commands/test_orm_command.go example_application/module/command-module/service/example_mysql_service.go
```

Expected: no whitespace errors; semantic diff contains only package/import/selector changes plus the two new tests. If the normal diff shows unrelated whole-file changes that disappear with `--ignore-space-at-eol`, stop and use `normalizing-git-line-endings` before continuing.

- [ ] **Step 7: Run focused tests and the repository build**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-task1 go test ./component/container
GOCACHE=/tmp/fiberhouse-component-task1 go build ./...
```

Expected: both commands exit 0; the new tests report PASS.

- [ ] **Step 8: Verify the old root component API is gone**

Run:

```bash
rg -n '"github\.com/lamxy/fiberhouse/component"|\bcomponent\.(DigContainer|NewDigContainerOnce|NewWrap|Invoke)' --glob '*.go' .
rg --files component | rg '^component/[^/]+\.go$'
```

Expected: both commands produce no matches and exit 1 because no old import/selector or root-level Go file remains.

- [ ] **Step 9: Commit the container migration**

Run:

```bash
git add component/container context_impl.go context_interface.go example_application/command/application/commands/test_orm_command.go example_application/module/command-module/service/example_mysql_service.go
git diff --cached --check
git diff --cached --name-status
git commit -m "refactor: move dig container into component namespace"
```

Expected: the staged set contains the Dig container rename, the new test, and exactly four consumer files.

---

### Task 2: Move Cache Packages under `component/cache`

**Files:**
- Move: all 12 tracked files under `cache/` to the same relative paths under `component/cache/`.
- Modify after move: `component/cache/cache_option.go`
- Modify after move: `component/cache/cache2/level2_cache.go`
- Modify after move: `component/cache/cachelocal/local_cache.go`
- Modify after move: `component/cache/cacheremote/cache_model.go`
- Modify after move: `component/cache/cacheremote/redis_cache.go`
- Modify: `example_application/application_impl.go`
- Modify: `example_application/command/application/application.go`
- Modify: `example_application/module/task_impl.go`
- Modify: `example_application/module/example-module/service/example_service.go`

**Interfaces:**
- Consumes: existing packages `cache`, `cache/cache2`, `cache/cachelocal`, and `cache/cacheremote` with all current exported symbols.
- Produces: the same package names and symbols at `component/cache`, `component/cache/cache2`, `component/cache/cachelocal`, and `component/cache/cacheremote`.

- [ ] **Step 1: Move the complete cache tree**

Run:

```bash
git mv cache component/cache
```

Confirm the count:

```bash
rg --files component/cache | wc -l
```

Expected: `12`.

- [ ] **Step 2: Run cache tests to verify the red state**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-task2 go test ./component/cache/...
```

Expected: non-zero exit because cache2/cachelocal/cacheremote still import one or more old `github.com/lamxy/fiberhouse/cache...` paths.

- [ ] **Step 3: Update imports inside the moved cache tree**

Apply this exact path mapping in the moved Go source:

```text
github.com/lamxy/fiberhouse/cache/cachelocal
-> github.com/lamxy/fiberhouse/component/cache/cachelocal

github.com/lamxy/fiberhouse/cache
-> github.com/lamxy/fiberhouse/component/cache
```

The affected implementation files are exactly:

```text
component/cache/cache2/level2_cache.go
component/cache/cachelocal/local_cache.go
component/cache/cacheremote/cache_model.go
component/cache/cacheremote/redis_cache.go
```

Also update the literal import shown in the documentation comment in `component/cache/cache_option.go`:

```go
import "github.com/lamxy/fiberhouse/component/cache"
```

Do not change package clauses or selectors; package names remain `cache`, `cache2`, `cachelocal`, and `cacheremote`.

- [ ] **Step 4: Update the four application consumers**

Apply these exact import path mappings:

```text
github.com/lamxy/fiberhouse/cache
-> github.com/lamxy/fiberhouse/component/cache

github.com/lamxy/fiberhouse/cache/cache2
-> github.com/lamxy/fiberhouse/component/cache/cache2

github.com/lamxy/fiberhouse/cache/cachelocal
-> github.com/lamxy/fiberhouse/component/cache/cachelocal

github.com/lamxy/fiberhouse/cache/cacheremote
-> github.com/lamxy/fiberhouse/component/cache/cacheremote
```

Update only these files:

```text
example_application/application_impl.go
example_application/command/application/application.go
example_application/module/task_impl.go
example_application/module/example-module/service/example_service.go
```

Keep all selectors (`cache.Cache`, `cache.GetCached`, `cache2.NewLevel2Cache`, `cachelocal.NewLocalCache`, and `cacheremote.NewRedisDb`) unchanged.

- [ ] **Step 5: Format only touched cache consumers and implementations**

Run:

```bash
gofmt -w component/cache/cache2/level2_cache.go component/cache/cachelocal/local_cache.go component/cache/cacheremote/cache_model.go component/cache/cacheremote/redis_cache.go example_application/application_impl.go example_application/command/application/application.go example_application/module/task_impl.go example_application/module/example-module/service/example_service.go
git diff --check
git diff --stat
```

Expected: no whitespace errors and no package name or behavior changes.

- [ ] **Step 6: Run cache tests and the repository build**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-task2 go test ./component/cache/...
GOCACHE=/tmp/fiberhouse-component-task2 go build ./...
```

Expected: both commands exit 0.

- [ ] **Step 7: Verify old cache imports and the old directory are gone**

Run:

```bash
rg -n 'github\.com/lamxy/fiberhouse/cache(/[^"[:space:]]*)?' --glob '*.go' .
test ! -d cache
```

Expected: `rg` prints no matches and exits 1; `test` exits 0.

- [ ] **Step 8: Commit the cache migration**

Run:

```bash
git add component/cache example_application/application_impl.go example_application/command/application/application.go example_application/module/task_impl.go example_application/module/example-module/service/example_service.go
git diff --cached --check
git diff --cached --name-status
git commit -m "refactor: move cache into component namespace"
```

Expected: the staged set contains 12 cache renames plus four consumer files, with no database or documentation edits.

---

### Task 3: Move Database Packages and Internalize MongoDecimal

**Files:**
- Move: all 6 tracked files under `database/` to the same relative paths under `component/database/`.
- Move: `component/mongodecimal/mongo_decimal.go` -> `component/database/dbmongo/internal/mongodecimal/mongo_decimal.go`
- Modify after move: `component/database/dbmongo/mongo.go`
- Modify: `example_application/application_impl.go`
- Modify: `example_application/command/application/application.go`
- Modify: `example_application/module/example-module/model/example_model.go`
- Modify: `example_application/module/command-module/model/mongodb_model.go`
- Modify: `example_application/module/command-module/model/mysql_model.go`

**Interfaces:**
- Consumes: existing `dbmysql`, `dbmongo`, and public `mongodecimal.MongoDecimal` implementation.
- Produces: unchanged `dbmysql` and `dbmongo` APIs at `component/database/...`; private `dbmongo/internal/mongodecimal.MongoDecimal` implementing `bson.ValueEncoder` and `bson.ValueDecoder`.

- [ ] **Step 1: Move database and the Mongo decimal codec**

Run:

```bash
git mv database component/database
mkdir -p component/database/dbmongo/internal
git mv component/mongodecimal component/database/dbmongo/internal/mongodecimal
```

Confirm the moved files:

```bash
rg --files component/database | sort
```

Expected: six dbmongo/dbmysql files plus `dbmongo/internal/mongodecimal/mongo_decimal.go`.

- [ ] **Step 2: Run database tests to verify the red state**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-task3 go test ./component/database/...
```

Expected: non-zero exit because `component/database/dbmongo/mongo.go` still imports the removed public `component/mongodecimal` path.

- [ ] **Step 3: Update dbmongo's private codec import**

In `component/database/dbmongo/mongo.go`, replace:

```go
"github.com/lamxy/fiberhouse/component/mongodecimal"
```

with:

```go
"github.com/lamxy/fiberhouse/component/database/dbmongo/internal/mongodecimal"
```

Keep these registry calls unchanged:

```go
registry.RegisterTypeEncoder(reflect.TypeOf(decimal.Decimal{}), new(mongodecimal.MongoDecimal))
registry.RegisterTypeDecoder(reflect.TypeOf(decimal.Decimal{}), new(mongodecimal.MongoDecimal))
```

Do not rename or unexport `MongoDecimal`; Go `internal` supplies the visibility boundary.

- [ ] **Step 4: Update the five application consumers**

Apply these exact path mappings:

```text
github.com/lamxy/fiberhouse/database/dbmongo
-> github.com/lamxy/fiberhouse/component/database/dbmongo

github.com/lamxy/fiberhouse/database/dbmysql
-> github.com/lamxy/fiberhouse/component/database/dbmysql
```

Update only:

```text
example_application/application_impl.go
example_application/command/application/application.go
example_application/module/example-module/model/example_model.go
example_application/module/command-module/model/mongodb_model.go
example_application/module/command-module/model/mysql_model.go
```

Keep every `dbmongo.*` and `dbmysql.*` selector unchanged.

- [ ] **Step 5: Format touched database and consumer files**

Run:

```bash
gofmt -w component/database/dbmongo/mongo.go component/database/dbmongo/internal/mongodecimal/mongo_decimal.go example_application/application_impl.go example_application/command/application/application.go example_application/module/example-module/model/example_model.go example_application/module/command-module/model/mongodb_model.go example_application/module/command-module/model/mysql_model.go
git diff --check
git diff --stat
```

Expected: no whitespace errors; the codec implementation body, constructors, and model behavior remain unchanged.

- [ ] **Step 6: Run database tests and the repository build**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-task3 go test ./component/database/...
GOCACHE=/tmp/fiberhouse-component-task3 go build ./...
```

Expected: both commands exit 0. Tests compile dbmysql/dbmongo without calling their connection constructors.

- [ ] **Step 7: Verify old database and mongodecimal paths are gone**

Run:

```bash
rg -n 'github\.com/lamxy/fiberhouse/database(/[^"[:space:]]*)?|github\.com/lamxy/fiberhouse/component/mongodecimal' --glob '*.go' .
test ! -d database
test ! -d component/mongodecimal
```

Expected: `rg` prints no matches and exits 1; both `test` commands exit 0.

- [ ] **Step 8: Commit the database migration**

Run:

```bash
git add -A -- component/database component/mongodecimal example_application/application_impl.go example_application/command/application/application.go example_application/module/example-module/model/example_model.go example_application/module/command-module/model/mongodb_model.go example_application/module/command-module/model/mysql_model.go
git diff --cached --check
git diff --cached --name-status
git commit -m "refactor: move database into component namespace"
```

Expected: Git stages the six database renames, the mongodecimal relocation, and exactly five consumer files.

---

### Task 4: Document the Component Namespace and Current Paths

**Files:**
- Modify: `docs/reference/components.md`
- Modify: `docs/guides/cache.md`
- Modify: `docs/guides/database.md`
- Modify: `docs/reference/feature-status.md`
- Modify: `.codegraph-qa-out/codebase_summary.md`

**Interfaces:**
- Consumes: the completed directory layout from Tasks 1-3 and the approved design spec.
- Produces: current user documentation and CodeGraph durable summary that describe `component/` as a namespace and link only to current source paths.

- [ ] **Step 1: Add the formal component definition to the component reference**

Near the top of `docs/reference/components.md`, after the opening description, add this exact policy text:

```markdown
`component/` 是 FiberHouse 提供的内置、可选装配、可复用能力的命名空间，不是严格的底层依赖层。根目录不提供 Go API；每个一级子目录代表可独立理解和装配的能力。

子组件可以依赖 FiberHouse 核心接口，但依赖 root package 的组件必须由应用注册器或 Provider 装配，root package 和父级命名空间不得反向聚合导入。仅服务于单个组件的辅助实现放入该组件的 `internal/` 子树。
```

Replace the affected table rows with these exact current entries:

```markdown
| `component/container` | 基于 Uber Dig 的启动期依赖注入容器与泛型解析包装器 | `CmdContext`、CLI `test-orm` 与其 service 装配 | 单例容器只用于启动装配；`Provide` 错误显式收集，`ResetDigContainer` 不支持并发运行期调用 | 内部工具 | [命令行指南](../guides/command-line.md) |
| `component/cache` | 通用 `Cache` 接口、选项池、缓存读取工具与保护机制 | 示例 service、任务注册器和应用全局 initializer | 实例由应用显式注册；目录不统一接管 local/remote/L2 的创建、等待和关闭 | 实验性 | [缓存指南](../guides/cache.md)、[GlobalManager](../guides/global-manager.md) |
| `component/cache/cachelocal` | 基于 Ristretto 的本地缓存 | Web/CLI 应用 initializer 与 L2 cache | 应用持有实例；异步写入后可调用 `Wait`，关闭后操作返回缓存关闭错误 | 实验性 | [缓存指南](../guides/cache.md) |
| `component/cache/cacheremote` | 基于 go-redis 的远程缓存、Redis client 与缓存定位辅助 | Web/CLI initializer、任务系统与 L2 cache | 应用持有 Redis client；连接、重建、熔断及关闭语义保持由实现暴露 | 实验性 | [缓存指南](../guides/cache.md) |
| `component/cache/cache2` | 组合 local/remote 的二级缓存和异步同步策略 | Web 应用的 GlobalManager initializer | 持有两个 ants pool；应用负责创建依赖 cache 并在停止阶段关闭 | 实验性 | [缓存指南](../guides/cache.md) |
| `component/database/dbmysql` | GORM/MySQL client、连接池、健康检查及 model locator | 示例 Web/CLI 的 GlobalManager initializer 与 MySQL model/service | 应用持有并负责 `Close`；初始化会校验 DSN、连接并 ping；`Rebuild` 替换 client 但不关闭旧连接，读侧未与替换锁配套 | 实验性 | [数据库指南](../guides/database.md)、[GlobalManager](../guides/global-manager.md) |
| `component/database/dbmongo` | MongoDB v2 client、连接选项、健康检查及 model locator | 示例 Web/CLI initializer 与 Mongo model | 应用持有并负责 `Disconnect`；连接/命令错误向上传递；`Rebuild` 同样不关闭旧 client，读侧未与替换锁配套 | 实验性 | [数据库指南](../guides/database.md)、[GlobalManager](../guides/global-manager.md) |
| `component/database/dbmongo/internal/mongodecimal` | 在 `decimal.Decimal` 与 BSON Decimal128 间转换 | 仅 `dbmongo.NewClient` 的 BSON registry | dbmongo 私有无状态 codec；类型不符、解析或读写失败均返回错误 | 内部实现 | [数据库指南](../guides/database.md) |
```

- [ ] **Step 2: Update cache guide links**

In `docs/guides/cache.md`, replace the opening source link with:

```markdown
FiberHouse 在 [`component/cache`](../../component/cache/) 下定义统一 `Cache` 接口，并分别提供基于 Ristretto 的本地缓存、基于 go-redis 的远程缓存，以及组合二者的 L2 缓存。
```

Replace the source entry links with:

```markdown
源码入口：[`component/cache/cache_interface.go`](../../component/cache/cache_interface.go)、[`component/cache/cache_option.go`](../../component/cache/cache_option.go)、[`component/cache/cache_utility.go`](../../component/cache/cache_utility.go)、[`component/cache/cachelocal`](../../component/cache/cachelocal/)、[`component/cache/cacheremote`](../../component/cache/cacheremote/) 与 [`component/cache/cache2`](../../component/cache/cache2/)。
```

Keep configuration examples and `cache.*` key names unchanged.

- [ ] **Step 3: Update database guide links**

In `docs/guides/database.md`, replace the source entry line with:

```markdown
源码入口：[`component/database/dbmysql/mysql.go`](../../component/database/dbmysql/mysql.go)、[`component/database/dbmysql/mysql_model_impl.go`](../../component/database/dbmysql/mysql_model_impl.go)、[`component/database/dbmongo/mongo.go`](../../component/database/dbmongo/mongo.go) 与 [`component/database/dbmongo/mongo_model_impl.go`](../../component/database/dbmongo/mongo_model_impl.go)。
```

Keep `database.mysql`, `database.mongodb`, schema, collection, and model terminology unchanged.

- [ ] **Step 4: Update the feature-status path inventory and CodeGraph summary**

In `docs/reference/feature-status.md`, change the current-source inventory so it lists:

```markdown
`component/`（包含 `container/`、`cache/`、`database/` 等组件）
```

and no longer lists root `cache/` or root `database/` as separate directories.

In `.codegraph-qa-out/codebase_summary.md`, replace the three old directory rows:

```markdown
| `component/` | 通用組件（validate、dig 容器等） |
| `cache/` | 緩存抽象層 |
| `database/` | DB 初始化 |
```

with one current row:

```markdown
| `component/` | 內置可選組件命名空間，包含 container、cache、database、validate、codec 等能力 |
```

Preserve all unrelated summary edits already present in the execution branch baseline.

- [ ] **Step 5: Verify current documentation links and old path references**

Run:

```bash
rg -n '\.\./\.\./(cache|database)/|`(cache|database)/' docs/guides docs/reference
rg -n '^\| `(cache|database)/' .codegraph-qa-out/codebase_summary.md
git diff --check -- docs/reference/components.md docs/guides/cache.md docs/guides/database.md docs/reference/feature-status.md .codegraph-qa-out/codebase_summary.md
```

Expected: both `rg` commands print no matches and exit 1; the diff check exits 0. Historical specs/plans and historical CodeGraph analyses may retain old paths when they explicitly describe a migration or past source location.

- [ ] **Step 6: Commit the documentation update**

Run:

```bash
git add docs/reference/components.md docs/guides/cache.md docs/guides/database.md docs/reference/feature-status.md .codegraph-qa-out/codebase_summary.md
git diff --cached --check
git diff --cached --name-status
git commit -m "docs: define component namespace layout"
```

Expected: exactly five documentation/current-analysis files are staged.

---

### Task 5: Run the Final Architecture and Regression Audit

**Files:**
- Verify: all files changed by Tasks 1-4.
- Do not create or modify files unless a verification command exposes a migration defect.

**Interfaces:**
- Consumes: `component/container`, `component/cache/...`, `component/database/...`, updated application assembly, and updated current documentation.
- Produces: evidence that the new package graph builds, focused tests pass, old product imports are gone, and the full-suite failure set has not increased.

- [ ] **Step 1: Verify the exact package set**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-final go list ./component/container ./component/cache/... ./component/database/...
```

Expected package paths:

```text
github.com/lamxy/fiberhouse/component/container
github.com/lamxy/fiberhouse/component/cache
github.com/lamxy/fiberhouse/component/cache/cache2
github.com/lamxy/fiberhouse/component/cache/cachelocal
github.com/lamxy/fiberhouse/component/cache/cacheremote
github.com/lamxy/fiberhouse/component/database/dbmongo
github.com/lamxy/fiberhouse/component/database/dbmongo/internal/mongodecimal
github.com/lamxy/fiberhouse/component/database/dbmysql
```

No command may report an import cycle or missing package.

- [ ] **Step 2: Prove old product import paths are absent**

Run:

```bash
rg -n 'github\.com/lamxy/fiberhouse/(cache|database)(/[^"[:space:]]*)?' --glob '*.go' .
rg -n '"github\.com/lamxy/fiberhouse/component"|github\.com/lamxy/fiberhouse/component/mongodecimal' --glob '*.go' .
rg --files component | rg '^component/[^/]+\.go$'
GOCACHE=/tmp/fiberhouse-component-final go list -f '{{join .Imports "\n"}}' . | rg 'github\.com/lamxy/fiberhouse/component/(cache|database)'
```

Expected: all four search/filter commands print no matches and exit 1. The final command proves the root `fiberhouse` package does not import cache or database implementations.

- [ ] **Step 3: Run focused tests**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-final go test ./component/container/...
GOCACHE=/tmp/fiberhouse-component-final go test ./component/cache/...
GOCACHE=/tmp/fiberhouse-component-final go test ./component/database/...
```

Expected: all three commands exit 0.

- [ ] **Step 4: Run the full build and test suite**

Run:

```bash
GOCACHE=/tmp/fiberhouse-component-final go build ./...
GOCACHE=/tmp/fiberhouse-component-final go test ./...
```

Expected:

- `go build ./...` exits 0.
- `go test ./...` either exits 0 or reproduces only the exact packages/tests recorded in Task 1's baseline. No new failure, missing package, undefined selector, or import cycle is acceptable.

- [ ] **Step 5: Prove dependency/configuration files did not change**

Run:

```bash
git diff --exit-code "$(git merge-base HEAD main)"..HEAD -- go.mod go.sum example_config
```

Expected: exit 0 with no diff.

- [ ] **Step 6: Audit commit and file scope**

Run:

```bash
git diff --check "$(git merge-base HEAD main)"..HEAD
git log --oneline --decorate "$(git merge-base HEAD main)"..HEAD
git diff --name-status "$(git merge-base HEAD main)"..HEAD
git status --short
```

Expected:

- Diff check exits 0.
- Four implementation commits appear: container, cache, database, and documentation.
- Name-status output contains only the moves, tests, consumers, docs, and current CodeGraph summary listed in this plan.
- Worktree status is clean.

- [ ] **Step 7: Refresh CodeGraph semantically without editing generated index data**

Run:

```bash
codegraph explore "component/container component/cache component/database/dbmongo internal/mongodecimal dependency paths and callers"
```

Expected: current source paths are returned. If the watcher is stale, record that fact in the handoff; do not edit `.codegraph/codegraph.db*` manually.

## Final Handoff

Report:

- The four commit hashes.
- Focused test results.
- Full build result.
- Full-suite result compared with the Task 1 baseline.
- Confirmation that old Go imports are absent.
- Any stale CodeGraph index observation.
- The integration options provided by `superpowers:finishing-a-development-branch`.
