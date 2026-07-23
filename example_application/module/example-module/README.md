# Canonical example module

This is the transport-independent MongoDB CRUD slice used by both HTTP
adapters. Dependencies point inward:

```text
Fiber/Gin handler -> ExampleUseCase -> ExampleStore -> ExampleModel -> MongoDB
```

The entity stores a MongoDB `ObjectID`, `name` (unique), optional
`description`, `status` (`active` or `archived`), string `tags`, and UTC
`created_at`/`updated_at` fields. HTTP DTOs keep BSON types out of the public
JSON contract.

## Routes

Both engines expose the same contract:

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

List reads use normalized page, page-size, and status values in the cache key.
The level-two cache is read-through with a 30-second TTL and the caller's
context. Mutations emit a best-effort `example:changed` task only after the
database write succeeds. There is no prefix invalidation; freshness is bounded
by TTL expiry.

## Local services and tests

Defaults match the repository's Docker Compose services:

- MongoDB: `mongodb://admin:admin@127.0.0.1:27037/?authSource=admin`
- Redis: `127.0.0.1:6379`

Override them with `FIBERHOUSE_MONGODB_URI`,
`FIBERHOUSE_MONGODB_DATABASE`, `FIBERHOUSE_MONGODB_COLLECTION`, and
`FIBERHOUSE_REDIS_ADDR`.

```bash
go test ./example_application/module/example-module/... -count=1
FIBERHOUSE_INTEGRATION=1 go test ./example_application/module/example-module -count=1
```

Integration data and Redis keys contain a timestamp and process ID. Cleanup
targets only the document and key created by the test.
