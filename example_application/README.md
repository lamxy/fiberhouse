# example_application

`example_application` 是一个可复制、面向生产的 FiberHouse 参考模板。它不是玩具式的
"hello world"——而是用一个真实的 CRUD 资源、两个可互换的 HTTP 适配器、一个独立的
CLI 工具、Swagger 文档，以及单元测试与可选的集成测试，演练 FiberHouse 期望应用自身
拥有的分层（框架不生成也不强制业务目录）。

把它当作起点：复制该目录、重命名 module、删掉你不需要的部分即可。

## 本模板演示了什么

- **与传输层无关的业务逻辑。** 同一个规范的、基于 MongoDB 的 `Example` 用例被 Fiber
  与 Gin 适配器以完全一致的方式提供——证明 FiberHouse 的核心切换能力（`fiber` \| `gin`）
  无需为每种框架重复业务规则。
- **第二个、刻意独立的 MySQL 切片**，完全由一个 `urfave/cli` 命令驱动，表明持久化/服务
  栈是按模块划分的，而非全局共享。
- **严格分层**（见下文），依赖只指向一个方向：transport → service → repository → model。
- **Swagger/OpenAPI 生成**，源自源码注解，由配置控制开关。
- **`.http` 文件**，用于手动的、工具辅助的接口测试。
- **处处覆盖单元测试，真实服务的集成测试可选启用。**

## 分层与依赖方向

本模板中的每个模块都遵循同一条向内的依赖链：

```text
transport (HTTP handler / CLI command) -> service -> repository -> model
```

handler/command 绝不直接触碰 repository 或 model，repository 也绝不反向触及 service
或 transport 包。这一点在各模块的 README 中有深入说明：

- [`module/README.md`](module/README.md) —— 三个模块如何关联（共享的规范模块 vs. Gin
  适配器 vs. 独立的 MySQL 模块）以及顶层的本地验证命令概览。
- [`module/example-module/README.md`](module/example-module/README.md) —— 规范的、
  与传输层无关的 MongoDB CRUD 切片
  （`Fiber/Gin handler -> ExampleUseCase -> ExampleStore -> ExampleModel -> MongoDB`），
  包含路由、请求示例、缓存行为与错误映射规则。
- [`module/example-ginapi-module/README.md`](module/example-ginapi-module/README.md) ——
  复用规范 service 而非重新实现的 Gin 传输适配器。
- [`module/command-module/README.md`](module/command-module/README.md) —— 独立的
  MySQL CLI 切片
  （`urfave/cli command -> ExampleMysqlService -> ExampleRepository -> GORM -> MySQL`）。

本 README 是入口；它链接到那些模块 README，而不重复其内容。

## 为什么 Gin 复用规范的 Example service

`module/example-ginapi-module` 刻意**不是**第二个业务模块——它只是覆盖在同一个
`service.ExampleUseCase`、repository、entity 与 MongoDB model（Fiber 适配器
`module/example-module` 所用的那套）之上的一个轻量 Gin 传输适配器。Gin handler 绑定并
校验 Gin 特有的请求类型，把 `c.Request.Context()` 转发进共享用例，并把相同的领域错误
映射为 HTTP 响应。

其中的启示：当 FiberHouse 允许你切换或新增核心 HTTP 引擎时，只有面向框架的边缘
（请求绑定、响应写出、错误翻译）才应随引擎变化。字段校验规则、状态语义、分页、缓存
行为与任务派发都留在同一处。为每个适配器复制一份 service 会让两个引擎的行为悄然漂移；
共享它则保证 Fiber 与 Gin 在 HTTP 边界上契约完全一致。完整解释见
[`module/example-ginapi-module/README.md`](module/example-ginapi-module/README.md)。

## 所需的本地服务

针对真实后端运行 HTTP CRUD 接口或 CLI 的 `example` 命令需要：

- **MySQL** —— 仅由 CLI 的 `command-module` 使用。
- **MongoDB** —— 由规范的 `example-module`（两个 HTTP 适配器）使用。
- **Redis** —— 作为 `example-module` 中列表查询的二级 read-through 缓存。

一份带匹配默认值、开箱即用的 Compose 文件位于
[`../docs/docker_compose_db_redis_yaml/docker-compose.yml`](../docs/docker_compose_db_redis_yaml/docker-compose.yml)
（MongoDB 映射到宿主端口 `27037`、Redis `6379`、MySQL `3306`，root 密码 `root`）。
它仅供本地开发便利，并非生产配置。

### 配置从何处加载

两个可运行入口都从仓库根目录的 `example_config/` 目录加载配置（**而非**
`example_application/` 内部的目录）：

- HTTP 服务入口 `example_main/main.go` 在其 `fiberhouse.BootConfig` 中设置
  `ConfigPath: "./example_config"`（相对于运行该二进制的仓库根目录）。
- CLI 入口 `example_application/command/main.go` 调用
  `bootstrap.NewConfigOnce("./../../example_config")`（相对于
  `example_application/command/`，同样解析到仓库根目录的 `example_config/`）。

`example_config/` 根据 `application.env` 设置选择 `application_{test,dev,prod}.yml`
（环境变量覆盖方案见 [`../example_config/README.md`](../example_config/README.md)）。
与上文 Compose 文件匹配的相关键及其默认值为：

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

模块级的测试默认值及其环境变量覆盖（`FIBERHOUSE_MONGODB_URI`、`FIBERHOUSE_MYSQL_DSN`、
`FIBERHOUSE_REDIS_ADDR` 等）在上文链接的各模块 README 中说明。

## HTTP CRUD 路由

Fiber 适配器（`module/example-module`）与 Gin 适配器（`module/example-ginapi-module`）
都在应用根路径上注册完全相同的路由集，无路径前缀（已在各模块的
`api/register_api_router.go` 中核实）：

```text
POST   /examples
GET    /examples/:id
GET    /examples?page=1&page_size=20&status=active
PUT    /examples/:id
DELETE /examples/:id
```

同一时刻只运行一个核心引擎——`CoreType` 在 `example_main/main.go` 的
`fiberhouse.BootConfig` 中选择（`fiber` 或 `gin`）。无论哪个引擎在运行，上述契约都完全
一致：相同的请求 VO、相同的 `response.RespInfo{code,msg,data}` 信封、相同的校验与
错误状态码映射。请求示例：

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

进行交互式/手动测试时，使用 `.http` 文件（VS Code REST Client / JetBrains HTTP Client
格式）而非手写 curl：

- [`module/example-module/api/example_api.http`](module/example-module/api/example_api.http) ——
  Fiber 适配器。
- [`module/example-ginapi-module/api/example_api.http`](module/example-ginapi-module/api/example_api.http) ——
  Gin 适配器。
- [`api-tests.http`](api-tests.http) —— 指向上述两个文件的便捷索引（二者覆盖相同的 5 个
  接口；针对 `http://localhost:8080` 一次运行一个适配器）。

## MySQL CLI（`example` 命令）

CLI 独立于 HTTP 层，通过自己的 `ExampleMysqlService -> ExampleRepository -> GORM` 栈与
MySQL 交互。它从 `example_application/command/` 运行，CLI 的 `main.go` 就在那里：

```bash
cd example_application/command
go run . example migrate
go run . example create --name alpha --description first --status active
go run . example get --id 1
go run . example list --page 1 --page-size 20 --status active
go run . example update --id 1 --status archived
go run . example delete --id 1
```

这六个子命令（`migrate`、`create`、`get`、`list`、`update`、`delete`）注册于
`example_application/command/application/commands/test_orm_command.go` 的
`NewExampleCommand`/`exampleCommand.GetCommand`，并经
`example_application/command/application/application.go` 中的
`commands.NewExampleCommand(app.Ctx)` 接入 CLI 应用。所有命令都把结果以 JSON 写到
stdout。在任何其他子命令之前先运行一次 `migrate`——它会创建/更新 `example_records` 表。
表结构、集成测试默认值与 DSN 覆盖（`FIBERHOUSE_MYSQL_DSN`）见
[`module/command-module/README.md`](module/command-module/README.md)。

## Swagger / OpenAPI 文档

Swagger UI 由配置控制，并非始终开启。要启用它，在生效的 `example_config/
application_{env}.yml` 中设置 `application.swagger.enable: true`（在
`providers/module/fiber_route_register.go` 的 `RegisterFiberSwagger` 与
`providers/module/gin_route_register.go` 的 `RegisterGinSwagger` 中检查，二者都读取
`ctx.GetConfig().Bool("application.swagger.enable")`）。启用后，UI 服务于：

```text
GET /swagger/*
```

检入的 `docs/doc.go` 是一个可编译的**占位符**，带一个空的 `paths` 文档——它不是真正
生成的规范。要从 Fiber handler 上的 swaggo 注解重新生成真实文档（只有 Fiber 带注解；
Gin 被记录为行为一致——见下方原因），安装 `swag` 并从仓库根目录运行所提供的脚本：

```bash
go install github.com/swaggo/swag/cmd/swag@latest
bash example_application/generate_swagger.sh
```

关于为何只注解 Fiber、为何 `doc.go` 是占位符，以及通用 Swagger 注解
（`@title`/`@host`/`@BasePath`，来源于 `example_main/main.go`）如何配合的完整原因，见
[`docs/README.md`](docs/README.md)——重新生成前请先阅读它。

## 运行测试

单元测试（无需外部服务）：

```bash
go test ./example_application/... -count=1
```

集成测试为可选启用，需要上文的真实本地服务。它们默认被跳过，仅当设置
`FIBERHOUSE_INTEGRATION=1` 时才运行（直接在各模块的 `integration_test.go` 中检查，
例如 `module/example-module/integration_test.go` 与
`module/command-module/integration_test.go`）：

```bash
FIBERHOUSE_INTEGRATION=1 go test \
  ./example_application/module/example-module \
  ./example_application/module/command-module -count=1
```

集成测试会创建唯一命名/带时间戳的数据，并只清理自己创建的部分。连接目标的按模块环境
变量覆盖（`FIBERHOUSE_MONGODB_URI`、`FIBERHOUSE_MYSQL_DSN`、`FIBERHOUSE_REDIS_ADDR`
等）在上文链接的模块 README 中说明。
