# FiberHouse Critical Test Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace unstable legacy tests and add meaningful hermetic coverage for FiberHouse's provider lifecycle, dual HTTP cores, error/recover/response path, global resource lifecycle, and reusable components.

**Architecture:** Tests are layered from pure state machines through in-memory fakes to Fiber/Gin HTTP contract tests. Default verification never opens a listener or connects to Redis, MySQL, or MongoDB. A production change is allowed only after a regression test fails for the intended reason and must be the smallest compatible fix.

**Tech Stack:** Go 1.25+, `testing`, `testify`, Fiber `App.Test`, Gin `httptest.ResponseRecorder`, CodeGraph, ast-grep, Go race detector and coverage tooling.

## Global Constraints

- Work only in `/mnt/d/code/github_opensource/tmp/fiberhouse/.worktrees/test-coverage-core-20260717` on `test/meaningful-coverage-20260717`; do not merge the branch.
- Read `CLAUDE.md`; use CodeGraph before raw source search when locating unchanged code. CodeGraph is indexed from the main worktree, so re-read a file from this worktree after it has been edited.
- Default tests must not listen on real ports or access Redis, MySQL, MongoDB, DNS, or the public network.
- Do not use fixed `time.Sleep` as proof that asynchronous work completed. Use `Close`, `Wait`, channels, wait groups, or bounded polling of an observable condition.
- Do not use `t.Parallel()` in tests that touch package globals, `sync.Once`, Gin JSON API, environment variables, process arguments, or singleton registries.
- Use `t.TempDir`, `t.Setenv`, and `t.Cleanup`; tests must leave `git status --short` clean apart from intended source changes.
- Test observable behavior, not mock call counts, except when order/delegation is itself the lifecycle contract.
- Never read an object after returning it to a pool, and never assert that `sync.Pool` returns the same address.
- No production refactor solely for coverage. For each production fix: run the new focused test and record the expected RED output before editing production code, then run focused and package tests GREEN.
- Preserve public APIs unless this plan explicitly requires correcting an already broken implementation behind that API.
- Each task ends with focused tests, package race tests where applicable, `gofmt`, `git diff --check`, self-review, and one intentional commit.

---

### Task 1: Stabilize bootstrap and asynchronous writer tests

**Files:**

- Rewrite: `bootstrap/bootstrap_test.go`
- Rewrite: `component/logging/writer/async_diode_writer_test.go`
- Modify only after a RED close test: `component/logging/writer/async_diode_writer.go`

**Interfaces:**

- Consumes: `bootstrap.NewConfigOnce`, `bootstrap.NewLoggerOnce`, `writer.NewAsyncDiodeWriter`.
- Produces: stable package-global reset helpers and a documented writer close/write contract.

- [ ] **Step 1: Reproduce the legacy failures**

Run `go test ./bootstrap ./component/logging/writer -count=1`.

Expected RED: bootstrap cannot find `application_dev.yml`; writer tests read before flush; write-after-close expects success contrary to implementation.

- [ ] **Step 2: Rewrite bootstrap fixtures around the real selection contract**

Create config fixtures as `application_<env>.yml`. Add default `dev` and `APP_ENV_application_env=prod` cases. Assert this load order:

```text
APP_ENV_ selects environment -> YAML supplies values -> selected environment is written back -> APP_CONF_ overrides YAML
```

Replace the obsolete appType filename test with `TestConfig_AppTypeDoesNotChangeFilename`. Reset `cfgOnce`, `AppConfigured`, `logOnce`, and `Logger` only in package-local cleanup after closing any logger. Keep these tests serial.

- [ ] **Step 3: Rewrite writer tests to use Close as the completion barrier**

Use a small test config (diode size 128, buffer 256, long flush interval) and implement:

```go
func TestAsyncDiodeWriter_CloseDrainsBeforeReturning(t *testing.T)
func TestAsyncDiodeWriter_MultipleWritesAreDrained(t *testing.T)
func TestAsyncDiodeWriter_CopiesInputSlice(t *testing.T)
func TestAsyncDiodeWriter_ConcurrentWritesAreDrained(t *testing.T)
func TestAsyncDiodeWriter_WriteAfterCloseRejected(t *testing.T)
func TestAsyncDiodeWriter_CloseIsIdempotent(t *testing.T)
```

Each successful-write test calls `Close()` before `os.ReadFile`. The post-close test asserts `n == 0` and `error != nil`. Delete the fake `D:/invalid/path` test and all sleep-based file checks.

- [ ] **Step 4: Verify idempotent close RED, then minimally fix it**

Run `go test ./component/logging/writer -run TestAsyncDiodeWriter_CloseIsIdempotent -count=1`.

Expected RED before the fix: panic from closing `stopCh` twice. Make `Close` use a single-winner atomic transition; later calls wait for the original close and return without closing the channel again. Do not change post-close `Write`.

- [ ] **Step 5: Verify stability and commit**

```bash
go test ./bootstrap ./component/logging/writer -count=20
go test -race ./bootstrap ./component/logging/writer -count=1
git diff --check
git add bootstrap/bootstrap_test.go component/logging/writer/async_diode_writer.go component/logging/writer/async_diode_writer_test.go
git commit -m "test: stabilize bootstrap and async writer coverage"
```

### Task 2: Cover provider, location, context, and BootConfig state machines

**Files:**

- Create: `provider_type_test.go`
- Create: `provider_location_test.go`
- Create: `provider_impl_test.go`
- Create: `provider_manager_impl_test.go`
- Create: `context_impl_test.go`
- Create: `boot_config_test.go`
- Modify after RED: `provider_location.go`, `provider_manager_impl.go`
- Modify only after a race report: `boot.go`

**Interfaces:**

- Consumes: provider type/location registries, Provider/Manager, DefaultStorage, AppContext and BootConfig options.
- Produces: package-local provider/context fakes reusable by later root tests.

- [ ] **Step 1: Add registry and value-object characterization tests**

Instantiate fresh registries inside package `fiberhouse`. Cover default/custom ID boundaries, duplicate names across namespaces, lookups, must-panics, `PType` getters, `State.Set/SetState`, provider fluent setters, one-time type setting, `Check`, parent mounting and registration delegation.

- [ ] **Step 2: Write focused RED provider lifecycle tests**

```go
func TestPLocationBind_AllowsDistinctManagersAtSameLocation(t *testing.T)
func TestPLocationBind_RejectsNilAndExactDuplicate(t *testing.T)
func TestPLocationGetManagers_ReturnsOrderedCopy(t *testing.T)
func TestProviderManagerUnregister_RemovesProvider(t *testing.T)
func TestDefaultPManagerLoadProvider_SuccessDoesNotAggregateNil(t *testing.T)
func TestDefaultPManagerLoadProvider_AggregatesOnlyRealErrors(t *testing.T)
```

Record RED caused by location-ID duplicate detection, no-op unregister and nil entries in `errs`.

- [ ] **Step 3: Apply minimal provider fixes**

`PLocation.Bind` rejects nil and the exact same manager instance while allowing different managers at one location in insertion order. `ProviderManager.Unregister` returns `ErrProviderNotFound` for a missing name and deletes an existing name. `DefaultPManager.LoadProvider` appends only non-nil errors and succeeds when all matching initializers succeed.

- [ ] **Step 4: Cover remaining manager branches**

Test duplicate provider names, missing lookup, unique binding for same/different/multiple providers, base `LoadProvider` without/with invalid child, child delegation, callback error, target filtering and auto-run behavior. Assert initializer outputs/errors.

- [ ] **Step 5: Cover storage, context and BootConfig**

Test DefaultStorage CRUD, overwrite, early-stop Range, keys/length/clear and concurrent readers/writers. Test AppContext boot-config/app-state once semantics, starter registration, and logger-origin missing/success paths using isolated keys. Test every BootConfig option, `Finally`, custom values, missing/must-get panic and concurrent custom reads/writes.

- [ ] **Step 6: Verify, fix only a reproduced BootConfig race, and commit**

```bash
gofmt -w provider_type_test.go provider_location_test.go provider_impl_test.go provider_manager_impl_test.go context_impl_test.go boot_config_test.go provider_location.go provider_manager_impl.go boot.go
go test . -run 'Test(BootConfig|DefaultStorage|AppContext|Provider|PLocation|DefaultPManager)' -count=1
go test -race . -run 'Test(BootConfig|DefaultStorage|AppContext|Provider|PLocation|DefaultPManager)' -count=1
git diff --check
git add provider_type_test.go provider_location_test.go provider_impl_test.go provider_manager_impl_test.go context_impl_test.go boot_config_test.go provider_location.go provider_manager_impl.go boot.go
git commit -m "test: cover provider and context state machines"
```

If `WithCustom`/`GetValue` races, guard reads and writes with an RWMutex without changing option signatures.

### Task 3: Cover global resources, exceptions, response encodings, utilities, and pools

**Files:**

- Create: `globalmanager/manager_lifecycle_test.go`
- Create: `exception/exception_error_test.go`
- Create: `response/response_msgpack_impl_test.go`
- Extend: `response/response_proto_impl_test.go`
- Create: `utils/common_test.go`
- Create: `component/bufferpool/buffer_test.go`
- Modify after RED: `globalmanager/manager.go`, `exception/exception_error.go`, `utils/common.go`, `component/bufferpool/buffer.go`, `response/response_msgpack_impl.go`

**Interfaces:**

- Consumes: GlobalManager, exception registry, MsgPack/Protobuf responses, utility helpers and pools.
- Produces: in-memory response recorders reusable by root response/recover tests.

- [ ] **Step 1: Write and run GlobalManager lifecycle regressions**

Cover transient initializer failure followed by success, cached success, panic isolation, closable release/reinitialize, close error, health, rebuild success/error/type change, ReleaseAll, Clear/ClearAll and unregister. Required RED tests:

```go
func TestRelease_ClosableClosesAndCanReinitialize(t *testing.T)
func TestGet_TransientFailureThenSuccessClearsCachedError(t *testing.T)
```

Use `NewGlobalManager()`, not the singleton. Reset released state without `atomic.Value.Store(nil)`; clear old `initErr` after a successful retry.

- [ ] **Step 2: Write exception-key regressions**

Register a temporary `ExceptionMap`, recover panics, and prove `Throw(key)` and `VeThrow(key)` use the caller's key and carry optional error data. Cover unknown keys, missing registry, New/Get/RespData, reset/from, send/json status and release. Fix only the wrong map lookup after RED.

- [ ] **Step 3: Cover MsgPack and Protobuf HTTP behavior**

Using an in-memory core context, test default/custom status, data present/absent, valid round trips, typed MsgPack decode, malformed bytes, missing/wrong field types returning errors rather than panics, and pool reset. Extend protobuf tests through `SendWithCtx` and `JsonWithCtx`.

- [ ] **Step 4: Cover utilities and buffer pools**

Test JSON validity, Unicode whitespace, file existence, unsafe empty/non-empty conversions, stack helpers, all supported `ValidConstant` kinds, `binaryCeil` boundaries, shard capacity, oversize buffers, reset-on-put and generic pools. Define zero lower bound as a minimum one-byte shard. The zero-bound regression must not leave an infinite goroutine.

- [ ] **Step 5: Verify and commit**

```bash
gofmt -w globalmanager/manager_lifecycle_test.go globalmanager/manager.go exception/exception_error_test.go exception/exception_error.go response/response_msgpack_impl_test.go response/response_msgpack_impl.go response/response_proto_impl_test.go utils/common_test.go utils/common.go component/bufferpool/buffer_test.go component/bufferpool/buffer.go
go test -race ./globalmanager ./exception ./response ./utils ./component/bufferpool -count=1
git diff --check
git add globalmanager exception response utils component/bufferpool
git commit -m "test: cover resource and serialization lifecycles"
```

### Task 4: Establish Fiber/Gin context, error adaptor, and core selection contracts

**Files:**

- Create: `adaptor/context/core_ctx_wrap_fiber_test.go`
- Create: `adaptor/context/core_ctx_wrap_gin_test.go`
- Create: `adaptor/errorhandler/errorhandler_contract_test.go`
- Create: `core_starter_provider_test.go`
- Create: `core_starter_init_test.go`
- Modify after RED: `adaptor/errorhandler/fiber_error_handler.go`, `core_starter_gin_provider.go`, `option/app_starter_option.go`

**Interfaces:**

- Consumes: both core context wrappers, error adaptors, starter providers and core options.
- Produces: dual-core request helpers and verified core choice without listeners.

- [ ] **Step 1: Add context wrapper contract tests**

For both cores verify native `GetCtx`, request-header lookup, response-header setting, JSON status/body, raw byte status/body and error propagation. Fiber uses an in-memory app request; Gin uses a recorder/context.

- [ ] **Step 2: Write the Fiber error-adaptor RED test**

Have the callback write a 418 JSON body and return nil while the route returns a sentinel error. Assert final Fiber response remains 418 and matches Gin. Change Fiber adaptor to return the callback result exactly.

- [ ] **Step 3: Write core provider RED tests**

Test Fiber/Gin providers with no callback, empty options, callback error and wrong payload. Assert Gin provider returns `*CoreWithGin`, never Fiber. Correct its error label at the same site.

- [ ] **Step 4: Verify `WithCoreCfg` configures Fiber**

Apply `option.WithCoreCfg`, initialize without listening, and assert the supplied Fiber config is used. If RED because it is a no-op, set the supported concrete Fiber core config without changing the public option signature.

- [ ] **Step 5: Cover no-listener core initialization**

Using isolated contexts and codec managers, verify app creation, app-state early return, manager selection, missing/wrong/error manager paths, Gin server address/timeouts, module registration, Swagger enable/disable and hooks. Never call `AppCoreRun`.

- [ ] **Step 6: Verify and commit**

```bash
gofmt -w adaptor/context/core_ctx_wrap_fiber_test.go adaptor/context/core_ctx_wrap_gin_test.go adaptor/errorhandler/errorhandler_contract_test.go adaptor/errorhandler/fiber_error_handler.go core_starter_provider_test.go core_starter_init_test.go core_starter_gin_provider.go option/app_starter_option.go
go test -race ./adaptor/context ./adaptor/errorhandler . -run 'Test(Core|Fiber|Gin|Context|Error)' -count=1
git diff --check
git add adaptor core_starter_provider_test.go core_starter_init_test.go core_starter_gin_provider.go option/app_starter_option.go
git commit -m "test: enforce dual core adapter contracts"
```

### Task 5: Cover recover and response facade behavior through both HTTP cores

**Files:**

- Create: `recover_config_test.go`
- Create: `recover_error_handler_test.go`
- Create: `recover_http_contract_test.go`
- Create: `response_facade_test.go`
- Modify after RED: `recover_config.go`, `recover_error_handler_impl.go`, `recover_fiber_impl.go`, `recover_gin_impl.go`

**Interfaces:**

- Consumes: recover configuration/middleware, root error handler and response manager.
- Produces: dual-core error/panic-to-HTTP contract tests.

- [ ] **Step 1: Cover recover helpers and configuration**

Test default/override config, stack-handler injection, sensitive-header sanitization, params/query/header JSON encoding success/failure and wrong native context. Run concurrent configuration under `-race`; if shared `ConfigConfigured` races, return a local value instead of mutating shared state.

- [ ] **Step 2: Build an isolated response manager fixture**

Reset/bind response manager state with cleanup. Test `extractPrimaryMimeType`, JSON fallback, binary disabled, MsgPack/Protobuf selection, unknown MIME, custom status and one-time release. Preserve current first-value Accept behavior.

- [ ] **Step 3: Run a shared table through Fiber and Gin**

```text
panic(error), panic(*Exception), panic(*ValidateException), runtime panic,
string panic, debug off/on, Next=true, 404, 405, ordinary returned error
```

Assert status, content type, envelope code/message/data visibility, downstream execution count and stack-handler count using `app.Test` and `ServeHTTP`.

- [ ] **Step 4: Apply only fixes proven by RED cases**

Handle both `fiber.Error` and `*fiber.Error`; preserve 404/405 status; prevent `Next=true` from executing downstream twice; release wrappers on normal and panic paths. Do not redesign shutdown or add public interfaces.

- [ ] **Step 5: Verify and commit**

```bash
gofmt -w recover_config_test.go recover_error_handler_test.go recover_http_contract_test.go response_facade_test.go recover_config.go recover_error_handler_impl.go recover_fiber_impl.go recover_gin_impl.go
go test -race . -run 'Test(Recover|ErrorHandler|Response|ExtractPrimaryMimeType)' -count=1
go test . -run 'Test(Recover|ErrorHandler|Response|ExtractPrimaryMimeType)' -count=20
git diff --check
git add recover_config_test.go recover_error_handler_test.go recover_http_contract_test.go response_facade_test.go recover_config.go recover_error_handler_impl.go recover_fiber_impl.go recover_gin_impl.go
git commit -m "test: cover recovery and response negotiation"
```

### Task 6: Cover hermetic cache, validation, codec, task, and logging components

**Files:**

- Create: `component/cache/cachelocal/local_cache_test.go`
- Create: `component/cache/cache2/level2_cache_test.go`
- Extend: `component/cache/cache_option_test.go`
- Create: `component/codec/json/jsoncodec_test.go`
- Create: `component/validate/validate_wrapper_test.go`
- Create: `component/task/logadaptor/logger_adapter_test.go`
- Create: `task_test.go`
- Modify after RED: `component/cache/cache2/level2_cache.go`, `component/cache/cache_option.go`

**Interfaces:**

- Consumes: Local/Level2 cache, CacheOption, JSON codecs, validation, task logger, task mux and PayloadBase.
- Produces: hermetic component coverage and fake cache implementations.

- [ ] **Step 1: Cover LocalCache and CacheOption**

Test string/bytes/object set-wait-get, miss, TTL with bounded polling, delete, metrics, close and post-close errors. Extend CacheOption for clone/reset/pool reuse, context preservation, disabled-cache loader behavior, fixed/random TTL bounds and invalid ranges.

- [ ] **Step 2: Write Level2Cache close-state RED tests**

```go
func TestLevel2Close_IsIdempotentAndClosesChildrenOnce(t *testing.T)
func TestLevel2Close_MarksClosed(t *testing.T)
func TestLevel2OperationsAfterClose_ReturnErrCacheClosed(t *testing.T)
func TestLevel2Wait_PropagatesChildErrors(t *testing.T)
```

Restore the closed CAS/state transition and aggregate child close/wait errors without closing channels twice.

- [ ] **Step 3: Cover JSON codecs and validation**

For Std and each Sonic constructor test marshal/unmarshal/indent, stream encoder/decoder and malformed input. For validation cover en/zh-cn/zh-tw struct/var/map translations, unsupported-language fallback, language lists, duplicate registration and custom-tag error aggregation using fresh config/wrap instances.

- [ ] **Step 4: Cover task payload and logger without Redis**

Construct an asynq mux/client but never run a Redis server/client operation. Process through the mux to assert `ContextKeyAppCtx` and handler error propagation. Test PayloadBase nil fallback, container hit, missing key and wrong type. Capture zerolog output for task log levels and non-string/multiple args.

- [ ] **Step 5: Verify and commit**

```bash
gofmt -w component/cache/cachelocal/local_cache_test.go component/cache/cache2/level2_cache_test.go component/cache/cache2/level2_cache.go component/cache/cache_option_test.go component/cache/cache_option.go component/codec/json/jsoncodec_test.go component/validate/validate_wrapper_test.go component/task/logadaptor/logger_adapter_test.go task_test.go
go test -race ./component/cache/... ./component/codec/json ./component/validate ./component/task/logadaptor . -run 'Test(LocalCache|Level2|CacheOption|JSON|Validate|Task|Payload)' -count=1
git diff --check
git add component/cache component/codec/json component/validate component/task/logadaptor task_test.go
git commit -m "test: cover hermetic framework components"
```

### Task 7: Cover startup orchestration, frame/command lifecycle, and high-value gaps

**Files:**

- Create: `boot_lifecycle_test.go`
- Create: `frame_starter_impl_test.go`
- Create: `starter_provider_manager_test.go`
- Create: `commandstarter/lifecycle_test.go`
- Extend: `globalmanager/manager_test.go`
- Extend: `appconfig/config_test.go`
- Modify only after focused RED: `frame_starter_impl.go`, `commandstarter/frame_cmd_application.go`

**Interfaces:**

- Consumes: `RunApplicationStarter`, FrameApplication, starter providers/managers, command lifecycle and existing suites.
- Produces: final lifecycle coverage before whole-branch verification.

- [ ] **Step 1: Verify Web lifecycle order**

With a recording `ApplicationStarter`, assert:

```text
RegisterToCtx -> RegisterApplicationGlobals -> InitCoreApp ->
RegisterAppHooks -> RegisterAppMiddleware -> RegisterModuleInitialize ->
RegisterModuleSwagger -> RegisterTaskServer -> RegisterGlobalsKeepalive -> AppCoreRun
```

Assert the same manager slice reaches every accepting stage. Do not call blocking `FiberHouse.RunServer`.

- [ ] **Step 2: Cover FrameApplication guards**

Test application/module/task registration/getters, context registration, app-state early returns, missing application panic, initializer registration, required-key success/failure logging, logger-origin registration, validator initializers/tags, task disabled/nil and keepalive disabled. Do not start a ticker or task server.

- [ ] **Step 3: Cover starter providers/managers**

Test Frame/Core provider missing callback, callback error, wrong option type, correct options, target filtering, missing provider, wrong returned type and mount behavior.

- [ ] **Step 4: Cover command lifecycle without os.Exit**

Use a recording `CommandStarter` for `RunCommandStarter` order. Test `CoreCmdCli` init/register/run with explicit args and error-returning actions. Test FrameCmdApplication registration and logger-origin setup. Replace and restore `cli.OsExiter` before testing exit paths.

- [ ] **Step 5: Strengthen weak old assertions**

In appconfig tests replace boolean-OR assertions, assert defensive-copy/idempotence, and avoid `require` inside goroutines. In globalmanager tests replace the skipped rebuild fake with the actual `Rebuilder` signature and assert outcomes.

- [ ] **Step 6: Cover exact remaining pure high-value files**

Generate coverage, then add tests for observable branches in: `global_utils.go`, `locator_interface.go`, `service_impl.go`, `repository_impl.go`, `api_impl.go`, `model_interface.go`, `default.go`, `response_providers_and_manager.go`, and `json_codec_manager.go`. Cover constructors, fluent context/name methods, namespace/key building, manager selection and errors. Do not test placeholder plugins or external DB clients.

- [ ] **Step 7: Verify and commit**

```bash
gofmt -w boot_lifecycle_test.go frame_starter_impl_test.go starter_provider_manager_test.go commandstarter/lifecycle_test.go globalmanager/manager_test.go appconfig/config_test.go
go test -race . ./commandstarter ./globalmanager ./appconfig -count=1
go test . ./commandstarter ./globalmanager ./appconfig -count=10
go test ./... -count=1 -covermode=atomic -coverprofile=/tmp/fiberhouse-task7.out
go tool cover -func=/tmp/fiberhouse-task7.out | tail -n 1
git diff --check
git add -A
git commit -m "test: cover application lifecycle orchestration"
```

## Final Verification and Review

After Tasks 1-7 are individually reviewed:

```bash
go test ./... -count=1
go test ./... -count=10
go test -race ./... -count=1
go test ./... -count=1 -covermode=atomic -coverprofile=/tmp/fiberhouse-final-coverage.out
go tool cover -func=/tmp/fiberhouse-final-coverage.out | tail -n 1
git diff --check
git status --short
```

Calculate full, library and hermetic scope with the exclusions in `design.md`. If hermetic coverage is below 55%, return to Task 7's exact file list and add observable branch tests until the threshold is met or a documented non-hermetic boundary accounts for the gap. Review the whole branch from `8c10a025fdbc046b55066d1a46e1456b5644508f` to HEAD. Fix Critical and Important findings through one fix subagent and repeat final verification.
