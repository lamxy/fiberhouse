# v1.x.x 生產級穩定性：阻塞項代碼級核實（經三輪交叉審計）

> 日期：2026-09-11；基線 `main@c9017ad`，分支 `stability/v1-promotion-round2`
> 用途：`.taskmaster/docs/prd.txt`（最終版）的事實底稿。
> 實測基線：`go vet` 通過；`go test ./...` **878 用例 / 77 套件**；`-race` 零 data race；根套件 76.0%。
> 本文所有數字經三個獨立子代理審計後由主代理回源碼裁決，**不採信任何單方數字**。

## 範圍決策（用戶裁決）

| 決策 | 內容 |
|---|---|
| A | 內部工具（Dig、jsonconvert、mongodecimal、logging adaptor）**不作為公共能力，不參與晉級** |
| B | panic 治理**排除 `example_application/`**，只針對框架套件；示例限制僅列出 |
| C | 覆蓋率**不作硬性門檻**，改為「不無故大幅下降 + 逐項歸因」 |

決策 A 的關鍵依據：`mongodecimal` 位於 `component/database/dbmongo/internal/mongodecimal/`。
Go `internal/` 包無法被倉庫外示例導入（唯一導入者是 `dbmongo/mongo.go:18,90-91`），
故條件 5「文檔示例確實執行該路徑」**在結構上不可滿足**——`internal/` 包本質上不可能是公共能力。

## 一、數字爭議的最終裁決

三份審計對同一事實給出不同數字，以下為主代理實測結果：

| 項目 | 技術審計 | 範圍審計 | **實測（採用）** |
|---|---|---|---|
| `AppState` 非測試引用 | 30 | 42 | **42**（30 守衛 + 3 寫入 + 4 宣告 + 5 註解）跨 **6 檔案** |
| 可執行 panic/fatal | 90 / 87 | 106 | **89**（排除註解、測試、快照） |
| `feature-status.md` 能力行 | — | 19 | **19** |
| 測試用例 | — | 878 | **878** |

**最重要的修正**：前一版 PRD 寫「31 處、4 個檔案」，漏掉 `context_interface.go:44,46`
（介面宣告）與 `context_impl.go:82,90`（唯一實作，`sync.Once` latch 本體）——
**缺陷震央被排除在遷移清單外**，照此施工會重演 T2 第一版「編譯通過、測試全綠、缺陷原封不動」。
另漏 hertz `:414` 第三個寫入點。

## 二、panic 最終分流（89 處）

| 類別 | 數 | 處置 | 依據 |
|---|---|---|---|
| `example_application/` | 7 | 決策 B 排除 | 用戶裁決 |
| `exception/` 業務慣用法 | 11 | 不動 | `exception_error.go:20-38` 文件化慣例。`:26` 是 `/* */` 文件區塊內的範例（非可執行，但 grep 計入）；`:28` 已 `//` 註解故不計入 |
| `Must*` 已有 error 姊妹 | 13 | 不動，已合規 | 見下 |
| `LoggerWrap.Fatal()` API | 3 | 不動 | `bootstrap.go:159-169` 是日誌器暴露 zerolog Fatal 級別的 API，非框架 fatal 呼叫 |
| 模型構造器 | 2 | 不動 | `mysql_model_impl.go:33`←`:46`、`mongo_model_impl.go:36`←`:49`，是條件 4 的判例來源 |
| **C 類待處理** | **53** | EPIC C | — |

**13 處已合規 `Must*`**（前一版誤歸「B 類：無調用方可處理」，**理由錯誤**）：
`provider_type.go:175,216,243`←`Default/Custom/Type`；`provider_location.go:248,289,316`←
`Default/Custom/Location`；`global_utils.go:100,105`←`GetInstance:82`、`:141`←`RecoverMiddleware:126`；
`context_impl.go:127`←`GetLoggerWithOrigin:105`、`:213`←`:191`；`boot.go:117,122`←`GetValue:100`。
正確理由是**「已有返回 error 的替代入口」**——錯誤理由會誤導執行者不去檢查其餘 `Must*` 家族。

## 三、審計推翻的四項原判斷（已複驗屬實）

1. **EPIC E2 原會交付無人呼叫的 `Shutdown`**。全庫零個非測試 worker 停止點。
   worker 雖註冊進容器（`task_impl.go:85`），卻被 `frame_starter_impl.go:365` 取出後直接
   `RunServer()`——**不實作 `Close` 就不受 `ReleaseAll` 管轄**，與 dispatcher 情況不同。
   最終版強制二選一接入方式，推薦讓 `TaskWorker` 實作 `Closable`。

2. **EPIC I 的 singleflight 指錯程式碼**。`Level2Cache` 零 singleflight 引用；
   它在 `cache_utils.go:79` 的 `Coordinator` 上，使用者是 `example_service.go:165,279,293`，
   屬應用層旁路快取，與 L2 是兩套獨立機制。前一版沿用 `feature-status.md:72` 舊描述未回查程式碼。
   最終版增設 I0 範圍定位閘門。

3. **兩處「文件化為契約」是零工程量空交付**：CLI 健康檢查一次性已在
   `command-line.md:107`、`frame_cmd_application.go:120-124`、`feature-status.md:74` 三處文件化；
   `ServerRunAfter` 已在 `provider_location.go:139,175` 與 `startup-lifecycle.md:46,96,105` 文件化。
   最終版移出交付物。

4. **Fiber 須新建 `initErr` 機制而非「沿用」**。實測 Fiber 零命中，Gin 13 處、hertzcore 15 處。

## 四、能力 11 條件 5 的隱患（審計發現）

「擴展運行位點與關閉鏈」的條件 5 目前僅由 pprof 支撐，而 pprof 在
`application_dev.yml:102`、`application_prod.yml:102` 皆 `enable: false`，
只有 CI smoke 的 `application_test.yml:102` 為 `true`
（`.github/workflows/go1.yml:66-70` 注入 `APP_ENV_application_env: test`）。
**文檔示例的預設路徑不執行該關閉位點。**

## 五、依賴誤判修正

- **B 不完全依賴 A**：B1（新增 `ClosableProvider`）與 B3（`List()` 順序、重複綁定語義）
  不觸及 `AppState`，僅 B2 與 A 有交集。
- **H 不依賴 A**：`ICommandContext`（`context_interface.go:54-63`）不含 AppState 方法，
  CLI 全程不讀。

## 六、仍然成立的原判斷

- `AppState` 三語義 + `sync.Once` latch，`Shutdown` 靠缺陷運作 —— 三份審計一致 CONFIRMED
- 全庫 24 個非測試 `Initialize` 中僅 `PprofStartProvider` 持有 OS 資源
- `TaskDispatcher` 補 `Close` 即被容器回收（但有三個前提：`enableServer=true`、
  `conform=true`、實例已落位）
- L2 `finishShutdown` 確實 Release 兩個 ants pool，真缺口在 `Wait()` 不等在途任務
- `GlobalManager` 三處並發缺口（`Rebuild` 不退役舊實例、`Release` 不使已持有引用失效、
  `Get` 的 `-1` 重試路徑不受維護閘門保護）

## 現有 live 覆蓋（14 個，勿重複建設）

Redis 3、Mongo 3、MySQL 3、readiness 2、task 3。全帶 `liveintegration` tag，已進 CI smoke。
