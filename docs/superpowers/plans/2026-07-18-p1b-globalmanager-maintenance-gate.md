# P1-B GlobalManager Maintenance Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent overlapping `Rebuild` and `Release` callbacks within one registered `entry` generation with a private fail-fast CAS gate while preserving current deletion, rebuild publishing, release reset, and public API behavior.

**Architecture:** Add one `atomic.Bool` maintenance gate per existing `entry`. `Rebuild` and `Release` acquire it after resolving the entry and before reading the instance; a conflict returns a wrapped private busy error, and `defer` releases the gate on every success, error, and panic-unwinding path. Deterministic channel tests prove the logical concurrency contract; deletion tests only characterize the existing locator semantics.

**Tech Stack:** Go 1.25, `sync.Map`, `sync/atomic`, standard `errors`, channel-based deterministic tests, Go race detector, CodeGraph, ast-grep.

## Global Constraints

- Work only in `/mnt/d/code/github_opensource/tmp/fiberhouse/.worktrees/p1b-globalmanager-maintenance` on branch `p1b-globalmanager-maintenance`.
- Do not merge, rebase, cherry-pick, push, synchronize to `main`, delete worktrees, or modify the user's untracked analysis document.
- Read `AGENTS.md` and `CLAUDE.md`; use `codegraph explore` before grep or direct source reads for code understanding.
- Preserve all public method signatures and exported symbols; `errMaintenanceInProgress` remains private.
- Preserve `ClearAll(true)` as deletion-only and do not add resource closing to `Rebuild`.
- Do not change `Get`, `CheckHealth`, `Register`, `Unregister`, `ClearAll`, or `ReleaseAll` behavior.
- Do not hold `entry.mu` or a new mutex across `Rebuild`, `GetConfPath`, or `Close` callbacks.
- The gate covers only `Rebuild` and `Release` on the same registered `entry` generation; delete plus same-name re-registration creates an independent generation, different entries remain independent, and `Get` does not acquire the gate.
- All concurrency synchronization uses channels; timeouts are deadlock guards, not proof of ordering.
- Every production change follows RED → GREEN and every task ends with an independent commit and review.

---

### Task 1: Characterize deletion during an in-flight Get

**Files:**
- Modify: `globalmanager/manager_lifecycle_test.go`

**Interfaces:**
- Consumes: existing `GlobalManager.Register`, `Get`, `Unregister`, `ClearAll`, and `IsRegistered`.
- Produces: test helpers `lifecycleAwait`, `lifecycleReceive[T]`, and the locked-in rule that deletion affects future lookups but does not cancel a `Get` that already obtained its entry.

- [ ] **Step 1: Add bounded channel helpers**

Add `time` to the test imports and add:

```go
const lifecycleTestTimeout = 2 * time.Second

func lifecycleAwait(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(lifecycleTestTimeout):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func lifecycleReceive[T any](t *testing.T, result <-chan T, label string) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(lifecycleTestTimeout):
		var zero T
		t.Fatalf("timed out waiting for %s", label)
		return zero
	}
}

type lifecycleGetResult struct {
	value interface{}
	err   error
}
```

- [ ] **Step 2: Add the green characterization test**

Append:

```go
func TestGetInitialization_RemovalOnlyAffectsFutureLookups(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*GlobalManager)
	}{
		{name: "unregister", remove: func(manager *GlobalManager) { manager.Unregister("resource") }},
		{name: "clear-all", remove: func(manager *GlobalManager) { manager.ClearAll(true) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := NewGlobalManager()
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)

			if !manager.Register("resource", func() (interface{}, error) {
				close(entered)
				<-release
				return "detached", nil
			}) {
				t.Fatal("Register(resource) = false")
			}

			result := make(chan lifecycleGetResult, 1)
			go func() {
				value, err := manager.Get("resource")
				result <- lifecycleGetResult{value: value, err: err}
			}()

			lifecycleAwait(t, entered, "initializer entry")
			test.remove(manager)
			if manager.IsRegistered("resource") {
				t.Fatal("removal retained resource")
			}
			if _, err := manager.Get("resource"); err == nil {
				t.Fatal("Get after removal error = nil")
			}

			unblock()
			got := lifecycleReceive(t, result, "in-flight Get")
			if got.err != nil || got.value != "detached" {
				t.Fatalf("in-flight Get = (%v, %v), want detached, nil", got.value, got.err)
			}
			if _, err := manager.Get("resource"); err == nil {
				t.Fatal("later Get after removal error = nil")
			}
		})
	}
}
```

- [ ] **Step 3: Verify the characterization**

Run:

```bash
gofmt -w globalmanager/manager_lifecycle_test.go
GOCACHE=/tmp/fiberhouse-p1b-task1 go test ./globalmanager -run '^TestGetInitialization_RemovalOnlyAffectsFutureLookups$' -count=50
GOCACHE=/tmp/fiberhouse-p1b-task1-race go test -race ./globalmanager -run '^TestGetInitialization_RemovalOnlyAffectsFutureLookups$' -count=10
```

Expected: both commands pass on the pre-gate implementation. This task records current deletion behavior; it is not a RED bug fix.

- [ ] **Step 4: Commit Task 1**

```bash
git add globalmanager/manager_lifecycle_test.go
git commit -m "test: characterize GlobalManager removal during Get"
```

---

### Task 2: Add the fail-fast maintenance gate

**Files:**
- Modify: `globalmanager/manager.go`
- Modify: `globalmanager/manager_lifecycle_test.go`

**Interfaces:**
- Consumes: Task 1 `lifecycleAwait`, `lifecycleReceive[T]`, and current `entry` lifecycle fields.
- Produces: private `errMaintenanceInProgress`, `(*entry).beginMaintenance(KeyName) error`, `(*entry).endMaintenance()`, and fail-fast exclusion shared by `Rebuild` and `Release`.

- [ ] **Step 1: Add controlled maintenance fakes**

Append to the test file:

```go
type lifecycleControlledResource struct {
	path    func() string
	rebuild func(...interface{}) (interface{}, error)
	close   func() error
}

func (r *lifecycleControlledResource) GetConfPath() string {
	if r.path != nil {
		return r.path()
	}
	return "config.yml"
}

func (r *lifecycleControlledResource) Rebuild(arguments ...interface{}) (interface{}, error) {
	if r.rebuild == nil {
		return nil, errors.New("unexpected Rebuild call")
	}
	return r.rebuild(arguments...)
}

func (r *lifecycleControlledResource) Close() error {
	if r.close == nil {
		return errors.New("unexpected Close call")
	}
	return r.close()
}

type lifecycleErrorResult struct {
	err error
}

func lifecycleAsyncError(call func() error) <-chan lifecycleErrorResult {
	result := make(chan lifecycleErrorResult, 1)
	go func() { result <- lifecycleErrorResult{err: call()} }()
	return result
}

func assertLifecycleBusy(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "global object maintenance already in progress") {
		t.Fatalf("maintenance error = %v, want busy", err)
	}
}
```

- [ ] **Step 2: Add failing same-operation tests**

Add `TestRebuild_ConcurrentMaintenanceReturnsBusy` and
`TestRelease_ConcurrentMaintenanceReturnsBusyAndClosesOnce` with these exact
events and assertions:

```go
func TestRebuild_ConcurrentMaintenanceReturnsBusy(t *testing.T) {
	manager := NewGlobalManager()
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var calls atomic.Int32

	candidate := &lifecycleControlledResource{}
	candidate.rebuild = func(...interface{}) (interface{}, error) { return candidate, nil }
	current := &lifecycleControlledResource{}
	current.rebuild = func(...interface{}) (interface{}, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return candidate, nil
	}
	manager.Register("resource", func() (interface{}, error) { return current, nil })
	if _, err := manager.Get("resource"); err != nil {
		t.Fatal(err)
	}

	first := lifecycleAsyncError(func() error { return manager.Rebuild("resource") })
	lifecycleAwait(t, entered, "first rebuild")
	second := lifecycleAsyncError(func() error { return manager.Rebuild("resource") })
	assertLifecycleBusy(t, lifecycleReceive(t, second, "conflicting rebuild").err)
	if got := calls.Load(); got != 1 {
		t.Fatalf("Rebuild callbacks = %d, want 1", got)
	}
	unblock()
	if err := lifecycleReceive(t, first, "first rebuild result").err; err != nil {
		t.Fatalf("first Rebuild error = %v", err)
	}
	if got, err := manager.Get("resource"); err != nil || got != candidate {
		t.Fatalf("Get rebuilt resource = (%v, %v), want candidate, nil", got, err)
	}
	if err := manager.Rebuild("resource"); err != nil {
		t.Fatalf("Rebuild after gate release error = %v", err)
	}
}
```

For the concurrent release test, use a `closeCalls atomic.Int32`; run both
release calls with `lifecycleAsyncError`. The first
`Close` closes `entered` and waits on `release`, while later unexpected calls
return immediately. Assert the second `Release` is busy, `closeCalls == 1`, the
first release succeeds after unblocking, and a later `Get` invokes the
initializer again.

- [ ] **Step 3: Add failing cross-operation tests**

Add one table-driven `TestMaintenance_RebuildAndReleaseConflictReturnsBusy`
with two subtests:

- `release-blocks-rebuild`: block the winner in `Close`; assert `Rebuild`
  returns busy and the rebuild callback count remains zero.
- `rebuild-blocks-release`: block the winner in `Rebuild`; assert `Release`
  returns busy and the close callback count remains zero.

Run the winner and conflicting operation through `lifecycleAsyncError`, obtain
the conflicting result through `lifecycleReceive`, then unblock the winner and
require its result to be nil. Finally invoke a later maintenance operation to
prove the gate was released. Use the same
bounded helpers and `sync.Once` cleanup pattern as the exact test above.

Also add `TestMaintenance_SameEntryReentryReturnsBusy` with two subtests. Run
each outer call with `lifecycleAsyncError` and obtain it with
`lifecycleReceive` so an incorrect waiting lock is bounded:

- `rebuild-calls-release`: inside `Rebuild`, synchronously call
  `manager.Release("resource")`, store the nested error, and return a valid
  candidate; require outer success, nested busy, and zero `Close` calls;
- `close-calls-rebuild`: inside `Close`, synchronously call
  `manager.Rebuild("resource")`, store the nested error, and return nil;
  require outer success, nested busy, and zero rebuild callback calls.

Add green characterization `TestMaintenance_DifferentKeysRemainIndependent`:
block a rebuild callback for key `one`, then synchronously rebuild key `two` and
require its callback and return to complete before key `one` is unblocked.
Finally unblock and require key `one` to finish. This protects against an
accidental manager-wide gate.

- [ ] **Step 4: Run RED against the old implementation**

Run:

```bash
GOCACHE=/tmp/fiberhouse-p1b-task2-red go test ./globalmanager -run 'Test(Rebuild_ConcurrentMaintenance|Release_ConcurrentMaintenance|Maintenance_(RebuildAndReleaseConflict|SameEntryReentry|DifferentKeys))' -count=1
```

Expected: FAIL. The old implementation enters overlapping callbacks or returns
nil rather than a busy error. A timeout is a test defect; fix test handshakes
before production code.

- [ ] **Step 5: Implement the private gate**

Add `errors` to `globalmanager/manager.go` imports. Add to `entry`:

```go
maintenance atomic.Bool
```

Add after `storedError`:

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

In both `Rebuild` and `Release`, immediately after the `*entry` type assertion
and before any instance read, add:

```go
if err := entity.beginMaintenance(name); err != nil {
	return err
}
defer entity.endMaintenance()
```

Do not move callback code under `entity.mu`, close old rebuild values, or alter
any other method.

- [ ] **Step 6: Run GREEN and internal error wrapping check**

Add this same-package unit check:

```go
func TestEntry_BeginMaintenanceWrapsPrivateBusyError(t *testing.T) {
	entity := &entry{}
	if err := entity.beginMaintenance("resource"); err != nil {
		t.Fatalf("first beginMaintenance error = %v", err)
	}
	defer entity.endMaintenance()
	if err := entity.beginMaintenance("resource"); !errors.Is(err, errMaintenanceInProgress) {
		t.Fatalf("second beginMaintenance error = %v, want private busy sentinel", err)
	}
}
```

Run:

```bash
gofmt -w globalmanager/manager.go globalmanager/manager_lifecycle_test.go
GOCACHE=/tmp/fiberhouse-p1b-task2 go test ./globalmanager -run 'Test(Rebuild_ConcurrentMaintenance|Release_ConcurrentMaintenance|Maintenance_(RebuildAndReleaseConflict|SameEntryReentry|DifferentKeys)|Entry_BeginMaintenance)' -count=50
GOCACHE=/tmp/fiberhouse-p1b-task2-race go test -race ./globalmanager -count=1
```

Expected: all commands pass and race reports no data race.

- [ ] **Step 7: Commit Task 2**

```bash
git add globalmanager/manager.go globalmanager/manager_lifecycle_test.go
git commit -m "fix: reject overlapping global maintenance"
```

---

### Task 3: Prove gate release after errors and panics

**Files:**
- Modify: `globalmanager/manager_lifecycle_test.go`

**Interfaces:**
- Consumes: Task 2 `beginMaintenance`/`endMaintenance` behavior through public `Rebuild` and `Release` calls.
- Produces: regression coverage proving production `defer` releases the gate after callback errors and panic unwinding without changing panic propagation.

- [ ] **Step 1: Add returned-error release tests**

Add `TestMaintenanceGate_ReleasesAfterReturnedError` with subtests `rebuild` and
`release`:

- rebuild fake returns `errors.New("rebuild failed")` on call 1 and returns a
  valid controlled candidate on call 2; require the first error text and the
  second call to succeed;
- close fake returns `errors.New("close failed")` on call 1 and nil on call 2;
  require the first error text, verify `Get` still returns the original after
  failure, and require the second release to succeed.

Use `atomic.Int32` call counts and require exactly two callbacks in each
subtest. No goroutine or timeout is needed because these calls are sequential.

- [ ] **Step 2: Add panic-unwinding release tests**

Add helper:

```go
func captureLifecyclePanic(call func()) (recovered interface{}) {
	defer func() { recovered = recover() }()
	call()
	return nil
}
```

Add `TestMaintenanceGate_ReleasesAfterPanic` with subtests:

- `get-conf-path`: `path` panics with `"path panic"` only on its first call;
  recover that exact value, then require a later rebuild to succeed;
- `rebuild`: callback panics with `"rebuild panic"` only on its first call;
  recover it, then require a later rebuild to succeed;
- `close`: callback panics with `"close panic"` only on its first call; recover
  it, then require a later release to succeed.

Use `atomic.Int32` to make each fake panic only once. Do not add production
recovery; the tests recover outside the manager call.

- [ ] **Step 3: Verify failure-path hardening**

Run:

```bash
gofmt -w globalmanager/manager_lifecycle_test.go
GOCACHE=/tmp/fiberhouse-p1b-task3 go test ./globalmanager -run '^TestMaintenanceGate_ReleasesAfter' -count=50
GOCACHE=/tmp/fiberhouse-p1b-task3-race go test -race ./globalmanager -run '^TestMaintenanceGate_ReleasesAfter' -count=20
GOCACHE=/tmp/fiberhouse-p1b-task3-package go test ./globalmanager -count=1
```

Expected: all commands pass; panics remain observable to the outer test
recovery and later maintenance is not busy.

- [ ] **Step 4: Commit Task 3**

```bash
git add globalmanager/manager_lifecycle_test.go
git commit -m "test: cover maintenance gate failure paths"
```

---

### Task 4: Record the partial P1-B contract

**Files:**
- Modify: `docs/reference/feature-status.md`
- Modify: `.codegraph-qa-out/readme-current-status-optimization-todo.md`

**Interfaces:**
- Consumes: Tasks 1–3 verified behavior and implementation HEAD.
- Produces: accurate P1-B status without claiming complete GlobalManager ownership or lifecycle closure.

- [ ] **Step 1: Update feature status narrowly**

In the GlobalManager row, record:

- within one registered entry generation, `Rebuild`/`Release` maintenance is
  fail-fast mutually exclusive;
- a conflicting call returns an ordinary experimental busy error;
- deletion does not cancel an already-started `Get` initializer.

Retain every existing limitation about returned-reference lifetime,
old-instance retirement, owner/locator, aliases/composites, task lifecycle,
custom starter responsibility, and deletion-only `ClearAll`.

- [ ] **Step 2: Add a P1-B execution record**

In `.codegraph-qa-out/readme-current-status-optimization-todo.md`:

- keep overall status `P1 部分执行`;
- do not check the full legal-state-transition or old-instance-retirement items;
- add a dated `P1-B 执行记录（2026-07-18）` with the implementation HEAD and
  exact targeted/repeated/race commands actually run;
- state that the private busy sentinel is not a stable exported retry contract;
- state that automatic rebuild retirement remains unsafe for the current
  in-place database/cache wrappers.

- [ ] **Step 3: Verify documentation and full branch**

Run:

```bash
rg -n 'maintenance|busy|Rebuild|Release|P1-B|P1 部分执行|ClearAll' docs/reference/feature-status.md .codegraph-qa-out/readme-current-status-optimization-todo.md
rg -n --fixed-strings -- '- [ ] 定义并测试 `Register`、`Get`、`Rebuild`、`Release`、`Unregister`、`ClearAll` 的合法状态转移。' .codegraph-qa-out/readme-current-status-optimization-todo.md
rg -n --fixed-strings -- '- [ ] 定义 Rebuild 成功后旧实例的关闭行为；避免无主 client/连接池泄漏。' .codegraph-qa-out/readme-current-status-optimization-todo.md
GOCACHE=/tmp/fiberhouse-p1b-final-vet go vet ./...
GOCACHE=/tmp/fiberhouse-p1b-final-test go test ./... -count=1
GOCACHE=/tmp/fiberhouse-p1b-final-race go test -race ./... -count=1
git diff --check 2e6bc79..HEAD
```

Expected: commands exit 0; documentation records only the partial gate and
characterization contract.

- [ ] **Step 4: Commit Task 4**

```bash
git add docs/reference/feature-status.md .codegraph-qa-out/readme-current-status-optimization-todo.md
git commit -m "docs: record P1-B maintenance contract"
```

---

## Final Verification

After all task reviews are clean, run fresh from the isolated worktree:

```bash
GOCACHE=/tmp/fiberhouse-p1b-accept-vet go vet ./...
GOCACHE=/tmp/fiberhouse-p1b-accept-test go test ./... -count=1
GOCACHE=/tmp/fiberhouse-p1b-accept-race go test -race ./... -count=1
git diff --check 2e6bc79..HEAD
git status --short --branch
test "$(git -C ../.. rev-parse --short HEAD)" = "2e6bc79"
test -f ../../.codegraph-qa-out/readme-current-status-analysis-2026-07-17.md
```

Expected:

- vet, normal tests, race tests, and diff check exit 0;
- the isolated worktree is clean;
- `main` remains at `2e6bc79` with the user's untracked analysis document;
- no merge, rebase, cherry-pick, push, synchronization, or worktree cleanup has occurred.
