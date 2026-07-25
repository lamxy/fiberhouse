# Gin example 适配器

本目录是一个 Gin 传输适配器，而非第二个业务模块。它绑定并校验 Gin 请求，把
`c.Request.Context()` 传给规范的 `example-module` 用例，并把领域错误映射为 HTTP 响应。

Gin 刻意复用规范的 service、repository、entity 与 MongoDB model。这使得字段规则、状态
语义、分页、缓存行为、任务派发与错误翻译都与 Fiber 适配器完全一致。只有框架特有的
绑定与响应代码不同。

路由为 `POST /examples`、`GET /examples/:id`、
`GET /examples?page=1&page_size=20&status=active`、`PUT /examples/:id` 与
`DELETE /examples/:id`。完整的请求示例与本地服务要求见
[`../example-module/README.md`](../example-module/README.md)。
