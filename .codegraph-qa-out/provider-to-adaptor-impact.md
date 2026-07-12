# `provider/` → `adaptor/`、`provider/adaptor/` → `adaptor/errorhandler/` 影響分析與設計

## 已批准的目標

本次只做根目錄 package 路徑及其必要名稱對齊：

```text
provider/context/  -> adaptor/context/
provider/adaptor/  -> adaptor/errorhandler/
```

採用直接破壞式搬移，不保留 `provider/context` 或 `provider/adaptor` 相容 shim。

明確排除：

- 暫不更新 `README.md` 與 `docs/*.md`；這些文件存在延遲和部分錯誤，後續另行專題整理。
- 不重命名 root package 的 `Provider`、`IProvider`、`ProviderManager`、`ProviderType`、`ProviderLocation` 等 provider 架構概念。
- 不重命名 `example_application/providers/`；它是應用範例的 provider 集合，不是本次根目錄 `provider/` package。
- 不順帶補齊低覆蓋率測試，也不修復既有 `bootstrap`、`component/writer` 測試失敗。
- 不改動與此次路徑搬移無關的檔名、型別名、錯誤訊息或程式行為。

## 結論與範圍統計

CodeGraph、`ast-grep` 與精確字串搜尋交叉確認後，本倉庫的產品碼必改範圍是 24 個 Go 檔：

| 類別 | 數量 |
| --- | ---: |
| 搬移的 Go 定義檔 | 5 |
| 目標目錄外直接消費 Go 檔 | 19 |
| 必改 Go 檔合計 | 24 |
| 舊 context import 使用檔 | 19（其中 2 個位於被搬移的 error handler package） |
| 舊 error handler import 使用檔 | 2 |
| 本次更新的分析文檔 | 1 |
| README / `docs/*.md` | 0（明確排除） |

`ast-grep` 找到：

- `github.com/lamxy/fiberhouse/provider/context`：19 個 Go 檔。
- `github.com/lamxy/fiberhouse/provider/adaptor`：2 個 Go 檔。

CodeGraph 找到的公開符號 blast radius：

| 符號 | 倉庫內直接呼叫/引用 | 直接測試覆蓋 |
| --- | ---: | --- |
| `ICoreContext` | 30 | 未找到 |
| `WithGinContext` | 11 | 未找到 |
| `WithFiberContext` | 10 | 未找到 |
| `GinErrorHandler` | 1 | 未找到 |
| `FiberErrorHandler` | 1 | 未找到 |

其中 `ICoreContext` 已進入 root package、`response`、`exception` 和範例 API 的公開或半公開簽名，因此 context package identity 的破壞面大於 error handler。

## 目標 package 與命名

### `adaptor/context`

保留 `package context` 和下列公開符號，不修改行為：

- `ICoreContext`
- `FiberContext`
- `GinContext`
- `WithFiberContext`
- `WithGinContext`

消費端統一使用 alias，避免與標準庫 `context` 混淆：

```go
adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
```

### `adaptor/errorhandler`

兩個檔案的 package clause 由 `package adaptor` 改為 `package errorhandler`。root starter 明確使用 import alias：

```go
adaptorerrorhandler "github.com/lamxy/fiberhouse/adaptor/errorhandler"
```

依已批准的精確 selector：

```go
adaptorerrorhandler.FiberErrorHandler
adaptorerrorhandler.GinErrorHandler
```

`GinErrorHandler` 與 `FiberErrorHandler` 均保留原公開名稱；本次不增加 import path 搬移以外的導出符號改名。

## 搬移的 5 個定義檔

| 現有檔案 | 搬移後 | 必要調整 |
| --- | --- | --- |
| `provider/context/core_ctx_wrap_interface.go` | `adaptor/context/core_ctx_wrap_interface.go` | 只搬移；保留 `package context` 與公開介面 |
| `provider/context/core_ctx_wrap_fiber_impl.go` | `adaptor/context/core_ctx_wrap_fiber_impl.go` | 只搬移；保留公開符號與行為 |
| `provider/context/core_ctx_wrap_gin_impl.go` | `adaptor/context/core_ctx_wrap_gin_impl.go` | 只搬移；保留公開符號與行為 |
| `provider/adaptor/fiber_error_handler.go` | `adaptor/errorhandler/fiber_error_handler.go` | `package errorhandler`；context import 改新路徑和 alias |
| `provider/adaptor/gin_error_handler.go` | `adaptor/errorhandler/gin_error_handler.go` | `package errorhandler`；context import 改新路徑和 alias；保留 `GinErrorHandler` |

新依賴方向維持：

```text
adaptor/errorhandler -> adaptor/context
```

`adaptor/context` 不反向依賴 `adaptor/errorhandler`，不會引入 import cycle。

## 目標目錄外的 19 個直接消費檔

### Root package：10 個

| 檔案 | 使用面 | 修改 |
| --- | --- | --- |
| `core_fiber_starter_impl.go` | `adaptor.FiberErrorHandler` | import 改為 `adaptor/errorhandler` 並 alias `adaptorerrorhandler`；selector 改為 `adaptorerrorhandler.FiberErrorHandler` |
| `core_gin_starter_impl.go` | `adaptor.GinErrorHandler` | import 改為 `adaptor/errorhandler` 並 alias `adaptorerrorhandler`；selector 改為 `adaptorerrorhandler.GinErrorHandler` |
| `ctx_core_adapter.go` | `ICoreContext` | import 改為 `adaptor/context`；alias/selector 統一為 `adaptorctx` |
| `ctx_fiber_adaptor_provider.go` | `WithFiberContext` | 同上 |
| `ctx_gin_adaptor_provider.go` | `WithGinContext` | 同上 |
| `recover_config.go` | `ICoreContext` | 同上 |
| `recover_error_handler_impl.go` | `ICoreContext` | 同上 |
| `recover_interface.go` | `ICoreContext` | 同上 |
| `recover_recoveries_impl.go` | `ICoreContext`、`WithFiberContext`、`WithGinContext` | 同上 |
| `response.go` | `ICoreContext` | 同上 |

### `response` package：4 個

- `response/response_interface.go`
- `response/response_impl.go`
- `response/response_proto_impl.go`
- `response/response_msgpack_impl.go`

四個檔案全部改用 `adaptor/context` 和 `adaptorctx`。`response.IResponse` 的公開方法含 `ICoreContext`，外部實作者必須跟隨新 import path 重新編譯。

### `exception` package：1 個

- `exception/exception_error.go`：更新 import path、alias，以及 `Exception` / `ValidateException` 的 `JsonWithCtx`、`SendWithCtx` selector。

### 範例 API：4 個

- `example_application/module/example-ginapi-module/api/common_api.go`
- `example_application/module/example-ginapi-module/api/example_api.go`
- `example_application/module/example-module/api/common_api.go`
- `example_application/module/example-module/api/example_api.go`

四個檔案改用 `adaptor/context` 和 `adaptorctx.WithGinContext` / `adaptorctx.WithFiberContext`。

## Package 依賴變化

搬移前：

```text
fiberhouse root --------------------> provider/adaptor
       |                                      |
       +--------------------------------------+--> provider/context
exception -----------------------------------> provider/context
response ------------------------------------> provider/context
example .../example-ginapi-module/api -------> provider/context
example .../example-module/api --------------> provider/context
```

搬移後：

```text
fiberhouse root --------------------> adaptor/errorhandler
       |                                      |
       +--------------------------------------+--> adaptor/context
exception -----------------------------------> adaptor/context
response ------------------------------------> adaptor/context
example .../example-ginapi-module/api -------> adaptor/context
example .../example-module/api --------------> adaptor/context
```

## 不應被機械改名的項目

以下名稱仍描述 provider 機制或 adaptor 角色，不屬於目錄 import path：

- `Provider`、`IProvider`、`ProviderManager`、`ProviderType`、`ProviderLocation` 等 root API。
- `example_application/providers/` 及其子目錄。
- `ctx_fiber_adaptor_provider.go`、`ctx_gin_adaptor_provider.go`。
- `CoreCtxFiberProvider`、`CoreCtxGinProvider`。
- 錯誤訊息中的 `provider` 或 `core context provider`。

對整個 `provider` 字詞做全倉替換會破壞框架核心概念，是高風險錯誤操作。

## 文件、生成資料與運行產物

- 不更新 `README.md` 與 `docs/*.md`，即使其中仍保留 `./provider/adaptor/*.go` 舊連結；這是已知、刻意保留的後續文檔債務。
- 更新本分析文檔，使 `.codegraph-qa-out/` 保存最新決策與準確影響面。
- 不手工修改 `.codegraph/codegraph.db*`、daemon log/pid；檔案搬移後由 CodeGraph watcher 或後續索引更新處理。
- 不修改 `example_main/logs/` 中包含舊 stack trace 路徑的歷史運行日誌。
- 不修改 `graphify-out/`；`CLAUDE.md` 已明確禁止依賴 graphify 工作流。

## 詳細修改步驟

1. 在獨立 worktree 分支中確認工作區乾淨和測試基線。
2. 以 `git mv provider adaptor` 搬移根目錄，保留 Git rename history。
3. 以 `git mv adaptor/adaptor adaptor/errorhandler` 解決搬移後的雙層 `adaptor/adaptor`。
4. 將 `adaptor/errorhandler/*.go` 的 package clause 改為 `package errorhandler`。
5. 保留 `GinErrorHandler` 與 `FiberErrorHandler` 的原公開名稱。
6. 更新兩個 error handler 檔案內部的 context import 為 `adaptor/context`，alias 統一為 `adaptorctx`。
7. 更新 17 個其他 context 消費檔的 import path、alias 和 selector。
8. 更新兩個 starter 的 error handler import，alias 為 `adaptorerrorhandler`，使用已批准的精確 selector。
9. 僅對 24 個受影響 Go 檔執行 `gofmt`；審閱 Git diff，排除無關換行或格式變動。
10. 執行靜態歸零掃描、package 發現、建置和測試基線比較。
11. 用 CodeGraph 查詢新路徑/符號，確認 source/caller 路徑不再指向舊 package；若 watcher 尚未更新，明確記錄索引陳舊而不手改資料庫。

## 驗證契約

### 舊產品碼路徑歸零

```bash
ast-grep run --pattern '"github.com/lamxy/fiberhouse/provider/context"' --lang go .
ast-grep run --pattern '"github.com/lamxy/fiberhouse/provider/adaptor"' --lang go .
rg -n 'github\.com/lamxy/fiberhouse/provider/(context|adaptor)' --glob '*.go'
```

三個命令都應無匹配。README、`docs/*.md` 和本分析中的歷史/遷移文字不納入產品碼歸零判定。

### 新 package 與 selector

```bash
go list ./adaptor/...
rg -n 'adaptorerrorhandler\.(FiberErrorHandler|GinErrorHandler)' \
  core_fiber_starter_impl.go core_gin_starter_impl.go
rg -n '^package errorhandler$' adaptor/errorhandler/*.go
```

預期 `go list` 只列出：

```text
github.com/lamxy/fiberhouse/adaptor/context
github.com/lamxy/fiberhouse/adaptor/errorhandler
```

### 格式、建置與測試

```bash
git diff --check
go build ./...
go build -o ./example_main/target/fhweb ./example_main/main.go
go test ./...
```

完成標準：

- `git diff --check`、兩個 build 命令成功。
- `go test ./...` 不要求全綠，但只能重現改名前基線中的既有失敗：
  - `bootstrap.Test_Config_EnvOverrideAndSingleton` 缺少臨時 `application_dev.yml`。
  - `component/writer` 的日誌檔與 close 語義測試失敗。
- 不允許出現任何舊 import path、package not found、undefined selector、import cycle 或新增失敗 package。

## 主要風險點

1. **公開 import path 破壞**：模組已是 v1，刪除 `provider/context` 和 `provider/adaptor` 會使所有未遷移的外部消費者編譯失敗；本倉庫無法枚舉外部使用者。
2. **公開型別 identity 變化**：`ICoreContext`、`FiberContext`、`GinContext` 搬到新 package path，公開簽名和外部實作需要重新編譯與更新 import。
3. **名稱碰撞**：新 `adaptor/context` 容易與標準庫 `context` 混淆；統一 `adaptorctx` alias 可降低誤用風險。
4. **漏改內部依賴**：兩個被搬移的 error handler 自身也匯入舊 context，若只改外部消費者會導致新 package 無法編譯。
5. **誤改 provider 架構**：機械替換 `provider` 字詞會錯誤修改大量核心 API、範例目錄、檔名和錯誤訊息。
6. **文件暫時失真**：README 與 `docs/*.md` 的舊連結會在此次改名後失效；這是已批准的暫存風險，必須在後續文檔專題追蹤。
7. **CodeGraph 索引陳舊**：worktree 或 watcher 可能延遲更新，查詢可能暫時顯示舊 source path；不能將生成資料手工編輯成新路徑。
8. **既有測試紅燈**：完整測試基線已失敗，驗證必須比較失敗集合，不能將所有紅燈歸因於本次改名，也不能忽略新增編譯失敗。
9. **換行噪音**：目前 checkout 中部分 Go 檔為 CRLF，而索引和 `.gitattributes` 規定 LF；`gofmt` 後需以語義 diff 審閱，避免把無關全檔變動混入改名提交。

## 已記錄的改名前基線

在隔離 worktree、任何產品碼修改前執行 `go test ./...`：

- `go mod download` 成功。
- `go test ./...` 失敗於 `bootstrap` 和 `component/writer`，與 `.codegraph-qa-out/todo.md` 記錄一致。
- 舊 package `github.com/lamxy/fiberhouse/provider/context` 和 `github.com/lamxy/fiberhouse/provider/adaptor` 可被 Go 正常發現。

本次實作應以「建置通過、舊 import 歸零、測試失敗集合不增加」作為核心完成條件。
