# GlobalManager stability promotion — audit record

Date: 2026-09-08

Branch: `stability/globalmanager-promotion`

Companion to
[the design record](2026-09-08-globalmanager-stability-promotion-design.md).
Three independent audits ran during this work. Each was dispatched to a separate
agent with no access to the reasoning that produced the code under review, and
each was told to report findings with `file:line` evidence and to refuse to
speculate. This file records what they found and what was done about it, so the
promotion decision can be re-examined later without re-deriving it.

Every finding below was independently reproduced before being acted on. Two of
the three audits returned `needs-revision` against work that looked finished.

## Audit 1 — shutdown fix design (before implementation)

**Verdict: `needs-revision`.** The defect was real; the proposed fix was not.

The plan was to call `ReleaseAll(true)` before `ClearAll` in
`clearApplicationGlobals`. The audit showed this introduces a use-after-close.

| Finding | Severity | Disposition |
|---|---|---|
| `ReleaseAll` had zero production callers; the shutdown path only calls `ClearAll`, which never closes. Confirmed leak. | critical | Confirmed. Fixed in `b42c3a5`. |
| Aliased + composite resources make a naive sweep unsafe: `KEY_REMOTE_CACHE` returns the same `*RedisDb` as `KEY_REDIS`, and `Level2Cache` owns both children and drains before closing them. `sync.Map.Range` order is unspecified, so Redis can be closed while L2 is still draining. | critical | Confirmed by reading `application_impl.go:94-107` and `level2_cache.go:512-524`. Drove the `CompositeClosable` design. |
| `MongoDb.Close` had no idempotency guard, unlike Redis and LocalCache. | major | Confirmed. Guard added; `TestLive_MongoDb_CloseIsIdempotent` fails without it with `client is disconnected`. |
| `ReleaseAll` reported failures via `fmt.Printf` — invisible to log aggregation in containers. | major | Confirmed. Added `ReleaseAllWithObserver`; the framework injects a logger callback. |
| A fourth shutdown path (Hertz) never calls `clearApplicationGlobals`. | major | Confirmed. Scoped out and recorded — see the design record. |

Options offered for ordering were (a) idempotency guards only, (b) explicit
ordering, (c) container dependency metadata. **(b) was chosen.** (a) leaves the
drain-window race — the exact defect being fixed. (c) is a container redesign
disproportionate to promotion.

## Audit 2 — full branch cross-audit

**Verdict: `needs-revision`.** Found a defect in the fix from audit 1.

| Finding | Severity | Disposition |
|---|---|---|
| Two-pass ordering handles composite-vs-plain but not composite-vs-composite. For A owns B owns C the owner counts are 0/1/1; B and C tie and fall back to `sync.Map.Range`. | major | **Confirmed by direct probe** — the partition reproducibly returned `[A C B]`. Replaced owner-count with transitive depth in `a5ff379`. The existing nested test had missed it because A's `Close` cascades through B and C, masking the wrong order; the fix was to assert the partition order directly. |
| `Get` returns a typed-nil instance alongside the error when an initializer is `return NewXWithError(ctx)`. | major | **Confirmed by test** — the regression test failed against the then-current code. Fixed in `a5ff379`. |
| `ClearAll`'s doc comment still claimed it clears "全部对象和资源". | major | Confirmed. Comment corrected; the delete-only semantics are deliberate and now explained. |
| `KEY_LEVEL2_CACHE` registered but never instantiated, so the example never exercised the composite ordering. Condition 5 not met. | major | Confirmed by grep. Added to `ConfigRequiredGlobalKeys`; `InitializeGlobalRequired` forces construction via `Get`. |
| CLI has no shutdown path at all — `commandstarter/` never releases globals. | major | Confirmed. Belongs to the CLI capability, not the three being promoted. Recorded in its `feature-status.md` row and the design record. |
| `MysqlDb.Close` has no guard, unlike its peers. | minor | Confirmed, but correct: `sql.DB.Close` is documented idempotent. Documented the reliance and added `TestLive_MysqlDb_CloseIsIdempotent` to verify the external contract rather than assume it. |
| `feature-status.md` not updated; rows carried claims this branch had already falsified. | major | Confirmed. Fixed in `a1ef467`. |
| Model constructors still panic in the example. | minor | Partly addressed: `RegisterKeyExampleModel` uses the error variant. The wire providers keep the panicking form because wire cannot consume an error-returning provider. Stated in the row. |
| `Release` holds the maintenance gate but not `entry.mu` while calling `Close`, so a concurrent `Get` can return an instance being closed. | minor | Pre-existing; the full `Get`/`Rebuild`/`Release` state machine is explicitly listed as unclosed in the promoted row. Not addressed. |
| Ordering/nil-guard/deadlock concerns on the new interfaces. | minor | Audited as non-issues with evidence. No change. |

## Audit 3 — final acceptance

**Verdict: `reject`**, with three blockers. All three were confirmed and fixed.

Gates it ran itself: `go build ./...`, `go vet ./...`, and
`go test -race ./... -count=1` (819 passed, 78 packages, exit 0).

| Blocker | Disposition |
|---|---|
| **The typed-nil fix was only half done.** `manager.go:185` checked `err == nil && instance == nil`, which catches untyped nil only. An initializer returning `((*T)(nil), nil)` published a typed-nil that was then cached forever and returned by every subsequent `Get`. Reachable from `RegisterKeyExampleModel`, the exact pattern the earlier fix was written for. | **Confirmed by probe** — `first Get: inst=(*tnRes)(nil) err=<nil>`. Fixed with `isNilInstance` using reflection over pointer/interface/map/slice/chan/func kinds. Two tests now pin both the success and failure paths. |
| **The capability's primary guide contradicted the promotion.** `docs/guides/global-manager.md` still documented `Release` as not resetting non-`Closable`, `ReleaseAll` as printing to stdout, the shutdown path as calling `ClearAll` rather than `ReleaseAll`, and closed with "因此……将 GlobalManager 归为实验性生命周期能力" — the direct opposite of the promotion. Condition 5 requires documentation closure. | Confirmed. Guide rewritten: the operations table, the `Release` semantics paragraph, the shutdown-sequence paragraph, the known-limits list, and the closing statement. Added the `errors.Is(err, ErrMaintenanceInProgress)` retry example the text promised but never showed. |
| **The cited evidence trail did not exist for anyone cloning the repo.** `feature-status.md` linked the design record under `docs/superpowers/`, which `.gitignore:29` excludes. | Confirmed. Both the design record and this audit record are now force-added and tracked. |

Non-blocking items it raised, and what was done:

- Three more docs carried the same stale "only `ClearAll`, never `Close`" claim
  (`docs/guides/database.md`, `docs/guides/web-runtime.md`,
  `docs/reference/examples.md`) plus `docs/concepts/context-and-locators.md`.
  All four corrected, each stating that release happens on the Fiber/Gin paths
  and that CLI and hertzcore do not.
- `docs/reference/known-test-failures.md` lists six tests as failing; all of
  them now pass (verified by running `./bootstrap/` and
  `./component/logging/writer/`). Left unchanged: it predates this branch, and
  correcting it responsibly means finding out *why* they now pass, which is not
  this branch's scope.
- `Rebuild` does not clear `initErr` when storing a new instance. Harmless
  today because it requires an already-initialized entry. Not addressed.

Fixing the typed-nil guard changed which layer rejects a typed-nil, so two
existing tests in `example_application/module/task_impl_test.go` that asserted
the downstream message ("invalid redis instance", "assertion failure") were
updated to assert the container-level one. Their contract is unchanged: an
error is returned and the dispatcher is nil. Rejecting at the container boundary
is the better outcome — it stops a typed-nil from ever being cached.

## Outcome

After these fixes: `go vet` clean, `go test -race ./... -count=1` 820 passed /
78 packages / exit 0, and `go test -race -tags=liveintegration ./... -run
TestLive_` 11 passed / exit 0 against live redis, mongo and mysql containers
matching the CI service definitions.

## Method note

The two `needs-revision` verdicts both landed on work that had already passed
`go vet`, the full race suite, and the live suite. Green tests did not catch
either defect: the nested-composite bug was masked by a correct cascade, and the
typed-nil bug sat on a path no existing test exercised. In both cases the
decisive step was building a probe that **fails against the current code** before
accepting the finding — reasoning alone had produced a wrong conclusion about
the typed-nil path earlier in the session.

Every fix on this branch carries that evidence in its commit message: the exact
failure observed when the fix is disabled.
