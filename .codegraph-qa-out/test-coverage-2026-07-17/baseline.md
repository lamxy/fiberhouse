# Test Coverage Baseline

Date: 2026-07-17

Base commit: `8c10a025fdbc046b55066d1a46e1456b5644508f`

## Commands

```bash
go mod download
go test ./... -count=1 -covermode=atomic -coverprofile=/tmp/fiberhouse-baseline-coverage.out
```

The baseline full-suite command failed before coverage completion because existing tests did not match current contracts:

- `bootstrap/Test_Config_EnvOverrideAndSingleton` created `application_web_dev.yml`, while the implementation selects `application_dev.yml`.
- asynchronous writer tests sampled output with fixed sleeps shorter than the configured one-second flush interval.
- the old write-after-close test expected success although the implementation rejects the write.
- an invalid-path test generated the repository artifact `component/logging/writer/D:/invalid/path/test.log`.

The generated artifact was removed before implementation commits.

## Coverage Snapshot

The initial profile contained 352 covered statements out of 5,208 total statements: **6.8%** repository-wide.

For planning, two additional scopes were calculated from the same profile:

- Library scope, excluding examples, generated protobuf and plugins: 352 / 3,898 = **9.0%**.
- Hermetic scope, additionally excluding database and remote-cache integrations that require external services: 352 / 3,533 = **10.0%**.

These scoped figures are navigation aids, not substitutes for the final whole-repository test result. The project goal is meaningful coverage of core behavior and failure paths rather than a 100% target.

## Structural Inventory

CodeGraph was used first for symbol and call-path exploration because the repository contains `.codegraph/`. AST-based inventory with ast-grep then identified approximately:

- 289 production functions;
- 762 production method matches;
- 82 tests;
- 14 benchmarks;
- 11 existing test files.

## Initial Risk Areas

Parallel read-only audits prioritized:

1. provider/location/context and global lifecycle state machines;
2. switchable Fiber/Gin HTTP context, error, response and recovery contracts;
3. asynchronous logging and reusable component primitives;
4. startup/frame/command orchestration after lower-level contracts are stable.

External database and remote-cache integration paths are explicitly deferred unless they can be tested hermetically without containers, live ports or credentials.
