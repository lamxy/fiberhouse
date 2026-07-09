# CodeGraph Q&A: Web 樣例路由 `/get/:id` 到數據模型全鏈路

## 問題

使用 codegraph mcp，探索接口 `/get/:id` 路由到數據庫模型的路徑。

## 回答

使用 CodeGraph MCP 追蹤後，這個接口在 Fiber 下完整路徑是 `/example/get/:id`，在 Gin 下也有一條對應的 `/gin/example/get/:id`。

### Fiber 路由入口

`GET /example/get/:id`

入口位於 `example_application/module/example-module/api/register_api_router.go:10` 的 `RegisterRouteHandlers`。

關鍵註冊點：

```go
exampleGroup := app.Group("/example")
exampleGroup.Get("/get/:id", exampleApi.GetExample).Name("ex_get_example")
```

### 依賴注入鏈路

路由 handler 由 Wire 生成代碼建立，位於 `example_application/module/example-module/api/api_provider_wire_gen.go:18`。

依賴建立順序：

```text
InjectExampleApi(ctx)
  -> model.NewExampleModel(ctx)
  -> repository.NewExampleRepository(ctx, exampleModel)
  -> service.NewExampleService(ctx, exampleRepository)
  -> NewExampleHandler(ctx, exampleService)
```

### Handler 到 Service

Handler 位於 `example_application/module/example-module/api/example_api.go:85`。

`ExampleHandler.GetExample` 的流程：

1. 從 header 讀語言：`X-Language`，默認 `en`
2. 從路由參數取 `id := c.Params("id")`
3. 用 `requestvo.ObjId` 做 validator 校驗
4. 呼叫 `h.Service.GetExample(id)`
5. 成功後用 `fiberhouse.Response().SuccessWithData(resp).SendWithCtx(...)` 返回

核心調用點：

```go
resp, err := h.Service.GetExample(id)
```

### Service 到 Repository

Service 位於 `example_application/module/example-module/service/example_service.go:75`。

核心調用：

```go
example, err := s.Repo.GetExampleById(id)
```

之後把 `entity.Example` 映射成 `responsevo.ExampleRespVo`，字段包括：

- `ExamName`
- `ExamAge`
- `Courses`
- `Profile`
- `CreatedAt`
- `UpdatedAt`

### Repository 到 Model

Repository 位於 `example_application/module/example-module/repository/example_repository.go:46`。

核心流程：

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := r.Model.GetExampleByID(ctx, id)
```

錯誤處理：

- `mongo.ErrNoDocuments` 轉成 `exception.GetNotFoundDocument()`
- 其他錯誤會 `exception.GetInternalError().RespData(err.Error()).Panic()`

### Model 到 MongoDB

Model 位於 `example_application/module/example-module/model/example_model.go:43`。

`ExampleModel.GetExampleByID` 的流程：

1. `bson.ObjectIDFromHex(oid)` 把路由 id 轉 Mongo ObjectID
2. filter 是 `bson.D{{"_id", _id}}`
3. projection 排除 `_id` 和 `profile`
4. 執行 `FindOne(...).Decode(&example)`

核心查詢：

```go
filter := bson.D{{"_id", _id}}
opts := options.FindOne().SetProjection(bson.M{
    "_id":     0,
    "profile": 0,
})

var example entity.Example
err = m.GetCollection(m.GetColl()).FindOne(ctx, filter, opts).Decode(&example)
```

### Model 初始化與 Mongo Collection

`ExampleModel` 初始化位於 `example_application/module/example-module/model/example_model.go:22`。

```go
dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).
    SetDbName(constant.DbNameMongo).
    SetTable(constant.CollExample).
    SetName(GetKeyExampleModel())
```

底層 collection 取得邏輯位於 `database/dbmongo/mongo_model_impl.go:126`：

```go
return mo.Db.Client.Database(mo.dbName).Collection(coll, opts...)
```

### 完整鏈路

```text
GET /example/get/:id
  -> RegisterRouteHandlers()
  -> ExampleHandler.GetExample()
  -> ExampleService.GetExample(id)
  -> ExampleRepository.GetExampleById(id)
  -> ExampleModel.GetExampleByID(ctx, id)
  -> MongoModel.GetCollection(coll)
  -> mongo.Collection.FindOne(ctx, {"_id": ObjectID(id)}, projection)
  -> Decode(&entity.Example)
```

### Gin 對應路由

Gin 版本入口位於 `example_application/module/example-ginapi-module/api/register_api_router.go:8`。

對應路由：

```go
exampleGroup := app.Group("/gin/example")
exampleGroup.GET("/get/:id", exampleApi.GetExample)
```

Gin handler 位於 `example_application/module/example-ginapi-module/api/example_api.go:89`，後續 service、repository、model 鏈路與 Fiber 版本共用同一套 example-module service/repository/model。

## 補充觀察

`ExampleModel.GetExampleByID` 的 projection 排除了 `profile`，但 `ExampleService.GetExample` 又把 `example.Profile` 填進 response。這意味著該接口返回的 `Profile` 很可能是空值，除非其他地方有額外填充。
