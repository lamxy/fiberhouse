# Build and run the example command

Run from this directory because `main.go` resolves configuration at
`../../example_config` relative to the working directory:

```bash
cd example_application/command
go run . --help
go run . example --help
```

Build and execute:

```bash
go build -o ./target/fiberhouse-example-command .
./target/fiberhouse-example-command example migrate
./target/fiberhouse-example-command example list --page 1 --page-size 20
```

Windows may use `./target/fiberhouse-example-command.exe`. MySQL must be
available using `database.mysql.dsn` in the active
`example_config/application_<env>.yml`. See
[`../module/command-module/README.md`](../module/command-module/README.md) for
the full CRUD command set and integration test.
