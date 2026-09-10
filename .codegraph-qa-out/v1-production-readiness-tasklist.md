# v1.x.x 生產級穩定性任務清單

> 基線：`main@46c6618`（工作樹乾淨，`main == origin/main`，無分歧）
> 分支：`stability/v1-production-readiness`
> 範圍：**僅 v1.x.x**。不涉及 v2.0.0 重構（統一生命週期、跨組件所有權契約、公共 API 遷移均排除）。
> 約束：優先局部補丁；不新增公共運行入口；不建立統一關閉架構；不遷移公共 API。

## 已驗證基線

| 檢查 | 結果 |
|---|---|
| `go vet ./...` | 通過 |
| `go test ./... -count=1` | 全通過，40 套件 |
| `go test -race ./... -count=1` | 全通過，零 data race |
| 預設 profile 總覆蓋率 | 62.2% |
| `-tags=liveintegration` 本機 | 連線被拒（無 Docker），**環境問題非回歸**；CI 有服務容器 |

---

## T1 — 修 Makefile 測試 target（阻塞訊號污染）

**問題**：4 個 target 指向不存在的 `./frame/...`。Go 對不匹配 pattern 靜默成功退出，`make test` 印「測試完成」但零測試執行。

```
Makefile:33  go test ./frame/... -race -cover -coverprofile=coverage.out
Makefile:37  go test -v ./frame/... -race -cover
Makefile:40  go test ./frame/... -race -cover -coverprofile=coverage.out
Makefile:45  go test -bench=. ./frame/... -benchmem
```

**次要缺陷**：`set CGO_ENABLED=1 &&` 是 Windows CMD 語法。在 zsh/bash 下執行 shell 內建 `set`，`CGO_ENABLED` 實際未設定。

**修復方案**：`./frame/...` → `./...`；`set X=1 &&` → `CGO_ENABLED=1 <cmd>` 前綴形式。

**驗收**：`make test` 實際執行 40 個套件並回報覆蓋率；`make test-bench` 能跑到 benchmark。

**風險**：零。純工具修復，不觸及產品程式碼。

---

## T2 — Fiber 關閉路徑順序缺陷（唯一具使用者可見後果的缺陷）

**問題**：Fiber 有兩條並存關閉路徑，`ServerShutdownAfter` 位點在資源銷毀**之後**執行。

路徑 A（`core_fiber_starter_impl.go:357-364`）—— `OnShutdown` hook：
```go
cf.coreApp.Hooks().OnShutdown(func() error {
    clearApplicationGlobals(cf.GetAppContext())   // 釋放全域物件 + ClearAll 清空容器
    _ = cf.GetAppContext().GetLogger().Close()    // 關閉日誌 writer
    return nil
})
```

路徑 B（`core_fiber_starter_impl.go:401-440`）—— `Shutdown` 方法，其第 428 行 `cf.coreApp.Shutdown()` 正是觸發路徑 A 的動作。

**實際執行序**：
```
SIGTERM → coordinateServerRun select 命中 (server_lifecycle.go:30)
       → appStarter.Shutdown()            (server_lifecycle.go:31)
           → ServerShutdown 位點
           → ServerShutdownBefore 位點
           → coreApp.Shutdown()            (fiber:428)
               → OnShutdown hook
                   → clearApplicationGlobals()  ← 容器清空、全域資源關閉
                   → logger.Close()             ← 日誌 writer 關閉
           → ServerShutdownAfter 位點       (fiber:436)  ← 在無資源環境下執行
```

**Gin 無此問題**（`core_gin_starter_impl.go:585-600`）：線性順序，先跑完 `ServerShutdownAfter`，再 `clearApplicationGlobals`，最後 `logger.Close()`。

**可達性已確認**：
- `NewPprofShutdownProvider()` / `NewPprofShutdownPManager()` 在預設集合中（`default.go:48-49, 158-159`）
- `PprofShutdownPManager` 綁定到 `LocationServerShutdownAfter`（`pprof_providers_and_manager.go:215`）
- `example_main/main.go:71,110,126` 確實裝配兩個預設集合並 `RunServer()`
- 僅由 `application.pprof.enable`（預設 false）開關擋住 → **潛在但可達**

**嚴重度**：`LoggerWrap.Close()` 只關 `wCloser`，`lw.logger` 指標仍有效 → 關閉後寫入是**靜默丟失**，非 panic。故為「診斷資訊遺失 + 位點語義錯誤」，非崩潰。

**修復方案（選項 A：關閉路徑線性化，對齊 Gin）**：
將 `clearApplicationGlobals` 與 `logger.Close()` 移出 `OnShutdown` hook，置於 `Shutdown` 方法末端、`ServerShutdownAfter` 位點之後。

**必要前置**：先寫**紅燈測試**釘住現行順序缺陷（斷言 `ServerShutdownAfter` 執行時日誌器已關閉/容器已清空），有紅燈再修。
`pprof_lifecycle_test.go:145` 以 nil starter 直接驅動位點，**不** pin Fiber 內部順序，紅燈測試不會與之衝突。

**行為變更**：`ServerShutdownAfter` 語義由「資源銷毀後」改為「資源銷毀前」。屬 breaking change，v2.0.0 前允許，**須同步記入 CHANGELOG 與 `docs/reference/feature-status.md`**。

**風險**：中。需覆蓋 `Shutdown` 被非 `RunServer` 路徑呼叫的情形。

---

## T3 — CLI 與 hertzcore 缺關閉路徑（真實資源洩漏）

**問題**：`clearApplicationGlobals` 全庫僅兩處呼叫 —— `core_fiber_starter_impl.go:360`、`core_gin_starter_impl.go:594`。

- **CLI**（`commandstarter/frame_cmd_application.go:15-22`）：`RunCommandStarter` 跑完 `AppCoreRun()` 即結束，**無關閉段**。註冊的 MySQL 連線池、Mongo client、Redis client 從未 `Close`。
- **hertzcore**（`example_application/hertzcore/starter/core_hertz_starter.go:415-450`）：`Shutdown` 有完整位點編排與 `coreApp.Shutdown`，**獨缺 `clearApplicationGlobals`**。

**影響分級**：短命 CLI 影響小；長跑 CLI（daemon / cron worker）為真實洩漏，Mongo/Redis 背景 goroutine 不停，MySQL 伺服器端連線留到 `wait_timeout`。

**修復方案**：
- `commandstarter/` 為正式套件 → **優先**。障礙：`clearApplicationGlobals` 收 `IApplicationContext`，CLI 持 `ICommandContext`，兩者不同介面。需確認容器取得路徑，或抽共用釋放函式（**不新增公共入口**）。
- `hertzcore` 屬 `example_application/` 示例碼 → 次優先。

**明確排除**：`RunCommandStarter` 第 21 行 `_ = starter.AppCoreRun()` 丟棄錯誤 —— 專案 TODO 已列「暫不進入優化佇列」，理由是改變公共入口語義。**尊重既有決定，不動。**

**風險**：中低。新增關閉段，不改既有簽名。

---

## T4 — 覆蓋率度量修正（非補測試）

**問題**：`dbmysql` 20.9% / `dbmongo` 19.3% 是**度量假象**。`Close`/`IsHealthy`/`PingTry`/`NewClient` 顯示 0%，但確實被 live 測試覆蓋，只因測試在 `//go:build liveintegration` 後，預設 profile 不編譯。

已確認 live 測試清單：
```
TestLive_MysqlDb_CreateWriteReadDropClose / CloseIsIdempotent / ConcurrentReadWriteWithHealthCheck
TestLive_MongoDb_CreateWriteReadDropClose / CloseIsIdempotent / ConcurrentReadWriteWithHealthCheck
```
`Close` 連呼三次斷言冪等、`IsHealthy` 併發呼叫、`PingTry` 均有明確 assertion。

**後果**：CI `quality` job 的 62.2% 系統性低估了 DB 與 Redis 三個**已晉級為「穩定公共能力」**的模組。

**修復方案**：CI 覆蓋率步驟加 `-tags=liveintegration`，或合併 hermetic + live 兩份 profile 後再報總數。

**真正未測、值得補**（不在 live 測試內）：
- `cache2/level2_cache.go:458 OwnsChildren` 0%
- `cache2/level2_cache.go:468 OwnsInstance` 0%
  → 兩者是 `CompositeOwnershipInspector` 實作，**正是 `ReleaseAll` 決定巢狀關閉順序所依賴的方法**，與 T2/T3 關閉正確性直接相關 → **優先補**
- `cache2/level2_cache.go:310 asyncSetLocalIfCurrent` / `336 asyncSetLocal` 0%
  → L2 仍為實驗性，文件已記載「非同步 flush 所有權不完整」，屬已知限制 → 低優先

**風險**：低。CI 設定 + 增量測試。

---

## T5 — Fiber 測試補強（晉級條件 3）

**不對稱事實**：`core_starter_init_test.go` 27 個測試中，Gin 專項約 14 個（logger lease 衝突、owner release 終止性、重入 no-op、listener 失敗釋放、TLS 憑證載入/拒絕、loopback TLS 握手、mode fallback…）；**Fiber 專項僅 1 個**（`core_starter_provider_test.go:78`），其餘為 Fiber+Gin 共用表格測試。

```
CoreWithGin   在測試出現 30 次
CoreWithFiber 在測試出現 18 次
```

**覆蓋率缺口**（Fiber 不涉 build tag，數字為真）：
```
core_fiber_starter_impl.go:190  RegisterAppMiddleware   0.0%   ← 完全未測
core_fiber_starter_impl.go:368  AppCoreRun             33.3%   ← Listen 後分支全未測
core_fiber_starter_impl.go:320  RegisterAppHooks       39.1%   ← 路徑 A hook 本體在此
core_fiber_starter_impl.go:401  Shutdown               63.2%
```

`RegisterAppHooks` 39.1% 尤其關鍵 —— T2 的順序缺陷正落在其未覆蓋部分。

**修復方案**：照搬 Gin 已驗證的 loopback listener 策略到 Fiber。非新設計，是對齊既有模式。

**風險**：低。純增量測試。

---

## 明確排除（v2.0.0 範圍）

- **`AppState` 語義重構**：單一布林承載三種互斥語義（冪等守衛 / 執行中 / 已結束），且 `sync.Once` 使其無法回退。`context_impl_test.go:93-96` 證明先呼叫 `RegisterAppState(false)` 會永久鎖死。這是晉級條件 1 的根因，但觸及 `IApplicationContext` 公共介面與 30+ 呼叫點，**超出局部補丁約束** → 留 v2.0.0。
- 統一 provider 關閉契約 / 跨組件資源所有權契約（`IProvider` 無任何 close 方法，屬刻意省略）。
- `RunCommandStarter` 錯誤傳播語義變更。
- `dbmysql/mysql.go:58` GORM logger adapter 在 `FatalLevel` 走 `os.Exit(1)` —— 邊緣設定，記為已知限制，不在本輪修。

---

## 執行順序

1. **T1**（零風險，先消除訊號污染，讓後續驗證可信）
2. **T5 + T4 未測部分**（純增量測試，為 T2/T3 建立安全網）
3. **T2**（先紅燈測試，再修；同步 CHANGELOG + feature-status.md）
4. **T3**（CLI 優先於 hertzcore）
5. **T4 CI 度量修正**

每個關鍵節點派子代理審計。
