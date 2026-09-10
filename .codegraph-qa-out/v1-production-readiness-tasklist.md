# v1.x.x 生產級穩定性任務清單

> 基線：`main@46c6618`；分支 `stability/v1-production-readiness`
> 範圍：**僅 v1.x.x**。不涉及 v2.0.0 重構（統一生命週期、跨組件所有權契約、公共 API 遷移均排除）。
> 約束：優先局部補丁；不新增公共運行入口；不建立統一關閉架構；不遷移公共 API。
> 狀態：已完成兩輪子代理審計（技術面 C1–C8、範圍面 S1–S5），結論已併入下文。

## 非目標（明確聲明）

**T1–T5 全部完成後，`feature-status.md` 五條晉級條件下不會有任何能力晉級。**
本分支是穩定性**維護**，不是晉級包。晉級需另立範圍（見文末「晉級阻塞分析」）。

## 已驗證基線

| 檢查 | 結果 |
|---|---|
| `go vet ./...` | 通過 |
| `go test ./... -count=1` | 全通過，40 套件 |
| `go test -race ./... -count=1` | 全通過，零 data race |
| 預設 profile 總覆蓋率 | 62.2% |
| `-tags=liveintegration` 本機 | 連線被拒（無 Docker），**環境問題非回歸** |

---

## T1 — 修 Makefile 測試 target ✅ 已完成（commit `661a93a`）

四個 target 由不存在的 `./frame/...` 改為 `./...`；移除 Windows CMD 語法 `set CGO_ENABLED=1 &&`（在 sh/zsh 下執行 shell 內建 `set`，變數從未匯出，經實測確認）。

**驗收（已達成）**：`make test` 實際執行 40 套件、`-race` 全通過並回報覆蓋率。先前靜默跑零測試卻印「測試完成」。

---

## T2 — Fiber 關閉路徑順序缺陷 ✅ 已完成（commit `6a6d5b8`）

三部分：頂部 `defer` 取代尾端語句；`OnShutdown` hook 保留但在 `shutdownCoordinated` 置位時讓位；
移除尾端重複的 `logger.Close()`。並更新六處文件。

**中途犯過一次實質錯誤，記錄以免重蹈**：第一版依審計建議「保留 hook 並委派到同一個 once-guarded
清理函式」。它**編譯通過、既有測試全綠，但缺陷原封不動**——因為 hook 仍在 `coreApp.Shutdown()` 內
搶先執行清理。是新寫的順序測試抓到的（`"2" is not less than "1"`）。教訓：審計建議之間可能互相衝突
（「保留 hook」與「修正順序」），採納前必須自己驗證，且必須有能證偽的測試。

**flake 歸因的兩次修正**：期間觀察到 `-race` 3/12 失敗，先後誤判為「T2 引入 race 迴歸」與
「日誌器關閉導致 Listen 卡住」。兩者皆錯——`newTask4InternalAppContext` 不注入 `wCloser`，
`LoggerWrap.Close()` 是 no-op，不可能卡住 `Listen`。實測數據：預設並行度下 20 次全過，
`-p 1` 下 2/10 失敗；把 T5 測試中等待 `Listen` 返回的期限由 5s 放寬到 30s 後，`-p 1` 下 0/10，
與 T2 前用同一條件測得的 0/10 一致。真正原因是**測試期限在滿載機器上不足**，非產品缺陷。

---

## T2 原始分析（保留）

### 可達性：已用可執行測試證實（審計爭議已裁決）

範圍審計主張 `Shutdown` 因 `AppState` 守衛（`core_fiber_starter_impl.go:402`）在正常路徑不可達，故 T2 打在死程式碼上。**此主張已被實測推翻。**

關鍵事實：`RegisterAppState(true)` 在 `:396`，位於 `Listen` (`:390`) **返回之後**。而 `Listen` 會一直阻塞到伺服器停止。`coordinateServerRun` (`server_lifecycle.go:24-31`) 把 `AppCoreRun` 丟進 goroutine 並 `select` 等待；SIGTERM 到達時 `Listen` 仍在阻塞，`:396` 尚未執行，故 `AppState` 仍為 `false`，`:402` 守衛**放行**。

已寫臨時探針測試實測確認 `AppState=false` during listen（驗證後已刪除，未進入版本庫）。範圍審計把 `:396` 讀成「成功監聽後即設 true」，漏掉 `Listen` 的阻塞語義。

**結論：T2 的目標區塊可達，T2 成立。**

### 缺陷本體

Fiber 有兩條並存關閉路徑。路徑 A（`:357-364`）為 `OnShutdown` hook，內含 `clearApplicationGlobals()` + `logger.Close()`；路徑 B（`:401-445`）為 `Shutdown` 方法，其 `:428` 的 `cf.coreApp.Shutdown()` 正是觸發路徑 A 的動作。

技術審計已核對 fiber@v2.52.14 原始碼（`app.go:889`、`hooks.go:194-200`）：`defer app.hooks.executeOnShutdownHooks()` 是純同步迴圈，**確認 hook 在 `:428` 內同步執行完畢**，早於 `:436` 的 `ServerShutdownAfter` 載入。

執行序：
```
SIGTERM → coordinateServerRun select (server_lifecycle.go:30)
       → Shutdown()  守衛放行（AppState 仍 false）
           → ServerShutdown / ServerShutdownBefore 位點
           → coreApp.Shutdown()            (:428)
               → OnShutdown hook（同步）
                   → clearApplicationGlobals()  ← 容器清空、全域資源關閉
                   → logger.Close()             ← 日誌 writer 關閉
           → ServerShutdownAfter 位點       (:436)  ← 在無資源環境下執行
           → logger.Close() 再次             (:444)
```

**Gin 無此問題**：`core_gin_starter_impl.go:582` 的 `ServerShutdownAfter` 早於 `:594` 的 `clearApplicationGlobals` 共 12 行。

### 實際後果（較初稿收斂）

技術審計指出：pprof 關閉提供者自行解析 server 物件（`resolvePprofServer`，`:162`），**不讀容器**，故能倖存於提早的 `ClearAll`。真實損害是**日誌靜默遺失**——`logger.Close()` 在 `:363` 已執行，任何在 `ServerShutdownAfter` 記錄日誌的 provider 都寫入已關閉的 writer。

嚴重度為「靜默遺失」而非崩潰，已逐一核對三個 writer：`AsyncChannelWriter` 以 atomic `closed` 旗標守衛（`:27,93-96`）返回 error；`AsyncDiodeWriter` 同（`:111-114`）；`LoggerWrap.Close()` 只關 `wCloser`，`lw.logger` 指標仍有效。**無任何 writer panic。**

**附帶缺陷**：Fiber 重複關閉日誌器（`:363` 與 `:444`）。今日無害，因兩個 async writer 的 `Close` 以 CAS + `closeDone` 實作冪等，但這依賴的是**契約而非強制**。

### 可達性（生產）

`NewPprofShutdownProvider`/`NewPprofShutdownPManager` 在預設集合（`default.go:48-49,158-159`），綁定 `LocationServerShutdownAfter`（`pprof_providers_and_manager.go:215`），`example_main/main.go:71,110,126` 確實裝配。僅由 `application.pprof.enable`（預設 false）擋住 → **潛在但可達**。

### 修復方案（第三輪審計後大幅修訂）

初稿方案「移出 hook、置於 `Shutdown` 末端」**已被審計判定為 unsound**，逐條複核後屬實，方案作廢。

**初稿為何不成立**：`Shutdown` 有六個提前返回，全部早於末端的 `:444`——
`:403`（appState 守衛）、`:412`（shutdown 位點載入失敗）、`:415`（replaced）、
`:424`（pre-shutdown 載入失敗）、`:431`（`coreApp.Shutdown()` 失敗）、`:440`（post-shutdown 載入失敗）。
把清理放在末端，等於這六條路徑全部拿不到清理。

更關鍵：已核對 fiber@v2.52.14 `app.go:888-895`，`defer app.hooks.executeOnShutdownHooks()`
位於 `if app.server == nil` 檢查**之上**，故 hook **無條件觸發**。今日即使 `coreApp.Shutdown()`
失敗（`:431`）也仍有清理；初稿方案會把這條路徑的清理一併移除——**用一個缺陷換另一個**。

Gin 之所以能用末端寫法，是因為它同時有 `defer cg.releaseGinLogger()`（`core_gin_starter_impl.go:531`）
配套。Fiber 需要同樣的紀律。

**修訂後方案**：

1. 清理改為 `Shutdown` 頂部的 `defer`，以**每實例** `sync.Once` 守衛（不可用套件級 —— 同進程
   第二個 app 實例會拿不到清理）。`CoreWithFiber` 目前只有 4 個欄位，需新增該欄位。
2. **保留** `OnShutdown` hook，但改為委派同一個 once-guarded 清理函式。once 守衛使重複執行成為
   no-op，因此保留 hook 零成本，且維持「直接呼叫 `coreApp.Shutdown()`」這條路徑的既有行為。
   初稿「移除 hook」是整個提案中不必要的那一半。
3. 移除 `:444` 的重複 `logger.Close()`。`LoggerWrap.Close()`（`bootstrap.go:94-99`）**無冪等守衛**，
   僅委派 `wCloser.Close()`，故 once 守衛是**功能性必需**而非美化。

### 兩項待決策（不逕行採納）

審計另提兩項建議，但兩者都超出局部補丁邊界，**列為待決策而非直接實作**：

- **`runErr` 分支的清理**：`server_lifecycle.go:28-29` 的 `case runErr = <-runResult` 直接返回，
  從不呼叫 `Shutdown`，涵蓋 Listen 綁定失敗與 run goroutine panic 兩條路徑。**今日同樣洩漏**
  （hook 只在 `coreApp.Shutdown()` 時觸發，而該分支從不呼叫），故屬**既有缺陷、非本次新增**。
  但修好 T2 後文件將宣稱關閉順序正確，而這兩條路徑仍零清理 —— 需決定是否納入本輪。
- **`appState` latch 防護**：`RegisterAppState` 是 `sync.Once` latch（首次呼叫即定值）。倉庫內
  不可達，但使用方若提前 pin `true`，`Shutdown` 會在 `:403` 直接返回並跳過清理。防護需把清理
  移到守衛之前，等於變更 `Shutdown` 對 replaced/no-op 情形的契約 —— 觸及 v2.0.0 排除的
  `AppState` 語義，傾向不在本輪處理。

### 紅燈測試策略（審計修正）

初稿設想的「容器隔離」**不可行**：`AppContext.container` 來自 `NewGlobalManagerOnce()`
（`context_impl.go:53,183`），該單例無 reset 入口、無注入點。

改用**注入 writer 觀測順序**：`bootstrap.NewLoggerWrap` 接受 `io.WriteCloser`
（`bootstrap/bootstrap.go:71-77`），故可注入一個記錄 close 時序的 writer，斷言
`ServerShutdownAfter` 位點早於日誌器關閉。**完全不觸碰全域單例**，不會污染同套件其他測試
（此即 T5 踩到的坑）。

**前置**：先寫紅燈測試釘住現行順序。`pprof_lifecycle_test.go:145` 以 nil starter 直接驅動位點，不 pin Fiber 內部順序，紅燈測試不會衝突。

### 必須同步更新的文件（審計修正）

- **無 CHANGELOG 檔案**（已確認），初稿的「記入 CHANGELOG」不可執行 → 改為：於 commit message 完整記錄，或另立 CHANGELOG（本輪不新增）。
- `docs/concepts/startup-lifecycle.md:108` — 現稱 `LocationServerShutdownBefore` 與 `LocationServerShutdownAfter`「只有声明，不能当作可用关闭 hook」，**此陳述當前即為假**（兩核心都消費：fiber:436、gin:582）。連同其上方的位點消費表一併修正。
- `docs/reference/feature-status.md` — 「扩展运行位点与关闭链」列。
- `pprof_providers_and_manager.go:13` — 註解稱關閉發生在「业务引擎关闭之后」。

**行為變更**：`ServerShutdownAfter` 語義由「資源銷毀後」改為「資源銷毀前」，屬 breaking change，v2.0.0 前允許。

**風險**：中。需覆蓋 `Shutdown` 被非 `RunServer` 路徑呼叫的情形。

---

## T3 — CLI 與 hertzcore 缺關閉路徑（真實資源洩漏）

`clearApplicationGlobals` 全庫僅兩處呼叫：`core_fiber_starter_impl.go:360`、`core_gin_starter_impl.go:594`。

- **CLI**（`commandstarter/frame_cmd_application.go:15-22`）：`RunCommandStarter` 跑完 `AppCoreRun()` 即結束，無關閉段。
- **hertzcore**：**兩輪審計一致指出比初稿描述更嚴重**。`core_hertz_starter.go:361-366` 的 `OnShutdown` hook **只記錄日誌**（已親自核對），`Shutdown`（`:414-459`）也不釋放全域物件 → Hertz 完全洩漏 MySQL/Mongo/Redis 連線池，而非僅順序錯誤。

### 初稿的阻礙描述是錯的（兩輪審計一致指出）

初稿稱「`clearApplicationGlobals` 收 `IApplicationContext`，CLI 持 `ICommandContext`，兩者不同介面」，暗示需抽共用函式。**此前提為假**：`GetContainer()`（連同 `GetLogger()`、`GetConfig()`）定義在 `IContext`（`context_interface.go:24-25`），而 `ICommandContext` 內嵌 `IContext`（`context_interface.go:54-55`）。`releaseApplicationGlobals` 僅用這三個方法。

**修正後方案**：把 `clearApplicationGlobals` / `releaseApplicationGlobals` 的參數收窄為 `IContext`，或 CLI 直接經容器呼叫 `ReleaseAllWithObserver` + `ClearAll`。收窄未匯出函式的參數屬純局部改動。

**優先序**：`commandstarter/` 為正式套件 → 優先；`hertzcore` 屬 `example_application/` 示例碼 → 次之。

### CLI 半部 ✅ 已完成

`RunCommandStarter` 以 `defer releaseCommandGlobals(starter)` 補上關閉段。

**實作中推翻了兩輪審計的共同建議**：兩份審計都說「把 `clearApplicationGlobals` 參數收窄為 `IContext`」即可。
實測**不成立** —— 該函式第一步 `stopFrameHealthCheck` 需要 `IApplicationContext.GetStarterApp()` 回傳的
`ApplicationStarter`，而 `ICommandContext.GetStarterApp()` 回傳 `CommandStarter`（`context_interface.go:42` vs `:61`），
型別不同，收窄後無法編譯。且 CLI 健康檢查是一次性的（註冊期跑一輪，無後台 ticker），**沒有可停止的東西**，
故只需 release + clear 兩步，不需三步。

已用探針證實舊行為確實洩漏：僅 `ClearAll`（只刪條目）時，已註冊的 `Closable` 的 `Close` 呼叫次數為 **0**。

覆蓋率：`RunCommandStarter` 100%、`releaseCommandGlobals` 78.6%；`-race -count=5` 通過。

### hertzcore 半部 ⏸ 待決策

`core_hertz_starter.go:32,63` 已持有 `IApplicationContext`，插入點無障礙。**但**該套件的套件註解
（`:1-12`）明言「僅依賴框架的匯出 API」，用以證明「新增核心引擎不需修改框架任何一行程式碼」。
而 `clearApplicationGlobals`/`releaseApplicationGlobals` **皆未匯出**，倉庫亦無任何匯出的釋放入口。

兩條路：(a) 新增匯出 API —— 違反「不新增公共運行入口」約束；(b) hertzcore 自行內聯
`GetContainer().ReleaseAllWithObserver(...)` + `ClearAll(true)`（兩者在 `*GlobalManager` 上已匯出）——
遵守約束且保住該套件「只用匯出 API」的示範價值。傾向 (b)，但屬設計分叉，待決策。

**明確排除**：`RunCommandStarter:21` 丟棄 `AppCoreRun()` 錯誤——`follow-up-local-patch-optimization-todo.md:40` 已列為不進入佇列，尊重既有決定。

**風險**：中低。

---

## T4 — 覆蓋率度量修正

### C6 已被審計推翻，初稿說法過度概化

初稿稱 DB 低覆蓋「全是」build tag 造成的度量假象。**技術審計 REFUTED，經覆核屬實**：
- `IsHealthy` / `PingTry` 確為 live-tag-only（0.0%）→ 度量假象成立
- 但 `NewClient` 31.7%、`dbmongo Close` 28.6% —— **由一般非 live 測試部分覆蓋**：`mysql_test.go:62 TestNewClient_PingFailureReturnsError`、`mongo_model_impl_test.go:118-124 TestMongoDbClose_NilClientIsSafe`（對應 `mongo.go:119` 的 nil-client 守衛與 `closed` CAS）

**修正**：T4 僅對 `IsHealthy`/`PingTry` 主張度量假象，不對整包。

**方案**：CI 覆蓋率步驟加 `-tags=liveintegration`，或合併 hermetic + live 兩份 profile。

### 真正未測且在生產關閉路徑上 ✅ 已完成（commit `491467c`）

新增 `component/cache/cache2/level2_ownership_release_test.go`，四個用例，把真實 `Level2Cache`
按示例應用的別名結構註冊進真實 `GlobalManager`，驅動 `ReleaseAll` 200 輪。

`OwnsChildren` / `OwnsInstance`：**0% → 100%**；`cache2` 套件：73.3% → 77.5%。`-race` 通過。

關鍵細節：helper 必須在 `Register` 後 `Get` 一次讓實例落位——`partitionByOwnership` 只快照
已初始化的 entry，否則組合對象被當作未初始化條目跳過，測試會恆真通過而不檢驗任何東西。

以下為原始分析：

`cache2/level2_cache.go:458 OwnsChildren` 與 `:468 OwnsInstance` 皆 0%，**無 build tag，是真未測**。

技術審計指出其嚴重性高於初稿判斷，已親自核對確認：`example_application/application_impl.go:97-107,123` 把 `KEY_LEVEL2_CACHE` 列入 `ConfigRequiredGlobalKeys`，`KEY_REMOTE_CACHE` 是 `KEY_REDIS` 的別名；`Level2Cache` 是本倉庫**唯一** `CompositeClosable`。`globalmanager/manager.go:448` 用 `OwnsChildren` 分桶、`:497` 用 `OwnsInstance` 算遞移深度。

而 `level2_cache.go:452-457` 的註解正描述這兩個方法要防止的故障：容器若先關 remote（同一 Redis 客戶端另註冊在別的 key 下），L2 排空在途非同步寫入時就會打在已關閉的客戶端上。**即：0% 覆蓋的排序邏輯，正守著一條會造成資料遺失的路徑，且在 Fiber 的出貨關閉路徑上。**

→ **優先補測試**。

低優先：`:310 asyncSetLocalIfCurrent`、`:336 asyncSetLocal`（L2 仍實驗性，非同步 flush 所有權為已載明限制）。

**風險**：低。

---

## T5 — Fiber 測試補強 ✅ 已完成（commit `98b3ca0`）

新增 `core_fiber_starter_lifecycle_test.go`，六個用例，照搬 Gin 已驗證的 loopback listener 策略。

實測覆蓋率變化：

```
RegisterAppMiddleware   0.0%  → 91.3%
AppCoreRun             33.3%  → 86.7%
Shutdown               63.2%  → 68.4%
RegisterAppHooks       39.1%  → 39.1%（未變，理由見下）
```

### 施工中發現的約束（T2 必須據此設計）

初稿在此測試中呼叫了 `RegisterAppHooks`，覆蓋率一度到 78.3%，但**導致整包測試隨機失敗**。

根因：`RegisterAppHooks` 安裝的 `OnShutdown` hook 會執行 `clearApplicationGlobals`，而容器來自
`NewGlobalManagerOnce()` —— **進程級單例**。同一進程內重複執行時，上一輪的 hook 拆掉下一輪
依賴的共享狀態（容器被 `ClearAll`、框架日誌器被 `Close`），使 `Listen` 無法在期限內解開。

失敗訊息：`AppCoreRun did not return after Shutdown closed the listener`，`-count=5` 下穩定復現。
移除該呼叫後連續 30 次通過。

**對 T2 的意義**：T2 的紅燈測試必須直面這個共享單例。任何驅動 Fiber `OnShutdown` hook 的測試
都會污染同進程的其他測試，因此 T2 用例需自備容器隔離（或明確接受單次執行語義），不能沿用
本檔案的 fixture。這是實作中發現的硬約束，不是可選建議。

原始不對稱數據：`core_starter_init_test.go` 27 個測試中 Gin 專項約 14 個；Fiber 專項僅 1 個（`core_starter_provider_test.go:78`）。出現次數 Gin 30 / Fiber 18。

覆蓋率缺口（Fiber 不涉 build tag，數字為真）：
```
:190 RegisterAppMiddleware   0.0%   ← 完全未測
:368 AppCoreRun             33.3%
:320 RegisterAppHooks       39.1%   ← T2 的 hook 本體在此
:401 Shutdown               63.2%
```

**方案**：照搬 Gin 已驗證的 loopback listener 策略。非新設計。

**風險**：低。

---

## 晉級阻塞分析（範圍審計 S5，說明「非目標」的依據）

| 能力 | 阻塞條件 | 原因 |
|---|---|---|
| Fiber HTTP 內核 | 條件 1 | `AppState` 單一布林承載三種互斥語義，且 `sync.Once` 使其無法回退 → 生命週期契約不成立。修復觸及 `IApplicationContext` 與 30+ 呼叫點，屬 v2.0.0 |
| CLI | 條件 1、5 | 即使 T3 補上關閉，`AppCoreRun` 失敗傳播仍因既有排除而無契約 |
| 擴展運行位點與關閉鏈 | 條件 1、5 | 正是被延後的「統一 provider 關閉接口與跨組件所有權契約」 |
| L2 快取 | 條件 1 | 非同步 flush 所有權延後 |
| hertzcore | 原則上不可 | `example_application/` 無成熟度承諾（`feature-status.md:68`），無能力列 |

---

## 已知既有 flake（非本輪引入，未修）

`TestPprofServerShutdownReleasesPort`（`pprof_server_test.go:173`）間歇失敗。

**已證實為既有問題**：在 `origin/main`（`46c6618`，不含本分支任何改動）的獨立 worktree 上以相同
指令重跑 6 次，重現 1 次；在本分支重現 2/5。與本輪任何改動無關。

機制：`freePort` 綁定 `:0` 取得埠號後隨即關閉，`Start()` 再綁定同一埠，`Shutdown` 釋放後測試又
綁定第三次——對臨時埠的三重 TOCTOU。任一空隙被其他行程搶佔即失敗。

不在 T1–T5 範圍內，但 CI 可能偶發撞上。修法（若日後處理）：讓 `PprofServer` 支援 `:0` 並回報實際
埠號，避免「取號→關閉→重綁」的競態。

---

## 明確排除（v2.0.0 範圍）

- `AppState` 語義重構（條件 1 根因，但超出局部補丁約束）
- 統一 provider 關閉契約（`IProvider` 無 close 方法，屬刻意省略）
- `RunCommandStarter` 錯誤傳播（已收口決定）
- `dbmysql/mysql.go:58` GORM logger adapter 在 `FatalLevel` 走 `os.Exit(1)`（邊緣設定，記為已知限制）

---

## 執行順序

1. ~~T1 Makefile~~ ✅ 完成（`661a93a`）
2. ~~T4 高優先部分：`OwnsChildren`/`OwnsInstance` 測試~~ ✅ 完成（`491467c`）
3. ~~T5 Fiber 測試補強~~ ✅ 完成（`98b3ca0`）
4. **← 當前戰線：T2** 紅燈測試 → 修復 → 同步四份文件（注意 T5 發現的共享單例約束）
5. T3：CLI 優先，hertzcore 次之
6. T4 CI 度量修正

原始順序：
3. **T5**：Fiber 測試補強
4. **T2**：紅燈測試 → 修復 → 同步四份文件
5. **T3**：CLI 優先，hertzcore 次之
6. **T4 CI 度量修正**

關鍵節點再派子代理審計。
