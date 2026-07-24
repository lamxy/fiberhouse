# 规范 example 模块

这是两个 HTTP 适配器共用的、与传输层无关的 MongoDB CRUD 切片。依赖指向内部：

```text
Fiber/Gin handler -> ExampleUseCase -> ExampleStore -> ExampleModel -> MongoDB
```

entity 存储一个 MongoDB `ObjectID`、`name`（唯一）、可选的 `description`、
`status`（`active` 或 `archived`）、字符串 `tags`，以及 UTC 的
`created_at`/`updated_at` 字段。HTTP DTO 把 BSON 类型隔离在对外的 JSON 契约之外。

## 路由

两个引擎暴露相同的契约：

```text
POST   /examples
GET    /examples/:id
GET    /examples?page=1&page_size=20&status=active
PUT    /examples/:id
DELETE /examples/:id
```

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

列表读取在缓存键中使用规范化后的 page、page-size 与 status 值。二级缓存为
read-through，TTL 为 30 秒，并使用调用方的上下文。写操作仅在数据库写入成功后才尽力
（best-effort）发出一个 `example:changed` 任务。没有前缀失效机制；数据新鲜度由 TTL
到期界定。

两个 HTTP 适配器对规范领域失败的映射完全一致：输入或 ID 非法返回 400，example 不存在
返回 404，冲突或内容未变化的更新返回 409。未知错误保留框架既有的 500 信封。

## 本地服务与测试

默认值与仓库的 Docker Compose 服务匹配：

- MongoDB: `mongodb://admin:admin@127.0.0.1:27037/?authSource=admin`
- Redis: `127.0.0.1:6379`

用 `FIBERHOUSE_MONGODB_URI`、`FIBERHOUSE_MONGODB_DATABASE`、
`FIBERHOUSE_MONGODB_COLLECTION` 与 `FIBERHOUSE_REDIS_ADDR` 覆盖它们。

```bash
go test ./example_application/module/example-module/... -count=1
FIBERHOUSE_INTEGRATION=1 go test ./example_application/module/example-module -count=1
```

集成测试数据与 Redis 键包含时间戳与进程 ID。清理只针对测试所创建的文档与键。
