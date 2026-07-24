# example_application Swagger / OpenAPI docs

This directory holds the swaggo/swag-generated OpenAPI documentation for
`example_application`'s HTTP API (the `/examples` CRUD resource). It ships
with a **compile-safe placeholder** (`doc.go`) so the module builds and
vets cleanly without ever requiring `swag init` to have been run.

## What's here

- `doc.go` — hand-written placeholder implementing the same package/variable
  shape (`package docs`, `var SwaggerInfo *swag.Spec`, `func init()` calling
  `swag.Register`) that `swag init` itself generates. It registers an empty
  (`"paths": {}`) OpenAPI document under the `"swagger"` instance name so the
  already-wired Swagger UI handlers
  (`example_application/providers/module/fiber_route_register.go`'s
  `RegisterFiberSwagger` and `gin_route_register.go`'s `RegisterGinSwagger`,
  both gated by the `application.swagger.enable` config flag) have something
  valid to serve even before generation.
- `README.md` — this file.
- `../generate_swagger.sh` — the exact, NOT-executed `swag init` command that
  regenerates this package for real (with full paths/definitions parsed from
  the annotations below).

`swag init` **overwrites `doc.go`** with a generated file of the same shape
(same package, same `SwaggerInfo` variable) but populated `paths`/
`definitions` derived from the `@...` annotations on the handlers. That is
expected — do not hand-edit the generated output; edit the source
annotations and regenerate instead.

## How to regenerate for real

1. Install the swag CLI (not installed/run as part of this task):
   ```bash
   go install github.com/swaggo/swag/cmd/swag@latest
   ```
2. Run the generation script from the repo root:
   ```bash
   bash example_application/generate_swagger.sh
   ```
   This runs (see that file for the authoritative, exact invocation):
   ```bash
   swag init \
     -g example_main/main.go \
     -d . \
     -o example_application/docs \
     --parseDependency --parseInternal \
     --parseDepth 2
   ```
3. Commit the regenerated `doc.go` (+ `swagger.json`/`swagger.yaml` if you
   want those checked in too) once verified.

## Decisions and rationale (required reading before regenerating)

### 1. Fiber vs Gin: which adapter owns the swaggo annotations

`example_application` registers **identical routes** on both the Fiber and
Gin adapters — `POST /examples`, `GET /examples/{id}`, `GET /examples`,
`PUT /examples/{id}`, `DELETE /examples/{id}` — with no path prefix on
either side (verified in
`module/example-module/api/register_api_router.go` and
`module/example-ginapi-module/api/register_api_router.go`). swaggo/swag
parses `@Router` annotations across the whole scan tree and de-duplicates
by `method + path`; annotating both adapters with the same `@Router` value
produces a duplicate-route error/warning during `swag init`, and even if
suppressed, an ambiguous "which handler is canonical" spec.

**Decision: only the Fiber handlers
(`example_application/module/example-module/api/example_api.go`) carry
full swaggo annotations** (`@Summary/@Description/@Tags/@Accept/@Produce/
@Param/@Success/@Failure/@Router/@ID`). The Gin handlers
(`example_application/module/example-ginapi-module/api/example_api.go`)
get plain Go doc comments cross-referencing the Fiber spec, with **no**
`@Router`/swaggo tags, so `swag init` has exactly one source of truth per
path and does not collide.

Rationale for picking Fiber as the source of truth over Gin, or over
per-adapter distinct `@ID`s / distinct paths:
- Distinct `@ID` alone does not resolve the collision: swag keys `paths` by
  `method+path` in the generated spec, so two operations at the same
  `@Router /examples [post]` still collide regardless of `@ID`.
  Giving each adapter a distinct synthetic path (e.g. `/fiber/examples` vs
  `/gin/examples`) was rejected because it would misrepresent the real,
  identical routes both adapters actually serve at the app root — violating
  "do not invent routes/prefixes".
- Fiber is FiberHouse's primary/default core (`CoreTypeWithDefault` in the
  framework, and the Gin adapter is presented as an alternative/pluggable
  core throughout `example_application/providers`), so it is the more
  natural single documented contract.
- Both adapters share the same request/response VOs, the same
  `service.ExampleUseCase`, and the same `transport.MapDomainError` mapping
  — they are contractually identical at the HTTP boundary (same JSON shapes,
  same status codes, same validation rules). Documenting one is sufficient
  to describe both; this is stated explicitly in the Gin handler's doc
  comments.

**If you switch cores at build/run time to Gin-only, the generated spec
still accurately describes the wire contract**, because the two adapters
are behaviorally identical — only the annotated source file differs.

### 2. `docs/doc.go` compile-safety approach

**Decision: a minimal, hand-written placeholder package** (this `doc.go`)
mirroring the shape `swag init` itself outputs (see
`example_main/docs/docs.go` for the real generated reference), but with an
empty `"paths": {}` / `"definitions": {}` document instead of a full spec.

Alternatives considered and rejected:
- *Gate the blank import behind a build tag* (e.g. only compiled when a
  `swagger` tag is passed) — rejected because it would mean the docs
  package (and its blank import) is invisible to a normal `go build ./...`,
  which defeats the goal of having Swagger UI "just work" once wired, and
  it adds a build-tag concept not used elsewhere in this template.
- *Ship a fully hand-authored spec matching every annotation* — rejected as
  duplicated, drift-prone effort; the whole point of swaggo is that the
  spec is derived from the annotations, and hand-syncing both would be
  worse than the placeholder approach.

The placeholder satisfies `go build ./...` / `go vet ./...` today, and is
silently and completely replaced the moment someone runs
`generate_swagger.sh` for real — no manual cleanup step needed.

### 3. General annotations (`@title/@version/@host/@BasePath`) and entrypoint wiring

`example_application` has **no standalone HTTP server `main` package of its
own** — it is a library of providers/modules/handlers. The actual process
that boots an HTTP server serving `example_application`'s routes (Fiber and
Gin) and mounts the Swagger UI is `example_main/main.go`, which already
carries:
```go
_ "github.com/lamxy/fiberhouse/example_main/docs" // swagger docs
// @title XXX Service APIs
// @version 1.0
// ...
func main() { ... }
```
That file is explicitly READ-ONLY reference material generated for the
**old** (pre-CRUD-refactor) API and out of scope to modify for this task.

**Decision: do NOT add a blank import of `example_application/docs` to
`example_main/main.go`, and do NOT modify its general annotations.**
Reasons:
- `swag.Register` keys documents by `InstanceName` (default: `"swagger"`) in
  a single process-wide map (`github.com/swaggo/swag` `swagger.go`). If
  `example_main/main.go` blank-imported *both* `example_main/docs` and
  `example_application/docs`, the second package's `init()` would silently
  overwrite the first's registration for the same instance name — breaking
  the existing, working Swagger UI for the old API without any compiler or
  runtime error to signal it.
- The brief marks `example_main/main.go` read-only reference; editing it
  risks regressing already-verified behavior for an unrelated task.

**Where the general annotations must live instead:** when you build a real
standalone entrypoint for `example_application` (or repurpose
`example_main/main.go` to serve the new CRUD API instead of the old one),
put the swaggo general annotations there, e.g.:
```go
package main

import (
    _ "github.com/lamxy/fiberhouse/example_application/docs" // swagger docs
)

// @title       FiberHouse Example Application API
// @version     1.0
// @description CRUD API for the example resource (Fiber primary; Gin mirrors the same contract).
// @host        localhost:8080
// @BasePath    /
// @schemes     http https
func main() { ... }
```
and point `generate_swagger.sh`'s `-g` flag at that file instead of
`example_main/main.go`. Until such an entrypoint exists, `generate_swagger.sh`
uses `example_main/main.go` as the `-g` target purely to source the
`@title/@version/@host/@BasePath` general annotations (swag requires exactly
one `-g` entrypoint file for these); the resulting spec's `paths`/
`definitions` are still driven entirely by the `@Router`/`@Success`/...
annotations on the Fiber handlers in `example_application`, scanned via
`--parseInternal` over `-d .` (repo root).

## Response envelope reference

All success and failure responses are wrapped in the framework's standard
envelope, `response.RespInfo` (`github.com/lamxy/fiberhouse/response`):
```go
type RespInfo struct {
    Code int         `json:"code"`
    Msg  string      `json:"msg"`
    Data interface{} `json:"data"`
}
```
`fiberhouse.Response().SuccessWithData(resp).JsonWithCtx(...)` returns
`Code: 0, Msg: "ok", Data: resp`. Failures returned via
`transport.MapDomainError` (`fiber.NewError(status, msg)` /
`c.Error(err)` on the Gin side) are rendered through the framework's error
handler into the same `RespInfo` envelope with a non-zero `Code` and the
mapped HTTP status. The swaggo `@Success`/`@Failure` annotations reference
`response.RespInfo{data=...}` accordingly, with `responsevo.ExampleRespVo`
/ `responsevo.ExampleListRespVo` as the generic `data` payload for the 2xx
cases.
