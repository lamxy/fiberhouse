# CodeGraph Q&A: Recent Notes

## Q: 哪一個路由調用了 `CreateExample`？

Fiber:

- `POST /example/create`
- 註冊點：`example_application/module/example-module/api/register_api_router.go:29`
- Handler：`example_application/module/example-module/api/example_api.go:174`

Gin:

- `POST /gin/example/create`
- 註冊點：`example_application/module/example-ginapi-module/api/register_api_router.go:25`
- Handler：`example_application/module/example-ginapi-module/api/example_api.go:187`

## Q: `ExampleService` 的節點/上下文是什麼？

位置：`example_application/module/example-module/service/example_service.go:18`

角色：

- 位於 API handler 與 repository/model 之間。
- 依賴 `ExampleRepository`，由 Wire 構造注入。
- 繼承 `fiberhouse.ServiceLocator`，可取得 context/config/logger/container。

主要方法：

- `GetExample(id)` -> `Repo.GetExampleById(id)` -> 映射 `ExampleRespVo`
- `GetExampleWithTaskDispatcher(id)` -> 查詢 example 後推送 asynq 延遲任務
- `CreateExample(vo)` -> `Repo.CreateExample(vo)`
- `GetExamples(page, size)` -> `cache.GetCached(...)` -> `Repo.GetExamples(...)`

注意：

- `GetExample()` 返回 `Profile`，但 `ExampleModel.GetExampleByID()` projection 排除了 `profile`。
- `GetExampleWithTaskDispatcher()` 中 dispatcher/task 建立失敗後仍可能繼續 enqueue，錯誤處理值得檢查。

## Q: 可以使用 `codegraph node ExampleService` 嗎？與 MCP 探索有何區別？

可以。本地 CLI 支援：

```bash
codegraph node ExampleService
```

效果：

- 適合查看單一符號節點。
- 對 `ExampleService` 會列出 struct 位置、方法列表、caller trail。
- 若要看具體方法 body，可查 `codegraph node ExampleService.GetExample`。

與 `codegraph_explore` 差異：

- `codegraph node <symbol>`：單點上下文，窄、快、像符號跳轉。
- `codegraph explore "<question>"` / MCP `codegraph_explore`：面向問題或鏈路，會拉多個相關符號、文件、調用路徑與影響面。

## Q: 請求生命周期如何從路由器流轉至持久化層？

啟動期：

```text
FiberHouse.RunServer()
  -> WebApplication.RegisterModuleInitialize(...)
  -> CoreWithFiber.RegisterModuleInitialize(...)
  -> Module.RegisterModuleRouteHandlers(...)
  -> RegisterRouteHandlers(...)
  -> exampleGroup.Get/Post(...)
```

運行期，以 `GET /example/get/:id` 為例：

```text
HTTP request
  -> Fiber router
  -> ExampleHandler.GetExample
  -> ExampleService.GetExample
  -> ExampleRepository.GetExampleById
  -> ExampleModel.GetExampleByID
  -> MongoModel.GetCollection
  -> mongo.Collection.FindOne
  -> Decode(&entity.Example)
  -> response VO
  -> HTTP response
```

核心文件：

- `boot.go`: `RunServer`
- `core_fiber_starter_impl.go`: `RegisterModuleInitialize`
- `example_application/module/module_impl.go`: `RegisterModuleRouteHandlers`
- `example_application/module/example-module/api/register_api_router.go`: route registration
- `example_application/module/example-module/api/example_api.go`: handler
- `example_application/module/example-module/service/example_service.go`: service
- `example_application/module/example-module/repository/example_repository.go`: repository
- `example_application/module/example-module/model/example_model.go`: model
- `database/dbmongo/mongo_model_impl.go`: Mongo collection access
