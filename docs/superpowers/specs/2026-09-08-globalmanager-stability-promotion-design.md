# GlobalManager stability promotion design

Date: 2026-09-08

Branch: `stability/globalmanager-promotion`

## Outcome

Close the lifecycle-contract gaps that currently block `GlobalManager`,
`MySQL / MongoDB` and `本地缓存与 Redis 缓存` from being promoted from
“实验性” to “稳定公共能力” in `docs/reference/feature-status.md`.

The three capabilities share one blocker: their shutdown contract is delegated
to `GlobalManager.Release` / `ClearAll`, and both of those methods currently
have defined-but-wrong or undefined behavior. Fixing `GlobalManager` unblocks
all three at once.

## Scope decision: which capabilities are in play

Assessed against the five promotion conditions. Condition 2 (compatibility
promise) is exempt before v2.0.0 — recorded in `feature-status.md`, decided by
the repository owner on 2026-09-08 — so the effective bar is 1, 3, 4, 5.

| Capability | C1 | C3 | C4 | C5 | In scope |
|---|---|---|---|---|---|
| GlobalManager | ✗ | partial | ✓ | partial | yes |
| MySQL / MongoDB | ✗ (via GM) | partial | partial | ✓ | yes |
| 本地缓存与 Redis 缓存 | ✗ (via GM) | partial | ✓ | ✓ | yes |
| Gin 内核, L2 缓存, CLI, 异步任务 | — | — | — | — | no |

The four excluded capabilities have independent gaps unrelated to this branch
(Gin per-engine diagnostics isolation, L2 async flush ownership, CLI
`RunCommandStarter` discarding its return value, task dispatcher reclamation).
They are explicitly out of scope; widening this branch to cover them would make
the change set unreviewable.

## Evidence from the current implementation

Verified baseline on `main` at commit `731e7b6`: `go vet ./...` clean,
`go test ./... -count=1` exit 0 (797 tests / 78 packages), `go test -race
./... -count=1` exit 0, no data races.

### C1 blocker 1 — `ClearAll` leaks every managed resource

`globalmanager/manager.go:327-331`

```go
func (gm *GlobalManager) ClearAll(conform ...bool) {
	if len(conform) > 0 && conform[0] {
		gm.container.Clear()
	}
}
```

`sync.Map.Clear` drops the entries. Nothing calls `Closable.Close()`. Every
MySQL pool, Mongo client and Redis client registered in the container is
leaked — the sockets stay open until process exit. `feature-status.md:33`
already admits `ClearAll` 仅删除条目而不逐项 `Close`, so this is a known and
recorded defect, not a discovery.

The doc comment claims “不受阻碍的清空全局管理器管理的全部对象和资源”. It
clears objects but not resources, so the comment is also wrong.

### C1 blocker 2 — `Release` silently no-ops for non-`Closable` objects

`globalmanager/manager.go:288-303`

```go
if closable, ok := instance.(Closable); ok {
	err := closable.Close()
	...
	entity.instance.Store(nil)
	entity.initErr.Store(nil)
	atomic.StoreInt32(&entity.initialized, 0)
	entity.once.Store(&sync.Once{})
}
return nil
```

The whole state reset lives inside the `Closable` type switch. An object that
does not implement `Closable` takes the `beginMaintenance` / `endMaintenance`
round trip and returns `nil` — reporting success — while `initialized` stays
`1` and the instance stays live. A caller that calls `Release` then `Get`
receives the *same* instance it believed it had just released.

Both outcomes are defensible in isolation; what is not defensible is returning
`nil` for a request that was not honored.

### C1 blocker 3 — the busy sentinel is unexported

`globalmanager/manager.go:66`

```go
var errMaintenanceInProgress = errors.New("global object maintenance already in progress")
```

`Rebuild` and `Release` wrap this with `%w`, so the sentinel is genuinely
propagated — but it is package-private, so no caller outside `globalmanager`
can `errors.Is` against it. `feature-status.md:33` records this as
“busy error 的 private sentinel 不是稳定公开的 retry 分类”. Contention on a
maintenance operation is a *recoverable, retryable* condition, and the caller
has no supported way to classify it. This is condition 1 (failure path
contract), and it borders on condition 4.

### C3 gap — live tests cover one sequential path each

`grep -rn "func TestLive_"` returns exactly four tests:

- `TestLive_RedisDb_PingSetGetDeleteClose`
- `TestLive_MongoDb_CreateWriteReadDropClose`
- `TestLive_MysqlDb_CreateWriteReadDropClose`
- `TestLive_TaskWorkerDispatcher_EnqueueConsumeShutdown`

Each drives a single create-write-read-close sequence on one goroutine. After
this branch removed `Rebuilder`, the surviving concurrency claim is that a
constructed `Client` is never replaced, so reads need no lock. **No test
exercises that claim.** Condition 3 requires external dependencies to be
covered by repeatable live integration tests; concurrent access is the
specific uncovered case, and it is the case the no-rebuild design rests on.

Separately, the readiness endpoint added in `af8b272` has no end-to-end
verification: nothing proves that breaking a dependency actually flips
`/readyz` to 503 and that recovery flips it back. That path is the headline
benefit of the keepalive → health-check rework.

### C4 gap — model constructors panic on a recoverable condition

`component/database/dbmysql/mysql_model_impl.go:33-38` and
`component/database/dbmongo/mongo_model_impl.go:36-41`

```go
db, err := ctx.GetContainer().Get(key)
if err != nil {
	panic(err.Error())
}
return &MysqlModel{Ctx: ctx, Db: db.(*MysqlDb)}
```

Two problems. The `panic(err.Error())` discards the error chain — the panic
value is a bare string, so a recovering caller cannot inspect the cause. And
`db.(*MysqlDb)` is an unchecked assertion: if `key` was registered with a
different type, `Get` succeeds and the assertion panics with a message that
does not name the key.

Call-path check (`grep -rn "NewMysqlModel\|NewMongoModel"`): the only
non-test callers are `example_application/.../mysql_model.go:23` and
`example_application/.../example_model.go:86`, both on the module-registration
path, i.e. startup-time. The blast radius is a failed boot, not a killed
request. That lowers severity but does not satisfy condition 4, which is about
*assembly* errors as much as runtime ones.

## Revision after independent audit (2026-09-08)

An independent audit of the planned shutdown fix returned `needs-revision`. The
defect was confirmed, the proposed fix was not sound. Verified independently
before acting; all three findings hold.

### The audit was right and the original step 2 was wrong

The original plan was “call `ReleaseAll(true)` before `ClearAll` in
`clearApplicationGlobals`”. That introduces a use-after-close.

The container holds **aliased** and **composite** resources
(`example_application/application_impl.go:68-108`):

- `KEY_REMOTE_CACHE`'s initializer is `return container.Get(KEY_REDIS)` — the
  same `*RedisDb` pointer under a second key.
- `KEY_LEVEL2_CACHE` wraps `KEY_LOCAL_CACHE` and `KEY_REMOTE_CACHE`, and is
  constructed with `initLifecycle(true)` → `ownChildren = true`
  (`component/cache/cache2/level2_cache.go:140`), so it closes both children
  itself.
- `Level2Cache.finishShutdown` waits on `<-drain` **before** closing children
  (`level2_cache.go:512-524`).

`ReleaseAll` iterates via `sync.Map.Range`, whose order is unspecified and
varies per run. If `KEY_REDIS` is swept before `KEY_LEVEL2_CACHE`, the L2
cache's in-flight `asyncSetRemote` writes drain against an already-closed Redis
client. Intermittent, load-dependent, and effectively unreproducible in tests
unless the ordering is asserted directly.

### Decisions taken

**Ordering — option B, explicit shutdown ordering.** Considered: (a) idempotency
guards plus relying on `ownChildren`; (b) explicit ordering; (c) dependency
metadata in the container.

(a) was rejected because idempotent `Close` only makes a *second* close safe —
it does nothing about operations still draining when the resource is pulled, so
the race that motivated this work would survive. (c) was rejected as a container
redesign disproportionate to promotion, and it is already listed under
“Explicitly not doing”.

(b) is implemented as a `CompositeClosable` marker interface rather than a
hardcoded key list, so the framework does not need to know application key names
like `KEY_LEVEL2_CACHE`; the constraint is expressed in the type system. Mongo's
idempotency guard is added anyway as defence in depth, since aliasing makes
repeated `Close` structural rather than exceptional.

**Error reporting.** `ReleaseAll` reported failures via `fmt.Printf`
(`manager.go:314` pre-fix) — stdout, which containerized deployments generally
do not collect, making every shutdown close failure invisible. Fixed by adding
`ReleaseAllWithObserver` and injecting a framework-logger callback from
`releaseApplicationGlobals`. `globalmanager` keeps zero logging dependencies.

**Hertz — out of scope, recorded.** `example_application/hertzcore/starter/
core_hertz_starter.go:415-459` never calls `clearApplicationGlobals`, so it
does not benefit from this fix and keeps leaking. It is excluded because
`feature-status.md:52` already states that everything under
`example_application/` only demonstrates that a call path exists and carries no
maturity commitment, and because no Hertz row exists in the capability tables.
Revisit if Hertz becomes a supported core.

### Residual risk accepted

A concurrent application-side `Release`/`Rebuild` during shutdown makes
`beginMaintenance` fail fast, silently skipping that object's close. Low
likelihood — `stopHealthCheck` drains the checker before release begins
(`frame_starter_impl_test.go:455`) — and the failure is now reported through the
observer rather than swallowed. Not addressed further on this branch.

Serial release also extends shutdown; `MongoDb.Close` alone allows a 5s timeout.
Acceptable against Gin's 30s budget, but worth revisiting if the registered set
grows.

## Plan

Ordered so that each step is independently reviewable and independently
revertable. Every step keeps `go test -race ./... -count=1` green.

1. **Doc** — record the condition 2 exemption in `feature-status.md`. (done)
2. **`ClearAll` closes what it clears** — iterate, attempt `Close` on every
   `Closable`, aggregate failures, then delete. Preserve the `conform` guard.
3. **`Release` defines its non-`Closable` outcome** — move the state reset out
   of the type switch so a release always releases, and return a typed error
   when the object cannot be closed.
4. **Export the busy sentinel** — publish `ErrMaintenanceInProgress` so callers
   can `errors.Is`.
5. **Model constructors stop discarding the cause** — wrap the error and name
   the key; make the type assertion checked.
6. **Concurrent live tests** — one per wrapper: N goroutines reading/writing
   while `CheckHealth` runs concurrently, under the `liveintegration` tag.
7. **Readiness end-to-end live test** — dependency up → 200, forced down → 503,
   restored → 200.
8. **Cross-audit** — dispatch an independent subagent to audit steps 2-7
   against this document before the final status update.
9. **Promote** — update `feature-status.md` rows only for capabilities whose
   conditions are then demonstrably met, citing the new evidence.

## Shutdown paths surveyed

Four paths can end an application. Only the first two are fixed on this branch.

| Path | Location | Releases globals? |
|---|---|---|
| Fiber | `core_fiber_starter_impl.go:360` → `clearApplicationGlobals` | yes, after this branch |
| Gin | `core_gin_starter_impl.go:594` → `clearApplicationGlobals` | yes, after this branch |
| Hertz | `example_application/hertzcore/starter/core_hertz_starter.go:415` | no — never calls it |
| CLI | `commandstarter/frame_cmd_application.go` | no — has no shutdown path at all |

The CLI runs a single health-check sweep (`frame_cmd_application.go:77`) and
never releases anything: there is no `ClearAll`, no `ReleaseAll`, and no
shutdown hook in `commandstarter/`. So a CLI command that registers MySQL or
Redis leaks those connections on exit.

That is a real defect, but it belongs to the **CLI** capability, which is a
separate row in `feature-status.md:36` with its own recorded gaps (“健康检查只
执行一次，`RunCommandStarter` 丢弃返回值”) and is not among the three
capabilities this branch promotes. Fixing it means designing a CLI shutdown
lifecycle, not reusing the web one — `clearApplicationGlobals` takes an
`IApplicationContext` the CLI does not have. Recorded here and left for the CLI
capability's own work.

## Explicitly not doing

- Removing the legacy `application.globalManage.keepAlive` config key. It has a
  working fallback plus a deprecation log (`frame_starter_impl.go:37-61`).
  Breaking changes are allowed, but removal is unrelated to promotion and
  belongs to the v2.0.0 sweep.
- Reworking `Get` / `Rebuild` / `Release` into a full concurrent state machine.
  `feature-status.md:33` flags this as open. It is a redesign, not a gap fix,
  and it is out of proportion to promotion.
- Touching the four out-of-scope capabilities listed above.

## What actually shipped

Deviations from the plan above, and why.

**Step 2 changed shape.** The plan said “`ClearAll` closes what it clears”. It
does not. Closing has to wait for in-flight work to drain and can block for
seconds, while `ClearAll` promises to return immediately; folding one into the
other breaks that promise. Instead `clearApplicationGlobals` now runs
stop-health-check → `ReleaseAllWithObserver` → `ClearAll`, and `ClearAll` keeps
its delete-only semantics with a corrected doc comment (it previously claimed to
clear “全部对象和资源”, which was never true). `TestClearAll_DoesNotCloseResources`
therefore still passes unchanged — it pins deliberate behavior, not the bug.

**A second audit caught a defect in the first fix.** The two-pass sweep ordered
composites before plain objects but left composite-vs-composite order to
`sync.Map.Range`. For A owns B owns C the owner counts are 0/1/1, B and C tie,
and the partition reproducibly produced `[A C B]`. Replaced with transitive
depth (longest ownership chain, computed by bounded relaxation so a declared
cycle terminates instead of overflowing the stack). The original nested test had
missed this because A's `Close` cascades through B and C, masking the wrong
container order behind a correct cascade — the fix was to assert the partition
order directly rather than only observe close behavior.

**Two further defects surfaced from the audit.** `Get` returned a typed-nil
instance alongside the error when an initializer had the shape
`return NewXWithError(ctx)`: `instance` is a named return the closure had
already written, so a caller checking `instance != nil` got a value that panics
on dereference. And `KEY_LEVEL2_CACHE` was registered but never instantiated —
absent from `ConfigRequiredGlobalKeys`, resolved by no route — so the example
never constructed the only `CompositeClosable` in the repo, leaving the ordering
work unexercised by the documented path. Both fixed.

**Steps 5 and 9 landed narrower than written.** `NewMysqlModel` /
`NewMongoModel` keep their panicking form because wire-generated assembly cannot
consume an error-returning provider; the `*WithError` variants sit alongside
them, and `RegisterKeyExampleModel` — a `globalmanager` initializer, which is
already error-shaped — now uses one. Promotion covers the three capabilities
scoped at the top and no others.

## Rollback

Every step is a separate commit on `stability/globalmanager-promotion`. `main`
is untouched until this branch is reviewed. Individual steps can be reverted
without disturbing the others; steps 6-7 add only test files.
