# P1-A Keepalive and ClearAll Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop the default global-health keepalive before built-in HTTP cleanup and replace unsafe `sync.Map` field assignment with deletion-only `sync.Map.Clear`, without changing public APIs or resource ownership semantics.

**Architecture:** Preserve the existing `RunServer -> AppCoreRun` architecture. Add private cancel/wait state only to the default `FrameApplication`, expose it to the built-in cores through a package-private optional interface, and keep `GlobalManager.ClearAll` deletion-only. Fiber and Gin retain their current signal/server shutdown logic.

**Tech Stack:** Go 1.25, `context`, `sync`, `sync.Map.Clear`, Fiber v2 hooks, `net/http`, Go race detector, CodeGraph, ast-grep.

## Global Constraints

- Work only in branch `p1a-keepalive-clearall` and worktree `.worktrees/p1a-keepalive-clearall`.
- Do not merge, cherry-pick, rebase, push, or otherwise synchronize this branch with `main`.
- Preserve the public signatures and method sets of `RunServer`, `CoreStarter`, `FrameStarter`, `IApplicationContext`, `GlobalManager`, and all existing exported interfaces.
- `ClearAll(true)` remains deletion-only; it must not call `Release`, `ReleaseAll`, or any resource `Close` method.
- Do not add `ReleaseAll(true)` to Fiber or Gin shutdown.
- Do not change task worker/dispatcher lifecycle, Redis aliases, L2 cache ownership, logger ownership, `Rebuild`, server error propagation, or shutdown provider locations.
- Keep custom `FrameStarter` behavior unchanged; private stopping applies only when the mounted frame implements the package-private stopper.
- Once a default `FrameApplication` starts health checking, it may be stopped repeatedly but must not restart.
- Stop must cancel and wait before container clear; an in-progress `IsHealthy` or `Rebuild` is allowed to delay stop because those interfaces have no context.
- Use CodeGraph before reading code for new codebase questions; use ast-grep for structural call-site checks.
- Implement behavior with TDD, commit each task, and preserve the untracked analysis document in the main checkout.

---

### Task 1: Make `ClearAll` clear the existing `sync.Map`

**Files:**
- Modify: `globalmanager/manager.go:300-305`
- Modify: `globalmanager/manager_lifecycle_test.go:374-412`

**Interfaces:**
- Consumes: existing `func (gm *GlobalManager) ClearAll(conform ...bool)`.
- Produces: the same method and confirmation semantics, implemented with `gm.container.Clear()`.

- [ ] **Step 1: Add tests that preserve deletion-only semantics and exercise concurrent clear**

Append tests equivalent to:

```go
func TestClearAll_DoesNotCloseResources(t *testing.T) {
	manager := NewGlobalManager()
	closed := 0
	manager.Register("resource", func() (interface{}, error) {
		return &lifecycleClosable{closed: &closed}, nil
	})
	if _, err := manager.Get("resource"); err != nil {
		t.Fatal(err)
	}

	manager.ClearAll(true)

	if closed != 0 {
		t.Fatalf("ClearAll(true) closed %d resources, want 0", closed)
	}
	if manager.IsRegistered("resource") {
		t.Fatal("ClearAll(true) retained resource")
	}
}

func TestClearAll_ConcurrentMapOperations(t *testing.T) {
	manager := NewGlobalManager()
	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			manager.Register("resource", func() (interface{}, error) { return i, nil })
			_, _ = manager.Get("resource")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			manager.Range(func(_, _ interface{}) bool { return true })
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			manager.ClearAll(true)
		}
	}()

	wg.Wait()
}
```

Add `sync` to the test imports.

- [ ] **Step 2: Run the race test against the old implementation**

Run:

```bash
GOCACHE=/tmp/fiberhouse-p1a-task1 go test -race ./globalmanager -run 'TestClearAll_(DoesNotCloseResources|ConcurrentMapOperations)$' -count=1
```

Expected: the semantic test passes, while the concurrent test reports a race involving assignment to `gm.container` and concurrent `sync.Map` operations.

- [ ] **Step 3: Replace the field assignment with `sync.Map.Clear`**

Use exactly:

```go
func (gm *GlobalManager) ClearAll(conform ...bool) {
	if len(conform) > 0 && conform[0] {
		gm.container.Clear()
	}
}
```

- [ ] **Step 4: Verify Task 1**

Run:

```bash
gofmt -w globalmanager/manager.go globalmanager/manager_lifecycle_test.go
GOCACHE=/tmp/fiberhouse-p1a-task1 go test ./globalmanager -count=1
GOCACHE=/tmp/fiberhouse-p1a-task1-race go test -race ./globalmanager -count=1
```

Expected: both test commands exit 0 with no race report.

- [ ] **Step 5: Commit Task 1**

```bash
git add globalmanager/manager.go globalmanager/manager_lifecycle_test.go
git commit -m "fix: clear global manager map safely"
```

---

### Task 2: Make the default health keepalive stoppable and waitable

**Files:**
- Modify: `frame_starter_impl.go:18-24,234-282`
- Modify: `frame_starter_impl_test.go`

**Interfaces:**
- Consumes: existing `FrameApplication`, `RegisterGlobalsKeepalive`, and `startHealthCheck`.
- Produces: private `func (fa *FrameApplication) stopHealthCheck()` and private `func (fa *FrameApplication) checkGlobalsHealthOnce()`.

- [ ] **Step 1: Add failing lifecycle tests**

Add a blocking health checker and tests with an isolated manager:

```go
type frameBlockingHealthChecker struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (h *frameBlockingHealthChecker) IsHealthy() bool {
	if h.calls.Add(1) == 1 {
		close(h.entered)
	}
	<-h.release
	return true
}

func isolateFrameHealthManager(t *testing.T, ctx IApplicationContext) *globalmanager.GlobalManager {
	t.Helper()
	appCtx := ctx.(*AppContext)
	manager := globalmanager.NewGlobalManager()
	appCtx.container = manager
	return manager
}
```

Add tests that perform these exact assertions:

```go
func TestFrameApplication_StopHealthCheckWaitsAndPreventsRestart(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	manager := isolateFrameHealthManager(t, ctx)
	checker := &frameBlockingHealthChecker{entered: make(chan struct{}), release: make(chan struct{})}
	manager.Register("health", func() (interface{}, error) { return checker, nil })
	_, _ = manager.Get("health")
	frame := &FrameApplication{Ctx: ctx}

	frame.startHealthCheck(time.Millisecond)
	<-checker.entered
	stopped := make(chan struct{})
	go func() {
		frame.stopHealthCheck()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("stopHealthCheck returned before active check completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(checker.release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stopHealthCheck did not return")
	}

	before := checker.calls.Load()
	frame.startHealthCheck(time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	if got := checker.calls.Load(); got != before {
		t.Fatalf("health check restarted: calls=%d, want %d", got, before)
	}
}

func TestFrameApplication_StopHealthCheckIsConcurrentAndIdempotent(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	isolateFrameHealthManager(t, ctx)
	frame := &FrameApplication{Ctx: ctx}
	frame.startHealthCheck(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			frame.stopHealthCheck()
		}()
	}
	wg.Wait()
}

func TestFrameApplication_StartHealthCheckRejectsInvalidInterval(t *testing.T) {
	ctx, logs := newFrameTestContext(t, nil)
	isolateFrameHealthManager(t, ctx)
	frame := &FrameApplication{Ctx: ctx}
	frame.startHealthCheck(0)
	if frame.healthCancel != nil {
		t.Fatal("invalid interval started health check")
	}
	assert.Contains(t, logs.String(), "health check interval must be positive")
}
```

Add `context`, `sync`, and `sync/atomic` to the production/test imports as required.

- [ ] **Step 2: Run the new tests against the old implementation**

Run:

```bash
GOCACHE=/tmp/fiberhouse-p1a-task2 go test . -run 'TestFrameApplication_(StopHealthCheck|StartHealthCheck)' -count=1
```

Expected: FAIL because `stopHealthCheck`, lifecycle fields, and invalid-interval handling do not exist.

- [ ] **Step 3: Add private lifecycle state and context-controlled loop**

Add to `FrameApplication`:

```go
healthMu     sync.Mutex
healthCancel context.CancelFunc
healthWG     sync.WaitGroup
```

Implement the following behavior:

```go
func (fa *FrameApplication) startHealthCheck(interval time.Duration) {
	if interval <= 0 {
		fa.GetContext().GetLogger().ErrorWith(fa.GetContext().GetConfig().LogOriginFrame()).
			Msg("health check interval must be positive")
		return
	}

	fa.healthMu.Lock()
	if fa.healthCancel != nil {
		fa.healthMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	fa.healthCancel = cancel
	fa.healthWG.Add(1)
	fa.healthMu.Unlock()

	go func() {
		defer fa.healthWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer fa.recoverHealthCheckPanic()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fa.checkGlobalsHealthOnce()
			}
		}
	}()
}

func (fa *FrameApplication) stopHealthCheck() {
	fa.healthMu.Lock()
	cancel := fa.healthCancel
	fa.healthMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	fa.healthWG.Wait()
}
```

Move the existing panic logging into `recoverHealthCheckPanic`, and move one `Range` pass into `checkGlobalsHealthOnce`. When `Rebuild` fails, log the error and return from that `Range` callback without logging success.

- [ ] **Step 4: Verify Task 2**

Run:

```bash
gofmt -w frame_starter_impl.go frame_starter_impl_test.go
GOCACHE=/tmp/fiberhouse-p1a-task2 go test . -run 'TestFrameApplication_' -count=1
GOCACHE=/tmp/fiberhouse-p1a-task2-race go test -race . -run 'TestFrameApplication_' -count=1
```

Expected: both commands exit 0; the race run reports no data race.

- [ ] **Step 5: Commit Task 2**

```bash
git add frame_starter_impl.go frame_starter_impl_test.go
git commit -m "fix: stop global health keepalive cleanly"
```

---

### Task 3: Connect built-in Fiber and Gin cleanup to the private stopper

**Files:**
- Modify: `frame_starter_impl.go`
- Modify: `frame_starter_impl_test.go`
- Modify: `core_fiber_starter_impl.go:257-265`
- Modify: `core_gin_starter_impl.go:363-373`

**Interfaces:**
- Consumes: Task 2 `(*FrameApplication).stopHealthCheck`.
- Produces: private `healthCheckStopper`, `stopFrameHealthCheck`, and `clearApplicationGlobals` helpers used by both built-in cores.

- [ ] **Step 1: Add failing helper tests**

Add tests that mount the default frame through the real outer `WebApplication` path:

```go
func TestClearApplicationGlobalsStopsMountedFrameAndClearsContainer(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	manager := isolateFrameHealthManager(t, ctx)
	frame := &FrameApplication{Ctx: ctx}
	starter := &WebApplication{FrameStarter: frame, CoreStarter: &lifecycleRecordingStarter{
		managerCalls: make(map[string][]IProviderManager),
	}}
	ctx.RegisterStarterApp(starter)
	manager.Register("value", func() (interface{}, error) { return 1, nil })
	_, _ = manager.Get("value")
	frame.startHealthCheck(time.Hour)

	clearApplicationGlobals(ctx)

	if manager.IsRegistered("value") {
		t.Fatal("clearApplicationGlobals retained container entry")
	}
	if frame.healthCancel == nil {
		t.Fatal("clearApplicationGlobals did not stop mounted frame")
	}
}

func TestStopFrameHealthCheckIgnoresMissingStarter(t *testing.T) {
	ctx, _ := newFrameTestContext(t, nil)
	assert.NotPanics(t, func() { stopFrameHealthCheck(ctx) })
}
```

- [ ] **Step 2: Run the helper tests against the Task 2 implementation**

Run:

```bash
GOCACHE=/tmp/fiberhouse-p1a-task3 go test . -run 'Test(ClearApplicationGlobals|StopFrameHealthCheck)' -count=1
```

Expected: FAIL because the private helpers do not exist.

- [ ] **Step 3: Add the private optional interface and helpers**

Use:

```go
type healthCheckStopper interface {
	stopHealthCheck()
}

func stopFrameHealthCheck(ctx IApplicationContext) {
	if ctx == nil {
		return
	}
	starter := ctx.GetStarterApp()
	if starter == nil {
		return
	}
	frame := starter.GetFrameApp()
	if stopper, ok := frame.(healthCheckStopper); ok {
		stopper.stopHealthCheck()
	}
}

func clearApplicationGlobals(ctx IApplicationContext) {
	stopFrameHealthCheck(ctx)
	ctx.GetContainer().ClearAll(true)
}
```

- [ ] **Step 4: Replace the two built-in clear call sites**

In Fiber's existing `OnShutdown` hook, replace the direct `ClearAll(true)` with `clearApplicationGlobals(cf.GetAppContext())`, then retain logger close.

In Gin after `http.Server.Shutdown`, replace direct `ClearAll(true)` with `clearApplicationGlobals(cg.GetAppContext())`; emit `"Gin server shutdown complete"` before calling logger `Close()`, and remove the post-close log.

Do not change signal handling, shutdown timeouts, public signatures, or `Fatal` behavior in this task.

- [ ] **Step 5: Verify structural call sites and tests**

Run:

```bash
ast-grep run --pattern '$_CTX.GetContainer().ClearAll(true)' --lang go core_fiber_starter_impl.go core_gin_starter_impl.go
GOCACHE=/tmp/fiberhouse-p1a-task3 go test . -count=1
GOCACHE=/tmp/fiberhouse-p1a-task3-race go test -race . -count=1
```

Expected: ast-grep returns no matches in the two core files; both Go commands exit 0.

- [ ] **Step 6: Commit Task 3**

```bash
git add frame_starter_impl.go frame_starter_impl_test.go core_fiber_starter_impl.go core_gin_starter_impl.go
git commit -m "fix: stop keepalive before web cleanup"
```

---

### Task 4: Update capability status and P1-A execution record

**Files:**
- Modify: `docs/reference/feature-status.md`
- Modify: `.codegraph-qa-out/readme-current-status-optimization-todo.md`

**Interfaces:**
- Consumes: Tasks 1-3 verified behavior and commit SHAs.
- Produces: accurate documentation that marks only the two completed P1.3 bullets, without claiming full resource ownership or lifecycle completion.

- [ ] **Step 1: Update the GlobalManager limitation text**

Record that default keepalive now has cancel/wait and built-in Fiber/Gin stop it before deletion-only clear. Retain these limitations:

- `Get`/`Rebuild`/`Release` concurrency state machine is not closed;
- `Rebuild` does not safely retire the old instance;
- `ClearAll` deletes entries but does not close resources;
- shared aliases/composite ownership and task lifecycle remain unresolved;
- custom `FrameStarter` keepalive remains the custom implementation's responsibility.

- [ ] **Step 2: Update the P1 checklist narrowly**

Check only:

```markdown
- [x] 禁止在已使用的 `sync.Map` 上通过结构赋值实现并发清空，采用可证明安全的清理方式。
- [x] 为 keepalive 增加 context/cancel、等待退出和重复停止语义。
```

Add a dated P1-A execution record with the implementation HEAD and the exact verification commands. Keep all other P1 items unchecked and keep overall status `P1 部分执行` rather than `P1 已完成`.

- [ ] **Step 3: Verify documentation claims against the diff**

Run:

```bash
rg -n 'ClearAll|keepalive|sync.Map|P1-A|P1 部分执行' docs/reference/feature-status.md .codegraph-qa-out/readme-current-status-optimization-todo.md
git diff --check 484e300..HEAD
```

Expected: claims match Tasks 1-3; diff check exits 0.

- [ ] **Step 4: Commit Task 4**

```bash
git add docs/reference/feature-status.md .codegraph-qa-out/readme-current-status-optimization-todo.md
git commit -m "docs: record P1-A lifecycle fixes"
```

---

## Final Verification

Run from the isolated worktree:

```bash
GOCACHE=/tmp/fiberhouse-p1a-final go vet ./...
GOCACHE=/tmp/fiberhouse-p1a-final go test ./... -count=1
GOCACHE=/tmp/fiberhouse-p1a-final-race go test -race ./... -count=1
git diff --check 484e300..HEAD
git status --short
```

Expected:

- vet, normal tests, and race tests all exit 0;
- diff check exits 0;
- the isolated worktree is clean after the four task commits;
- `main` remains at `484e300` and retains its untracked analysis document;
- no merge, cherry-pick, rebase, push, or branch synchronization has occurred.
