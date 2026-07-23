# Final Branch Fix Report

## Outcome

All Critical, Important, and Minor findings in `final-branch-review.md` were
fixed without changing routes, schemas, cache TTL, best-effort task dispatch,
CLI JSON output, or public framework APIs.

## Changes

- Added one shared `errors.Is` transport mapper used by Fiber and Gin:
  invalid input/ID = 400, not found = 404, conflict/unchanged = 409, unknown
  errors retain the existing 500 behavior.
- Added cross-adapter end-to-end tests through the real error handler. Every
  stable error now asserts the same HTTP status and `code`/`msg`/`data`
  envelope in Fiber and Gin.
- Enabled the production level-two list cache with the existing 30-second TTL.
  A real option/container/cache-path test proves two identical list requests
  invoke the store loader once and preserve the caller context.
- Added service-owned normalize-then-validate rules using rune counts for
  create and every supplied update field, with stable `ErrInvalidInput`.
- Changed MySQL zero-row updates to fetch the row, returning the existing
  record for a no-op and `ErrNotFound` only when the row is absent.
- Added driver-faithful and live same-value MySQL update coverage.
- Bounded MySQL integration database cleanup to 10 seconds and reports drop
  failures with `t.Errorf`.
- Kept the documented cache TTL and no-invalidation policy unchanged, and
  documented the shared HTTP error statuses.

## TDD Evidence

### RED

Command:

```text
GOCACHE=/tmp/fiberhouse-final-fix-cache rtk proxy go test \
  ./example_application/module/example-module/service \
  ./example_application/module/example-module/api \
  ./example_application/module/example-ginapi-module/api \
  ./example_application/module/command-module/repository
```

Expected failures observed:

```text
undefined: ErrInvalidInput
TestRepositoryUpdateReturnsExistingRecordWhenMySQLReportsNoChangedRows:
Update() error = example record not found
FAIL
```

### GREEN — focused affected packages

```text
ok github.com/lamxy/fiberhouse/example_application/module/example-module/service
ok github.com/lamxy/fiberhouse/example_application/module/example-module/api
ok github.com/lamxy/fiberhouse/example_application/module/example-ginapi-module/api
ok github.com/lamxy/fiberhouse/example_application/module/command-module/repository
```

### GREEN — example-wide

Command:

```text
GOCACHE=/tmp/fiberhouse-final-fix-cache rtk proxy go test ./example_application/...
```

Result: exit 0; every package passed or reported no test files.

### GREEN — live MongoDB, Redis, and MySQL

Command:

```text
FIBERHOUSE_INTEGRATION=1 GOCACHE=/tmp/fiberhouse-final-fix-cache \
  rtk proxy go test \
  ./example_application/module/example-module \
  ./example_application/module/command-module
```

Result:

```text
ok github.com/lamxy/fiberhouse/example_application/module/example-module
ok github.com/lamxy/fiberhouse/example_application/module/command-module
```

### Additional verification

- Focused race run: exit 0.
- `go vet ./example_application/...`: exit 0.
- `go build ./example_application/...`: exit 0.
- `git diff --check`: clean.

The first extra vet/build attempt encountered the sandbox's read-only default
Go module stat cache. Re-running with
`GOMODCACHE=/tmp/fiberhouse-final-fix-modcache` completed successfully.

## Files

- `example_application/module/example-module/transport/domain_error.go`
- `example_application/module/example-module/api/example_api.go`
- `example_application/module/example-ginapi-module/api/example_api.go`
- `example_application/module/example-module/service/example_service.go`
- `example_application/module/command-module/repository/example_repository.go`
- Corresponding service, adapter, repository, and live integration tests
- `example_application/module/example-module/README.md`

## Self-review

- The mapper matches wrapped errors with `errors.Is` and deliberately returns
  the original unknown error so private causes stay on the existing 500 path.
- Canonical validation runs before any create write or update read/write.
- Rune counts, not bytes, enforce every documented text boundary.
- The cache regression uses the actual production option chain and global
  container lookup rather than the prior `listCached` test seam.
- The MySQL fix adds one existence fetch only when changed rows are zero;
  normal updates keep the existing query path and caller context.
- No cache invalidation, TTL, task dispatch, route, schema, CLI, or framework
  API behavior was broadened.

## Commit

This report is part of the single final-fix commit. Resolve its immutable hash
with:

```text
git rev-parse HEAD
```
