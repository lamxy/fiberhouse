# Gin example adapter

This directory is a Gin transport adapter, not a second business module. It
binds and validates Gin requests, passes `c.Request.Context()` to the canonical
`example-module` use case, and maps domain errors to HTTP responses.

Gin deliberately reuses the canonical service, repository, entity, and MongoDB
model. That keeps field rules, status semantics, pagination, cache behavior,
task dispatch, and error translation identical to the Fiber adapter. Only the
framework-specific binding and response code differs.

The routes are `POST /examples`, `GET /examples/:id`,
`GET /examples?page=1&page_size=20&status=active`, `PUT /examples/:id`, and
`DELETE /examples/:id`. Full request examples and local-service requirements
are in [`../example-module/README.md`](../example-module/README.md).
