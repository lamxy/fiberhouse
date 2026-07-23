# MySQL command module

The command slice is independent of HTTP:

```text
urfave/cli command -> ExampleMysqlService -> ExampleRepository -> GORM -> MySQL
```

`example_records` uses an auto-incrementing unsigned ID, unique `name`
(`varchar(80)`), `description` (`varchar(500)`), indexed `status`
(`active`/`archived`), and GORM timestamps. Run migration before CRUD:

```bash
cd example_application/command
go run . example migrate
go run . example create --name alpha --description first --status active
go run . example get --id 1
go run . example list --page 1 --page-size 20 --status active
go run . example update --id 1 --status archived
go run . example delete --id 1
```

The repository config defaults to
`root:root@tcp(127.0.0.1:3306)/test?...`. The opt-in integration test accepts
`FIBERHOUSE_MYSQL_DSN`; its default connects without a database, creates a
uniquely named isolated database, and drops only that database during cleanup.

```bash
FIBERHOUSE_INTEGRATION=1 go test ./example_application/module/command-module -count=1
```
