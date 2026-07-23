# Example application modules

The examples share domain behavior instead of duplicating it by framework:

```text
Fiber adapter ----\
                   -> canonical MongoDB example module
Gin adapter ------/

CLI adapter --------> independent MySQL command module
```

- [`example-module`](example-module/README.md) owns HTTP CRUD behavior,
  MongoDB persistence, list caching, and the `example:changed` task contract.
- [`example-ginapi-module`](example-ginapi-module/README.md) adapts Gin to that
  same canonical use case.
- [`command-module`](command-module/README.md) demonstrates complete MySQL CRUD
  through `urfave/cli`.

Local verification:

```bash
go test ./example_application/... -count=1
FIBERHOUSE_INTEGRATION=1 go test \
  ./example_application/module/example-module \
  ./example_application/module/command-module -count=1
```

Integration defaults are MongoDB on `127.0.0.1:27037`, MySQL on
`127.0.0.1:3306`, and Redis on `127.0.0.1:6379`. Environment overrides are
documented in each module README.
