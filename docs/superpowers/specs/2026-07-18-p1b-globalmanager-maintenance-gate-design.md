# P1-B GlobalManager Maintenance Gate Design

**Date:** 2026-07-18

**Status:** Approved direction, written specification pending user review

## Goal

Eliminate three deterministic same-key lifecycle conflicts in `GlobalManager`
without changing its public API or introducing a general resource-ownership
framework:

1. concurrent `Rebuild` calls both operate on the same old instance and the
   later store silently discards the other candidate;
2. `Rebuild` and `Release` overwrite each other's compound state changes;
3. concurrent `Release` calls can invoke `Close` twice on the same instance.

The fix is a private, fail-fast maintenance gate on each `entry`. It does not
claim to complete the full `Get`/`Rebuild`/`Release`/deletion state machine.

## Evidence and Constraints

`entry.instance`, `entry.once`, `entry.initErr`, and `entry.initialized` are
individually atomic, but `Rebuild` and `Release` perform multi-step state
transitions. Atomic fields therefore prevent many Go data races while still
allowing non-linearizable logical results.

The existing `entry.mu` only protects reset steps after a successful close. It
does not prevent concurrent user callbacks because `Rebuild` does not acquire
it and `Release` calls `Close` before acquiring it.

`GlobalManager` must not automatically close the instance replaced by
`Rebuild`. The concrete MySQL, MongoDB, and Redis rebuilders replace their
inner client in place and return the same wrapper. Closing the old wrapper
would close the newly installed client. Existing aliases, composite caches,
logger ownership, and task borrowers also prevent inferring ownership from a
container key or from the `Closable` interface.

## Considered Approaches

### 1. Private fail-fast CAS gate — selected

Add `maintenance atomic.Bool` to `entry`. `Rebuild` and `Release` attempt a
compare-and-swap before reading the current instance or invoking a user
callback. If another maintenance operation owns the gate, the new operation
returns a wrapped private busy error immediately.

Benefits:

- minimal production change;
- no public signature or exported API change;
- no slow user callback executes while holding a mutex;
- same-key re-entry returns an error instead of deadlocking;
- different keys remain independent.

Tradeoff: callers may now receive a busy error from a same-key concurrent
maintenance request. This is intentional fail-fast behavior; this phase does
not promise waiting or automatic retry. Although no exported symbol is added,
the extra transient error is publicly observable. Because its sentinel is
private, external callers can only handle it as an ordinary maintenance
failure; reliable busy classification and retry policy are explicitly not a
stable API in this experimental phase.

### 2. Hold `entry.mu` across maintenance — rejected

This would serialize operations, but a slow `Rebuild` or `Close` would make the
second caller wait indefinitely. A callback that re-enters `Rebuild` or
`Release` for the same key would deadlock. It would also overload a mutex that
currently protects reset bookkeeping with a new long-running role.

### 3. Full lifecycle state machine — deferred

States such as ready, rebuilding, releasing, and retired plus generations and
completion channels could coordinate more cases without holding locks across
I/O. That design first requires explicit answers for stale candidate disposal,
old-resource retirement, aliases, borrowers, deletion, cancellation, and
close failure recovery. It exceeds this targeted correction.

## Production Design

### Entry state

Add one zero-value-ready field:

```go
type entry struct {
	// existing fields
	maintenance atomic.Bool
}
```

Keep `entry.mu` and its existing reset responsibility unchanged.

### Busy error and helpers

Define an unexported sentinel and private helpers in `globalmanager/manager.go`:

```go
var errMaintenanceInProgress = errors.New("global object maintenance already in progress")

func (e *entry) beginMaintenance(name KeyName) error {
	if !e.maintenance.CompareAndSwap(false, true) {
		return fmt.Errorf("%w: %s", errMaintenanceInProgress, name)
	}
	return nil
}

func (e *entry) endMaintenance() {
	e.maintenance.Store(false)
}
```

The sentinel remains private, so this change does not add a stable public error
type. Package tests may use `errors.Is`; external callers continue to receive a
normal `error` and must not rely on the exact text as a versioned API.

### Rebuild

After loading and type-checking the `entry`, but before reading its instance:

```go
if err := entity.beginMaintenance(name); err != nil {
	return err
}
defer entity.endMaintenance()
```

All current validation and publishing behavior remains unchanged. In
particular, a successful rebuild stores the returned instance but does not
close the prior value.

### Release

Acquire the same gate after loading and type-checking the `entry`, but before
reading its instance or calling `Close`. Release it with `defer`.

The current contracts remain:

- a successful `Close` resets instance, initialization state, error, and
  `sync.Once` so a later `Get` may initialize again;
- a failed `Close` returns an error and retains the existing instance/state;
- a non-`Closable` instance remains a no-op success;
- panics still propagate, while deferred gate release prevents permanent busy
  state.

`ReleaseAll` is not redesigned. If it meets a same-key operation already in
progress, its existing best-effort error reporting observes the busy error.

## Concurrency Contract

For one registered key:

- at most one `Rebuild` or `Release` user callback may execute at a time;
- a conflicting maintenance call fails immediately without entering its user
  callback;
- the winning operation releases the gate on success, returned error, or panic
  unwinding;
- a later maintenance call may proceed after the winner returns;
- re-entry from `Rebuild` or `Close` into the same key fails busy rather than
  deadlocking.

Across different keys, maintenance remains fully independent.

`Get` and `CheckHealth` do not acquire the gate. A caller may still hold or
obtain a resource whose close/rebuild is in progress; solving returned-reference
lifetime requires leases, reference tracking, or owner-specific coordination
and is explicitly deferred.

## Deterministic Test Design

All concurrency tests use channel handshakes rather than scheduling sleeps.
Every blocking receive has a bounded timeout used only as deadlock protection.
Goroutines report through buffered result channels and never call `t.Fatal`
directly. Test cleanup uses `sync.Once` only to close callback-unblock channels
so a failed assertion cannot leave a goroutine blocked. Tests never mutate the
production maintenance gate; only production `defer entity.endMaintenance()`
may release it.

### RED/GREEN maintenance cases

1. **Rebuild during Rebuild**
   - first callback signals entry and blocks;
   - second `Rebuild` must return `errMaintenanceInProgress` without entering a
     second callback;
   - release the first callback and verify its candidate is published; the
     candidate also implements `Rebuilder` so another rebuild is meaningful;
   - run another rebuild to prove the gate was released.

2. **Rebuild during Release**
   - `Close` signals entry and blocks;
   - `Rebuild` must return busy without invoking the rebuild callback;
   - release `Close`, verify release state, then prove subsequent maintenance is
     allowed.

3. **Release during Rebuild**
   - rebuild callback signals entry and blocks;
   - `Release` must return busy without invoking `Close`;
   - release rebuild and verify its candidate is published.

4. **Release during Release**
   - the fake's first `Close` signals entry and blocks, while any unexpected
     later `Close` increments the count and returns immediately; this makes the
     old implementation fail by observable double close rather than timeout;
   - second `Release` must return busy and the close count must remain one;
   - release the first close and verify a later `Get` reinitializes normally.

The old implementation should fail these logical assertions even if the race
detector reports no manager-field data race.

### Gate release after returned errors and panics

Add four focused cases that prove the `defer` covers non-success paths:

1. a rebuild callback returns an error once; the error is preserved and a later
   rebuild enters its callback and succeeds;
2. `Close` returns an error once; the existing instance remains published and a
   later `Release` enters `Close` again and succeeds;
3. a rebuilder panics once from `GetConfPath` or `Rebuild`; the test recovers the
   original panic, then a later rebuild enters normally;
4. `Close` panics once; the test recovers the original panic, then a later
   release enters normally.

The panic fakes panic only on their first invocation. Recovery exists only in
the tests; production continues to propagate the panic after its deferred gate
release.

### Green characterization cases

Table-drive `Unregister` and `ClearAll(true)` while a `Get` initializer is
blocked after it has obtained the entry:

- deletion returns and future lookup reports not found;
- the already-started initializer may finish and its original `Get` returns the
  created value;
- later lookups remain not found.

This records the current locator/deletion boundary. It does not claim that
deletion cancels initialization or owns and retires a detached result.

### Verification

At minimum:

```bash
go test ./globalmanager -run 'Test(Rebuild|Release|GetInitialization)' -count=1
go test ./globalmanager -run 'Test(Rebuild|Release|GetInitialization)' -count=50
go test -race ./globalmanager -count=1
go vet ./...
go test ./... -count=1
go test -race ./... -count=1
```

## Documentation

Update `docs/reference/feature-status.md` and
`.codegraph-qa-out/readme-current-status-optimization-todo.md` to record only
the completed maintenance gate and characterization contract.

Overall status remains `P1 部分执行`. Do not mark the full GlobalManager state
transition or old-instance retirement checklist items complete.

## Explicit Non-Goals

- no automatic close of a rebuilt instance or its previous wrapper;
- no exported busy error, retry API, waiting, backoff, or cancellation;
- no change to `Get`, `CheckHealth`, `Register`, `Unregister`, `ClearAll`, or
  `ReleaseAll` signatures;
- no deletion barrier and no cancellation of an initializer already in flight;
- no manager-wide lock;
- no database wrapper/client synchronization change;
- no alias, composite cache, logger, task, or shutdown-registry ownership work;
- no change to `ClearAll` deletion-only semantics.

## Acceptance Criteria

- same-key `Rebuild`/`Release` maintenance callbacks never overlap;
- a conflicting call returns a wrapped private busy error immediately;
- gate release is proven after success, returned-error, and panic-unwinding
  paths; the panic test recovers only in the test and does not change production
  panic behavior;
- successful rebuild/release behavior remains compatible;
- deletion characterization tests document current in-flight `Get` behavior;
- targeted, repeated, race, vet, and full-suite verification pass;
- no public API or unrelated production code changes.
