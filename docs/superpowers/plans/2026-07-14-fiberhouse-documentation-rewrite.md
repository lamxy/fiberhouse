# FiberHouse Documentation Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the oversized, partially stale root README with a source-backed entry document and add a structured FiberHouse manual under `docs/`.

**Architecture:** Documentation is organized by reader intent rather than Go package boundaries. Reference pages establish vocabulary and maturity first; concept and guide pages then describe verified behavior; the root README and getting-started path are written after those pages so they can remain concise and link to one canonical explanation per topic.

**Tech Stack:** Markdown, Go 1.25, CodeGraph CLI, ast-grep, ripgrep, Git, Docker Compose for the included example verification.

## Global Constraints

- Root `README.md` primarily serves business application developers who are evaluating or first adopting FiberHouse.
- Documentation is Simplified Chinese; Go identifiers, configuration keys, paths, MIME types, and protocol names retain their source spelling.
- Current source is the authority. `.codegraph-qa-out/` is prior analysis only and must be rechecked against CodeGraph/current files.
- Before locating or understanding code, use CodeGraph because this repository contains `.codegraph/`; use exact file reads, ast-grep, or rg only for gaps and cross-checks.
- Do not modify runtime code, configuration semantics, generated files, or example business logic.
- Do not describe `example_main`, `example_config`, or `example_application` as a production template or stable API.
- Do not describe plugins, RPC, MQ, i18n, Gin TLS, incomplete cache protection, or unused lifecycle hooks as complete production capabilities.
- Every page must cover the relevant responsibilities, entry points, defaults, assembly conditions, lifecycle, error semantics, concurrency boundaries, and known limitations.
- Examples and commands must match Go 1.25, module `github.com/lamxy/fiberhouse`, current repository paths, and current exported APIs.
- Use repository-relative Markdown links. Do not use stale `provider/context`, `provider/adaptor`, or other pre-refactor paths.
- Use `apply_patch` for documentation edits. Preserve unrelated user changes and avoid line-ending churn outside changed Markdown files.
- A source-level concern is documented as a limitation or static-analysis observation; this documentation task does not fix the underlying implementation.

## File Structure

**Modify:**

- `README.md` — concise public entry point and documentation gateway.

**Create:**

- `docs/README.md` — manual index and reading paths.
- `docs/getting-started.md` — repository example and external application onboarding.
- `docs/concepts/architecture.md` — system structure and dependency boundaries.
- `docs/concepts/startup-lifecycle.md` — Web startup and shutdown sequence.
- `docs/concepts/provider-system.md` — Provider/Manager/Location mechanism.
- `docs/concepts/context-and-locators.md` — contexts, locators, and container access.
- `docs/guides/configuration.md` — BootConfig/AppConfig and configuration precedence.
- `docs/guides/logging.md` — logger and writer behavior.
- `docs/guides/web-runtime.md` — Fiber/Gin runtime and context adaptation.
- `docs/guides/response-and-serialization.md` — response envelope and negotiation.
- `docs/guides/errors-and-recovery.md` — error propagation, exceptions, and panic recovery.
- `docs/guides/global-manager.md` — lazy global object lifecycle.
- `docs/guides/cache.md` — local, Redis, and L2 caching.
- `docs/guides/command-line.md` — CLI application lifecycle.
- `docs/guides/background-tasks.md` — Asynq dispatcher/worker integration.
- `docs/guides/database.md` — MySQL/MongoDB integration and model bases.
- `docs/guides/validation.md` — multilingual validation and custom tags.
- `docs/guides/extending-fiberhouse.md` — supported extension contracts.
- `docs/reference/examples.md` — example directory map and caveats.
- `docs/reference/components.md` — internal/helper component catalog.
- `docs/reference/feature-status.md` — implementation maturity matrix.

---

### Task 1: Establish the reference baseline

**Files:**

- Create: `docs/reference/feature-status.md`
- Create: `docs/reference/components.md`
- Create: `docs/reference/examples.md`

**Interfaces:**

- Consumes: `default.go`, `plugins/`, `rpc/`, placeholder packages, `component/`, `example_main/`, `example_config/`, `example_application/`, and the Global Constraints.
- Produces: the status labels `已接入`, `实验性`, `内部工具`, and `预留/占位`; the canonical boundary between framework/default/example behavior used by every later task.

- [ ] **Step 1: Reconfirm feature wiring and example boundaries**

Run focused CodeGraph queries for `DefaultProviders`, `DefaultPManagers`, plugin/RPC call paths, component callers, and the three example directories. Cross-check placeholder/TODO declarations with:

```bash
rg -n 'placeholder|TODO|待完善|未实现|尚未实现' plugins rpc component middleware --glob '*.go' --glob '*.md'
```

Expected: concrete TODOs in `plugins`, placeholder Markdown under component/middleware areas, and no evidence of a complete plugin/RPC server lifecycle.

- [ ] **Step 2: Write the feature status matrix**

Create `docs/reference/feature-status.md` with these exact sections:

```markdown
# 功能状态
## 如何理解状态
## 已接入的核心能力
## 实验性或存在明显限制的能力
## 内部工具
## 预留与占位
## 判断依据
```

The tables must distinguish implementation, default registration, application opt-in, and example-only use. Include Fiber, Gin, Provider/Manager/Location, bootstrap/config/logging, response JSON/MsgPack/Protobuf, recovery, GlobalManager, local/Redis/L2 cache, tasks, CLI, MySQL/MongoDB, validation, plugins, RPC, MQ, i18n, and unused/incomplete lifecycle hooks. Name the canonical guide for each limited capability as plain text; Task 8 converts those cross-references to links after every target exists, so Task 1 does not intentionally introduce broken links.

- [ ] **Step 3: Write the component catalog**

Create `docs/reference/components.md` with one compact table containing: component/package, purpose, primary caller, lifecycle, status, and related guide. Cover bufferpool, Dig container, jsoncodec, jsonconvert, mongodecimal, writer, tasklog, validate, database helpers, and empty/placeholder component directories. Do not turn internal helpers into public recommendations.

- [ ] **Step 4: Write the example map**

Create `docs/reference/examples.md` with these sections:

```markdown
# 示例目录
## 三个目录分别演示什么
## Web 示例装配入口
## 配置示例
## CLI 示例入口
## 可以借鉴的部分
## 已知不完善处
```

State explicitly that the examples demonstrate assembly and call paths, require optional infrastructure for their full flow, contain incomplete branches, and are neither production templates nor API compatibility promises.

- [ ] **Step 5: Verify and commit Task 1**

Run:

```bash
rg -n '^# ' docs/reference/feature-status.md docs/reference/components.md docs/reference/examples.md
rg -n '已接入|实验性|内部工具|预留/占位' docs/reference/feature-status.md
rg -n '生产模板|稳定 API' docs/reference/examples.md
rg -n 'TBD|TODO|待定|稍后补充' docs/reference --glob '*.md'
git diff --check
```

Expected: one H1 per page; all four status labels are defined; example limitations are explicit; the placeholder scan has no matches; diff check has no errors beyond pre-existing line-ending warnings for untouched files.

Commit:

```bash
git add docs/reference
git commit -m "docs: define FiberHouse feature status and examples"
```

---

### Task 2: Document the core architecture and lifecycle

**Files:**

- Create: `docs/concepts/architecture.md`
- Create: `docs/concepts/startup-lifecycle.md`
- Create: `docs/concepts/provider-system.md`
- Create: `docs/concepts/context-and-locators.md`

**Interfaces:**

- Consumes: Task 1 status vocabulary; `boot.go`, `application_interface.go`, `provider_*`, `frame_starter_*`, `core_*_starter_*`, `context_*`, locator implementations, `default.go`.
- Produces: canonical definitions and lifecycle order referenced by every guide and README.

- [ ] **Step 1: Trace the architecture and dynamic dispatch**

Use CodeGraph for `FiberHouse.RunServer`, `FrameStarter`, `CoreStarter`, `ApplicationStarter`, `IProvider`, `IProviderManager`, `IProviderLocation`, `IContext`, and Locator implementations. Record exact current source locations and dynamic-dispatch links. Reconcile them with `.codegraph-qa-out/codebase_summary.md` rather than copying that note.

- [ ] **Step 2: Write the architecture page**

Use these sections:

```markdown
# 架构总览
## FiberHouse 的职责边界
## Web 与 CLI 两种运行形态
## 核心组成
## 依赖方向
## 框架、默认装配与业务应用
## 一次 Web 请求的位置
## 阅读源码的入口
```

Include one small relationship diagram showing Boot → Context → Frame/Core Starter, with Provider/Manager/Location feeding lifecycle stages. Keep databases, cache, tasks, and examples outside the framework-core box.

- [ ] **Step 3: Write the startup lifecycle page**

Document the exact `RunServer` order: bootstrap location, provider distribution, zero-location loading, Frame/Core options, starter creation, context registration, globals, core engine, hooks, application middleware, module middleware/routes, Swagger, task server, global keepalive, before-run, run/shutdown, and after-run. Distinguish declared Locations from Locations actually consumed by the current flow.

- [ ] **Step 4: Write the provider system page**

Explain Type, Target, Provider, Manager, Location, provider states, parent mounting, unique-provider mode, default manager fallback, default collections, and the current error/logging behavior for duplicate or unmatched registration. Include a minimal custom provider/manager skeleton only when every symbol matches current interfaces.

- [ ] **Step 5: Write the context and locator page**

Explain `IContext`, `IApplicationContext`, `ICommandContext`, `AppContext`, config/logger/container/validator access, Starter back-reference, `Locator`, Api/Service/Repository bases, generic global lookup helpers, and how this differs from compile-time Wire injection. State startup-only mutation and runtime read expectations.

- [ ] **Step 6: Verify and commit Task 2**

Run:

```bash
rg -n 'Boot|FrameStarter|CoreStarter|ApplicationStarter' docs/concepts/architecture.md docs/concepts/startup-lifecycle.md
rg -n 'Provider|Manager|Location|Type|Target' docs/concepts/provider-system.md
rg -n 'IContext|IApplicationContext|ICommandContext|Locator|GlobalManager' docs/concepts/context-and-locators.md
rg -n 'TBD|TODO|待定|稍后补充' docs/concepts --glob '*.md'
git diff --check
```

Expected: every core abstraction and both runtime shapes are defined once; no placeholders; no formatting errors.

Commit:

```bash
git add docs/concepts
git commit -m "docs: explain FiberHouse architecture and lifecycle"
```

---

### Task 3: Document configuration, logging, and global objects

**Files:**

- Create: `docs/guides/configuration.md`
- Create: `docs/guides/logging.md`
- Create: `docs/guides/global-manager.md`

**Interfaces:**

- Consumes: Task 2 lifecycle/context terminology; `boot.go`, `appconfig/config.go`, `bootstrap/bootstrap.go`, `component/writer/`, `globalmanager/`, and `frame_starter_impl.go` keepalive behavior.
- Produces: canonical configuration precedence and process-wide lifecycle rules used by all later operational guides.

- [ ] **Step 1: Reconfirm bootstrap and container behavior**

Trace `NewConfigOnce`, `NewLoggerOnce`, `AppConfig.Initialize`, `GlobalManager.Register/Get/Rebuild/Release/ClearAll`, and `FrameApplication.RegisterGlobalsKeepalive`. Check all default values against constructors and `example_config/application_dev.yml` separately; never treat the example file as a framework default.

- [ ] **Step 2: Write the configuration guide**

Use these sections:

```markdown
# 配置与引导
## BootConfig 与 AppConfig
## 配置加载顺序
## APP_ENV_ 选择环境文件
## APP_CONF_ 覆盖配置
## 常用配置分组
## 读取与启动期修改
## 单例与测试隔离限制
```

State the exact precedence `APP_ENV_ → application_<env>.yml → application.env 回写 → APP_CONF_`, case-sensitive key mapping, default `dev`, config directory behavior, typed getters, BootConfig overrides, and startup-write/runtime-read constraint.

- [ ] **Step 3: Write the logging guide**

Cover zerolog wrapper, Origin map, console/file selection, lumberjack rotation, channel/diode async writers, buffer/flush settings, close ownership, dropped-message metrics where implemented, and known writer/test limitations. Avoid “lossless” or “production-ready” claims.

- [ ] **Step 4: Write the GlobalManager guide**

Cover initializer registration, lazy singleton retrieval, bulk registration, health checker contract, keepalive scanning, rebuild, release/clear behavior, generic lookup helpers, error caching, startup versus runtime mutation, and known lifecycle/concurrency risks identified by source review.

- [ ] **Step 5: Verify and commit Task 3**

Run:

```bash
rg -n 'APP_ENV_|application_<env>\.yml|APP_CONF_' docs/guides/configuration.md
rg -n 'zerolog|lumberjack|channel|diode|Origin|Close' docs/guides/logging.md
rg -n 'Register|Get|Rebuild|Release|Clear|keepalive|Health' docs/guides/global-manager.md
rg -n 'TBD|TODO|待定|稍后补充' docs/guides/configuration.md docs/guides/logging.md docs/guides/global-manager.md
git diff --check
```

Expected: precedence and lifecycle terms appear; placeholder scan is empty; diff check is clean.

Commit:

```bash
git add docs/guides/configuration.md docs/guides/logging.md docs/guides/global-manager.md
git commit -m "docs: cover configuration logging and global manager"
```

---

### Task 4: Document the Web runtime, responses, and recovery

**Files:**

- Create: `docs/guides/web-runtime.md`
- Create: `docs/guides/response-and-serialization.md`
- Create: `docs/guides/errors-and-recovery.md`

**Interfaces:**

- Consumes: Task 2 architecture/provider definitions and Task 3 configuration rules; Fiber/Gin starters, adaptor packages, JSON codec providers, response facade/package, exception package, recovery providers/config/error handlers.
- Produces: canonical request/response/error flow used by README and getting started.

- [ ] **Step 1: Trace both Web paths**

Use CodeGraph to compare Fiber and Gin `InitCoreApp`, middleware registration, route registration, JSON codec installation, server run/shutdown, error handlers, recovery provider selection, and `ICoreContext` dynamic dispatch. Confirm content negotiation and response pooling directly from `response_facade.go` and response implementations.

- [ ] **Step 2: Write the Web runtime guide**

Explain CoreType selection, Fiber/Gin configuration keys, engine initialization, middleware order, route hooks, context adaptor scope, JSON codec selection, listen/shutdown behavior, Gin package-global codec side effect, and incomplete TLS/asymmetry boundaries. Include a comparison table rather than separate engine pages.

- [ ] **Step 3: Write response and serialization guide**

Explain `{code,msg,data}`, HTTP status versus business code, constructors/facade, `SendWithCtx`, JSON default, binary-support gate, request `Content-Type` then `Accept` lookup, first MIME selection, MsgPack/Protobuf providers, unknown-type JSON fallback, context/response pool ownership, and test coverage gaps. Keep engine JSON codec selection distinct from response negotiation.

- [ ] **Step 4: Write errors and recovery guide**

Explain validation/business/unknown errors; Fiber handler return errors; Gin `c.Error`/context error collection; panic path; Recovery manager choosing by CoreType; `RecoverConfig`; trace ID, params/query/header logging; sensitive-header sanitization; debug flag, stack printing, and production data hiding. Record actual status mapping even where example Swagger differs.

- [ ] **Step 5: Verify and commit Task 4**

Run:

```bash
rg -n 'Fiber|Gin|ICoreContext|JSON codec|shutdown|TLS' docs/guides/web-runtime.md
rg -n 'code|msg|data|Content-Type|Accept|MsgPack|Protobuf|对象池' docs/guides/response-and-serialization.md
rg -n 'Exception|ValidateException|panic|RecoverConfig|trace|脱敏|debug' docs/guides/errors-and-recovery.md
rg -n 'TBD|TODO|待定|稍后补充' docs/guides/web-runtime.md docs/guides/response-and-serialization.md docs/guides/errors-and-recovery.md
git diff --check
```

Expected: both engine paths and both error paths are explicit; no placeholders; clean diff.

Commit:

```bash
git add docs/guides/web-runtime.md docs/guides/response-and-serialization.md docs/guides/errors-and-recovery.md
git commit -m "docs: explain web response and recovery behavior"
```

---

### Task 5: Document data, cache, validation, and background tasks

**Files:**

- Create: `docs/guides/cache.md`
- Create: `docs/guides/background-tasks.md`
- Create: `docs/guides/database.md`
- Create: `docs/guides/validation.md`

**Interfaces:**

- Consumes: Task 2 context/container model and Task 3 configuration/global lifecycle; `cache/`, `task.go`, `TaskRegister`, database packages, validation component, and example registration points only as assembly illustrations.
- Produces: operational component guides referenced by onboarding and extension docs.

- [ ] **Step 1: Trace component creation and use**

Use CodeGraph for CacheOption/local/remote/L2 flows, cached read-through helpers, task dispatcher/worker registration, DB constructors/model bases, and validation wrapper/custom tags. For every feature, distinguish framework constructor behavior from the example application's chosen initializer keys.

- [ ] **Step 2: Write the cache guide**

Cover Cache interface, LocalCache/RedisCache/Level2Cache construction, keys, CacheOption, serialization, local/remote TTL and jitter, read-through loader, local backfill, write strategies, Wait/Close, metrics, goroutine pools, Bloom/singleflight/circuit breaker controls, and all statically identified protection/close limitations.

- [ ] **Step 3: Write the background-task guide**

Cover `TaskRegister`, handler map, dispatcher/worker constructors, Redis dependency, container registration, context injection, `application.task.enableServer`, sync/async worker modes, enqueue path, and resource/shutdown boundaries.

- [ ] **Step 4: Write the database guide**

Cover MySQL/GORM and MongoDB constructors, configuration keys, ping option, pool settings, health/rebuild/close contracts, global registration pattern, MySQL/Mongo model bases, collection/table selection, and MongoDecimal registry role. State that no database is automatically created merely by importing FiberHouse.

- [ ] **Step 5: Write the validation guide**

Cover wrapper initialization, built-in `zh-CN`, `zh-TW`, `en`, custom language initializers, custom tags/translations, error conversion, startup registration, runtime read-only use, and how errors enter the recovery/response path.

- [ ] **Step 6: Verify and commit Task 5**

Run:

```bash
rg -n 'Local|Remote|Level2|TTL|singleflight|Bloom|circuit|Close' docs/guides/cache.md
rg -n 'TaskRegister|TaskDispatcher|TaskWorker|enableServer|Redis' docs/guides/background-tasks.md
rg -n 'MySQL|GORM|MongoDB|MongoDecimal|Health|Close' docs/guides/database.md
rg -n 'zh-CN|zh-TW|en|tag|translation|启动' docs/guides/validation.md
rg -n 'TBD|TODO|待定|稍后补充' docs/guides/cache.md docs/guides/background-tasks.md docs/guides/database.md docs/guides/validation.md
git diff --check
```

Expected: all configured component types and their lifecycle boundaries are present; no placeholders.

Commit:

```bash
git add docs/guides/cache.md docs/guides/background-tasks.md docs/guides/database.md docs/guides/validation.md
git commit -m "docs: cover cache tasks database and validation"
```

---

### Task 6: Document CLI applications and supported extension contracts

**Files:**

- Create: `docs/guides/command-line.md`
- Create: `docs/guides/extending-fiberhouse.md`

**Interfaces:**

- Consumes: Tasks 1–5 terminology and limitations; `command_interface.go`, `commandstarter/`, option functions, provider system, response/provider implementations, and example command application.
- Produces: advanced-user path for CLI assembly and framework extension without overpromising unsupported APIs.

- [ ] **Step 1: Trace CLI and extension paths**

Use CodeGraph for `RunCommandStarter`, Frame/Core command starters, `ApplicationCmdRegister`, command/flag registration, global initialization, error handler, and core run. Separately map the smallest current contracts for custom Provider/Manager/Location, JSON codec provider, response provider, middleware/route provider, and a new CoreStarter.

- [ ] **Step 2: Write the command-line guide**

Cover command context creation, config/logger reuse, Frame/Core CLI composition, application registration, globals, commands, global flags/action, error handling, run order, example `test-orm` boundary, and current health-check/resource-cleanup limitations.

- [ ] **Step 3: Write the extension guide**

Organize by extension cost:

```markdown
# 扩展 FiberHouse
## 先选择已有扩展点
## 新增 Provider 与 Manager
## 新增执行 Location
## 新增中间件或路由注册器
## 新增 JSON codec
## 新增响应协议
## 新增 CoreStarter
## 验证清单
## 当前不承诺的扩展面
```

Every skeleton must call the required `SetType`, `SetTarget`, `MountToParent`, registration, and location binding methods used by current implementations. Explicitly exclude imaginary plugin/RPC APIs.

- [ ] **Step 4: Verify and commit Task 6**

Run:

```bash
rg -n 'RunCommandStarter|ICommandContext|ApplicationCmdRegister|RegisterCommands|AppCoreRun' docs/guides/command-line.md
rg -n 'Provider|Manager|Location|SetType|SetTarget|MountToParent|CoreStarter' docs/guides/extending-fiberhouse.md
rg -n 'TBD|TODO|待定|稍后补充' docs/guides/command-line.md docs/guides/extending-fiberhouse.md
git diff --check
```

Expected: the CLI lifecycle and every supported extension tier are described; unsupported plugin/RPC paths are excluded.

Commit:

```bash
git add docs/guides/command-line.md docs/guides/extending-fiberhouse.md
git commit -m "docs: add CLI and extension guides"
```

---

### Task 7: Rewrite the public entry path

**Files:**

- Modify: `README.md`
- Create: `docs/README.md`
- Create: `docs/getting-started.md`

**Interfaces:**

- Consumes: all pages from Tasks 1–6; current `go.mod`, `example_main/main.go`, `example_config/application_dev.yml`, Docker Compose example, route registrations, and Makefile/build instructions.
- Produces: the user-facing entry point, complete manual navigation, and commands later verified by Task 8.

- [ ] **Step 1: Verify onboarding facts before writing**

Confirm Go version `1.25.0`, module path, `BootConfig` defaults, Fiber/Gin constants, config/log paths, example port `8080`, Fiber health route `/health/livez`, and the example's Redis/MySQL/MongoDB registration. Confirm whether the Web example eagerly connects or merely registers each dependency; phrase prerequisites accordingly.

- [ ] **Step 2: Replace the root README completely**

The new README must use this exact high-level order:

```markdown
# FiberHouse
## FiberHouse 是什么
## 当前状态
## 核心能力
## 环境要求
## 五分钟体验
## 应用装配骨架
## 核心模型
## 启动主链
## 请求与响应主链
## Fiber 与 Gin
## 示例目录
## 文档导航
## 开发与验证
## 贡献
## 许可证
```

Keep the quick path concise. The repository example flow must name Docker Compose as the full-feature dependency setup, create the MySQL `test` database used by `application_dev.yml`, start with `go run ./example_main/main.go`, and use `GET http://localhost:8080/health/livez` as the Fiber liveness check. The assembly snippet may omit application-specific providers only if explicitly labeled as a skeleton and linked to the complete example; it must not claim that omitted registration is optional.

- [ ] **Step 3: Write the manual index**

`docs/README.md` must provide four reading paths: first run, understand architecture, use components, extend/maintain. Link every planned page once in a categorized index and define the maturity labels by linking `reference/feature-status.md`.

- [ ] **Step 4: Write the full getting-started guide**

Cover prerequisites, clone/download dependency choice, Docker services, config selection, environment overrides, Web example start/health check, Fiber/Gin switch, external project assembly responsibilities, where application/module/task registrars come from, and common failures for missing config/provider manager/infrastructure. Keep database CRUD and Wire details in the example reference rather than embedding them.

- [ ] **Step 5: Verify and commit Task 7**

Run:

```bash
rg -n '^## ' README.md
rg -n 'Go 1\.25|go run ./example_main/main.go|localhost:8080/health/livez' README.md docs/getting-started.md
rg -n 'concepts/|guides/|reference/' docs/README.md
rg -n 'TBD|TODO|待定|稍后补充' README.md docs/README.md docs/getting-started.md
git diff --check
```

Expected: README contains only the approved entry sections; commands and health URL are exact; docs index links all categories; no placeholders.

Commit:

```bash
git add README.md docs/README.md docs/getting-started.md
git commit -m "docs: rewrite FiberHouse README and onboarding"
```

---

### Task 8: Cross-document fact, link, and command verification

**Files:**

- Modify only when its named verification fails: `README.md`
- Modify only when its named verification fails: `docs/README.md`
- Modify only when its named verification fails: `docs/getting-started.md`
- Modify only when its named verification fails: `docs/concepts/*.md`
- Modify only when its named verification fails: `docs/guides/*.md`
- Modify only when its named verification fails: `docs/reference/*.md`

**Interfaces:**

- Consumes: all documentation produced by Tasks 1–7 and the current repository.
- Produces: one internally consistent, link-clean, source-backed documentation set with recorded build/test/runtime evidence.

- [ ] **Step 1: Complete cross-references and run a local Markdown link validator**

Convert the plain guide names recorded in `docs/reference/feature-status.md` into relative links to the now-existing targets. Add missing cross-links only where one page delegates detail to another; do not add repeated “related links” blocks to every page. Then run this checker from the repository root:

```bash
python3 - <<'PY'
from pathlib import Path
import re
import sys

files = [Path("README.md"), *Path("docs").rglob("*.md")]
errors = []
for source in files:
    text = source.read_text(encoding="utf-8")
    for target in re.findall(r"\[[^\]]+\]\(([^)]+)\)", text):
        if target.startswith(("http://", "https://", "mailto:", "#")):
            continue
        path_text = target.split("#", 1)[0]
        if not path_text:
            continue
        resolved = (source.parent / path_text).resolve()
        if not resolved.exists():
            errors.append(f"{source}: missing {target}")
if errors:
    print("\n".join(errors))
    sys.exit(1)
print(f"checked {len(files)} Markdown files")
PY
```

Expected: a line beginning with `checked `, a positive Markdown file count, and exit code 0.

- [ ] **Step 2: Scan for stale paths, placeholders, and overclaims**

Run:

```bash
rg -n 'provider/context|provider/adaptor|application_impl\.go|global_utility\.go|response_info_impl\.go' README.md docs --glob '*.md' -g '!docs/superpowers/**'
rg -n 'TBD|TODO|待定|稍后补充|开箱即用|生产级|完整插件|完整 RPC' README.md docs --glob '*.md' -g '!docs/superpowers/**'
```

Expected: no stale paths or placeholders. Any occurrence of a capability claim must be in a limitation/status context; otherwise revise it.

- [ ] **Step 3: Check headings, formatting, and terminology consistency**

Run:

```bash
rg -n '^# ' README.md docs --glob '*.md'
rg -n 'GlobalManager|Provider|Manager|Location|CoreStarter|FrameStarter|ICoreContext' README.md docs --glob '*.md'
git diff --check
```

Expected: one H1 in each changed page; canonical English identifiers retain consistent spelling; no whitespace errors.

- [ ] **Step 4: Build and test the repository**

Run:

```bash
GOCACHE=/tmp/fiberhouse-go-cache go build ./...
GOCACHE=/tmp/fiberhouse-go-cache go test ./...
```

Expected: build succeeds. Tests either pass or reproduce only pre-existing failures; because the change is Markdown-only, no new failing package or compilation error is acceptable. Record the exact observed test result in the task report.

- [ ] **Step 5: Verify the documented example flow**

Run the services and prepare the example database:

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml up -d
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml exec mysql mysql -uroot -proot -e 'CREATE DATABASE IF NOT EXISTS test'
```

Start `go run ./example_main/main.go` in a long-running PTY/session. After its startup output reports the listener, run this in a separate command:

```bash
curl -i http://localhost:8080/health/livez
```

Expected: the server listens on port 8080 and the liveness endpoint returns an HTTP success response. Send an interrupt to the saved server session, then clean up only the containers created by this Compose file:

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml down
```

If Docker is unavailable, do not claim runtime verification; retain the source-verified commands and record the environment limitation in the report.

- [ ] **Step 6: Review the complete documentation diff and commit fixes**

Determine the implementation branch point and inspect the whole documentation change:

```bash
git merge-base main HEAD
git diff --stat main...HEAD
git diff --check main...HEAD
```

Expected: only `README.md` and `docs/**/*.md` are changed from the branch point, with no whitespace errors.

Commit any integration fixes:

```bash
git add README.md docs
git commit -m "docs: verify FiberHouse documentation set"
```

If verification produces no file changes, skip the empty commit. Finally run `git status --short`; expected output is empty.
