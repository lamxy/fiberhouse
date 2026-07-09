# CodeGraph Q&A: `Example.Courses` 改型影響面

## Q

如果改變 `Example.Courses` 的類型，哪些代碼路徑會受影響？

## A

直接受影響的型別：

- `example_application/module/example-module/entity/types.go:13`
  - `Example.Courses []string`
- `example_application/apivo/example/requestvo/example_reqvo.go:11`
  - `ExampleReqVo.Courses []string`
- `example_application/apivo/example/responsevo/example_respvo.go:12`
  - `ExampleRespVo.Courses []string`

主要流轉路徑：

```text
POST /example/create
  -> ExampleHandler.CreateExample
  -> ExampleService.CreateExample
  -> ExampleRepository.CreateExample
  -> entity.Example{Courses: req.Courses}
  -> ExampleModel.SaveExample
  -> Mongo InsertOne

GET /example/get/:id
  -> ExampleHandler.GetExample
  -> ExampleService.GetExample
  -> ExampleRepository.GetExampleById
  -> ExampleModel.GetExampleByID
  -> responsevo.ExampleRespVo{Courses: example.Courses}

GET /example/list
  -> ExampleService.GetExamples
  -> responsevo.ExampleRespVo{Courses: list[i].Courses}
```

需要同步檢查：

- Repository：`example_repository.go:66` 的 `Courses: req.Courses`
- Service：`example_service.go:68`、`:83`、`:127` 的 response 映射
- 測試/示例：`test_service.go:55` 硬編碼 `[]string{"c1", "c2"}`
- Swagger：`example_main/docs/docs.go:249` 目前描述 `courses` 為 `array<string>`
- Validator：`new_tag_hascourses.go` 的 `hascourses` 自定義 tag 要求欄位是 slice
- Mongo 舊資料：舊文檔中的 `courses` 是字串陣列，非兼容改型可能導致 decode 失敗或零值

結論：

不要只改 `entity.Example.Courses`。若要完整改型，需要同步更新 entity、request VO、response VO、repository/service 映射、Swagger、測試資料、validator 假設，以及 Mongo 舊資料兼容策略。
