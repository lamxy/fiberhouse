# P1-A Keepalive and ClearAll Design

## Goal

Fix two concrete shutdown defects without changing FiberHouse's public startup interfaces or introducing a new lifecycle coordinator:

1. stop the default `FrameApplication` health-check goroutine before the existing container cleanup; and
2. stop replacing an already-used `sync.Map` value in `GlobalManager.ClearAll`.

## Scope

The existing startup and shutdown architecture remains intact:

- `RunServer` continues to assemble `WebApplication` and call `AppCoreRun`;
- Fiber and Gin continue to own their current signal handling and server shutdown;
- `CoreStarter`, `FrameStarter`, `IApplicationContext`, and all other public interfaces keep their current method sets;
- `ClearAll(true)` remains a deletion-only operation and does not acquire resource-closing semantics.

The change is limited to the existing default implementation and its two built-in HTTP cores.

## GlobalManager ClearAll

`GlobalManager.ClearAll(true)` currently assigns `sync.Map{}` to `gm.container` after the map has already been used. The implementation will instead call `gm.container.Clear()`.

This preserves the current contract:

- no confirmation argument, or `false`, does nothing;
- `true` removes registered keys;
- `ClearAll` does not call `Close` and does not replace `ReleaseAll`;
- concurrent use is data-race safe at the `sync.Map` level, without promising a transactional snapshot against concurrent stores.

`ReleaseAll(true)` will not be added to the Fiber or Gin shutdown paths. The current container contains aliases and composite ownership: remote cache aliases Redis, L2 cache closes its borrowed local/remote caches, the logger writer has another owner, and task worker/dispatcher are not `Closable`. Unordered release is therefore outside this change.

## Default Keepalive Lifecycle

`FrameApplication` will retain private lifecycle state:

```go
healthMu     sync.Mutex
healthCancel context.CancelFunc
healthWG     sync.WaitGroup
```

`startHealthCheck` will:

1. reject a non-positive interval by logging an error and returning;
2. start at most one health-check goroutine during the `FrameApplication` lifetime;
3. save a cancel function before starting the goroutine;
4. select between `ctx.Done()` and ticker events;
5. retain the current `Range`, `CheckHealth`, and `Rebuild` behavior; and
6. log rebuild success only when `Rebuild` succeeds.

Once started, `healthCancel` remains non-nil even after stopping. This intentionally prevents restarting the same `FrameApplication` after `Wait` has begun and avoids `WaitGroup.Add` racing with `Wait`.

`stopHealthCheck` will be private, idempotent, safe for concurrent callers, and will call cancel before waiting for the goroutine to exit. It has no timeout: the current `HealthChecker` and `Rebuilder` interfaces do not accept context, so continuing cleanup while one of those calls is still active would reintroduce the race this change is meant to remove.

## Existing Shutdown Integration

A private optional interface will connect the built-in cores to the default frame implementation:

```go
type healthCheckStopper interface {
	stopHealthCheck()
}
```

The helper will use the actual typed path:

```go
starter := ctx.GetStarterApp()
frame := starter.GetFrameApp()
```

If the frame implements `healthCheckStopper`, the helper stops it; otherwise it does nothing. This preserves compatibility with custom `FrameStarter` implementations, whose private background work remains their responsibility.

The existing cleanup order becomes:

```text
server shutdown completes
  -> stop and wait for default keepalive
  -> ClearAll(true), deletion only
  -> write final shutdown log
  -> close logger
```

Fiber invokes this from its existing `OnShutdown` hook. Gin invokes it after `http.Server.Shutdown` returns. Gin's current final log is moved before `logger.Close()` so no logging occurs after the writer is closed.

## Error and Concurrency Boundaries

- Cancellation stops future ticker events but cannot interrupt an in-progress `IsHealthy` or `Rebuild`; shutdown waits for that call.
- Concurrent and repeated `stopHealthCheck` calls are supported.
- A stopped `FrameApplication` cannot restart health checking.
- `sync.Map.Clear` removes the invalid field assignment but does not make `ClearAll`, `Get`, `Rebuild`, and `Release` a coordinated transactional state machine.
- The existing `Release`, `ReleaseAll`, task lifecycle, resource ownership aliases, `Rebuild` replacement semantics, server error propagation, and public API policy remain unchanged.

## Verification

Tests will prove:

- `ClearAll(true)` removes entries without closing them;
- concurrent `Register`/`Get`/`Range`/`ClearAll` is race-clean;
- health checking starts only once;
- repeated and concurrent stop calls complete without deadlock;
- stop waits for an active health pass and prevents later ticks;
- invalid intervals do not start a goroutine;
- the package-private stopper follows `GetStarterApp().GetFrameApp()` and safely ignores unsupported custom starters;
- built-in Fiber and Gin cleanup paths call the stopper before clearing the container;
- no shutdown-complete log is written after closing the Gin logger.

Verification commands:

```bash
go test ./globalmanager -count=1
go test -race ./globalmanager -count=1
go test . -count=1
go test -race . -count=1
go vet ./...
go test ./... -count=1
go test -race ./... -count=1
```

## Non-Goals

- No `Run(context.Context) error` API.
- No lifecycle coordinator or shutdown registry.
- No public interface changes.
- No automatic `ReleaseAll` during web shutdown.
- No task worker/dispatcher shutdown changes.
- No Redis alias or L2 ownership changes.
- No old-instance close during `Rebuild`.
- No consumption or removal of `ServerShutdownBefore`/`ServerShutdownAfter` in this change.
