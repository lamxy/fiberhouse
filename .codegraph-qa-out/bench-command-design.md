# CRUD API 壓測命令設計與實作

> 目標：新增一個 CLI 命令，在**預設時長內維持穩定負載**，供 pprof 採集 CPU/heap/goroutine/block/mutex profile，用以找出框架層性能問題。
>
> 實作日期：2026-09-03。

---

## 一、分析過程中的關鍵發現

### 1.1 兩套獨立的 example 棧（決定了壓測必須走 HTTP）

| | Web API | CLI 命令 |
|---|---|---|
| 模組 | `example-module` | `command-module` |
| 儲存 | **MongoDB** | **MySQL** |
| ID 型別 | ObjectID hex 字串 | `uint64` |
| ID 校驗 | `alphanum,min=18,max=32` | `> 0` |
| Service | `ExampleService` | `ExampleMysqlService` |

`test_orm_command.go` 操作 MySQL 棧，而要壓測的 `/examples` 走 MongoDB 棧，**兩者資料不互通**。

**結論**：壓測命令必須經 HTTP 打到運行中的服務，不能複用 command-module 的 service 層。

### 1.2 改名成本為零

`test_orm_command.go` 內部**早已**是 `exampleCommand` / `NewExampleCommand`，CLI 命令名也已是 `example`。改名僅是檔案重命名：

- `test_orm_command.go` → `example_command.go`
- `test_other_command.go` → `bench_command.go`（原檔僅一行註解）

### 1.3 限流器陷阱

`/common` 路由組掛了限流器，且 KeyGenerator 回傳**固定字串**：

```go
Max: 5, Expiration: 30 * time.Second,
KeyGenerator: func(*fiber.Ctx) string { return "limiter_key_unique" }
```

全域 30 秒僅 5 次。**壓測必須避開 `/common/*`**；`/examples` 與 `/health` 不受影響。

### 1.4 CLI 啟動期 panic（既有問題，已修）

`NewExampleCommand` 在**命令註冊期**就構造 MySQL 棧：

```
NewExampleCommand → NewExampleMysqlModel → dbmysql.NewMysqlModel
  → panic: entry '__key_mysql' not found for loading
```

只要 MySQL 未配置，**整個 CLI 無法啟動**，連與 MySQL 無關的命令（如 bench）也一併不可用。

已改為惰性接線：`resolve func()` + `sync.Once`，僅在 example 子命令實際執行時才構造。行為對既有使用者無變化，但讓 CLI 在無 MySQL 環境下可用。

### 1.5 錯誤訊息不可見（既有行為，已補救）

應用的 `ExitErrHandler`（`application.go:52`）只把錯誤寫入**日誌器**，不寫 stderr。壓測通常在關閉控制台日誌的配置下運行，操作者只會看到 `exit status 1`。

bench 命令因此在返回錯誤前額外向 CLI writer 打印一行 `bench failed: ...`。

---

## 二、負載模型

### 2.1 三階段

```
Phase 1  seed      建立 N 筆種子（預設 1000），收集 ID 池
Phase 2  bench     固定 RPS 開環注入，持續 --duration
Phase 3  cleanup   刪除本次建立的全部資料（--keep-data 可跳過）
```

`preflight` 在播種前先探測服務可達，避免空跑一輪才發現服務沒起來。

### 2.2 為何是開環（固定 RPS）而非閉環

**閉環**（固定併發、等回應再發下一個）存在**協調遺漏（coordinated omission）**：伺服器變慢時客戶端自動降速，於是系統越慢、施加的壓力越小，真實延遲被系統性低估。

**開環**按牆鐘時間固定速率發送，不等前一個請求返回。伺服器變慢時請求堆積，延遲如實反映。

對「觀察框架開銷」這個目的，開環還有一個關鍵好處：**可把 RPS 固定在 DB 不飽和的水位**，讓 CPU profile 上框架代碼的佔比最大化。

實作上以 `time.Ticker` 產生節拍投遞到 buffered channel，worker 池消費。若 channel 滿（worker 跟不上）則記錄 `dropped` 並在報告中**顯著告警**——該值非零代表測量結果不可信。

### 2.3 讀寫配比（預設 80/20）

| 操作 | 權重 | 路徑特徵 |
|---|---:|---|
| `GET /examples/cache/:id` | 40% | 二級快取，多數命中本地 → **框架開銷佔比最高** |
| `GET /examples/:id` | 25% | 直接 Mongo 單筆讀 |
| `GET /examples` | 15% | 分頁 + 列表快取 |
| `POST /examples` | 10% | 完整寫路徑 + asynq 派發 |
| `PUT /examples/:id` | 10% | 校驗 + 部分更新 |

`cache_get` 佔比最高是刻意設計：命中 ristretto 本地快取時 DB 與網路幾乎不參與，火焰圖上剩下的幾乎全是 sonic 編解碼、validator 反射、Fiber 中間件鏈、`Response()` 對象池——正是要暴露的目標。

`DELETE` **不進主壓測迴圈**：它會讓 ID 池失效並污染快取行為。刪除只在 cleanup 階段執行，該路徑仍會被 profile 到，但不干擾穩態測量。

---

## 三、實作要點

### 3.1 檔案

| 檔案 | 說明 |
|---|---|
| `commands/example_command.go` | 自 `test_orm_command.go` 改名；MySQL 接線改為惰性 |
| `commands/bench_command.go` | CLI 定義與 flag 解析（原 `test_other_command.go`） |
| `commands/bench_runner.go` | 負載產生器核心：播種、開環注入、統計、清理 |
| `commands/bench_report.go` | 統計報告格式化 |
| `commands/bench_runner_test.go` | 10 個測試，含假服務端 |
| `application.go` | 註冊 `NewBenchCommand` |
| `example_config/application_test.yml` | `enableConsole: false` |

### 3.2 三個容易踩的坑

**HTTP client 必須調參**。`http.Transport` 的 `MaxIdleConnsPerHost` 預設為 **2**，會讓客戶端連線複用成為瓶頸，測到的是壓測工具自身的限制而非服務端性能。已設為 `workers × 2`。

**必須讀完 response body**。否則連線無法複用，每個請求都新建 TCP 連線。

**warmup 不可省**。三樣東西需要暖機：`sync.Pool` 填充、ristretto 快取預熱、Mongo 連線池建立。前幾秒資料嚴重偏離穩態。

### 3.3 其他設計決策

- **每個 worker 持有獨立 `rand.Rand`**：共享 `rand` 會產生鎖爭用，干擾被測系統的 mutex profile。
- **cleanup 使用獨立 context**：壓測 context 可能已因超時或 Ctrl-C 取消，但清理仍應盡力完成以免資料殘留。
- **分位數用全量排序**：30000 個 `time.Duration` 僅 240KB，比 HDR 直方圖精確且零依賴。
- **`--workers` 自動推導**：`rps/10`，鉗制在 16..512。經驗上需覆蓋 `rps × p99延遲`。

---

## 四、使用方式

```bash
# 終端 1：啟動服務（application_test.yml 已開 pprof:6060）
go run ./example_main

# 終端 2：壓測
cd example_application/command
go run . bench --rps=500 --duration=90s --seed=1000

# 終端 3：跳過 warmup 與爬坡後採集
sleep 10
go tool pprof -http=:8000 http://127.0.0.1:6060/debug/pprof/profile?seconds=60
```

其餘四類 profile：

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
go tool pprof http://127.0.0.1:6060/debug/pprof/goroutine
go tool pprof http://127.0.0.1:6060/debug/pprof/block    # 已設 blockRate=10000
go tool pprof http://127.0.0.1:6060/debug/pprof/mutex    # 已設 mutexFraction=100
```

**heap 建議取兩次做差**（壓測開始後 10s、結束前 10s），排除暖機期的一次性分配：

```bash
go tool pprof -base heap_t1.pb.gz heap_t2.pb.gz
```

---

## 五、實測驗證

對 stub 服務端跑 `--rps=300 --duration=3s --seed=20`：

```
duration      3s           target rps  300      actual rps  299.8
requests      900          errors      0 (0.00%)
seeded        20           cleaned     154
latency       p50 318µs  p90 598µs  p99 911µs  max 1.4ms

cache_get 379 (42%)   get 208 (23%)   list 139 (15%)
create     96 (11%)   update 78 (9%)
```

- 開環速率準確：299.8 / 300
- 請求數精確：900 = 300 × 3s
- 權重符合預期（目標 40/25/15/10/10）
- 清理完整：154 = 20 種子 + 134 壓測期新建
- 零丟棄、零錯誤

全倉庫測試套件（含 `-race`）與 `go vet` 全綠。

---

## 六、預期在 profile 上觀察的重點

> 本節為實作當時的**預期**。實測驗證結果見 `bench-profiling-runbook.md` §8.1。

| 位置 | 預期現象 |
|---|---|
| `validator.Struct` | 反射開銷；每次 `GetValidate(lang)` 都取本地化實例 |
| `Response()` 對象池 | `ResponseWrap` + `RespInfo` 雙層池化，Release 時序值得看 |
| coreHttp 日誌中間件 | 每請求構造 zerolog event |
| `dispatchExampleChanged` | 每次寫操作派發 asynq 任務，Redis 往返 |
| `GetWithCache` | 快取協調器 + singleflight，看 mutex profile 是否有爭用 |

原本另列有 `toResponse`（結構轉換與 slice 分配），但 2026-09-03 以 398k 請求實測後在 profile 中完全不可見（成本低於 10ms 採樣精度），已移除。

---

## 七、遺留事項

- `application_test.yml` 的 `appLog.level` 仍為 `debug`。壓測時每請求構造 debug 級 event 仍有開銷，若想進一步排除日誌干擾可調為 `info`。本次未改動，因超出已確認範圍。
- `example_command.go` 的惰性接線修正了啟動 panic，但 `command-module` 的 MySQL 棧在缺少配置時仍會於首次執行 example 子命令時 panic（而非返回 error）。根治需改 `dbmysql.NewMysqlModel` 的錯誤處理，超出本次範圍。
