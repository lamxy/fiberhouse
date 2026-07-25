# example_application Swagger / OpenAPI 文档

本目录存放 `example_application` HTTP API（`/examples` CRUD 资源）的
swaggo/swag 生成的 OpenAPI 文档。它附带一个**可编译的占位符**（`doc.go`），使模块
在从未运行过 `swag init` 的情况下也能干净地 build 与 vet。

## 目录内容

- `doc.go` —— 手写的占位符，实现与 `swag init` 自身生成物相同的包/变量形态
  （`package docs`、`var SwaggerInfo *swag.Spec`、调用 `swag.Register` 的 `func init()`）。
  它以 `"swagger"` 实例名注册一个空（`"paths": {}`）的 OpenAPI 文档，使已接线的
  Swagger UI 处理器（`example_application/providers/module/fiber_route_register.go` 的
  `RegisterFiberSwagger` 与 `gin_route_register.go` 的 `RegisterGinSwagger`，二者都由
  `application.swagger.enable` 配置开关控制）即便在生成之前也有有效内容可供服务。
- `README.md` —— 本文件。
- `../generate_swagger.sh` —— 用于真正重新生成本包（从下述注解解析出完整
  paths/definitions）的、**不会被执行**的确切 `swag init` 命令。

`swag init` 会用同样形态的生成文件（同包、同 `SwaggerInfo` 变量）**覆盖 `doc.go`**，
但其 `paths`/`definitions` 会填充为源自 handler 上 `@...` 注解的内容。这是预期行为
——不要手改生成产物；应改源注解后重新生成。

## 如何真正重新生成

1. 安装 swag CLI（本任务不安装/不运行）：
   ```bash
   go install github.com/swaggo/swag/cmd/swag@latest
   ```
2. 从仓库根目录运行生成脚本：
   ```bash
   bash example_application/generate_swagger.sh
   ```
   它执行（权威、确切的调用见该文件）：
   ```bash
   swag init \
     -g example_main/main.go \
     -d . \
     -o example_application/docs \
     --parseDependency --parseInternal \
     --parseDepth 2
   ```
3. 验证无误后，提交重新生成的 `doc.go`（如需一并检入，还有
   `swagger.json`/`swagger.yaml`）。

## 决策与理由（重新生成前必读）

### 1. Fiber vs Gin：由哪个适配器持有 swaggo 注解

`example_application` 在 Fiber 与 Gin 两个适配器上注册**完全相同的路由**——
`POST /examples`、`GET /examples/{id}`、`GET /examples`、`PUT /examples/{id}`、
`DELETE /examples/{id}`——两侧都无路径前缀（已在
`module/example-module/api/register_api_router.go` 与
`module/example-ginapi-module/api/register_api_router.go` 中核实）。swaggo/swag 会在
整个扫描树中解析 `@Router` 注解并按 `method + path` 去重；给两个适配器标注相同的
`@Router` 值会在 `swag init` 期间产生重复路由错误/警告，即便被抑制，也会得到一份
"哪个 handler 才是权威"含糊不清的规范。

**决策：只有 Fiber handler
（`example_application/module/example-module/api/example_api.go`）携带完整 swaggo
注解**（`@Summary/@Description/@Tags/@Accept/@Produce/@Param/@Success/@Failure/@Router/@ID`）。
Gin handler（`example_application/module/example-ginapi-module/api/example_api.go`）
只获得交叉引用 Fiber 规范的普通 Go doc 注释，**不带** `@Router`/swaggo 标签，因此
`swag init` 对每条路径恰好只有一个权威来源、不发生冲突。

选择以 Fiber 而非 Gin 作为权威来源、或不采用按适配器区分 `@ID`/区分路径的理由：
- 仅靠区分 `@ID` 无法解决冲突：swag 在生成规范中按 `method+path` 索引 `paths`，因此
  同一 `@Router /examples [post]` 上的两个操作无论 `@ID` 如何仍会冲突。
  给每个适配器一条不同的合成路径（如 `/fiber/examples` vs `/gin/examples`）被否决，
  因为那会歪曲两个适配器实际在应用根提供的、真实且相同的路由——违反"不臆造
  路由/前缀"。
- Fiber 是 FiberHouse 的主/默认核心（框架中的 `CoreTypeWithDefault`，而 Gin 适配器在
  整个 `example_application/providers` 中被呈现为一个可替换/可插拔的核心），因此它是
  更自然的单一文档化契约。
- 两个适配器共享相同的请求/响应 VO、相同的 `service.ExampleUseCase`、相同的
  `transport.MapDomainError` 映射——它们在 HTTP 边界上契约完全一致（相同 JSON 形态、
  相同状态码、相同校验规则）。文档化其中一个足以描述二者；这一点在 Gin handler 的
  doc 注释中已明确声明。

**若你在 build/run 时切换为仅 Gin，生成的规范仍准确描述线上契约**，因为两个适配器
行为一致——只有带注解的源文件不同。

### 2. `docs/doc.go` 的可编译处理方式

**决策：一个精简的手写占位符包**（本 `doc.go`），镜像 `swag init` 自身输出的形态
（真实生成参考见 `example_main/docs/docs.go`），但用一个空的
`"paths": {}` / `"definitions": {}` 文档替代完整规范。

考虑并否决的备选：
- *把 blank import 藏在 build tag 后*（例如仅当传入 `swagger` tag 时才编译）——否决，
  因为那意味着 docs 包（及其 blank import）对普通的 `go build ./...` 不可见，违背了
  "接线后 Swagger UI 即刻可用"的目标，并引入了本模板别处未使用的 build-tag 概念。
- *提供一份与每条注解匹配的完全手写规范*——否决，因为它是重复、易漂移的工作；swaggo
  的全部意义就在于规范由注解派生，手动同步两者只会比占位符方式更糟。

占位符如今即可满足 `go build ./...` / `go vet ./...`，并在有人真正运行
`generate_swagger.sh` 的瞬间被静默且完全替换——无需任何手动清理步骤。

### 3. 通用注解（`@title/@version/@host/@BasePath`）与入口接线

`example_application` **没有自己独立的 HTTP 服务 `main` 包**——它是一个
providers/modules/handlers 的库。真正启动 HTTP 服务、提供 `example_application` 路由
（Fiber 与 Gin）并挂载 Swagger UI 的进程是 `example_main/main.go`，它已携带：
```go
_ "github.com/lamxy/fiberhouse/example_main/docs" // swagger docs
// @title XXX Service APIs
// @version 1.0
// ...
func main() { ... }
```
该文件是为**旧的**（CRUD 重构之前）API 生成的、明确只读的参考材料，本任务不在其修改
范围内。

**决策：不要给 `example_main/main.go` 添加 `example_application/docs` 的 blank import，
也不要修改其通用注解。** 原因：
- `swag.Register` 在一个进程级的单一 map 中按 `InstanceName`（默认：`"swagger"`）索引
  文档（`github.com/swaggo/swag` 的 `swagger.go`）。若 `example_main/main.go` 同时
  blank-import `example_main/docs` 与 `example_application/docs`，第二个包的 `init()`
  会静默覆盖第一个针对同一实例名的注册——在没有任何编译或运行时错误提示的情况下，
  破坏旧 API 那套已在工作的 Swagger UI。
- brief 将 `example_main/main.go` 标为只读参考；修改它有让一个无关任务已验证的行为
  发生回退的风险。

**通用注解应放在何处：** 当你为 `example_application` 构建真正独立的入口（或改造
`example_main/main.go` 使其服务新的 CRUD API 而非旧的），把 swaggo 通用注解放在那里，
例如：
```go
package main

import (
    _ "github.com/lamxy/fiberhouse/example_application/docs" // swagger docs
)

// @title       FiberHouse Example Application API
// @version     1.0
// @description CRUD API for the example resource (Fiber primary; Gin mirrors the same contract).
// @host        localhost:8080
// @BasePath    /
// @schemes     http https
func main() { ... }
```
并把 `generate_swagger.sh` 的 `-g` 标志指向那个文件，而非 `example_main/main.go`。
在这样的入口存在之前，`generate_swagger.sh` 使用 `example_main/main.go` 作为 `-g`
目标，纯粹是为了取得 `@title/@version/@host/@BasePath` 通用注解（swag 要求恰好一个
`-g` 入口文件来提供这些）；生成规范的 `paths`/`definitions` 仍完全由
`example_application` 中 Fiber handler 上的 `@Router`/`@Success`/… 注解驱动，经
`--parseInternal` 在 `-d .`（仓库根）上扫描得到。

## 响应信封参考

所有成功与失败响应都包裹在框架的标准信封 `response.RespInfo`
（`github.com/lamxy/fiberhouse/response`）中：
```go
type RespInfo struct {
    Code int         `json:"code"`
    Msg  string      `json:"msg"`
    Data interface{} `json:"data"`
}
```
`fiberhouse.Response().SuccessWithData(resp).JsonWithCtx(...)` 返回
`Code: 0, Msg: "ok", Data: resp`。经 `transport.MapDomainError` 返回的失败
（Fiber 侧 `fiber.NewError(status, msg)` / Gin 侧 `c.Error(err)`）会经框架的错误处理器
渲染进同一个 `RespInfo` 信封，带非零 `Code` 与映射后的 HTTP 状态。swaggo 的
`@Success`/`@Failure` 注解相应地引用 `response.RespInfo{data=...}`，其中 2xx 情形以
`responsevo.ExampleRespVo` / `responsevo.ExampleListRespVo` 作为泛型 `data` 载荷。
