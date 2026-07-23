# Gin Framework Logging Bridge Design

## Status

Approved for implementation.

## Context

`CoreWithGin` already sends FiberHouse lifecycle messages, HTTP access logs,
and recovery errors through the framework `bootstrap.LoggerWrapper`. Gin's own
diagnostic output and the standard library HTTP server error log still bypass
that logger:

- `gin.DebugPrintFunc` is unset, so startup and general debug messages use
  `gin.DefaultWriter`.
- `gin.DebugPrintRouteFunc` is unset, so route registration messages use the
  same global debug output.
- `gin.DefaultWriter` and `gin.DefaultErrorWriter` still point to stdout and
  stderr.
- `http.Server.ErrorLog` is nil, so server errors use the standard library
  default logger.

Gin exposes these diagnostic hooks as package globals, not fields on a
`*gin.Engine`. Installing a bridge for one engine therefore affects every Gin
engine in the process. FiberHouse already has one application context, one
framework logger, and one selected HTTP core per running application. The
bridge follows that process-level ownership model while preventing a second
active installer from silently replacing the first.

The current Gin mode configuration also has two related problems:

- Code reads `application.plugins.server.gin.mode`, while the example server
  configuration is under `application.plugins.engine.servers.gin`.
- `application.recover.debugMode` forces Gin into debug mode even though
  recovery response detail and Gin diagnostics are separate concerns.

## Goals

- Route Gin startup, route, debug, warning-like debug, and error output through
  the framework logger.
- Route `net/http.Server` internal errors through the same logger.
- Preserve the existing structured FiberHouse HTTP access and recovery
  middleware.
- Install the Gin globals before `gin.New` so its startup diagnostics are
  captured.
- Give one active FiberHouse Gin core exclusive, explicit ownership of the Gin
  global logging hooks.
- Restore the previous Gin globals when the core stops or initialization
  fails.
- Make acquisition and release deterministic, concurrency-safe, and
  idempotent.
- Keep the adapter independent of the root `fiberhouse` package to avoid an
  import cycle.
- Correct the canonical Gin mode configuration path while temporarily
  accepting the legacy path.
- Decouple Gin mode from recovery debug behavior.

## Non-Goals

- Routing different Gin engines to different framework loggers concurrently.
- Changing Gin itself or maintaining a Gin fork.
- Parsing Gin's private debug message text to infer warning severity.
- Replacing FiberHouse's existing HTTP access or recovery middleware.
- Adding `gin.Logger` or `gin.Recovery` to `gin.New`.
- Giving the adapter ownership of, or permission to close, the framework
  logger.
- Changing the public `CoreStarter.InitCoreApp` signature.
- Creating a new public logging abstraction beyond the existing
  `bootstrap.LoggerWrapper`.

## Selected Design

Add a logging adapter package:

```text
adaptor/logging/
├── gin_logger_adapter.go
├── gin_logger_install.go
└── gin_logger_adapter_test.go
```

The package imports `bootstrap`, `appconfig`, Gin, and zerolog as needed. It
does not import the root `fiberhouse` package because `CoreWithGin` must import
the adapter package.

The adapter receives its dependencies explicitly:

```go
type GinLoggerAdapter struct {
	logger bootstrap.LoggerWrapper
	origin appconfig.LogOrigin
}

func NewGinLoggerAdapter(
	logger bootstrap.LoggerWrapper,
	origin appconfig.LogOrigin,
) *GinLoggerAdapter
```

`CoreWithGin` constructs it with the application logger and
`cfg.LogOriginFrame()`. Gin-specific records add a stable
`Component="Gin"` field, so this change does not require a new
`IAppConfig` method or a new mandatory log-origin configuration entry.

## Log Mapping

The bridge maps each source according to the semantics Gin exposes:

| Source | Framework level | Additional fields |
| --- | --- | --- |
| `gin.DebugPrintFunc` | Debug | `Component="Gin"`, `Channel="debug"` |
| `gin.DebugPrintRouteFunc` | Debug | component, channel, method, path, handler, handler count |
| `gin.DefaultWriter` | Info | `Component="Gin"`, `Channel="writer"` |
| `gin.DefaultErrorWriter` | Error | `Component="Gin"`, `Channel="error"` |
| `http.Server.ErrorLog` | Error | `Component="Gin"`, `Channel="server"` |
| Existing access middleware | Info | existing HTTP fields plus `Component="Gin"` |

Gin sends startup warnings through `DebugPrintFunc` without a separate severity
argument. The bridge records all calls from that hook at Debug level. It does
not inspect strings such as `[WARNING]`, because those message formats are
private Gin implementation details and may change between versions.

`gin.DefaultWriter` is mapped to Info rather than Debug because application
code may explicitly install Gin's native access logger, which also uses that
writer. FiberHouse does not install the native middleware, so the default
startup path still has only one HTTP access record.

## Message Handling

The debug callback formats a message with `fmt.Sprintf(format, values...)`.
Writer adapters accept arbitrary byte slices and always return the original
input length after emitting a log record. They remove trailing CR and LF
characters but preserve all other message content.

An empty message does not emit a record. A multi-line Gin diagnostic remains
one structured event; it is not split into several events because splitting
would discard the atomic relationship between the lines.

Every write creates a new zerolog event. The adapter holds no message buffer
and does not mutate request-scoped state. This matches the existing assumption
that the shared framework logger and its configured writer can receive
concurrent request logs.

The adapter exposes a standard library logger for the HTTP server:

```go
func (a *GinLoggerAdapter) HTTPServerErrorLogger() *log.Logger
```

It uses an error-level writer and no standard library prefix or timestamp,
because the framework logger already supplies time and structured metadata.

## Process-Level Lease

Installation returns a lease:

```go
var ErrGinLoggerAlreadyInstalled = errors.New(
	"gin framework logger is already installed",
)

type GinLoggerLease struct {
	// Private ownership and once-only release state.
}

func InstallGinLogger(
	adapter *GinLoggerAdapter,
) (*GinLoggerLease, error)

func (l *GinLoggerLease) Release()
```

The package maintains one mutex-protected active installation. The first
installation:

1. Saves `gin.DebugPrintFunc`.
2. Saves `gin.DebugPrintRouteFunc`.
3. Saves `gin.DefaultWriter`.
4. Saves `gin.DefaultErrorWriter`.
5. Installs the adapter callbacks and writers.
6. Returns the owning lease.

A second installation while the lease is active returns
`ErrGinLoggerAlreadyInstalled`. It does not overwrite the active callbacks,
even if the second adapter wraps the same logger.

`Release` uses `sync.Once`, restores all four saved values, and clears the
active owner under the package mutex. The lease does not close the
`LoggerWrapper`. Callers must not mutate those Gin globals while a FiberHouse
lease is active; exclusive ownership is the contract that makes restoration
deterministic.

This model allows sequential cores and isolated tests. Multiple Gin engines
may exist while the lease is active, but their native Gin diagnostics all go
to the same framework logger.

## Core Integration

`CoreWithGin` gains private fields for the adapter lease and an initialization
error:

```go
type CoreWithGin struct {
	// Existing fields omitted.
	ginLoggerLease *logging.GinLoggerLease
	initErr        error
}
```

`InitCoreApp` follows this order:

1. Run the existing `LocationCoreEngineInit` provider handling.
2. Resolve the Gin mode.
3. Construct the adapter from the application context.
4. Acquire the Gin logger lease.
5. Store an acquisition error in `initErr`, log it through the framework
   logger, and return without creating an engine if acquisition fails.
6. Call `gin.SetMode`.
7. Call `gin.New`, which now emits through the installed bridge.
8. Complete codec and HTTP server initialization.
9. Assign `http.Server.ErrorLog` from the adapter when it is nil.

An explicitly supplied `http.Server.ErrorLog` remains an application override
and is not replaced. The default server path always receives the framework
adapter because it currently leaves this field nil.

The lease is released if initialization panics or exits before the core is
fully initialized. The existing panic is not swallowed; cleanup runs and the
panic continues.

Methods that register middleware, routes, Swagger, or hooks return immediately
when `initErr` is set. `AppCoreRun` returns the wrapped initialization error
before loading run providers or dereferencing the engine/server. This carries
the failure through the existing error-returning runtime boundary without
changing the public `InitCoreApp` interface.

`CoreWithGin` has one private idempotent release helper. Both `AppCoreRun` and
`Shutdown` call it:

- `AppCoreRun` defers release so a normal server exit, listener error, or
  replacement run provider cannot leave the global bridge installed.
- `Shutdown` guarantees release on every return path, including replacement
  and provider errors.
- On the normal shutdown path, `Shutdown` releases the Gin bridge explicitly
  before closing the framework logger. A deferred idempotent release remains
  as protection for early returns.

This dual call is intentional: shutdown and server return occur in different
goroutines during signal handling, and either may complete first.

## Existing Middleware

The current `CoreWithGin.loggerMiddleware` remains the only default HTTP access
logger. It receives `Component="Gin"` for consistent filtering while retaining
its existing `LogOriginCoreHttp`, method, path, status, latency, IP, body size,
query, and error fields.

The current FiberHouse recovery and Gin error-handler adapters remain
unchanged. They already send failures through `LoggerWrapper`. Installing
Gin's native Logger or Recovery middleware would duplicate records and is
explicitly excluded.

## Gin Mode Configuration

The canonical key becomes:

```yaml
application:
  plugins:
    engine:
      servers:
        gin:
          mode: debug
```

Resolution order is:

1. `application.plugins.engine.servers.gin.mode`
2. Legacy `application.plugins.server.gin.mode`
3. `gin.ReleaseMode`

Only `debug`, `release`, and `test` are accepted by Gin. The implementation
uses `gin.SetMode` as the final validator and preserves its existing invalid
mode failure behavior.

`application.recover.debugMode` no longer overrides Gin mode. It continues to
control recovery response detail and stack behavior only. Development and test
example configurations explicitly select Gin debug mode; production selects
release mode.

No bridge enable switch is added. Selecting the FiberHouse Gin core means its
native logs use the framework logger, while the framework log level determines
whether Debug records are emitted.

## Error Handling

- A nil adapter or nil framework logger is rejected during installation.
- A concurrent second installation returns
  `ErrGinLoggerAlreadyInstalled`.
- Adapter writes do not return framework logging errors because
  `LoggerWrapper` and zerolog events do not expose a write result at this
  boundary.
- `http.Server.ErrorLog` writes are treated as Error records without causing
  recursive standard library logging.
- Bridge restoration is best-effort-free: it performs only in-memory
  assignments and has no error result.
- The adapter never calls Fatal or Panic.
- Core initialization conflicts are returned later from `AppCoreRun`, not
  converted into a panic and not silently ignored.

## Test Strategy

Adapter unit tests cover:

- Debug messages use Debug level and Gin component/channel fields.
- Route messages contain method, path, handler, and handler count fields.
- Info and error writers trim line endings, preserve content, return the input
  length, and ignore empty messages.
- Multi-line diagnostics remain one event.
- The HTTP server logger writes at Error level without a standard prefix.
- Installation replaces all four Gin globals.
- A second active installation returns the sentinel error without changing
  the first installation.
- Release restores every previous value.
- Repeated and concurrent release is idempotent.
- A new installation succeeds after release.

Gin global tests run serially and always register cleanup before assertions.
They do not use `t.Parallel`.

Core integration and contract tests cover:

- The bridge is installed before `gin.New` and captures its debug startup
  message.
- Route registration uses the structured route callback.
- `http.Server.ErrorLog` is non-nil and reaches the framework logger.
- An installation conflict prevents engine creation and is returned by
  `AppCoreRun`.
- Registration methods do not dereference a missing engine after an
  initialization conflict.
- Listener failure releases the bridge.
- Normal server return and graceful shutdown both release the bridge exactly
  once.
- Shutdown provider errors and replacement paths still release the bridge.
- Existing access logging emits one record and includes `Component="Gin"`.
- The canonical mode key wins over the legacy key.
- The legacy key remains accepted when the canonical key is absent.
- Recovery debug mode no longer changes Gin mode.

Repository verification includes:

```bash
go test ./adaptor/logging ./...
go test -race ./adaptor/logging ./...
go build ./...
```

## Documentation

Update the Gin sections of the following documentation:

- `docs/guides/web-runtime.md`
- `docs/guides/logging.md`
- `docs/guides/configuration.md`
- `docs/reference/feature-status.md`
- Example development, test, and production YAML files

Documentation must state that Gin diagnostic hooks are process-global, one
active FiberHouse bridge owns them, multiple Gin engines share the same
framework logger, and per-engine native debug isolation is not supported.

## Risks And Mitigations

### Gin Globals Are Shared

Any library can mutate the same hooks. The lease declares exclusive ownership,
rejects a second FiberHouse owner, and restores the exact previous values.
External mutation during an active lease remains unsupported and is documented.

### Logger Closes Before Gin Stops

Gin globals could retain writers backed by a closed logger. Both run and
shutdown paths release idempotently, and the normal shutdown path restores Gin
before closing the framework logger.

### Initialization Cannot Return An Error Directly

`InitCoreApp` has no error result. The Gin core records the bridge acquisition
failure, stops later registration work, and returns it from `AppCoreRun`.
Changing the shared `CoreStarter` interface would create unrelated breakage and
is outside this design.

### Gin Warnings Use The Debug Hook

Gin does not provide a severity value to `DebugPrintFunc`. Treating all calls
as Debug avoids brittle parsing. Important application and server failures
still use the error writer or explicit FiberHouse error paths.

### Tests Share Package Globals

Tests that modify Gin globals can interfere with one another. Adapter and core
tests run serially, install cleanup immediately, and assert restoration.

### Legacy Mode Behavior Changes

Applications that relied on recovery debug mode to force Gin debug mode must
set the Gin mode explicitly. Example configuration and migration documentation
make this separation visible, while the legacy Gin mode path remains readable
for one compatibility period.
