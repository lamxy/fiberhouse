# Gin Framework Logging Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route Gin diagnostics and `net/http.Server` errors through the FiberHouse logger with a process-level, restorable single-owner lease.

**Architecture:** Add a root-independent `adaptor/logging` package that converts Gin callback and writer output into structured `bootstrap.LoggerWrapper` events. A mutex-protected lease installs and restores Gin's package globals. `CoreWithGin` owns that lease across initialization, serving, and shutdown, while retaining the existing FiberHouse access and recovery middleware.

**Tech Stack:** Go 1.25, Gin 1.12, zerolog, testify, standard `log`/`io`/`sync` packages.

## Global Constraints

- Work only in `/mnt/d/code/github_opensource/tmp/fiberhouse/.worktrees/gin-framework-logging-bridge`.
- Follow strict TDD: add a focused test, run it and observe the expected failure, then add the minimal implementation.
- Use `rtk go test` for test commands; set `GOCACHE` to a task-specific directory under `/tmp`.
- Do not add a Gin logger enable switch or a new `IAppConfig` method.
- Do not install `gin.Logger` or `gin.Recovery`.
- Do not parse Gin debug message strings to infer warning severity.
- Do not change the public `CoreStarter.InitCoreApp` signature.
- The adapter borrows `bootstrap.LoggerWrapper`; it never closes it.
- Only one Gin logging lease may be active in the process.
- Gin-global tests must remain serial and restore global state with `t.Cleanup`.
- Commit after each task and do not push.

---

### Task 1: Structured Gin Logger Adapter

**Files:**
- Create: `adaptor/logging/gin_logger_adapter.go`
- Create: `adaptor/logging/gin_logger_adapter_test.go`

**Interfaces:**
- Consumes: `bootstrap.LoggerWrapper`, `appconfig.LogOrigin`.
- Produces:

```go
type GinLoggerAdapter struct

func NewGinLoggerAdapter(
	logger bootstrap.LoggerWrapper,
	origin appconfig.LogOrigin,
) *GinLoggerAdapter

func (a *GinLoggerAdapter) DebugPrint(format string, values ...any)
func (a *GinLoggerAdapter) DebugPrintRoute(
	httpMethod string,
	absolutePath string,
	handlerName string,
	handlerCount int,
)
func (a *GinLoggerAdapter) InfoWriter() io.Writer
func (a *GinLoggerAdapter) ErrorWriter() io.Writer
func (a *GinLoggerAdapter) HTTPServerErrorLogger() *log.Logger
```

- [ ] **Step 1: Write failing adapter tests**

Build a test logger with JSON output:

```go
func newTestAdapter(t *testing.T) (*GinLoggerAdapter, *bytes.Buffer) {
	t.Helper()
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.DebugLevel)
	return NewGinLoggerAdapter(
		bootstrap.NewLoggerWrap(&logger),
		appconfig.LogOrigin("Frame"),
	), &output
}

func decodeRecords(t *testing.T, output *bytes.Buffer) []map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		records = append(records, record)
	}
	return records
}
```

Add focused tests that require:

```go
adapter.DebugPrint("registered %s\n", "route")
```

to emit one Debug record containing:

```text
Origin=Frame
Component=Gin
Channel=debug
message="registered route"
```

Require `DebugPrintRoute("GET", "/users/:id", "handler", 6)` to emit a Debug
record with `method`, `path`, `handler`, and numeric `handlerCount`.

Require the info and error writers to:

```go
n, err := adapter.InfoWriter().Write([]byte("native access\n"))
require.Equal(t, len("native access\n"), n)
require.NoError(t, err)
```

and emit Info/Error records with `Channel=writer` or `Channel=error`. Cover an
empty CR/LF-only write, which returns the original length without emitting a
record, and a multi-line message, which remains one event.

Require:

```go
adapter.HTTPServerErrorLogger().Print("accept failed")
```

to emit one Error record with `Channel=server`, no standard library timestamp,
and no duplicated prefix.

- [ ] **Step 2: Run adapter tests and verify RED**

Run:

```bash
GOCACHE=/tmp/fiberhouse-gin-logging-task1 rtk go test ./adaptor/logging -count=1
```

Expected: FAIL because `GinLoggerAdapter` and its constructor do not exist.

- [ ] **Step 3: Implement the adapter**

Use one stateless writer type with a fixed channel and level:

```go
type ginLogWriter struct {
	adapter *GinLoggerAdapter
	level   zerolog.Level
	channel string
}

func (w *ginLogWriter) Write(p []byte) (int, error) {
	length := len(p)
	message := strings.TrimRight(string(p), "\r\n")
	if message == "" {
		return length, nil
	}
	w.adapter.event(w.level, w.channel).Msg(message)
	return length, nil
}
```

Create events without exposing zerolog outside the package API:

```go
func (a *GinLoggerAdapter) event(
	level zerolog.Level,
	channel string,
) *zerolog.Event {
	var event *zerolog.Event
	switch level {
	case zerolog.DebugLevel:
		event = a.logger.DebugWith(a.origin)
	case zerolog.ErrorLevel:
		event = a.logger.ErrorWith(a.origin)
	default:
		event = a.logger.InfoWith(a.origin)
	}
	return event.
		Str("Component", "Gin").
		Str("Channel", channel)
}
```

`DebugPrint` uses `fmt.Sprintf`, trims only trailing CR/LF, skips an empty
result, and emits at Debug. `DebugPrintRoute` emits the fixed message
`"Gin route registered"` and structured route fields. `HTTPServerErrorLogger`
uses:

```go
log.New(a.ErrorWriterFor("server"), "", 0)
```

Keep the channel-specific writer constructor private except for
`InfoWriter`, `ErrorWriter`, and `HTTPServerErrorLogger`.

- [ ] **Step 4: Run adapter tests and verify GREEN**

Run:

```bash
GOCACHE=/tmp/fiberhouse-gin-logging-task1 rtk go test ./adaptor/logging -count=1
```

Expected: PASS.

- [ ] **Step 5: Format, inspect, and commit**

Run:

```bash
gofmt -w adaptor/logging/gin_logger_adapter.go adaptor/logging/gin_logger_adapter_test.go
GOCACHE=/tmp/fiberhouse-gin-logging-task1 rtk go test ./adaptor/logging -count=1
rtk git diff --check
```

Commit:

```bash
git add adaptor/logging/gin_logger_adapter.go adaptor/logging/gin_logger_adapter_test.go
git commit -m "feat: adapt Gin logs to framework logger"
```

---

### Task 2: Process-Level Gin Logging Lease

**Files:**
- Create: `adaptor/logging/gin_logger_install.go`
- Modify: `adaptor/logging/gin_logger_adapter_test.go`

**Interfaces:**
- Consumes: `GinLoggerAdapter.DebugPrint`, `DebugPrintRoute`, `InfoWriter`, and
  `ErrorWriter`.
- Produces:

```go
var ErrGinLoggerAlreadyInstalled error

type GinLoggerLease struct {
	// unexported state
}

func InstallGinLogger(
	adapter *GinLoggerAdapter,
) (*GinLoggerLease, error)

func (l *GinLoggerLease) Release()
```

- [ ] **Step 1: Write failing lease tests**

Save the four Gin globals at test start and restore them in `t.Cleanup`.
Install sentinel callbacks and buffer writers before acquiring the lease.

Require the first installation to:

```go
lease, err := InstallGinLogger(adapter)
require.NoError(t, err)
require.NotNil(t, lease)
gin.DebugPrintFunc("message %d", 1)
gin.DebugPrintRouteFunc("GET", "/route", "handler", 2)
_, _ = gin.DefaultWriter.Write([]byte("info\n"))
_, _ = gin.DefaultErrorWriter.Write([]byte("error\n"))
```

and route all four calls through the adapter.

Require a second active installation to return
`ErrGinLoggerAlreadyInstalled`, leave the first bridge active, and return no
lease.

Require `Release` to restore the sentinel callbacks/writers. Verify callbacks
by invoking them and checking sentinel counters, because Go functions cannot
be compared.

Call `Release` from multiple goroutines and require it to restore once without
panic or race. Finally, require a new installation to succeed after release.
Add nil adapter and nil logger cases and require a non-nil error.

- [ ] **Step 2: Run lease tests and verify RED**

Run:

```bash
GOCACHE=/tmp/fiberhouse-gin-logging-task2 rtk go test ./adaptor/logging -run 'TestInstallGinLogger|TestGinLoggerLease' -count=1
```

Expected: FAIL because `InstallGinLogger` and `GinLoggerLease` do not exist.

- [ ] **Step 3: Implement the lease**

Use one package-level owner:

```go
var ginLoggerInstallation struct {
	sync.Mutex
	active *GinLoggerLease
}

type GinLoggerLease struct {
	once sync.Once

	previousDebugPrint      func(string, ...any)
	previousDebugPrintRoute func(string, string, string, int)
	previousWriter          io.Writer
	previousErrorWriter     io.Writer
}
```

`InstallGinLogger` validates the adapter and its logger, locks the owner,
rejects an existing lease, captures all four globals, installs adapter
callbacks/writers, records the active lease, and returns it.

`Release` executes once. Under the owner mutex, restore all four globals only
when the lease is still the active owner, then clear the owner. Do not close
the adapter logger and do not return an error.

- [ ] **Step 4: Run lease tests and race verification**

Run:

```bash
GOCACHE=/tmp/fiberhouse-gin-logging-task2 rtk go test ./adaptor/logging -count=1
GOCACHE=/tmp/fiberhouse-gin-logging-task2-race rtk go test -race ./adaptor/logging -count=1
```

Expected: both PASS.

- [ ] **Step 5: Format, inspect, and commit**

Run:

```bash
gofmt -w adaptor/logging/gin_logger_install.go adaptor/logging/gin_logger_adapter_test.go
rtk git diff --check
```

Commit:

```bash
git add adaptor/logging/gin_logger_install.go adaptor/logging/gin_logger_adapter_test.go
git commit -m "feat: lease Gin global logging hooks"
```

---

### Task 3: Integrate The Lease With CoreWithGin

**Files:**
- Modify: `core_gin_starter_impl.go`
- Modify: `core_starter_init_test.go`
- Modify: `core_starter_provider_test.go` only if the existing provider contract
  needs an assertion for deferred initialization errors.

**Interfaces:**
- Consumes:

```go
adaptorlogging.NewGinLoggerAdapter(logger, origin)
adaptorlogging.InstallGinLogger(adapter)
(*adaptorlogging.GinLoggerLease).Release()
```

- Produces private Core behavior:

```go
func resolveGinMode(cfg appconfig.IAppConfig) string
func (cg *CoreWithGin) releaseGinLogger()
func (cg *CoreWithGin) initializationFailed() bool
```

- [ ] **Step 1: Write failing Core integration tests**

Add serial tests that restore Gin globals with `t.Cleanup`.

Cover mode resolution with an `IAppConfig` wrapper or test config:

```text
canonical key present -> canonical value
canonical absent, legacy present -> legacy value
both absent -> release
recover.debugMode=true with canonical release -> release
```

Add a test that initializes a Gin core in debug mode with a buffer-backed
framework logger and requires the `gin.New` startup diagnostic to appear as a
structured Gin Debug record.

Register a route after initialization and require one structured
`"Gin route registered"` event.

Require the default `http.Server.ErrorLog` to be non-nil and to write an Error
record through the framework logger. Add a custom-server case where a non-nil
caller-supplied `ErrorLog` remains unchanged.

Acquire an external lease first, then initialize a second `CoreWithGin`.
Require:

```go
require.Nil(t, core.GetCoreApp())
require.ErrorIs(t, core.AppCoreRun(), adaptorlogging.ErrGinLoggerAlreadyInstalled)
```

Call registration methods after that failed initialization and require no
panic.

Extend existing server-run/shutdown tests to require the bridge is released
after:

```text
listener/start failure
normal AppCoreRun return
successful graceful shutdown
shutdown provider error
replacement shutdown provider
```

Require the existing access log record to include `Component="Gin"` and remain
single, not duplicated.

- [ ] **Step 2: Run focused Core tests and verify RED**

Run:

```bash
GOCACHE=/tmp/fiberhouse-gin-logging-task3 rtk go test . -run 'TestCoreInit_Gin(Logger|Mode|HTTPError)|TestCoreRun_GinLogger|TestCoreShutdown_GinLogger' -count=1
```

Expected: FAIL because CoreWithGin does not install or own the bridge and still
couples Gin mode to recovery debug.

- [ ] **Step 3: Add Core fields and mode resolution**

Add:

```go
type CoreWithGin struct {
	ctx            IApplicationContext
	OptionFuncList []gin.OptionFunc
	coreApp        *gin.Engine
	httpServer     *http.Server
	ginLoggerLease *adaptorlogging.GinLoggerLease
	initErr        error
}

func resolveGinMode(cfg appconfig.IAppConfig) string {
	if mode := cfg.String(
		"application.plugins.engine.servers.gin.mode",
	); mode != "" {
		return mode
	}
	return cfg.String(
		"application.plugins.server.gin.mode",
		gin.ReleaseMode,
	)
}

func (cg *CoreWithGin) releaseGinLogger() {
	if cg.ginLoggerLease != nil {
		cg.ginLoggerLease.Release()
	}
}
```

Do not read `cfg.GetRecover().DebugMode` when selecting the Gin mode.

- [ ] **Step 4: Install before engine creation**

In the default `InitCoreApp` path, after location replacement handling and
before `gin.SetMode`/`gin.New`:

```go
adapter := adaptorlogging.NewGinLoggerAdapter(
	cg.GetAppContext().GetLogger(),
	cfg.LogOriginFrame(),
)
lease, err := adaptorlogging.InstallGinLogger(adapter)
if err != nil {
	cg.initErr = fmt.Errorf(
		"install Gin framework logger: %w",
		err,
	)
	cg.GetAppContext().GetLogger().
		ErrorWith(cfg.LogOriginFrame()).
		Err(cg.initErr).
		Msg("InitCoreApp Gin logger bridge failed")
	return
}
cg.ginLoggerLease = lease

initialized := false
defer func() {
	if !initialized {
		cg.releaseGinLogger()
	}
}()
```

Set `initialized = true` only after codec and HTTP server initialization
complete. After `initHttpServer`, assign:

```go
if cg.httpServer.ErrorLog == nil {
	cg.httpServer.ErrorLog = adapter.HTTPServerErrorLogger()
}
```

- [ ] **Step 5: Propagate init failure and release on all terminal paths**

At the start of every Gin registration method, return if `initErr != nil`.
`AppCoreRun` returns the wrapped `initErr` before using the server.

After the initialization check, `AppCoreRun` defers
`cg.releaseGinLogger()`. `Shutdown` also defers it before any early return.
On the successful shutdown path, call `cg.releaseGinLogger()` explicitly
before `LoggerWrapper.Close`; the deferred call remains safe.

Add `Component="Gin"` to the existing structured access event. Do not add
native Gin middleware.

- [ ] **Step 6: Run focused Core tests and verify GREEN**

Run:

```bash
GOCACHE=/tmp/fiberhouse-gin-logging-task3 rtk go test . -run 'TestCoreInit_Gin|TestCoreRun_Gin|TestCoreShutdown_Gin' -count=1
```

Expected: PASS. If loopback tests are blocked by the sandbox, run the focused
non-network tests there and leave the complete suite for the final
unsandboxed verification.

- [ ] **Step 7: Run package regression tests and commit**

Run:

```bash
gofmt -w core_gin_starter_impl.go core_starter_init_test.go core_starter_provider_test.go
GOCACHE=/tmp/fiberhouse-gin-logging-task3 rtk go test . -count=1
rtk git diff --check
```

Commit:

```bash
git add core_gin_starter_impl.go core_starter_init_test.go core_starter_provider_test.go
git commit -m "feat: route Gin core logs through framework logger"
```

Only add `core_starter_provider_test.go` if it changed.

---

### Task 4: Configuration, Documentation, And Repository Verification

**Files:**
- Modify: `example_config/application_dev.yml`
- Modify: `example_config/application_test.yml`
- Modify: `example_config/application_prod.yml`
- Modify: `docs/guides/web-runtime.md`
- Modify: `docs/guides/logging.md`
- Modify: `docs/guides/configuration.md`
- Modify: `docs/reference/feature-status.md`

**Interfaces:**
- Consumes the canonical key:

```text
application.plugins.engine.servers.gin.mode
```

- Produces user-facing migration and lifecycle documentation.

- [ ] **Step 1: Add explicit mode configuration**

Under each existing `application.plugins.engine.servers.gin` map, add:

```yaml
mode: debug
```

for development and test, and:

```yaml
mode: release
```

for production. Preserve existing comments and indentation.

- [ ] **Step 2: Update focused documentation**

Document:

- Gin diagnostics use the framework logger automatically.
- Debug records still obey the framework logger level.
- The canonical Gin mode key and accepted Gin values.
- `recover.debugMode` no longer changes Gin mode.
- The old `application.plugins.server.gin.mode` key is a compatibility fallback.
- The bridge owns Gin package globals for one active Core and restores them on
  exit.
- Multiple Gin engines share the same framework logger while the bridge is
  active; per-engine native debug isolation is unsupported.
- Existing FiberHouse access and recovery middleware remain authoritative.

Update the Gin and logging status evidence without promoting the support level
to stable.

- [ ] **Step 3: Run documentation and configuration checks**

Run:

```bash
rtk rg -n "plugins\\.server\\.gin\\.mode|recover\\.debugMode.*Gin|Gin.*stdout|Gin.*stderr" README.md docs example_config
rtk git diff --check
```

Expected: any old key occurrence is explicitly described as compatibility
behavior; no document claims recovery debug controls Gin mode or that Gin
diagnostics bypass the framework logger.

- [ ] **Step 4: Run full verification**

Run outside the restricted socket sandbox when necessary:

```bash
GOCACHE=/tmp/fiberhouse-gin-logging-final rtk go test ./... -count=1
GOCACHE=/tmp/fiberhouse-gin-logging-final-race rtk go test -race ./adaptor/logging ./... -count=1
GOCACHE=/tmp/fiberhouse-gin-logging-final-build go build ./...
rtk git diff --check
```

Expected: all tests and build pass with no diff-check errors.

- [ ] **Step 5: Commit documentation and configuration**

```bash
git add \
  example_config/application_dev.yml \
  example_config/application_test.yml \
  example_config/application_prod.yml \
  docs/guides/web-runtime.md \
  docs/guides/logging.md \
  docs/guides/configuration.md \
  docs/reference/feature-status.md
git commit -m "docs: describe Gin framework logging bridge"
```

After this commit, run:

```bash
rtk git status --short
rtk git log --oneline -5
```

Expected: clean worktree and four implementation commits after the plan commit.
