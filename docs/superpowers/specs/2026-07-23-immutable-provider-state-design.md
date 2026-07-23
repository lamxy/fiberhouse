# Immutable Provider State Design

## Status

Approved for implementation.

## Context

Provider state is currently represented by the mutable `IState` interface and a
pointer-backed `State` implementation. A `State` contains an ID, a name, and a
mutex, while `Set` and `SetState` mutate the same object in place.

This design creates unnecessary aliasing:

- `Provider.Status()` returns the mutable state object held by the provider.
- A caller can change provider state without going through `Provider.SetStatus`.
- Package-level state values must be cloned before a provider can safely hold
  them.
- State comparison is indirect and only compares IDs through `stateIs`.
- A lock is required inside each state object even though the state domain is a
  fixed set of lifecycle labels.

Provider state is a closed lifecycle enum. It does not need runtime identity,
registration, mutation, or custom names.

## Goals

- Replace provider state with an immutable, typed value.
- Preserve the numeric IDs of the four existing states.
- Preserve `StatePending` as the useful zero value.
- Make state assignment and comparison explicit value operations.
- Prevent callers from mutating the state returned by `Provider.Status()`.
- Preserve the public first-call-wins behavior of `Provider.SetStatus`.
- Preserve initialization result and error caching behavior.
- Update all implementations, examples, tests, and documentation to the new
  public API.

## Non-Goals

- Supporting runtime-defined provider states or custom state names.
- Adding a state registry or parsing API.
- Making provider initialization safe for concurrent execution.
- Changing provider initialization, result caching, or error caching semantics.
- Redesigning other provider interfaces or fluent setters.
- Changing the numeric IDs or names of existing states.

## Public API

The mutable interface and struct are replaced by a defined integer type:

```go
type State uint8

const (
	StatePending State = iota
	StateLoaded
	StateSkipped
	StateFailed
)

func (s State) Id() uint8
func (s State) Name() string
```

The constants have these stable representations:

| Constant | ID | Name |
| --- | ---: | --- |
| `StatePending` | 0 | `"pending"` |
| `StateLoaded` | 1 | `"loaded"` |
| `StateSkipped` | 2 | `"skipped"` |
| `StateFailed` | 3 | `"failed"` |

`StatePending` remains the zero value. A zero-initialized `State` or
zero-initialized provider status therefore means pending, matching the existing
numeric representation.

Both methods use value receivers. `Id` converts the typed value to `uint8`.
`Name` uses a switch over the four constants. For a value outside the defined
range, `Name` returns `"unknown"`. Go callers can still explicitly convert an
arbitrary integer to `State`, so unknown values must remain deterministic even
though they are not supported lifecycle states.

`IState` is removed. The provider interface uses `State` directly:

```go
type IProvider interface {
	// Existing methods omitted.
	Status() State
	SetStatus(State) IProvider
}
```

The method name remains `Id`, rather than changing to `ID`, to limit the
breaking surface to the state representation and the signatures that must
change.

## Provider Storage

Each `Provider` stores its own state value:

```go
type Provider struct {
	// Existing fields omitted.
	status   State
	statOnce sync.Once
}
```

`NewProvider` initializes `status` to `StatePending`. This assignment is
explicit even though it is also the zero value, because it documents the
provider lifecycle invariant.

`Status` returns the stored value:

```go
func (p *Provider) Status() State {
	return p.status
}
```

Returning a value prevents external mutation and removes the need for
`cloneState`. No provider shares mutable state with another provider or with a
package-level template.

State comparisons use typed equality:

```go
if p.status == StateLoaded {
	// Return the cached initialization result.
}
```

The `stateIs` helper is removed because it adds no semantics beyond equality
once states are values.

## State Transitions

A newly created provider starts in `StatePending`.

The public setter preserves its existing first-call-wins contract:

```go
func (p *Provider) SetStatus(status State) IProvider {
	p.statOnce.Do(func() {
		p.status = status
	})
	return p
}
```

Only calls to the public `SetStatus` method participate in `statOnce`.
Subsequent public calls are ignored, as they are today.

Package-owned lifecycle methods need to record terminal outcomes independently
of the public setter. They use an unexported assignment helper that bypasses
`statOnce`:

```go
func (p *Provider) setLifecycleStatus(status State) {
	p.status = status
}
```

The helper is only called with terminal states by package code:

- `SetAndReturnSucceededInitialized` assigns `StateLoaded`.
- `SetAndReturnFailedInitialized` assigns `StateFailed`.
- A custom provider can use its first public `SetStatus(StateSkipped)` call to
  stop initialization; the manager observes `StateSkipped` before recording a
  successful result.

The supported transition behavior is:

| Source | Trigger | Result |
| --- | --- | --- |
| `StatePending` | Successful initialization | `StateLoaded` |
| `StatePending` | Failed initialization | `StateFailed` |
| `StatePending` | First public `SetStatus(StateSkipped)` | `StateSkipped` |
| Any state | Later public `SetStatus` call | No change |
| Any non-failed state | Package records initialization failure | `StateFailed` |

Package lifecycle assignments intentionally bypass the public once gate. This
preserves the existing ability to record the actual terminal outcome after a
provider has used `SetStatus` during its initialization flow.

An unsupported value created through an explicit integer conversion is not a
terminal state. `Check` therefore continues initialization for it, assuming the
provider type is set. If it reaches `ReturnDirectly`, the default branch returns
the existing unknown-state error. It is never silently mapped to a defined
state.

## Result And Error Caching

Initialization result and error behavior does not change.

`SetAndReturnSucceededInitialized`:

1. Assigns `StateLoaded` through the internal lifecycle helper.
2. Stores `initInstance`.
3. Stores `initErr`.
4. Returns the stored instance and error.

`SetAndReturnFailedInitialized`:

1. Assigns `StateFailed` through the internal lifecycle helper.
2. Stores `initInstance`.
3. Stores `initErr`.
4. Returns the stored instance and error.

`ReturnDirectly` keeps its current behavior:

| State | Return behavior |
| --- | --- |
| `StateLoaded` | Return cached `initInstance` and `initErr` |
| `StateFailed` | Return `nil` and cached `initErr` |
| `StateSkipped` | Return the existing skipped-provider error |
| `StatePending` or unknown | Return the existing unknown-state error |

This design does not add synchronization around `status`, `initInstance`, or
`initErr`. Provider initialization remains governed by the existing startup
phase, non-concurrent write contract.

## Migration

Implementation migration consists of the following scoped changes:

1. Replace the mutable `State` struct and package-level pointer variables with
   `type State uint8` and typed constants.
2. Implement `Id` and `Name` with value receivers.
3. Remove `IState` from `provider_interface.go`.
4. Change `IProvider.Status` from `Status() IState` to `Status() State`.
5. Change `IProvider.SetStatus` from `SetStatus(IState) IProvider` to
   `SetStatus(State) IProvider`.
6. Change `Provider.status` from `IState` to `State`.
7. Replace `cloneState(StatePending)` with direct `StatePending` assignment.
8. Remove `State.Set`, `State.SetState`, the state mutex, `cloneState`, and
   `stateIs`.
9. Replace state mutation with direct assignment through the internal lifecycle
   helper.
10. Replace ID-based state checks with typed equality.
11. Update examples to pass the typed constants directly.
12. Replace tests that construct mutable or custom states with enum and provider
    lifecycle tests.
13. Update API documentation and examples that mention `IState`, `Set`,
    `SetState`, or runtime custom provider states.

Existing source that calls `SetStatus(StateLoaded)` continues to use the same
expression. Source that declares an `IState`, calls `new(State).Set`, implements
`IState`, or supplies a custom ID/name pair must be rewritten because runtime
custom states are deliberately removed.

## Test Strategy

Focused unit tests cover the new value contract:

- Assert that `StatePending`, `StateLoaded`, `StateSkipped`, and `StateFailed`
  have IDs `0`, `1`, `2`, and `3`.
- Assert that each constant returns its stable lowercase name.
- Assert that `var state State` equals `StatePending`.
- Assert that an unsupported converted value returns ID unchanged and name
  `"unknown"`.
- Assert that two new providers independently start in `StatePending`.
- Set one provider to a terminal state and assert that the other remains
  pending.
- Assert that the first public `SetStatus` call wins and later public calls are
  ignored.
- Assert that successful initialization records `StateLoaded` and preserves the
  cached instance and error return behavior.
- Assert that failed initialization records `StateFailed` and preserves the
  cached error return behavior.
- Assert that skipped initialization remains `StateSkipped` and returns the
  existing skipped-provider error.
- Assert that manager-driven initialization still reuses cached successful and
  failed results.
- Keep compile-time interface assertions for provider implementations so every
  implementation is migrated to the new signatures.

Example packages and API documentation are included in the normal build or
documentation checks to catch stale `IState` references.

No new concurrent initialization test is required because concurrent provider
initialization is explicitly outside this design.

## Risks And Mitigations

### Zero Value Means Pending

A missing explicit initialization can appear as a pending provider. This is an
intentional compatibility decision: pending already has ID zero, and
`NewProvider` still assigns it explicitly to document intent.

### Loss Of Runtime Extension

Callers can no longer attach custom names to provider states. This is accepted
because the provider lifecycle is closed. Callers that need domain-specific
metadata should model it separately instead of extending the lifecycle enum.

### Explicit Conversion Can Create Unknown Values

Go enum types cannot prevent `State(255)`. `Name` returns `"unknown"`, and
provider lifecycle logic keeps treating the value as unsupported. Tests cover
this behavior so invalid values do not accidentally acquire terminal meaning.

### Breaking Interface Change

Any external `IProvider` implementation must change `Status` and `SetStatus`
signatures. Removing `IState` also breaks custom implementations directly.
This is accepted as part of the approved breaking API migration and must be
called out in release notes.

### Concurrency Remains Unchanged

Removing the state mutex may look like a concurrency regression, but that mutex
only protected mutation inside a shared state object. The new value cannot be
mutated or aliased. Concurrent reads and writes of the containing Provider
remain outside the supported contract, as they are for the provider's cached
instance and error fields.
