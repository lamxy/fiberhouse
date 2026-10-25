# DefaultStackTraceHandler 包級變數寫競爭（2026-10-25 已修）

## 問題

`recover_error_handler_impl.go` 原有包級變數 `debugFlag`、`debugFlagValue`、`requestID`，
`(*ErrorHandler).DefaultStackTraceHandler` 每次呼叫都把配置值寫回這些變數。

觸發面比「panic 時」大得多：Fiber 的 `fiber.Config.ErrorHandler`（`core_fiber_starter_impl.go:198`）
與 Gin 的 `GinErrorHandler` 中間件（`core_gin_starter_impl.go:325`）都經 `ErrorHandler.ErrorHandler`
呼叫它，所以**每個回傳錯誤的請求**（404、驗證失敗等）都會寫一次。並發讀者還有
`GetJsonIndent` 與 `FiberRecovery.TraceID`。

附帶的語義錯誤：`FiberRecovery.TraceID` 在首次錯誤前用硬編碼 `"traceId"`，之後才變成配置值。

## 修法

配置值在 `AppConfig.Initialize` 時已快取為不可變結構（`GetRecover()`/`GetTrace()` 回傳副本），
直接每次讀取即可：刪除包級變數，改為局部變數 + `requestIDKey(appCtx)` 輔助函式（配置缺失時兜底 `"traceId"`）。
未採用 `sync.Once`/`atomic`：沒有需要共享的可變狀態，加同步只是掩蓋設計問題。

回歸測試：`recover_error_handler_race_test.go`（需 `-race`）。

## 未處理的相關問題

- `enableDebugFlag: true` 但 `debugFlagValue` 未配置（預設 `""`）時，`GetHeader(ctx, "")` 回 `""`，
  條件 `debugFlagFromHeader == debugFlagValue` 恆真，每個錯誤都打印堆棧到日誌。
- `GinRecovery` 的 `GetParamsJson` 等日誌欄位名硬編碼 `"traceId"`，不跟隨 trace 配置。
