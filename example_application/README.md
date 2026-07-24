# example_application

`example_application` is a copy-paste, production-oriented reference
template for building on FiberHouse. It is not a toy "hello world" — it
exercises the layering FiberHouse expects an application to own itself
(the framework does not generate or enforce business directories) with a
real CRUD resource, two interchangeable HTTP adapters, an independent CLI
tool, Swagger docs, and both unit and opt-in integration tests.

Use it as a starting point: copy the directory, rename the module, and
delete what you don't need.

## What this template demonstrates

- **Transport-independent business logic.** One canonical MongoDB-backed
  `Example` use case is served identically by Fiber and Gin adapters —
  proof that FiberHouse's core-swap story (`fiber` \| `gin`) doesn't
  require duplicating business rules per framework.
- **A second, deliberately separate MySQL slice** driven entirely from a
  `urfave/cli` command, showing that persistence/service stacks are
  per-module, not global.
- **Strict layering** (see below) with dependencies pointing one direction
  only: transport → service → repository → model.
- **Swagger/OpenAPI generation** from source annotations, gated by config.
- **`.http` files** for manual, tool-assisted endpoint testing.
- **Unit tests everywhere, real-service integration tests opt-in.**

## Layering and dependency direction

Every module in this template follows the same inward-pointing dependency
chain:

```text
transport (HTTP handler / CLI command) -> service -> repository -> model
```

Handlers/commands never touch a repository or model directly, and
repositories never reach back into a service or transport package. This is
documented in depth, per module, in:

- [`module/README.md`](module/README.md) — overview of how the three
  modules relate (shared canonical module vs. Gin adapter vs. independent
  MySQL module) and the top-level local-verification commands.
- [`module/example-module/README.md`](module/example-module/README.md) —
  the canonical, transport-independent MongoDB CRUD slice
  (`Fiber/Gin handler -> ExampleUseCase -> ExampleStore -> ExampleModel ->
  MongoDB`), including routes, request examples, caching behavior, and
  error-mapping rules.
- [`module/example-ginapi-module/README.md`](module/example-ginapi-module/README.md) —
  the Gin transport adapter that reuses the canonical service instead of
  reimplementing it.
- [`module/command-module/README.md`](module/command-module/README.md) —
  the independent MySQL CLI slice
  (`urfave/cli command -> ExampleMysqlService -> ExampleRepository -> GORM
  -> MySQL`).

This README is the entry point; it links to those module READMEs rather
than duplicating their content.

## Why Gin reuses the canonical Example service

`module/example-ginapi-module` is intentionally **not** a second business
module — it is a thin Gin transport adapter over the same
`service.ExampleUseCase`, repository, entity, and MongoDB model that the
Fiber adapter (`module/example-module`) uses. The Gin handler binds and
validates Gin-specific request types, forwards `c.Request.Context()` into
the shared use case, and maps the same domain errors to HTTP responses.

The lesson: when FiberHouse lets you swap or add a core HTTP engine, only
the framework-facing edge (request binding, response writing, error
translation) should change per engine. Field validation rules, status
semantics, pagination, cache behavior, and task dispatch stay in one place.
Duplicating the service per adapter would let the two engines' behavior
drift silently; sharing it guarantees Fiber and Gin are contractually
identical at the HTTP boundary. See
[`module/example-ginapi-module/README.md`](module/example-ginapi-module/README.md)
for the full explanation.

## Required local services

Running the HTTP CRUD endpoints or the CLI's `example` commands against
real backends requires:

- **MySQL** — used only by the CLI's `command-module`.
- **MongoDB** — used by the canonical `example-module` (both HTTP
  adapters).
- **Redis** — used as the level-two read-through cache for list queries in
  `example-module`.

A ready-to-use Compose file with matching defaults lives at
[`../docs/docker_compose_db_redis_yaml/docker-compose.yml`](../docs/docker_compose_db_redis_yaml/docker-compose.yml)
(MongoDB on host port `27037`, Redis on `6379`, MySQL on `3306` with root
password `root`). It is intended as a local dev convenience, not a
production configuration.

### Where config is loaded from

Both runnable entry points load configuration from the repo-root
`example_config/` directory (**not** a directory inside
`example_application/`):

- The HTTP server entry point, `example_main/main.go`, sets
  `ConfigPath: "./example_config"` (relative to the repo root, where that
  binary is run from) in its `fiberhouse.BootConfig`.
- The CLI entry point, `example_application/command/main.go`, calls
  `bootstrap.NewConfigOnce("./../../example_config")` (relative to
  `example_application/command/`, which also resolves to the repo-root
  `example_config/`).

`example_config/` selects `application_{test,dev,prod}.yml` based on the
`application.env` setting (see
[`../example_config/README.md`](../example_config/README.md) for the env
var override scheme). The relevant keys and their defaults, matching the
Compose file above, are:

```yaml
database:
  mongodb:
    applyURI: mongodb://admin:admin@localhost:27037/?authSource=admin
  mysql:
    dsn: "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
cache:
  redis:
    host: 127.0.0.1
    port: 6379
```

Module-level test defaults and their environment-variable overrides
(`FIBERHOUSE_MONGODB_URI`, `FIBERHOUSE_MYSQL_DSN`, `FIBERHOUSE_REDIS_ADDR`,
etc.) are documented per module in the READMEs linked above.

## HTTP CRUD routes

Both the Fiber adapter (`module/example-module`) and the Gin adapter
(`module/example-ginapi-module`) register the identical route set at the
application root, with no path prefix (verified in each module's
`api/register_api_router.go`):

```text
POST   /examples
GET    /examples/:id
GET    /examples?page=1&page_size=20&status=active
PUT    /examples/:id
DELETE /examples/:id
```

Only one core engine runs at a time — `CoreType` is selected in
`example_main/main.go`'s `fiberhouse.BootConfig` (`fiber` or `gin`).
Whichever engine is active, the contract above is identical: same request
VOs, same `response.RespInfo{code,msg,data}` envelope, same validation and
error-status mapping. Example requests:

```bash
curl -X POST http://127.0.0.1:8080/examples \
  -H 'content-type: application/json' \
  -d '{"name":"alpha","description":"first","status":"active","tags":["go","mongo"]}'
curl http://127.0.0.1:8080/examples/OBJECT_ID
curl 'http://127.0.0.1:8080/examples?page=1&page_size=20&status=active'
curl -X PUT http://127.0.0.1:8080/examples/OBJECT_ID \
  -H 'content-type: application/json' -d '{"status":"archived"}'
curl -X DELETE http://127.0.0.1:8080/examples/OBJECT_ID
```

For interactive/manual testing, use the `.http` files (VS Code REST
Client / JetBrains HTTP Client format) instead of hand-writing curl:

- [`module/example-module/api/example_api.http`](module/example-module/api/example_api.http) —
  Fiber adapter.
- [`module/example-ginapi-module/api/example_api.http`](module/example-ginapi-module/api/example_api.http) —
  Gin adapter.
- [`api-tests.http`](api-tests.http) — a convenience index pointing at the
  two files above (both cover the same 5 endpoints; run one adapter at a
  time against `http://localhost:8080`).

## MySQL CLI (`example` command)

The CLI is independent of the HTTP layer and talks to MySQL through its
own `ExampleMysqlService -> ExampleRepository -> GORM` stack. It is run
from `example_application/command/`, where the CLI's `main.go` lives:

```bash
cd example_application/command
go run . example migrate
go run . example create --name alpha --description first --status active
go run . example get --id 1
go run . example list --page 1 --page-size 20 --status active
go run . example update --id 1 --status archived
go run . example delete --id 1
```

These six subcommands (`migrate`, `create`, `get`, `list`, `update`,
`delete`) are registered in
`example_application/command/application/commands/test_orm_command.go`'s
`NewExampleCommand`/`exampleCommand.GetCommand`, and wired into the CLI app
via `commands.NewExampleCommand(app.Ctx)` in
`example_application/command/application/application.go`. All commands
write their result as JSON to stdout. Run `migrate` once before any other
subcommand — it creates/updates the `example_records` table. See
[`module/command-module/README.md`](module/command-module/README.md) for
the table schema, integration-test defaults, and DSN override
(`FIBERHOUSE_MYSQL_DSN`).

## Swagger / OpenAPI docs

Swagger UI is gated by config, not always on. To enable it, set
`application.swagger.enable: true` in the active `example_config/
application_{env}.yml` (checked in
`providers/module/fiber_route_register.go`'s `RegisterFiberSwagger` and
`providers/module/gin_route_register.go`'s `RegisterGinSwagger`, both of
which read `ctx.GetConfig().Bool("application.swagger.enable")`). When
enabled, the UI is served at:

```text
GET /swagger/*
```

The checked-in `docs/doc.go` is a compile-safe **placeholder** with an
empty `paths` document — it is not the real generated spec. To regenerate
real docs from the swaggo annotations on the Fiber handlers (only Fiber
carries the annotations; Gin is documented as behaviorally identical — see
rationale below), install `swag` and run the provided script from the repo
root:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
bash example_application/generate_swagger.sh
```

Full rationale for why only Fiber is annotated, why `doc.go` is a
placeholder, and how the general Swagger annotations
(`@title`/`@host`/`@BasePath`, sourced from `example_main/main.go`) fit
together is in [`docs/README.md`](docs/README.md) — read it before
regenerating.

## Running tests

Unit tests (no external services required):

```bash
go test ./example_application/... -count=1
```

Integration tests are opt-in and require the real local services above.
They are skipped by default and only run when `FIBERHOUSE_INTEGRATION=1`
is set (checked directly in each module's `integration_test.go`, e.g.
`module/example-module/integration_test.go` and
`module/command-module/integration_test.go`):

```bash
FIBERHOUSE_INTEGRATION=1 go test \
  ./example_application/module/example-module \
  ./example_application/module/command-module -count=1
```

Integration tests create uniquely named/timestamped data and clean up only
what they created. Per-module environment overrides for connection
targets (`FIBERHOUSE_MONGODB_URI`, `FIBERHOUSE_MYSQL_DSN`,
`FIBERHOUSE_REDIS_ADDR`, etc.) are documented in the module READMEs linked
above.
