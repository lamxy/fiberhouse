# MySQL command 模块

命令切片独立于 HTTP：

```text
urfave/cli command -> ExampleMysqlService -> ExampleRepository -> GORM -> MySQL
```

`example_records` 使用自增无符号 ID、唯一的 `name`（`varchar(80)`）、
`description`（`varchar(500)`）、带索引的 `status`（`active`/`archived`），以及 GORM
时间戳。在 CRUD 之前先运行迁移：

```bash
cd example_application/command
go run . example migrate
go run . example create --name alpha --description first --status active
go run . example get --id 1
go run . example list --page 1 --page-size 20 --status active
go run . example update --id 1 --status archived
go run . example delete --id 1
```

repository 配置默认为 `root:root@tcp(127.0.0.1:3306)/test?...`。可选启用的集成测试接受
`FIBERHOUSE_MYSQL_DSN`；其默认值在不指定数据库的情况下连接，创建一个唯一命名的隔离
数据库，并在清理时只删除该数据库。

```bash
FIBERHOUSE_INTEGRATION=1 go test ./example_application/module/command-module -count=1
```
