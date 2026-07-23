# Example Application Production Template Design

## Goal

Turn `example_application/module/*-module` into a small but coherent reference
application that demonstrates production-oriented boundaries, complete CRUD,
Fiber/Gin service reuse, MySQL/MongoDB persistence, Redis-backed cross-cutting
behavior, asynchronous tasks, and a readable CLI.

The result remains an example, not a general-purpose application framework.
It should teach a new maintainer where code belongs and why each layer exists.

## Current Problems

- The MongoDB flow exposes partial CRUD and mixes transport DTO mapping,
  persistence error translation, background contexts, cache setup, and task
  dispatch across inconsistent layers.
- MongoDB model methods sometimes ignore the caller context, project away
  fields unexpectedly, defer cursor cleanup before checking the cursor error,
  and use update-as-upsert for a normal update operation.
- The MySQL CLI models unrelated `User` and `Class` tables, performs business
  logic directly against the model, and exposes a `test-orm` command whose
  flags do not describe a real user workflow.
- The Gin API duplicates Fiber handler behavior while the intended architectural
  lesson is service-layer reuse.
- There are effectively no focused tests around module behavior, so the example
  cannot safely evolve or serve as a trustworthy template.
- Comments, names, routes, and response shapes use a mixture of test terminology,
  implementation detail, and obsolete CRUD spelling.

## Considered Approaches

### 1. Keep the existing domain and patch individual defects

This is the smallest diff, but it preserves unrelated MySQL entities, ambiguous
command behavior, and duplicated transport logic. It would improve correctness
without producing a clear template.

### 2. Make one repository dynamically switch between MySQL and MongoDB

This demonstrates a storage abstraction, but it hides important store-specific
data modeling and migration choices behind a lowest-common-denominator interface.
For a template, that abstraction is more confusing than useful.

### 3. Use one Example resource with store-specific application surfaces

Selected. The HTTP module owns a MongoDB-backed Example resource and both Fiber
and Gin adapters reuse its service. The command module owns a MySQL-backed
ExampleRecord resource and exposes explicit CRUD subcommands. Both use the same
field vocabulary and layer rules, while their IDs and persistence concerns stay
store-specific.

This makes every directory independently understandable and demonstrates both
databases without pretending they are interchangeable.

## Architecture

### Shared conventions

- Every externally initiated operation accepts and propagates
  `context.Context`.
- API and CLI layers parse inputs, invoke a service, and render outputs. They do
  not contain persistence logic.
- Services own use-case validation, DTO/entity mapping, cache/task coordination,
  and stable domain errors.
- Repositories own persistence-facing interfaces and translate driver errors
  into stable errors such as invalid ID, not found, and conflict.
- Models contain only database-driver operations and return driver-level results.
- Entities describe stored data and never import HTTP or CLI packages.
- Constructors receive dependencies explicitly. Framework locators remain only
  where they demonstrate FiberHouse integration.
- List operations use a normalized `page` and `page_size`, return deterministic
  ordering, and include total-count metadata.
- Create/update inputs use explicit types; update is not an upsert.
- Timestamps are generated consistently, and updates preserve `created_at`.

### MongoDB HTTP module

`example-module` remains the canonical business module:

```text
Fiber/Gin handler
  -> ExampleService
    -> ExampleRepository interface
      -> MongoExampleRepository
        -> ExampleModel
          -> MongoDB collection
```

The resource fields are intentionally boring and broadly reusable:

- `id`: MongoDB ObjectID rendered as hex
- `name`: required, trimmed, unique
- `description`: optional bounded text
- `status`: `active` or `archived`
- `tags`: bounded string list
- `created_at`, `updated_at`: UTC timestamps

CRUD operations:

- Create
- Get by ID
- List with pagination and optional status filter
- Update mutable fields
- Delete by ID

MongoDB setup creates a unique index on `name` and an index supporting stable
list order/filtering. The model uses only caller-provided contexts.

### Fiber and Gin adapters

Both adapters expose equivalent REST semantics:

```text
POST   /examples
GET    /examples/:id
GET    /examples
PUT    /examples/:id
DELETE /examples/:id
```

The Gin adapter imports and injects the canonical `ExampleService`; it does not
define a second service or repository. Framework-specific code is limited to
request binding, context extraction, validation integration, status codes, and
response writing.

Existing health/common endpoints may remain as focused demonstrations, but test
endpoints and misleading names are removed from the primary CRUD path.

### Redis and asynchronous tasks

Redis remains a cross-cutting example:

- List reads use the existing FiberHouse cache abstraction with a deterministic
  key derived from normalized pagination/filter parameters.
- Mutations explicitly invalidate or version relevant list cache keys where the
  framework API supports it; otherwise caching stays read-through with a short
  documented TTL and no false invalidation claim.
- Task dispatch is a separate service method or endpoint that enqueues an
  Example event after a successful read/mutation. Failure to enqueue is logged
  and returned according to the endpoint contract; it is never silently
  dereferenced after dispatcher creation fails.
- Task payloads contain stable identifiers and event metadata, not arbitrary
  age counters.

### MySQL command module

`command-module` becomes a complete CLI-oriented vertical slice:

```text
urfave/cli command
  -> ExampleService
    -> ExampleRepository
      -> ExampleModel
        -> GORM/MySQL table example_records
```

The table is `example_records` with:

- unsigned auto-increment primary key
- unique `name`
- `description`
- indexed `status`
- UTC `created_at` / `updated_at`
- nullable `deleted_at` only if the example explicitly demonstrates soft delete;
  otherwise deletion is hard and the field is omitted

The command surface is explicit:

```text
example migrate
example create --name ... [--description ...] [--status active]
example get --id ...
example list [--page 1] [--page-size 20] [--status active]
example update --id ... [--name ...] [--description ...] [--status ...]
example delete --id ...
```

Commands return errors rather than printing success before an operation is
known to have completed. Output uses a small injectable writer/renderer so
service tests do not assert terminal side effects.

## Error Handling

- Invalid MongoDB IDs and invalid CLI flags are input errors.
- Missing rows/documents produce a stable not-found error.
- Duplicate names produce a stable conflict error.
- Driver errors retain their cause with `%w` and are logged only at the
  application boundary to avoid duplicate logs.
- Model methods do not panic.
- API adapters hand errors to the existing FiberHouse exception/response path.
- CLI actions return actionable errors to the global CLI error handler.

## Dependency Injection

- Wire provider sets are updated for the MongoDB repository interface and shared
  service.
- Generated Wire files are kept consistent with provider declarations.
- The Gin adapter continues to use the same service constructor as Fiber.
- CLI dependencies are assembled once in a command-level factory; individual
  command actions do not mutate the Dig container on every invocation.

## Testing

All behavior changes follow red-green-refactor:

- Unit tests cover DTO normalization, service mapping, stable errors, pagination,
  update semantics, and CLI flag-to-input mapping.
- Repository/model tests cover MongoDB ObjectID handling, CRUD result handling,
  and MySQL pagination/update/delete behavior.
- Route tests verify Fiber and Gin expose equivalent methods, paths, and status
  semantics without duplicating service tests.
- CLI tests run commands with an in-memory writer and fake service.
- Integration tests are opt-in through environment/config availability and use
  the already running local MySQL, MongoDB, and Redis services. They create
  uniquely named test data and clean only that data.
- Full verification includes `go test ./...`, `go test -race` for changed module
  packages where practical, `go vet ./...`, and `go build ./...`.

## Documentation

- Each module README explains its responsibility, dependency direction, and
  runnable examples.
- The root example documentation states required local services and commands.
- Comments explain architectural intent; they do not narrate obvious Go syntax.
- Public symbols use consistent Go documentation and CRUD spelling.

## Compatibility and Scope

- Framework public APIs outside `example_application` are not changed unless a
  concrete test proves the example cannot be corrected without a small fix.
- Existing provider lifecycle, response wrappers, validation setup, and
  FiberHouse locator conventions are reused.
- The work does not add a generic ORM abstraction, event bus, schema migration
  framework, authentication, authorization, or production deployment manifests.
- Generated Swagger documentation is updated only if the repository already has
  a deterministic generation path; stale generated output is not hand-edited.

## Acceptance Criteria

- MongoDB HTTP CRUD is complete and consistent through model, repository,
  service, DTO, Fiber, and Gin layers.
- Gin demonstrably reuses the canonical Example service.
- MySQL CLI CRUD is complete, readable from `--help`, and backed by a coherent
  `example_records` schema.
- Redis/task examples are safe, contextual, and clearly separated from core
  persistence behavior.
- Focused tests exist for every new public behavior and fail before the
  corresponding implementation is added.
- The entire repository builds and its tests pass from the isolated worktree.
- Module READMEs explain the template without requiring readers to reverse
  engineer implementation details.
