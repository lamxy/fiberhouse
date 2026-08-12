# Cache Stabilization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a source-compatible, production-safe cache-aside coordinator with correct singleflight, Bloom readiness, Redis failure handling, two-level consistency, lifecycle, documentation, and performance evidence.

**Architecture:** Keep `Cache` as the storage compatibility boundary. Add a long-lived `Coordinator` in `cache_utility.go` that owns read-through policy and full-path request coalescing, with separately injectable existence filters and observers. Simplify Redis to a concurrency-safe storage adapter and make Level2 enforce remote-first mutation plus generation-fenced L1 backfill.

**Tech Stack:** Go 1.25, `golang.org/x/sync/singleflight`, Ristretto v2, go-redis v9, bits-and-blooms/bloom v3, gobreaker v2, standard `testing`/`httptest` patterns, optional Redis Stack live integration.

## Global Constraints

- Preserve the existing `Cache` method set and `Level`/`Strategy` numeric values.
- Preserve `CacheOption`, `GetCached`, existing constructors, error types, and consumed configuration keys as compatibility entry points.
- Never reject through Bloom until an authoritative preload has completed.
- Never place singleflight in a storage adapter; it must cover cache recheck, loader, encoding, and fill in `cache_utility.go`.
- Only canonical cache misses may invoke the loader by default; cancellation, close, codec, and infrastructure errors remain distinct.
- RedisBloom is optional and must not become a runtime requirement for plain Redis users.
- Use TDD for every behavior change and run Go tests with `GOCACHE=/tmp/fiberhouse-cache-stabilization-build` in this sandbox.
- Prefix high-noise shell commands with `rtk` as required by repository instructions.

---

## File structure

- `component/cache/cache_errors.go`: canonical error identities and compatibility wrappers.
- `component/cache/cache_option.go`: request validation, failure policy, negative TTL, and bounded shared-load timeout.
- `component/cache/cache_utility.go`: coordinator construction, legacy delegation, and typed `GetOrLoad` flow.
- `component/cache/cache_utility_test.go`: deterministic cache-aside and concurrency contracts.
- `component/cache/filter.go`: new context-aware existence-filter interfaces and in-memory adapter.
- `component/cache/filter_test.go`: readiness, preload, reset, and shard validation contracts.
- `component/cache/helper.go`: legacy Bloom and breaker compatibility implementation fixes.
- `component/cache/cacheremote/redis_bloom.go`: optional RedisBloom implementation.
- `component/cache/cacheremote/redis_bloom_test.go`: scripted RedisBloom command contract.
- `component/cache/cacheremote/redis_cache.go`: storage-only Redis adapter, canonical errors, instance breaker, safe client lifecycle.
- `component/cache/cacheremote/redis_cache_test.go`: hermetic Redis command, breaker, rebuild, and close tests.
- `component/cache/cache2/level2_cache.go`: remote-first consistency, generation fence, drain/shutdown.
- `component/cache/cache2/level2_cache_test.go`: interleaving and lifecycle tests.
- `component/cache/cache_benchmark_test.go`: stable benchmark entry points.
- `docs/guides/cache.md`: public construction, policies, migration, RedisBloom, and operational guidance.
- `example_config/application_dev.yml`, `application_test.yml`, `application_prod.yml`: canonical protection and jitter examples while retaining aliases.

---

### Task 1: Canonical errors and request policy

**Files:**
- Modify: `component/cache/cache_errors.go`
- Modify: `component/cache/cache_option.go`
- Modify: `component/cache/cache_option_test.go`

**Interfaces:**
- Produces: `IsMiss(error) bool`, `ErrOriginNotFound`, `CacheErrorPolicy`, `SetCacheErrorPolicy`, `SetNegativeTTL`, `SetLoadTimeout`, and their getters.
- Consumes: existing `ErrKeyNotFound`, typed Redis/Bloom/breaker errors, and `CacheOption.Clone/Reset/Valid`.

- [ ] **Step 1: Write failing canonical-error tests**

Add table tests proving `ErrRedisNil` and `ErrRejectedByBloomFilter` match
`ErrKeyNotFound`, while `ErrCacheClosed`, `context.Canceled`, and a generic network
error do not:

```go
func TestIsMiss(t *testing.T) {
    tests := []struct{ err error; want bool }{
        {ErrKeyNotFound, true},
        {NewErrRedisNil("k"), true},
        {NewErrRejectedByBloomFilter("k"), true},
        {ErrCacheClosed, false},
        {context.Canceled, false},
        {errors.New("network down"), false},
    }
    for _, tt := range tests {
        if got := IsMiss(tt.err); got != tt.want { t.Fatalf("IsMiss(%v)=%v", tt.err, got) }
    }
}
```

- [ ] **Step 2: Run the focused tests and confirm they fail to compile**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache -run 'TestIsMiss|TestCacheOption_Policy'`

Expected: FAIL because `IsMiss` and policy methods do not exist.

- [ ] **Step 3: Implement error wrapping and immutable policy defaults**

Implement `Is/Unwrap` on compatibility errors so `errors.Is(err,
ErrKeyNotFound)` works. Add:

```go
type CacheErrorPolicy uint8
const (
    ReturnCacheError CacheErrorPolicy = iota + 1
    LoadOnCacheError
)

var ErrOriginNotFound = errors.New("cache: origin value not found")

func IsMiss(err error) bool { return errors.Is(err, ErrKeyNotFound) }
```

Add `cacheErrorPolicy`, `negativeTTL`, and `loadTimeout` fields to `CacheOption`.
Default to `ReturnCacheError`, zero negative TTL, and 30 seconds load timeout. Clone
and Reset must copy/reset every field. `Valid` rejects nil request context, negative
TTLs, non-positive load timeout, and an L1 base TTL greater than L2 base TTL when
both are set.

- [ ] **Step 4: Add policy, clone, pool-reset, and validation tests**

Verify a pooled option cannot retain the previous request's failure policy,
negative TTL, or timeout. Verify `SetContextCtx(nil)` fails validation and
`SetLoadTimeout(0)` fails validation.

- [ ] **Step 5: Run package tests**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add component/cache/cache_errors.go component/cache/cache_option.go component/cache/cache_option_test.go
git commit -m "fix(cache): define canonical miss and request policies"
```

---

### Task 2: Full-path cache-aside coordinator

**Files:**
- Modify: `component/cache/cache_utility.go`
- Create: `component/cache/cache_utility_test.go`

**Interfaces:**
- Consumes: `Cache`, `CacheOption`, `IsMiss`, `CacheErrorPolicy`, and existing codec methods.
- Produces: `Coordinator`, `CoordinatorOption`, `NewCoordinator`, `WithObserver`, `Observer`, `Event`, and `GetOrLoad[T]`.

- [ ] **Step 1: Create a deterministic fake backend and failing coalescing test**

The fake must count gets/sets, store strings under a mutex, and return
`ErrKeyNotFound` on absence. Start 32 goroutines on one key, hold the loader behind
a channel, and assert loader count is exactly one after all callers return.

```go
coord, err := NewCoordinator(fake)
if err != nil { t.Fatal(err) }
var loads atomic.Int32
loader := func(context.Context) (payload, error) {
    loads.Add(1)
    <-release
    return payload{ID: 7}, nil
}
```

- [ ] **Step 2: Run the coalescing test and confirm compile failure**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache -run TestCoordinator_CoalescesCompleteLoad -count=1`

Expected: FAIL because coordinator APIs do not exist.

- [ ] **Step 3: Implement coordinator construction and fast hit path**

Use one `singleflight.Group` per coordinator. Reject nil backends. Add a no-op
observer and event enum without maps or variadic fields on the fast path. Implement
cache hit decode before entering singleflight.

- [ ] **Step 4: Implement miss-only `DoChan` read-through with double check**

Build a flight key from namespace, key, and `reflect.TypeFor[T]().String()`. Select
between the result channel and each waiter's context. Inside the flight use
`context.WithTimeout(context.WithoutCancel(ctx), request.GetLoadTimeout())`, re-read
the cache, call the loader, encode, and fill. A fill error emits an event and returns
the loaded value.

- [ ] **Step 5: Add failure-policy and context tests**

Prove:

- network errors do not invoke loader under `ReturnCacheError`;
- `LoadOnCacheError` invokes one loader;
- a canceled waiter returns promptly without canceling the shared loader;
- cancellation of the first caller does not poison a second waiter;
- distinct requested Go types do not share one flight;
- corrupt cached JSON returns a codec error and is not treated as a miss.

- [ ] **Step 6: Make legacy `GetCached` delegate to coordinators cached per backend**

Use a package `sync.Map` keyed by stable backend identity and preserve the existing
fallback behavior for `ErrCircuitBreakerOpen`. Add a non-panicking
`Factory.Lookup(Level) (Cache, error)` and make `GetCache` call it then panic only as
the deprecated compatibility behavior.

- [ ] **Step 7: Run focused and package tests under race**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache -run 'TestCoordinator|TestGetCached|TestFactory'`

Expected: PASS with loader count one.

- [ ] **Step 8: Commit**

```bash
git add component/cache/cache_utility.go component/cache/cache_utility_test.go
git commit -m "feat(cache): coordinate full cache-aside load path"
```

---

### Task 3: Authoritative in-memory Bloom filter

**Files:**
- Create: `component/cache/filter.go`
- Create: `component/cache/filter_test.go`
- Modify: `component/cache/helper.go`
- Modify: `component/cache/helper_test.go`
- Modify: `component/cache/cache_utility.go`

**Interfaces:**
- Consumes: coordinator options/events from Task 2 and legacy `CacheBloomFilter`.
- Produces: `ExistenceFilter`, `PreloadableFilter`, `MemoryBloomFilter`, `NewMemoryBloomFilter`, `WithExistenceFilter`, and `WithFilterFailureMode`.

- [ ] **Step 1: Write failing readiness and preload tests**

Assert an unready filter returns `MayContain=true` for every key (fail open), a
successful preload marks it ready, preloaded keys may exist, absent keys are
rejected, and `Reset` returns it to unready state. Test invalid capacity/error rate
and shard counts 3, 0, and 17.

- [ ] **Step 2: Run the filter tests and confirm compile failure**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache -run 'TestMemoryBloom|TestShardedBloom'`

Expected: FAIL because the new filter APIs do not exist and legacy Reset is empty.

- [ ] **Step 3: Implement filter contracts and in-memory adapter**

`Preload` builds a new complete set off to the side and swaps it under a mutex only
after every key is accepted. `Add` updates the current filter but never marks it
ready. Round shard count up to the next power of two or reject it consistently;
tests must lock the chosen behavior. Replace legacy `ShardedBloomFilter.Reset` with
fresh per-shard filters using stored capacity/error-rate parameters.

- [ ] **Step 4: Integrate the readiness gate into coordinator flow**

Only consult the filter before cache lookup when `request.GetBloomFilterState()` is
true. `Ready=false` bypasses rejection. After loader success and successful cache
fill, call `Add`; an Add error is observed but does not discard loaded data. A
definite negative returns `ErrRejectedByBloomFilter` without cache or loader calls.

- [ ] **Step 5: Add coordinator Bloom tests**

Prove cold valid keys reach loader, preloaded valid keys reach cache/loader, absent
keys are rejected only after preload, and filter errors obey fail-open/fail-closed
configuration.

- [ ] **Step 6: Run package race tests**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add component/cache/filter.go component/cache/filter_test.go component/cache/helper.go component/cache/helper_test.go component/cache/cache_utility.go
git commit -m "feat(cache): require authoritative Bloom preload"
```

---

### Task 4: Optional RedisBloom filter

**Files:**
- Create: `component/cache/cacheremote/redis_bloom.go`
- Create: `component/cache/cacheremote/redis_bloom_test.go`

**Interfaces:**
- Consumes: `cache.ExistenceFilter`, `cache.PreloadableFilter`, and go-redis `Cmder` results.
- Produces: `RedisBloomConfig`, `NewRedisBloomFilter`, `Prepare`, `Preload`, `Ready`, `MayContain`, `Add`, and `ErrRedisBloomUnavailable`.

- [ ] **Step 1: Write a scripted command-executor test**

Define a narrow internal executor:

```go
type commandExecutor interface {
    Do(context.Context, ...interface{}) *redis.Cmd
}
```

Record commands and return scripted responses. Assert `Prepare` issues
`BF.RESERVE`, preload batches use `BF.MADD`, lookup uses `BF.EXISTS`, and a Redis
unknown-command error maps to `ErrRedisBloomUnavailable`.

- [ ] **Step 2: Run and confirm compile failure**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache/cacheremote -run TestRedisBloom`

Expected: FAIL because the implementation is absent.

- [ ] **Step 3: Implement validated, namespaced RedisBloom operations**

Require non-empty filter key, capacity greater than zero, and error rate in `(0,1)`.
Treat BUSYKEY on reserve as an existing filter only when explicitly allowed by
config. Batch preload to a bounded size. Flip local readiness only after all
batches succeed; `Add` never flips readiness.

- [ ] **Step 4: Add cancellation, partial preload, and interface assertions**

Verify cancellation stops batching, partial failure leaves `Ready=false`, and:

```go
var _ cache.PreloadableFilter = (*RedisBloomFilter)(nil)
```

- [ ] **Step 5: Run cacheremote tests**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache/cacheremote`

Expected: PASS without Docker or RedisBloom installed.

- [ ] **Step 6: Commit**

```bash
git add component/cache/cacheremote/redis_bloom.go component/cache/cacheremote/redis_bloom_test.go
git commit -m "feat(cache): add optional RedisBloom existence filter"
```

---

### Task 5: Redis storage, breaker, and lifecycle correctness

**Files:**
- Modify: `component/cache/cacheremote/redis_cache.go`
- Modify: `component/cache/cacheremote/redis_cache_test.go`
- Modify: `component/cache/helper.go`
- Modify: `component/cache/helper_test.go`

**Interfaces:**
- Consumes: canonical `IsMiss`, existing `CacheCircuitBreaker`, and coordinator-owned protection.
- Produces: storage-only `RedisDb.Get/Set/Delete`, safe client snapshot/swap, idempotent close, and breaker classification.

- [ ] **Step 1: Write regression tests for Redis ownership and miss conversion**

Construct `RedisDb` with a scripted or test Redis client. Assert the struct no
longer owns singleflight or Bloom behavior, `redis.Nil` always maps to an error that
matches `ErrKeyNotFound`, and five misses do not open the breaker.

- [ ] **Step 2: Write concurrent rebuild/close regression tests**

Race `GetRedisClient`, `PingTry`, `Get`, `ReNewClient`, and `Close`; assert the old
client is closed after swap, repeated Close returns nil, and post-close operations
return `ErrCacheClosed`.

- [ ] **Step 3: Run focused race tests and confirm failure**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache/cacheremote -run 'TestRedisDb_(Miss|Rebuild|Concurrent|Close)'`

Expected: FAIL on current error identity/lifecycle behavior or race detector.

- [ ] **Step 4: Remove Redis-owned singleflight and Bloom registration**

Delete the Redis `sf` and `bloomFilter` fields and every dynamic add/test branch.
Protection configuration moves to coordinator construction. Keep old configuration
keys accepted by the application wiring for migration warnings, but Redis storage
must not use them.

- [ ] **Step 5: Make circuit breaker miss-aware**

Normalize `redis.Nil` before breaker accounting or return it as a successful
classified outcome outside `Execute`. Do not count caller cancellation. Make
`CircuitBreakerWrap.Allow` reflect actual state and make Reset reconstruct the
breaker from stored settings rather than remain a no-op.

- [ ] **Step 6: Synchronize client lifecycle**

Keep the exported `Client` field for compatibility but access it internally through
a lock-protected snapshot. Serialize swap/close, close the replaced client, and
reject rebuild after shutdown. Avoid holding a lock across network I/O.

- [ ] **Step 7: Run cacheremote and cache tests under race**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache/...`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add component/cache/cacheremote/redis_cache.go component/cache/cacheremote/redis_cache_test.go component/cache/helper.go component/cache/helper_test.go
git commit -m "fix(cache): make Redis a safe storage adapter"
```

---

### Task 6: Level2 consistency fence and drainable lifecycle

**Files:**
- Modify: `component/cache/cache_interface.go`
- Modify: `component/cache/cache2/level2_cache.go`
- Modify: `component/cache/cache2/level2_cache_test.go`

**Interfaces:**
- Consumes: unchanged `Cache` plus optional `Flusher`/`Shutdowner` capabilities.
- Produces: `NewLevel2CacheWithError`, `Flush(context.Context)`, `Shutdown(context.Context)`, remote-first mutation, and per-key generation fencing.

- [ ] **Step 1: Write remote-first write/delete tests**

Use controllable fake caches. Prove remote failure prevents local update/invalidate,
`WriteRemoteOnly` invalidates L1 after remote success, and Delete does not touch L1
until remote delete succeeds.

- [ ] **Step 2: Write stale-backfill interleaving test**

Pause remote Get after it reads old data, complete Delete or Set, then release the
Get. Assert the old value is not written to local after the key generation changes.

- [ ] **Step 3: Write flush/shutdown tests**

Queue async writes behind a gate. Assert `Flush(ctx)` waits for queued and running
work, deadline cancellation returns `ctx.Err()`, `Shutdown` is idempotent, and no
new task is admitted once shutdown begins.

- [ ] **Step 4: Run focused tests and confirm failures**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache/cache2 -run 'TestLevel2Cache_(RemoteFirst|NoStaleBackfill|Flush|Shutdown)'`

Expected: FAIL against parallel mutation and current pool polling.

- [ ] **Step 5: Implement a generation fence**

Maintain a bounded/sharded map of per-key generations. Capture generation before
remote read; only backfill local if unchanged. Increment before a mutating operation
becomes visible and finalize ordering so a failed remote mutation preserves the
old readable generation. Tests determine the exact increment/rollback protocol.

- [ ] **Step 6: Implement remote-first synchronous strategies**

For `WriteBoth`, set remote then local. For `WriteRemoteOnly`, set remote then delete
local. For Delete, delete remote then local. Return `errors.Join` with wrapped child
errors where multiple cleanup operations can fail.

- [ ] **Step 7: Track async admission and completion explicitly**

Add a task wait group incremented before pool submission and decremented on submit
failure or task completion. Detach request cancellation and apply configured
timeouts. `Flush(ctx)` waits on a completion channel; `Shutdown(ctx)` closes
admission, flushes, releases pools, and closes only owned children.

- [ ] **Step 8: Add an error-returning constructor**

`NewLevel2CacheWithError(...) (*Level2Cache, error)` returns pool construction and
configuration errors. Existing `NewLevel2Cache` delegates and preserves its
`cache.Cache` return, returning nil only as the deprecated compatibility behavior.

- [ ] **Step 9: Run Level2 and all cache race tests**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache/...`

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add component/cache/cache_interface.go component/cache/cache2/level2_cache.go component/cache/cache2/level2_cache_test.go
git commit -m "fix(cache): enforce Level2 mutation consistency"
```

---

### Task 7: Negative cache and avalanche defaults

**Files:**
- Modify: `component/cache/cache_utility.go`
- Modify: `component/cache/cache_utility_test.go`
- Modify: `component/cache/cache_option.go`
- Modify: `component/cache/cache_option_test.go`

**Interfaces:**
- Consumes: `ErrOriginNotFound`, coordinator fill path, and request TTL policy.
- Produces: versioned negative marker handling and validated TTL jitter behavior.

- [ ] **Step 1: Write negative-cache behavior tests**

Return `ErrOriginNotFound` from a loader with `negativeTTL > 0`. Assert the first
call returns the typed origin error and writes a versioned marker, the second call
does not invoke loader, marker expiry permits a new load, and normal payload equal
to a human-readable sentinel cannot collide.

- [ ] **Step 2: Write TTL jitter boundary tests**

Assert random ranges never produce zero/negative TTL, negative base/range values
fail validation, and local base TTL above remote base TTL fails unless an explicit
override method is set.

- [ ] **Step 3: Run focused tests and confirm failure**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache -run 'TestCoordinator_Negative|TestCacheOption_TTLValidation'`

Expected: FAIL before marker and validation implementation.

- [ ] **Step 4: Implement a binary-safe versioned marker**

Use an internal prefix containing a NUL byte plus version and kind. Detect it before
JSON decode. Clone the request and set the negative TTL on both relevant layers for
the marker write, then release the clone. Only `ErrOriginNotFound` is negative
cached; arbitrary loader failures are never cached.

- [ ] **Step 5: Make jitter validation deterministic and safe**

Validate base/range before calling `rand.Int63n`, avoid overflow when doubling the
range, and use a coordinator-injected random source in tests. Preserve existing
fixed TTL behavior.

- [ ] **Step 6: Run component race tests**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache/...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add component/cache/cache_utility.go component/cache/cache_utility_test.go component/cache/cache_option.go component/cache/cache_option_test.go
git commit -m "feat(cache): add typed negative cache and safe jitter"
```

---

### Task 8: Public documentation, examples, and compatibility tests

**Files:**
- Create: `docs/guides/cache.md`
- Modify: `README.md`
- Modify: `example_config/application_dev.yml`
- Modify: `example_config/application_test.yml`
- Modify: `example_config/application_prod.yml`
- Modify: `example_application/module/example-module/service/example_service.go`
- Create: `component/cache/compatibility_test.go`

**Interfaces:**
- Consumes: all public APIs from Tasks 1-7.
- Produces: documented coordinator construction and migration path with compiling examples.

- [ ] **Step 1: Add compile-time compatibility tests**

Assert a legacy fake still satisfies `Cache`, old option chaining compiles, existing
`GetCached` works, and `Level`/`Strategy` constants retain values 1-3 and 1-4.

- [ ] **Step 2: Update the example service to injected coordinator usage**

Keep one legacy test/example, but make the primary application path call
`GetOrLoad` with a request context rather than `context.Background()`. Do not pool
options across asynchronous ownership boundaries.

- [ ] **Step 3: Write the cache guide**

Document:

- backend versus coordinator ownership;
- miss/error policy and fallback semantics;
- singleflight context behavior;
- authoritative Bloom preload and why dynamic learning is not readiness;
- when to choose in-memory Bloom versus RedisBloom;
- negative caching and TTL jitter;
- Level2 synchronous/eventual strategies;
- Flush/Shutdown and migration from every deprecated API.

Include runnable Go snippets with exact constructors and errors.

- [ ] **Step 4: Align configuration examples**

Keep current keys accepted. Mark legacy Redis-owned `protection` keys deprecated
and show coordinator namespace/filter/breaker/jitter configuration. Fix documented
aliases such as `ignoreInternalCost` and idle-time names without silently changing
old behavior.

- [ ] **Step 5: Run compatibility and repository tests**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add docs/guides/cache.md README.md example_config component/cache/compatibility_test.go example_application/module/example-module/service/example_service.go
git commit -m "docs(cache): publish stable cache API and migration guide"
```

---

### Task 9: Benchmarks and completion audit

**Files:**
- Create: `component/cache/cache_benchmark_test.go`
- Modify: `docs/guides/cache.md`

**Interfaces:**
- Consumes: completed coordinator, filters, local, Redis, and Level2 APIs.
- Produces: repeatable benchmark names and recorded methodology, not machine-specific performance promises.

- [ ] **Step 1: Add allocation-aware benchmark entry points**

Create benchmarks named:

```go
BenchmarkCoordinatorHit
BenchmarkCoordinatorMiss
BenchmarkCoordinatorContendedMiss
BenchmarkMemoryBloomMayContain
BenchmarkLocalCacheHit
BenchmarkLevel2LocalHit
BenchmarkLevel2RemoteHitBackfill
BenchmarkCacheOptionPool
```

Use `b.ReportAllocs()`, fixed payload sizes, prewarming outside timed sections, and
`RunParallel` for contended cases.

- [ ] **Step 2: Run benchmark smoke tests**

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache/... -run '^$' -bench 'Benchmark(Coordinator|MemoryBloom|LocalCache|Level2|CacheOption)' -benchtime=100x -benchmem`

Expected: every benchmark executes and reports ns/op plus allocs/op.

- [ ] **Step 3: Run formatting and static analysis**

Run: `rtk gofmt -w component/cache example_application/module/example-module/service/example_service.go`

Run: `GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go vet ./...`

Expected: no findings.

- [ ] **Step 4: Run focused, race, repeated, and full verification**

Run:

```bash
GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./component/cache/...
GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -race ./component/cache/...
GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test -shuffle=on -count=20 ./component/cache/...
GOCACHE=/tmp/fiberhouse-cache-stabilization-build rtk go test ./...
```

Expected: all commands pass. Live Redis/RedisBloom tests remain opt-in and clearly
report their prerequisites.

- [ ] **Step 5: Audit the original objective requirement by requirement**

Record evidence in the final handoff for: singleflight location and loader count,
Bloom preload behavior and RedisBloom option, penetration/breakdown/avalanche
controls, L1/L2 consistency, stable API compatibility, lifecycle/race safety,
documentation, and performance smoke results.

- [ ] **Step 6: Commit**

```bash
git add component/cache/cache_benchmark_test.go docs/guides/cache.md
git commit -m "test(cache): add performance and completion gates"
```

## Plan self-review

- Every design requirement maps to at least one task: coordinator and singleflight
  (Task 2), Bloom/RedisBloom (Tasks 3-4), canonical errors and breaker (Tasks 1/5),
  Level2 consistency/lifecycle (Task 6), penetration/avalanche (Task 7), public API
  and migration (Task 8), and performance/full verification (Task 9).
- All produced interfaces are named before downstream consumption. `Cache` remains
  unchanged and optional lifecycle/filter capabilities are separate interfaces.
- There are no deferred placeholders; each behavior-changing task has an explicit
  failing test, failure command, implementation contract, passing command, and
  commit boundary.
