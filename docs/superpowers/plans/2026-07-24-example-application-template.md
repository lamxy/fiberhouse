# Example Application Production Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the ad-hoc example modules with a coherent MongoDB HTTP CRUD template, shared Fiber/Gin service layer, MySQL CLI CRUD template, safe Redis/task examples, focused tests, and runnable documentation.

**Architecture:** The canonical HTTP vertical slice is `transport -> service -> repository -> Mongo model`; Fiber and Gin are thin adapters over the same service. The CLI is a separate `command -> service -> repository -> MySQL model` vertical slice using the same field vocabulary without pretending both stores share one persistence abstraction.

**Tech Stack:** Go 1.25, Fiber v2, Gin v1.12, MongoDB driver v2, GORM/MySQL, go-redis/FiberHouse cache, Asynq, Wire, Dig, urfave/cli v2, testify.

## Global Constraints

- Work only in `.worktrees/example-application-template` on branch `refactor/example-application-template`.
- Do not change public framework APIs outside `example_application` unless a failing test proves a minimal framework fix is necessary.
- Propagate caller `context.Context`; no model/repository method may substitute `context.Background()` or `context.TODO()`.
- Models return driver-level results and errors; repositories translate invalid ID, not found, duplicate/conflict, and unchanged results; services own DTO/entity mapping.
- Update is never an upsert, list ordering is deterministic, and pagination defaults to page `1`, page size `20`, maximum page size `100`.
- The canonical statuses are exactly `active` and `archived`.
- New behavior is implemented test-first with a witnessed failing test before production code.
- Integration tests must isolate their own data and skip with a precise reason when the required local service/config is unavailable.
- Use existing FiberHouse locators, response wrappers, validation, cache, task, provider, and dependency-injection patterns.
- Do not hand-edit generated Swagger output without a deterministic generator command.

---

### Task 1: Canonical MongoDB Domain, Repository, and Service

**Files:**
- Modify: `example_application/module/common-module/fields/timestamps.go`
- Replace: `example_application/module/example-module/entity/types.go`
- Replace: `example_application/apivo/example/requestvo/example_reqvo.go`
- Replace: `example_application/apivo/example/responsevo/example_respvo.go`
- Replace: `example_application/module/example-module/model/example_model.go`
- Modify: `example_application/module/example-module/model/model_wireset.go`
- Replace: `example_application/module/example-module/repository/example_repository.go`
- Modify: `example_application/module/example-module/repository/repository_wireset.go`
- Replace: `example_application/module/example-module/service/example_service.go`
- Modify: `example_application/module/example-module/service/service_wireset.go`
- Create: `example_application/module/example-module/model/example_model_test.go`
- Create: `example_application/module/example-module/repository/example_repository_test.go`
- Create: `example_application/module/example-module/service/example_service_test.go`

**Interfaces:**
- Produces:

```go
// entity
type ExampleStatus string
const (
    ExampleStatusActive   ExampleStatus = "active"
    ExampleStatusArchived ExampleStatus = "archived"
)
type Example struct {
    ID          bson.ObjectID `json:"id" bson:"_id,omitempty"`
    Name        string        `json:"name" bson:"name"`
    Description string        `json:"description" bson:"description,omitempty"`
    Status      ExampleStatus `json:"status" bson:"status"`
    Tags        []string      `json:"tags" bson:"tags,omitempty"`
    fields.Timestamps `json:"-" bson:",inline"`
}

// requestvo
type CreateExampleReqVo struct {
    Name        string   `json:"name" validate:"required,min=2,max=80"`
    Description string   `json:"description" validate:"max=500"`
    Status      string   `json:"status" validate:"omitempty,oneof=active archived"`
    Tags        []string `json:"tags" validate:"max=10,dive,min=1,max=30"`
}
type UpdateExampleReqVo struct {
    Name        *string   `json:"name" validate:"omitempty,min=2,max=80"`
    Description *string   `json:"description" validate:"omitempty,max=500"`
    Status      *string   `json:"status" validate:"omitempty,oneof=active archived"`
    Tags        *[]string `json:"tags" validate:"omitempty,max=10,dive,min=1,max=30"`
}
type ListExamplesReqVo struct {
    Page     int    `query:"page" form:"page" validate:"omitempty,min=1"`
    PageSize int    `query:"page_size" form:"page_size" validate:"omitempty,min=1,max=100"`
    Status   string `query:"status" form:"status" validate:"omitempty,oneof=active archived"`
}
func (q ListExamplesReqVo) Normalize() ListExamplesReqVo

// responsevo
type ExampleRespVo struct {
    ID          string              `json:"id"`
    Name        string              `json:"name"`
    Description string              `json:"description,omitempty"`
    Status      string              `json:"status"`
    Tags        []string            `json:"tags"`
    commonvo.Timestamps
}
type ExampleListRespVo struct {
    Items    []ExampleRespVo `json:"items"`
    Page     int             `json:"page"`
    PageSize int             `json:"page_size"`
    Total    int64           `json:"total"`
}

// repository-facing model contract
type ExampleModelStore interface {
    EnsureIndexes(context.Context) error
    Insert(context.Context, *entity.Example) (bson.ObjectID, error)
    FindByID(context.Context, bson.ObjectID) (*entity.Example, error)
    Find(context.Context, model.ExampleFilter) ([]entity.Example, int64, error)
    Replace(context.Context, bson.ObjectID, *entity.Example) (bool, error)
    Delete(context.Context, bson.ObjectID) (bool, error)
}
type ExampleFilter struct {
    Page, PageSize int
    Status entity.ExampleStatus
}

// service-facing repository contract
var (
    ErrInvalidID = errors.New("invalid example id")
    ErrNotFound  = errors.New("example not found")
    ErrConflict  = errors.New("example already exists")
    ErrUnchanged = errors.New("example unchanged")
)
type ExampleStore interface {
    Create(context.Context, *entity.Example) error
    Get(context.Context, string) (*entity.Example, error)
    List(context.Context, ListOptions) ([]entity.Example, int64, error)
    Update(context.Context, string, *entity.Example) error
    Delete(context.Context, string) error
}

// service API consumed by transports
type ExampleUseCase interface {
    Create(context.Context, requestvo.CreateExampleReqVo) (*responsevo.ExampleRespVo, error)
    Get(context.Context, string) (*responsevo.ExampleRespVo, error)
    List(context.Context, requestvo.ListExamplesReqVo) (*responsevo.ExampleListRespVo, error)
    Update(context.Context, string, requestvo.UpdateExampleReqVo) (*responsevo.ExampleRespVo, error)
    Delete(context.Context, string) error
}
```

- [ ] **Step 1: Write request normalization and service mapping tests**

Cover default page/page-size, maximum size validation boundary, trimming names and
tags, default `active` status, create mapping, partial update preservation, empty
list as `[]`, stable error propagation, and caller context identity.

- [ ] **Step 2: Run focused tests and witness RED**

Run:

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/module/example-module/service ./example_application/module/example-module/repository ./example_application/module/example-module/model -count=1
```

Expected: compilation failure for the new DTOs/contracts or failing assertions
against the old partial CRUD behavior.

- [ ] **Step 3: Implement entity, DTO, model, repository, and service**

Use Mongo operations equivalent to:

```go
filter := bson.D{{Key: "_id", Value: id}}
update := bson.D{{Key: "$set", Value: bson.D{
    {Key: "name", Value: example.Name},
    {Key: "description", Value: example.Description},
    {Key: "status", Value: example.Status},
    {Key: "tags", Value: example.Tags},
    {Key: "updated_at", Value: example.UpdatedAt},
}}}
result, err := collection.UpdateOne(ctx, filter, update)
```

Create a unique ascending `name` index and a compound
`status, created_at, _id` list index. Check `Find` errors before deferring cursor
close. Use the passed `ctx` for `Find`, `All`, `CountDocuments`, and `Close`.

- [ ] **Step 4: Run focused tests and witness GREEN**

Run the Step 2 command.

Expected: all three packages pass with no warnings.

- [ ] **Step 5: Refactor mappings and verify**

Keep one `toResponse(entity.Example) responsevo.ExampleRespVo` helper in the
service package and one error-translation helper in the repository package.
Run the Step 2 command again.

- [ ] **Step 6: Commit**

```bash
rtk git add example_application/apivo/example \
  example_application/module/common-module/fields \
  example_application/module/example-module/entity \
  example_application/module/example-module/model \
  example_application/module/example-module/repository \
  example_application/module/example-module/service
rtk git commit -m "refactor(example): add complete MongoDB CRUD slice"
```

### Task 2: Fiber and Gin REST Adapters Sharing One Service

**Files:**
- Replace: `example_application/module/example-module/api/example_api.go`
- Replace: `example_application/module/example-module/api/register_api_router.go`
- Modify: `example_application/module/example-module/api/api_provider.go`
- Modify: `example_application/module/example-module/api/api_provider_wire_gen.go`
- Replace: `example_application/module/example-ginapi-module/api/example_api.go`
- Replace: `example_application/module/example-ginapi-module/api/register_api_router.go`
- Modify: `example_application/module/example-ginapi-module/api/api_provider.go`
- Modify: `example_application/module/example-ginapi-module/api/api_provider_wire_gen.go`
- Create: `example_application/module/example-module/api/example_api_test.go`
- Create: `example_application/module/example-ginapi-module/api/example_api_test.go`

**Interfaces:**
- Consumes: `service.ExampleUseCase` from Task 1.
- Produces:

```go
func NewExampleHandler(ctx fiberhouse.IApplicationContext, useCase service.ExampleUseCase) *ExampleHandler

// Fiber
func (h *ExampleHandler) Create(*fiber.Ctx) error
func (h *ExampleHandler) Get(*fiber.Ctx) error
func (h *ExampleHandler) List(*fiber.Ctx) error
func (h *ExampleHandler) Update(*fiber.Ctx) error
func (h *ExampleHandler) Delete(*fiber.Ctx) error

// Gin uses the same constructor dependency and method names with *gin.Context.
```

- [ ] **Step 1: Write route and handler contract tests**

Use a fake `ExampleUseCase` that records the received context and input. Verify
the five REST routes, `201` on create, `200` on get/list/update, `204` on delete,
request/parameter binding, caller-context propagation, and error forwarding for
both Fiber and Gin.

- [ ] **Step 2: Run adapter tests and witness RED**

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/module/example-module/api ./example_application/module/example-ginapi-module/api -count=1
```

Expected: old handlers/routes do not satisfy the new REST contract.

- [ ] **Step 3: Implement thin adapters and update Wire providers**

Register exactly:

```text
POST   /examples
GET    /examples/:id
GET    /examples
PUT    /examples/:id
DELETE /examples/:id
```

Keep health/common demonstrations outside the CRUD group. Bind the concrete
`*service.ExampleService` to `service.ExampleUseCase` and ensure both generated
injectors build the same service/repository/model chain.

- [ ] **Step 4: Run adapter tests and compile all example packages**

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/... -count=1
```

Expected: all example packages pass.

- [ ] **Step 5: Commit**

```bash
rtk git add example_application/module/example-module/api \
  example_application/module/example-ginapi-module/api
rtk git commit -m "refactor(example): align Fiber and Gin CRUD adapters"
```

### Task 3: Production-Readable MySQL CLI Vertical Slice

**Files:**
- Replace: `example_application/module/command-module/entity/mysql_types.go`
- Replace: `example_application/module/command-module/model/mysql_model.go`
- Create: `example_application/module/command-module/repository/example_repository.go`
- Replace: `example_application/module/command-module/service/example_mysql_service.go`
- Delete: `example_application/module/command-module/model/mongodb_model.go`
- Delete: `example_application/module/command-module/service/mongodb_service.go`
- Replace: `example_application/command/application/commands/test_orm_command.go`
- Modify: `example_application/command/application/application.go`
- Create: `example_application/module/command-module/model/mysql_model_test.go`
- Create: `example_application/module/command-module/repository/example_repository_test.go`
- Create: `example_application/module/command-module/service/example_service_test.go`
- Create: `example_application/command/application/commands/example_command_test.go`

**Interfaces:**
- Produces:

```go
type ExampleRecord struct {
    ID          uint64    `gorm:"primaryKey;autoIncrement"`
    Name        string    `gorm:"size:80;not null;uniqueIndex:ux_example_records_name"`
    Description string    `gorm:"size:500;not null;default:''"`
    Status      string    `gorm:"size:16;not null;index:idx_example_records_status_created"`
    CreatedAt   time.Time `gorm:"index:idx_example_records_status_created,sort:desc"`
    UpdatedAt   time.Time
}
func (ExampleRecord) TableName() string { return "example_records" }

type ListOptions struct { Page, PageSize int; Status string }
type ExampleRepository interface {
    Migrate(context.Context) error
    Create(context.Context, *entity.ExampleRecord) error
    Get(context.Context, uint64) (*entity.ExampleRecord, error)
    List(context.Context, ListOptions) ([]entity.ExampleRecord, int64, error)
    Update(context.Context, uint64, UpdateInput) (*entity.ExampleRecord, error)
    Delete(context.Context, uint64) error
}
type ExampleUseCase interface {
    Migrate(context.Context) error
    Create(context.Context, CreateInput) (*entity.ExampleRecord, error)
    Get(context.Context, uint64) (*entity.ExampleRecord, error)
    List(context.Context, ListInput) (*ListResult, error)
    Update(context.Context, uint64, UpdateInput) (*entity.ExampleRecord, error)
    Delete(context.Context, uint64) error
}
func NewExampleCommand(ctx fiberhouse.ICommandContext) fiberhouse.CommandGetter
```

- [ ] **Step 1: Write model/repository/service tests**

Use GORM dry-run or an isolated SQLite-free fake at the model contract boundary;
do not add SQLite. Verify table name/schema tags, pagination normalization,
duplicate/not-found translation, partial updates, hard delete, and context
propagation.

- [ ] **Step 2: Write CLI command tests**

Run an urfave CLI app with a fake `ExampleUseCase` and `bytes.Buffer`. Verify the
`migrate/create/get/list/update/delete` subcommands, required flags, defaults,
invalid IDs/statuses, and deterministic JSON output.

- [ ] **Step 3: Run command tests and witness RED**

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/module/command-module/... ./example_application/command/application/commands -count=1
```

Expected: old `test-orm` surface and direct model service fail the new contracts.

- [ ] **Step 4: Implement the MySQL vertical slice and command factory**

Use `db.WithContext(ctx)` for every GORM operation. Migrate only
`entity.ExampleRecord`. Use `Limit`, `Offset`, `Order("created_at DESC, id DESC")`
and a separate count. Updates must use a field map so explicit empty descriptions
are preserved.

Assemble dependencies once in the command constructor/factory. Replace
`test-orm` registration with `example`.

- [ ] **Step 5: Run focused and example-wide tests**

Run Step 3, then:

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/... -count=1
```

Expected: both commands pass.

- [ ] **Step 6: Commit**

```bash
rtk git add example_application/module/command-module \
  example_application/command/application
rtk git commit -m "refactor(example): add complete MySQL CRUD CLI"
```

### Task 4: Redis/Task Safety, Integration Coverage, and Documentation

**Files:**
- Replace: `example_application/module/example-module/task/task.go`
- Replace: `example_application/module/example-module/task/handler/handle.go`
- Modify: `example_application/module/example-module/task/handler/mount.go`
- Modify: `example_application/module/task_impl.go`
- Modify: `example_application/module/example-module/service/example_service.go`
- Create: `example_application/module/example-module/task/task_test.go`
- Create: `example_application/module/example-module/integration_test.go`
- Create: `example_application/module/command-module/integration_test.go`
- Replace: `example_application/module/example-module/README.md`
- Replace: `example_application/module/example-ginapi-module/README.md`
- Create: `example_application/module/command-module/README.md`
- Modify: `example_application/command/README_go_build.md`
- Create: `example_application/module/README.md`

**Interfaces:**
- Produces:

```go
const TypeExampleChanged = "example:changed"
type ExampleChangedPayload struct {
    ID        string `json:"id"`
    Operation string `json:"operation"`
}
func NewExampleChangedTask(ctx fiberhouse.IContext, payload ExampleChangedPayload) (*asynq.Task, error)
func HandleExampleChangedTask(context.Context, *asynq.Task) error
```

- [ ] **Step 1: Write task payload and dispatcher safety tests**

Verify stable JSON payloads, rejected empty IDs/operations, successful enqueue
options, dispatcher-construction errors, enqueue errors, and no nil dispatcher
dereference.

- [ ] **Step 2: Run task/service tests and witness RED**

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/module/example-module/task/... ./example_application/module/example-module/service -count=1
```

Expected: old age-counter task and unsafe dispatcher flow fail the new contract.

- [ ] **Step 3: Implement task event and safe cache/task coordination**

Use normalized list query values in cache keys. Pass the cache loader's `ctx`
through repository calls. Keep a short documented read-through TTL unless an
existing cache API can invalidate list prefixes without a framework change.
Never claim invalidation if only TTL expiry exists.

- [ ] **Step 4: Add opt-in real-service integration tests**

Gate with `FIBERHOUSE_INTEGRATION=1`. Use unique names containing a timestamp
and process ID. Clean only the created Mongo documents/MySQL rows/Redis keys.
Exercise create/get/list/update/delete and one Redis round trip.

- [ ] **Step 5: Rewrite module documentation**

Document dependency direction, field/schema choices, full route/command examples,
local MySQL/MongoDB/Redis prerequisites, integration-test command, and why Gin
reuses the canonical service.

- [ ] **Step 6: Run changed-package verification**

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/... -count=1
```

Expected: all example packages pass without integration mode.

- [ ] **Step 7: Run local integration verification**

```bash
FIBERHOUSE_INTEGRATION=1 GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./example_application/module/example-module ./example_application/module/command-module -count=1
```

Expected: pass when repository config matches the running local services; if
configuration is absent, tests skip with the exact missing key/path.

- [ ] **Step 8: Commit**

```bash
rtk git add example_application/module example_application/command/README_go_build.md
rtk git commit -m "test(example): verify infrastructure examples and document modules"
```

### Task 5: Whole-Repository Verification and Cleanup

**Files:**
- Modify only files required by failures directly caused by Tasks 1-4.

**Interfaces:**
- Consumes all prior tasks.
- Produces a clean, buildable branch with no stale `TestOrm`, old `ExamAge`,
  `CURD`, background-context, unsafe cursor, or update-upsert examples in
  `example_application`.

- [ ] **Step 1: Format changed Go files**

```bash
gofmt -w $(rtk git diff --name-only --diff-filter=ACM -- '*.go')
```

- [ ] **Step 2: Run stale-pattern checks**

```bash
rtk rg -n 'CURD|TestOrm|ExamAge|context\\.(Background|TODO)\\(\\)|SetUpsert\\(true\\)' example_application
```

Expected: no production example matches; explicitly justified lifecycle
background contexts outside module request flows may remain.

- [ ] **Step 3: Run full tests, vet, race tests, and build**

```bash
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test ./... -count=1
GOCACHE=/tmp/fiberhouse-example-template-go-cache rtk go test -race ./example_application/... -count=1
GOCACHE=/tmp/fiberhouse-example-template-go-cache go vet ./...
GOCACHE=/tmp/fiberhouse-example-template-go-cache go build ./...
```

Expected: all commands exit `0`.

- [ ] **Step 4: Verify diff quality**

```bash
rtk git diff --check
rtk git status --short
```

Expected: no whitespace errors and only intentional task changes.

- [ ] **Step 5: Commit verification-only fixes if any**

```bash
rtk git add example_application
rtk git commit -m "chore(example): finalize production template verification"
```

Skip the commit when verification required no source changes.
