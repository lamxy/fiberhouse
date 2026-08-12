# Cache component stabilization design

Date: 2026-08-13

## Outcome

Turn `component/cache` into a compatibility-preserving, cache-aside library whose
fast path is small and whose failure behavior is explicit. The component must:

- coalesce the complete cache-miss-to-origin-load path, not only a Redis `GET`;
- never use an uninitialized Bloom filter as authoritative evidence;
- distinguish a normal miss from infrastructure, cancellation, lifecycle, and
  serialization failures;
- prevent stale L1 values from surviving or being resurrected after L2 writes and
  deletes;
- expose constructors and lifecycle methods that return errors instead of panic or
  silently dropping asynchronous failures;
- remain source-compatible with the current `Cache`, `CacheOption`, `GetCached`,
  `Level`, `Strategy`, and configuration keys.

## Evidence from the current implementation

- `cacheremote.RedisDb.Get` owns singleflight, so concurrent callers coalesce only
  Redis reads. Every caller still executes the loader and cache fill in
  `cache_utility.GetCached`.
- Redis dynamically populates its Bloom filter from cache hits and writes. On a
  cold process it cannot prove that an origin key is absent; nevertheless it can
  return `ErrRejectedByBloomFilter` before the loader runs.
- With the circuit breaker enabled, `redis.Nil` is not normalized to the package's
  miss error and is counted as a breaker failure. Ordinary misses can open the
  circuit.
- `GetCached` treats every non-Bloom/non-breaker error as a miss. Redis timeouts,
  caller cancellation, and a closed cache can therefore fan out into origin load.
- `WriteRemoteOnly` leaves old L1 data intact. Concurrent Level2 delete and read can
  refill L1 from a remote value that is being deleted.
- `Level2Cache.Wait` waits on child backends but not on its ants queues. Redis client
  replacement races with reads and leaks the replaced client.
- Existing tests pass, including race and repeated shuffle runs, but do not cover
  these protection and interleaving contracts.

## Considered approaches

### 1. Patch Redis in place

Move the existing `singleflight.Group` a few lines upward, add a Bloom warm-up flag,
and preserve the rest. This is the smallest diff, but protection remains coupled to
one storage adapter and cannot cover L1, decoding, origin loading, or cache fill.
It also leaves public construction and error semantics unstable.

### 2. Put all protection in `Level2Cache`

This fixes two-level users, but local-only and remote-only callers still get
different semantics. It also conflicts with `GetCached`, which is the only layer
that can see the typed loader and fallback.

### 3. Add a cache-aside coordinator in `cache_utility.go` (selected)

Keep storage adapters narrow and put admission, read-through, coalescing, negative
cache, and policy decisions in a `Coordinator`. This gives every backend the same
contract and makes each concern injectable and testable. Existing entry points
delegate to this core so v1 consumers can migrate incrementally.

## Architecture

### Storage contract

`Cache` remains unchanged to preserve third-party implementations. Backends store
encoded bytes/strings and own only storage-specific behavior:

- Local: Ristretto lookup, storage, TTL, flush, and close.
- Redis: Redis commands, connection management, Redis-specific circuit breaker,
  and canonical Redis error conversion.
- Level2: L1/L2 lookup ordering, write/invalidation ordering, and async queue
  lifecycle.

Optional capabilities are discovered with separate interfaces rather than adding
methods to `Cache`:

```go
type Flusher interface { Flush(context.Context) error }
type Shutdowner interface { Shutdown(context.Context) error }
```

`Wait` and `Close` remain compatibility shims over these context-aware operations.

### Cache-aside coordinator

`cache_utility.go` exposes an explicitly constructed coordinator:

```go
type Coordinator struct { /* backend, flights, filter, policy, observer */ }

func NewCoordinator(backend Cache, opts ...CoordinatorOption) (*Coordinator, error)

func GetOrLoad[T any](
    ctx context.Context,
    coordinator *Coordinator,
    key string,
    request *CacheOption,
    loader func(context.Context) (T, error),
) (T, error)
```

The coordinator is long-lived and owns its `singleflight.Group`; `CacheOption`
remains request-scoped. The legacy generic `GetCached` resolves the configured
backend and delegates to a coordinator cached per backend instance. New code should
construct and inject a coordinator directly, avoiding service-locator panics.

The flight key includes backend identity, namespace, cache key, and requested Go
type. Waiters use `DoChan` and select on their own context. The shared operation
uses a detached, bounded context so cancellation of the first caller does not fail
all waiters. Inside the flight, the coordinator checks the cache again before
calling the loader.

### Read flow

1. Validate non-nil context, coordinator, request, key, codec, and TTL values.
2. If cache is disabled, call the loader directly.
3. If an existence filter is configured and ready, ask it about the key. A definite
   negative returns the canonical filtered-miss error. An unready filter always
   fails open. Filter infrastructure errors follow the configured fail-open or
   fail-closed policy and are observable.
4. Read the cache. Decode and return on hit.
5. Only a canonical miss enters the read-through flight. Infrastructure errors use
   the configured policy; cancellation and closed-cache errors are never misses.
6. Recheck cache inside the flight, then call the loader once.
7. A typed origin-not-found result writes a short-lived negative marker when
   negative caching is enabled. Other loader errors are returned without caching.
8. Encode and fill the cache. A fill failure is reported to the observer but, under
   the default availability policy, does not discard successfully loaded data.
9. Add a successfully loaded/written key to the filter as maintenance. This never
   changes filter readiness and therefore cannot substitute for initial preload.

### Bloom filter contract

Introduce a context-aware existence-filter contract while retaining the old
`CacheBloomFilter` as a deprecated compatibility surface:

```go
type ExistenceFilter interface {
    Ready() bool
    MayContain(context.Context, string) (bool, error)
    Add(context.Context, ...string) error
}

type PreloadableFilter interface {
    ExistenceFilter
    Preload(context.Context, []string) error
}
```

`Preload` is atomic from the coordinator's perspective: the filter becomes ready
only after the complete authoritative key set is accepted. Until then,
`MayContain` cannot reject requests.

Two implementations are supported:

- in-memory sharded Bloom for a single process, with power-of-two shard validation,
  functional reset, safe parameter validation, and explicit preload readiness;
- optional RedisBloom using `BF.RESERVE`, `BF.MADD`, and `BF.EXISTS`, injected with a
  Redis client and namespaced filter key. Missing RedisBloom commands produce a
  clear capability error; the base Redis cache does not require the module.

RedisBloom is recommended when multiple application instances must share one
authoritative filter. In-memory Bloom remains faster for process-local workloads.

### Circuit breaker and miss classification

All backends normalize absence to `ErrKeyNotFound`. `ErrRedisNil` remains available
and unwraps to `ErrKeyNotFound` for compatibility. Filter rejection and negative
cache hits are also discoverable with `errors.Is(err, ErrKeyNotFound)` while
retaining their typed `errors.As` identity.

The Redis breaker counts only Redis infrastructure failures. Cache miss,
`context.Canceled`, and `context.DeadlineExceeded` caused by the caller do not count
as dependency failures. Breakers and filters are instance/namespace scoped; no
fixed global container keys are used.

### Stampede, penetration, and avalanche controls

- Breakdown/stampede: full-path singleflight with double-checked cache reads.
- Penetration: authoritative Bloom preload plus optional typed negative caching.
- Avalanche: local and remote TTL jitter with validated non-negative ranges,
  sensible defaults, and L1 TTL constrained below L2 TTL unless explicitly
  overridden.
- Dependency outage: Redis circuit breaker plus explicit cache-error policy; an
  outage cannot silently multiply origin load without the caller choosing
  fail-open behavior.

### Level2 consistency

Synchronous writes commit remote first, then update or invalidate local. A remote
failure leaves the previously valid L1 entry untouched and returns an error.
`WriteRemoteOnly` commits remote and invalidates L1 before returning.

Deletes remove remote first and then invalidate local. A per-key generation fence
prevents an in-flight remote read from backfilling L1 after a newer set/delete has
advanced the key generation. Async strategies use detached bounded contexts, track
queued as well as running work, and expose failures through an observer. They stay
for compatibility but are documented as eventual-consistency modes.

### Lifecycle and construction

- Add constructors that return concrete types plus errors; old constructors remain
  wrappers where source compatibility requires their signatures.
- Coordinator does not own an injected backend unless `WithBackendOwnership(true)`
  is selected.
- Level2 explicitly records ownership of local and remote children.
- `Flush(ctx)` drains queued and running async work. `Shutdown(ctx)` stops admission,
  drains, then closes owned resources. Repeated shutdown is idempotent.
- Redis client replacement swaps safely, closes the old client, and is serialized
  with shutdown. New operations after shutdown return `ErrCacheClosed`.

## Compatibility and deprecation

The following remain source-compatible in this release:

- `Cache`, `CacheOption`, `GetCached`, `Factory.GetCache`;
- numeric values of `Level` and `Strategy`;
- existing configuration paths, including documented aliases.

Add non-panicking `Factory.Lookup(Level) (Cache, error)` and prefer it internally.
Deprecate, but do not remove, option pooling, `Factory.GetCache`, naked Redis
`NewClient`, and asynchronous strategies. Deprecation comments name the replacement
and no compatibility method is allowed to retain incorrect protection semantics.

## Observability

An optional observer receives low-cardinality events: L1/L2 hit and miss, filtered
request, negative hit, shared flight, loader duration/error, fill error, breaker
state, and async drop/failure. Keys and values are not emitted by default. A no-op
observer keeps the fast path allocation-free.

## Verification

Testing proceeds from contracts, not implementation details:

- deterministic concurrency tests prove N cold callers execute one loader, waiter
  cancellation is isolated, and a canceled leader does not poison other callers;
- Bloom tests cover cold fail-open, authoritative preload, valid-key lookup,
  negative rejection, reset, non-power-of-two shard input, and RedisBloom command
  errors;
- breaker tests prove misses and caller cancellation do not trip it while Redis
  failures do;
- Level2 interleaving tests prove remote-first write/delete and generation-fenced
  backfill cannot resurrect stale data;
- lifecycle tests race get/set/delete/rebuild/flush/shutdown under `go test -race`;
- compatibility tests compile and run current examples/configurations;
- hermetic Redis command tests use a test server where supported; RedisBloom command
  behavior uses a scripted command executor, while an opt-in Docker integration
  test verifies a real Redis Stack instance;
- benchmarks cover coordinator hit, miss, contended miss, local hit, remote hit,
  Bloom hit/miss, option allocation, and Level2 backfill with `-benchmem`.

Completion requires component tests, race tests, repository-wide tests, vet, and
benchmark smoke runs to pass from the isolated worktree.

## Self-review

The design contains no placeholders. Compatibility rules do not contradict the new
API: old entry points delegate to the coordinator, while optional interfaces avoid
expanding `Cache`. Bloom preload readiness and dynamic maintenance are explicitly
separate. The implementation can be delivered incrementally, but no increment may
re-enable Redis-owned singleflight or authoritative cold Bloom rejection.
