# `provider/` → `coreadaptor/`、`adaptor/` → `errorhandler/` 影響分析


## 結論

若目標路徑調整為：

```text
provider/context/  -> coreadaptor/context/
provider/adaptor/  -> coreadaptor/errorhandler/
```

則倉庫內的**必改範圍**是 24 個 Go 檔：5 個被搬移的定義檔，以及 19 個位於目標目錄之外的直接消費檔。另有 2 個 README 文件連結要更新。

這不是單純的檔案系統改名。Go package 的 import path 是公開 API 的一部分；刪除舊路徑後，所有仍匯入 `github.com/lamxy/fiberhouse/provider/context` 或 `.../provider/adaptor` 的內外部使用者都會編譯失敗。倉庫外的下游專案無法由本倉掃描得知，因此實際影響可能大於本文列出的本倉範圍。

建議完整對齊後的命名為：

```go
coreadaptorctx "github.com/lamxy/fiberhouse/coreadaptor/context"
errorhandler "github.com/lamxy/fiberhouse/coreadaptor/errorhandler"
```

- `coreadaptor/context` 仍可保留 `package context`；呼叫端以 `coreadaptorctx` alias 避免與標準庫 `context` 混淆，也符合 `coreadaptorctx.WithFiberContext(...)` / `WithGinContext(...)` 的預期用法。
- `coreadaptor/errorhandler` 建議把兩個檔案的 `package adaptor` 一併改為 `package errorhandler`，並將兩個啟動器中的 `adaptor.FiberErrorHandler` / `adaptor.GinErrorHandler` 改為 `errorhandler.*`。

## CodeGraph blast radius

CodeGraph MCP 對目標公開符號的結果：

| 符號 | 直接呼叫/引用 | 測試覆蓋 |
| --- | ---: | --- |
| `ICoreContext` | 30 | 未找到直接覆蓋測試 |
| `WithGinContext` | 11 | 未找到直接覆蓋測試 |
| `WithFiberContext` | 10 | 未找到直接覆蓋測試 |
| `GinErrorHandler` | 1 | 未找到直接覆蓋測試 |
| `FiberErrorHandler` | 1 | 未找到直接覆蓋測試 |

CodeGraph 此次共找到 62 個相關符號。它顯示錯誤處理器各只有一個啟動器呼叫點，而 `ICoreContext` 已進入 root package、`response`、`exception` 與範例 API 的公開或半公開簽名，因此 context 路徑的破壞面明顯大於 error handler 路徑。

`ast-grep` 對完整舊 import string 的交叉掃描結果為：

- `github.com/lamxy/fiberhouse/provider/context`：19 個 Go 檔。
- `github.com/lamxy/fiberhouse/provider/adaptor`：2 個 Go 檔。
- 其中兩個 `provider/adaptor/*.go` 本身也匯入舊 context 路徑。

## 目標目錄內的 5 個定義檔

### 搬到 `coreadaptor/context/`

| 現有檔案 | 搬移後 | 必要調整 |
| --- | --- | --- |
| `provider/context/core_ctx_wrap_interface.go` | `coreadaptor/context/core_ctx_wrap_interface.go` | 路徑搬移；`package context` 可保留 |
| `provider/context/core_ctx_wrap_fiber_impl.go` | `coreadaptor/context/core_ctx_wrap_fiber_impl.go` | 路徑搬移；公開符號不必改名 |
| `provider/context/core_ctx_wrap_gin_impl.go` | `coreadaptor/context/core_ctx_wrap_gin_impl.go` | 路徑搬移；公開符號不必改名 |

公開符號為 `ICoreContext`、`FiberContext`、`GinContext`、`WithFiberContext`、`WithGinContext`。只要舊 package 被移除，這些符號的完整 package identity 就會改變。

### 搬到 `coreadaptor/errorhandler/`

| 現有檔案 | 搬移後 | 必要調整 |
| --- | --- | --- |
| `provider/adaptor/fiber_error_handler.go` | `coreadaptor/errorhandler/fiber_error_handler.go` | 更新內部 context import；建議 `package adaptor` → `package errorhandler` |
| `provider/adaptor/gin_error_handler.go` | `coreadaptor/errorhandler/gin_error_handler.go` | 更新內部 context import；建議 `package adaptor` → `package errorhandler` |

兩個 package 的依賴方向仍然是 `errorhandler -> context`，新目錄布局不會引入 import cycle。

## 目標目錄外的 19 個直接消費檔

### Root package `github.com/lamxy/fiberhouse`：10 個

| 檔案 | 使用面 | 對齊修改 |
| --- | --- | --- |
| `core_fiber_starter_impl.go` | `adaptor.FiberErrorHandler` | import 改為 `coreadaptor/errorhandler`；selector 改為 `errorhandler.FiberErrorHandler` |
| `core_gin_starter_impl.go` | `adaptor.GinErrorHandler` | import 改為 `coreadaptor/errorhandler`；selector 改為 `errorhandler.GinErrorHandler` |
| `ctx_core_adapter.go` | `ICoreContext` | context import 改新路徑；alias/selector 改為 `coreadaptorctx` |
| `ctx_fiber_adaptor_provider.go` | `WithFiberContext` | 同上 |
| `ctx_gin_adaptor_provider.go` | `WithGinContext` | 同上 |
| `recover_config.go` | `ICoreContext` | 同上 |
| `recover_error_handler_impl.go` | `ICoreContext` | 同上 |
| `recover_interface.go` | `ICoreContext` | 同上 |
| `recover_recoveries_impl.go` | `ICoreContext`、`WithFiberContext`、`WithGinContext` | 同上 |
| `response.go` | `ICoreContext` | 同上 |

Root package 的公開簽名（例如 `CoreContext`、`ResponseWrap.SendWithCtx`、recover 相關 interface/config）會從新 package 路徑引用 `ICoreContext`。這是下游 API migration 的主要部分。

### `response` package：4 個

| 檔案 | 對齊修改 |
| --- | --- |
| `response/response_interface.go` | 更新 context import、alias 與 `ICoreContext` selector |
| `response/response_impl.go` | 同上 |
| `response/response_proto_impl.go` | 同上 |
| `response/response_msgpack_impl.go` | 現在是未加 alias 的 `context`；建議明確改成 `coreadaptorctx`，同步改兩個方法簽名 |

`response.IResponse` 的公開方法直接含有 `ICoreContext`，所以外部實作 `IResponse` 的程式也需要重新編譯並切換 import path。

### `exception` package：1 個

- `exception/exception_error.go`：更新 context import、alias，以及 `Exception` / `ValidateException` 的 `JsonWithCtx`、`SendWithCtx` 方法簽名。

### 範例 API：4 個

- `example_application/module/example-ginapi-module/api/common_api.go`
- `example_application/module/example-ginapi-module/api/example_api.go`
- `example_application/module/example-module/api/common_api.go`
- `example_application/module/example-module/api/example_api.go`

四個檔案都要把 context import 換成新路徑，並把現有 `providerctx.WithGinContext` / `providerctx.WithFiberContext` 對齊為 `coreadaptorctx.*`。

## Package 層級依賴

目前直接匯入兩個舊 package 的本倉 package 是：

```text
fiberhouse root --------------------> provider/adaptor
       |                                      |
       +--------------------------------------+--> provider/context
exception -----------------------------------> provider/context
response ------------------------------------> provider/context
example .../example-ginapi-module/api -------> provider/context
example .../example-module/api --------------> provider/context
```

搬移後應成為：

```text
fiberhouse root --------------------> coreadaptor/errorhandler
       |                                      |
       +--------------------------------------+--> coreadaptor/context
exception -----------------------------------> coreadaptor/context
response ------------------------------------> coreadaptor/context
example .../example-ginapi-module/api -------> coreadaptor/context
example .../example-module/api --------------> coreadaptor/context
```

直接需要改原始碼的 package 數量有限，但 root package 被大量其他 package 匯入，所以全倉重建/測試的間接 blast radius 很大。

## 文件與生成資料

必須更新的文件連結：

- `README.md:974-975`
- `docs/README_en.md:949-950`

兩處的 `./provider/adaptor/*.go` 應改為 `./coreadaptor/errorhandler/*.go`。顯示文字仍可稱為「error handler adapter」，因為函式本身確實是在 Fiber/Gin handler 與框架統一錯誤處理器之間做適配。

`.codegraph/codegraph.db*`、daemon log/pid 是生成/索引資料，不應手工修改。檔案搬移後應讓 CodeGraph watcher 自動重建路徑；驗證時再查詢新路徑，確認沒有 stale banner。

`.codegraph-qa-out/TODO.md` 中已有本題的待辦文字。它屬於分析庫的歷史/待辦記錄，不是產品碼；實作完成時可另行勾除或改成指向本報告，但本次分析不覆寫該使用者現有變更。

## 命名對齊：必要與建議分界

### 最小可編譯修改

若只追求編譯通過，可以：

1. 搬移兩個目錄。
2. 更新所有 21 個舊 import string。
3. 保留 `package context`、`package adaptor`。
4. 保留呼叫端的 `providerctx` / `providerCtx` / `adaptor` alias 或預設 package selector。

Go 允許目錄名 `errorhandler` 與 package declaration `adaptor` 不一致，因此技術上可行，但會留下 `errorhandler` 路徑卻使用 `adaptor.X` 的語義落差。

### 建議的完整對齊

除上述路徑修改外，再做：

1. `coreadaptor/errorhandler/*.go`：`package adaptor` 改成 `package errorhandler`。
2. 所有新 context import 統一 alias 為 `coreadaptorctx`；包含原本使用 `providerctx`、`providerCtx` 和未加 alias 的檔案。
3. 兩個 starter 統一使用 `errorhandler.FiberErrorHandler` / `errorhandler.GinErrorHandler`。
4. 以 `gofmt` 整理所有變更的 Go 檔。

以下名稱**不因本次目錄改名而必須修改**：

- root package 中的 `Provider`、`IProvider`、`ProviderManager`、`ProviderType`、`ProviderLocation` 等 provider 架構。
- `example_application/providers/` 及其子目錄；它不是根目錄的 `provider/`。
- `ctx_fiber_adaptor_provider.go`、`ctx_gin_adaptor_provider.go` 的檔名與 `CoreCtx*Provider` 型別名。這些名稱描述的仍是「context adaptor provider」角色；若要改，應當是另一個 API/命名重構，而不是此次路徑搬移的必要條件。
- 錯誤訊息中的「core context provider」；它描述 provider 機制，不是舊目錄路徑。

## 對外相容性選項

### 直接破壞式搬移

刪除舊路徑，所有下游使用者一次性修改 import。適合尚未穩定發布或可統一升級的專案，但應在 release note 提供舊/新 import 對照。

### 保留相容 shim（較平滑）

暫時保留：

- `provider/context`：以 type alias / forwarding function 重新導出 `ICoreContext`、`FiberContext`、`GinContext`、`WithFiberContext`、`WithGinContext`。
- `provider/adaptor`：forward 到 `coreadaptor/errorhandler` 的 `FiberErrorHandler`、`GinErrorHandler`。

其中 `ICoreContext`、`FiberContext`、`GinContext` 應使用 type alias，避免形成另一組不同型別。forwarding package 加上 Deprecated 註解，下一個 breaking release 再移除。這會多保留 5 個相容層檔案，但可降低外部專案一次性斷裂。

若本模組已承諾穩定 v1 API，完全移除舊 import path 屬於 breaking change；版本策略應與專案的發布政策一起決定。

## 建議實作順序

1. 用 `git mv provider coreadaptor`，再用 `git mv coreadaptor/adaptor coreadaptor/errorhandler`，保留歷史追蹤。
2. 先調整兩個新 package 的 package clause 與內部 import。
3. 更新 19 個外部直接消費檔的 import/alias/selector。
4. 更新中英文 README 的兩個連結。
5. 若採相容策略，補回舊 package shim 與 Deprecated 註解。
6. 對所有變更 Go 檔執行 `gofmt`。
7. 執行下列驗證。

## 驗證清單

```bash
# 舊路徑應歸零（若採 shim 策略，僅允許 shim 內出現）
rg -n 'github\.com/lamxy/fiberhouse/provider/(context|adaptor)' --glob '*.go'

# 文件舊連結應歸零
rg -n '\bprovider/(context|adaptor)\b' README.md docs .codegraph-qa-out

# 新 package 應能被 Go 發現
go list ./coreadaptor/...

# 編譯與測試所有間接消費者
go test ./...
```

目前兩個目標 package 都沒有 `_test.go`。建議至少新增：

- `coreadaptor/context`：Fiber/Gin wrapper 的 `GetCtx`、header、`JSON`、`Send` 行為測試。
- `coreadaptor/errorhandler`：Fiber handler 對原始錯誤/handler 錯誤的回傳，以及 Gin 的 `c.Errors`、自訂 `error` key 兩條分支。

## 當前測試基線注意事項

本次分析在未做重命名前執行 `go test ./...`，基線已失敗，主要可見問題包括：

- `github.com/bytedance/sonic/internal/rt` 編譯時找不到 `GoMapIterator`（執行環境 Go 為 1.26.2，而 `go.mod` 宣告 1.25.1；是否為版本相容根因需另案確認）。
- `bootstrap` 與 `component/writer` 的部分測試因暫存檔不存在而失敗。

因此日後驗證此次搬移時，應先固定/使用專案支援的 Go toolchain 並處理既有基線，或至少分開記錄「既有失敗」與「路徑搬移新增失敗」，不能直接把全倉 `go test` 的紅燈歸因於本次改名。

## 範圍統計

| 類別 | 數量 |
| --- | ---: |
| 搬移的 Go 定義檔 | 5 |
| 目標目錄外直接消費 Go 檔 | 19 |
| 必改 Go 檔合計（不採 shim） | 24 |
| 必改 README | 2 |
| 直接受影響的舊 import path | 2 |
| 目標 package 直接測試檔 | 0 |

