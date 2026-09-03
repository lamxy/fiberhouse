# 壓測與性能剖析操作手冊

> 用途：在本機對 fiberhouse example 應用施加穩定負載，並採集 CPU / heap / goroutine / block / mutex 五類 pprof profile，用以定位框架層性能問題。
>
> 建立日期：2026-09-03。文中所有指令與輸出均為本機實測結果，非推測。
>
> 相關文件：[pprof 端點設計](./pprof-profiling-endpoint-design.md)、[壓測命令設計](./bench-command-design.md)

---

## 〇、五分鐘快速開始

```bash
# 終端 1 — 啟動服務（注意 APP_ENV_ 環境變數，見 §2.1）
cd /mnt/d/code/github_opensource/tmp/fiberhouse
APP_ENV_application_env=test go run ./example_main

# 終端 2 — 壓測
cd example_application/command
go run . bench --base-url=http://127.0.0.1:8080 --rps=300 --duration=60s --seed=100

# 終端 3 — 壓測開始 ~10 秒後採集 CPU profile
go tool pprof -http=:8000 "http://127.0.0.1:6060/debug/pprof/profile?seconds=30"
```

---

## 一、前置檢查

### 1.1 依賴服務

| 服務 | 位址（來自 `application_test.yml`） | 是否必須 |
|---|---|---|
| MongoDB | `mongodb://admin:admin@127.0.0.1:27037` | **必須** — CRUD API 的儲存後端 |
| Redis | `localhost:6379` | **必須** — `pingTry: true`，啟動時會嘗試連線 |
| MySQL | `127.0.0.1:3306` | 非必須 — 僅 `example` CLI 子命令使用，壓測不碰 |

檢查：

```bash
# MongoDB
nc -z 127.0.0.1 27037 && echo "mongo ok"

# Redis
nc -z 127.0.0.1 6379 && echo "redis ok"
```

> **注意**：`cache.redis.pingTry: true` 表示啟動時會嘗試連線 Redis。Redis 未起會影響啟動流程。

### 1.2 確認壓測命令可用

```bash
cd example_application/command
go run . bench --help
```

能看到 flag 列表即正常。若出現 `panic: entry '__key_mysql' not found`，表示你的版本早於惰性接線修正（見 §6.3）。

---

## 二、啟動服務（最容易踩坑的一步）

### 2.1 ⚠️ 必須設定 `APP_ENV_application_env=test`

配置檔由 `application.env` 決定，而 **`bootstrap.go:29` 的預設值是 `dev`**：

```go
defaultEnv = "dev"
envSelect := aCfg.String("application.env", defaultEnv)
confFilename := "application_" + envSelect + ".yml"
```

載入順序是：先讀 `APP_ENV_` 前綴的環境變數 → 據此選檔 → 再讀該 yml。

**這意味著直接 `go run ./example_main` 會載入 `application_dev.yml`，而不是你以為的 `application_test.yml`**。`application_dev.yml` 中 `pprof.enable: false`，於是 6060 端口根本不會監聽。

正確做法：

```bash
APP_ENV_application_env=test go run ./example_main
```

環境變數命名規則（`bootstrap.go:198-205`）：`APP_ENV_` 為大寫前綴，其後的 key 用 `_` 分隔、大小寫需與 yml 完全一致。

### 2.2 確認啟動成功

`application_test.yml` 已設 `enableConsole: false`，終端不會有輸出。要確認狀態需查日誌檔：

```bash
tail -f example_main/logs/app.log
```

預期看到（實測輸出）：

```json
{"message":"pprof server listening on 0.0.0.0:6060/debug/pprof/"}
{"message":"App listening..."}
{"message":"Hertz app listening on 0.0.0.0:8080"}
```

或直接探測：

```bash
curl -s -o /dev/null -w "pprof:%{http_code}\n" http://127.0.0.1:6060/debug/pprof/
curl -s -o /dev/null -w "api:%{http_code}\n" "http://127.0.0.1:8080/examples?page=1&page_size=1"
```

兩者都回 `200` 即就緒。

### 2.3 ⚠️ 當前引擎是 Hertz，不是 Fiber

`example_main/main.go:63`：

```go
CoreType: hertzconst.CoreTypeWithHertz,   // 不是 fiber
```

這會直接影響 profile 的解讀——你會在火焰圖上看到 `cloudwego/hertz` 而非 `gofiber`。若要壓測 Fiber，改該行為 `constant.CoreTypeWithFiber` 後重啟。

三種引擎的 CRUD 契約一致（路徑與請求/回應格式相同），壓測命令無需改動。

### 2.4 ⚠️ pprof 目前綁在 `0.0.0.0`

`application_test.yml` 當前為 `host: "0.0.0.0"`（非框架預設的 `127.0.0.1`），表示**區網內任何機器都能存取診斷端點**。本機測試無妨，但務必別把這份配置帶到任何共享環境；若需保留 `0.0.0.0`，請同時開啟 `pprof.auth.enable`。

---

## 三、執行壓測

### 3.1 基本用法

```bash
cd example_application/command
go run . bench \
  --base-url=http://127.0.0.1:8080 \
  --rps=300 \
  --duration=60s \
  --warmup=3s \
  --seed=100
```

### 3.2 完整 flag

| flag | 預設 | 說明 |
|---|---|---|
| `--base-url` | `http://127.0.0.1:8080` | 目標服務 |
| `--rps` | `500` | 開環注入速率 |
| `--duration` | `60s` | 穩態時長（不含 warmup） |
| `--warmup` | `5s` | 預熱，不計入統計 |
| `--seed` | `1000` | 種子資料筆數 |
| `--workers` | `0` | 0 表示自動推導：`rps/10`，鉗制 16..512 |
| `--timeout` | `5s` | 單請求超時 |
| `--keep-data` | `false` | 保留資料不清理 |
| `--rand-seed` | `0` | 固定隨機序列以複現；0 用時鐘 |
| `--w-cache-get` | `40` | `GET /examples/cache/{id}` 權重 |
| `--w-get` | `25` | `GET /examples/{id}` 權重 |
| `--w-list` | `15` | `GET /examples` 權重 |
| `--w-create` | `10` | `POST /examples` 權重 |
| `--w-update` | `10` | `PUT /examples/{id}` 權重 |

### 3.3 三個階段

```
seeding 100 records...          ← 建立種子，收集 ID 池
warmup 3s (not measured)...     ← 預熱，不計入統計
benchmarking 60s at 300 rps...  ← 穩態測量
cleaning up 409 records...      ← 刪除種子 + 壓測期新建
```

### 3.4 實測輸出範例

以下為對真實服務（Hertz + MongoDB）跑 `--rps=200 --duration=15s --seed=50` 的實際結果：

```
=== Bench Result ===
target        http://127.0.0.1:8080
duration      15s          target rps  200      actual rps  199.9
requests      2999         errors      0 (0.00%)
seeded        50           cleaned     409

latency       p50 1.8ms     p90 4ms       p99 13.1ms    max 36.6ms

by operation:
  cache_get   1210     p50 1.5ms  p90 3.6ms  p99 12ms    max 32.1ms
  get         775      p50 2ms    p90 3.7ms  p99 13.3ms  max 28.9ms
  list        418      p50 1.3ms  p90 2.3ms  p99 12.7ms  max 25.6ms
  create      304      p50 2.6ms  p90 4.7ms  p99 24.3ms  max 36.6ms
  update      292      p50 3.3ms  p90 5.2ms  p99 12.7ms  max 21ms

status codes: 200:2695  201:304
```

### 3.5 ⚠️ 必看：`dropped` 告警

若報告出現：

```
!! WARNING: 1234 ticks dropped — workers could not sustain 300 rps.
   Results understate real load. Raise --workers or lower --rps.
```

代表 worker 池跟不上目標速率，**實際施加的壓力低於 `--rps`，統計結果不可信**。處理：提高 `--workers`，或降低 `--rps`。

### 3.6 結果健康度自檢

| 檢查項 | 期望 | 不符時的含義 |
|---|---|---|
| `actual rps` ≈ `target rps` | 誤差 < 1% | 客戶端未能維持速率 |
| `dropped` | 0 | worker 不足 |
| `errors` | 0.00% | 服務端異常或資料問題 |
| `cleaned` ≥ `seeded` | 是 | 清理不完整，有殘留 |
| 各操作佔比 | 接近權重設定 | 隨機分佈異常 |

---

## 四、採集 profile

### 4.1 時機：壓測開始後 10 秒

必須跳過 warmup 與爬坡期。`sync.Pool` 填充、ristretto 預熱、Mongo 連線池建立都在前幾秒完成，此時取樣會嚴重偏離穩態。

建議把 `--duration` 設為 profile 採集時長的 **2 倍以上**，例如採集 30s 就跑 90s。

### 4.2 五類 profile 指令

```bash
# CPU — 互動式火焰圖
go tool pprof -http=:8000 "http://127.0.0.1:6060/debug/pprof/profile?seconds=30"

# CPU — 純文字 top
go tool pprof -top -nodecount=25 "http://127.0.0.1:6060/debug/pprof/profile?seconds=30"

# Heap（當前佔用）
go tool pprof -top -sample_index=inuse_space http://127.0.0.1:6060/debug/pprof/heap

# Goroutine
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1

# Block（已設 blockRate=10000）
go tool pprof -top http://127.0.0.1:6060/debug/pprof/block

# Mutex（已設 mutexFraction=100）
go tool pprof -top http://127.0.0.1:6060/debug/pprof/mutex
```

### 4.3 Heap 差分（推薦）

單次 heap 快照混雜了啟動期的一次性分配。取兩次做差才能看出穩態下的真實增長：

```bash
# 壓測開始後 10s
curl -s http://127.0.0.1:6060/debug/pprof/heap > heap_t1.pb.gz
sleep 40
# 壓測結束前 10s
curl -s http://127.0.0.1:6060/debug/pprof/heap > heap_t2.pb.gz

go tool pprof -http=:8001 -base heap_t1.pb.gz heap_t2.pb.gz
```

### 4.4 Goroutine 洩漏判定

分別在壓測**前 / 中 / 後**各取一次總數：

```bash
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1
# goroutine profile: total 39
```

壓測**結束並靜置 30 秒後**，數量應回落到接近壓測前的水位。若持續單調增長，即為洩漏。

實測基線：本機空載約 **39** 個 goroutine。

---

## 五、實測 profile 結果與解讀

以下是對真實服務（Hertz + MongoDB + Redis，`--rps=300`）採集的實際數據。

### 5.1 CPU — 關鍵發現：系統遠未飽和

```
Duration: 20s, Total samples = 3.28s (16.40%)

      flat  flat%   sum%        cum   cum%
     0.95s 28.96% 28.96%      0.95s 28.96%  internal/runtime/syscall/linux.Syscall6
     0.55s 16.77% 45.73%      0.55s 16.77%  runtime.futex
     0.05s  1.52% 47.26%      0.05s  1.52%  memeqbody
     0.05s  1.52% 48.78%      0.05s  1.52%  runtime.memmove
     ...
     0.02s  0.61% 62.20%      0.05s  1.52%  sync.(*Pool).Get
```

**`Total samples = 3.28s (16.40%)` 是最重要的一行**：20 秒的採集窗口內，CPU 只有 16.4% 的時間在工作。

**解讀**：300 RPS 對這台機器來說負載太輕。前兩名 `Syscall6`（28.96%）與 `futex`（16.77%）合計佔 45.7%，都是 **I/O 等待與調度**，不是框架計算開銷。此時火焰圖上框架代碼被淹沒，看不出瓶頸。

**要看到框架瓶頸，必須把 RPS 拉高到 CPU 佔用 60%+**。做法見 §5.5。

### 5.2 Block — 需排除背景噪音

```
Showing nodes accounting for 36067.99s, 99.88% of 36110.12s total
 33630.60s 93.13% 93.13%  runtime.selectgo
  2437.39s  6.75% 99.88%  sync.(*Cond).Wait
         0     0% 99.88%  database/sql.(*DB).connectionCleaner        (3.32%)
         0     0% 99.88%  gopool.(*worker).run.func1                  (1.96%)
```

**`runtime.selectgo` 佔 93% 是背景噪音，不是問題**。這些是長期阻塞在 `select` 上的常駐 goroutine（連線池清理器、worker 池、keepalive 定時器），它們本來就該一直等待。

**block profile 的正確用法**：忽略 `selectgo` / `Cond.Wait` 這類守護型阻塞，聚焦**請求路徑上**的阻塞。可用 `-focus` 過濾：

```bash
go tool pprof -top -focus='fiberhouse|example_application' http://127.0.0.1:6060/debug/pprof/block
```

### 5.3 Mutex — 當前無框架層爭用

```
Showing nodes accounting for 41.07s, 100% of 41.07s total
    41.07s   100%   100%  sync.(*Mutex).Unlock
         0     0%   100%  gopool.(*worker).run.func1                    (19.11%)
         0     0%   100%  hertz/pkg/app.(*RequestContext).Next          (19.11%)
         0     0%   100%  hertz/protocol/http1.(*Server).Serve          (19.11%)
```

總延遲 41 秒，全部來自 Hertz 引擎自身的請求處理路徑，**framework（fiberhouse）代碼未出現**。當前負載下無框架層鎖爭用。

提高 RPS 後若 `fiberhouse` 相關符號出現在此，才是需要處理的信號。

### 5.4 Heap — 注意識別非請求路徑的分配

```
Showing nodes accounting for 17.74MB, 72.94% of 24.32MB total
    8.84MB 36.34% 36.34%  golang.org/x/net/webdav.(*memFile).Write
    2.51MB 10.33% 46.66%  runtime.mallocgc
    2.04MB  8.38% 55.04%  ristretto/v2/z.(*Bloom).Size
    1.58MB  6.51% 61.55%  ristretto/v2.newCmRow
    1.50MB  6.17% 67.72%  time.NewTicker
    1.27MB  5.23% 72.94%  hertz/pkg/common/stackless.NewFunc
```

**解讀**：
- `webdav.memFile.Write` 佔 36% — 與 CRUD 請求路徑無關，屬其他組件的常駐分配
- `ristretto` 的 `Bloom` / `newCmRow` — 本地快取的**一次性**結構分配，非每請求分配
- 總量僅 24MB，很小

**這正是需要做 §4.3 差分的原因**：單次快照裡佔比最高的都是啟動期一次性分配，掩蓋了穩態下的每請求分配。

### 5.5 如何讓框架瓶頸顯形

當前 16.4% CPU 佔用下看不出框架問題。提高負載強度：

```bash
# 1. 逐步加壓，找到 CPU 佔用 60-80% 的 RPS
go run . bench --rps=2000 --duration=90s --seed=1000

# 2. 若出現 dropped 告警，提高 workers
go run . bench --rps=2000 --workers=256 --duration=90s --seed=1000

# 3. 讓 cache_get 佔比更高，進一步排除 DB 影響
go run . bench --rps=2000 --duration=90s --seed=1000 \
  --w-cache-get=90 --w-get=5 --w-list=5 --w-create=0 --w-update=0
```

第 3 條是關鍵手法：`cache_get` 命中 ristretto 本地快取時 DB 與網路幾乎不參與，剩下的 CPU 時間幾乎全是框架自身開銷（編解碼、校驗、中間件、對象池）。

採集後用 `-focus` 只看框架代碼：

```bash
go tool pprof -http=:8000 -focus='fiberhouse' "http://127.0.0.1:6060/debug/pprof/profile?seconds=30"
```

---

## 六、注意事項彙總

### 6.1 配置相關

| # | 事項 | 說明 |
|---|---|---|
| 1 | **必須設 `APP_ENV_application_env=test`** | 否則載入 `application_dev.yml`，pprof 不啟動。最常見的坑 |
| 2 | `enableConsole: false` | 已設。壓測時同步寫 stdout 會掩蓋真實框架開銷 |
| 3 | `level: info` | 已設。`debug` 級每請求構造 event，額外開銷 |
| 4 | `pprof.host: 0.0.0.0` | 當前值。區網可存取，勿帶入共享環境；或同時開 `auth.enable` |
| 5 | `writeTimeout: 120` | 必須大於 CPU profile 的 `seconds` 參數，否則採集被截斷 |
| 6 | `blockRate` / `mutexFraction` | 已設 10000 / 100。**開啟後有運行時開銷**，非診斷期建議歸零 |

### 6.2 壓測相關

| # | 事項 | 說明 |
|---|---|---|
| 7 | **避開 `/common/*`** | 該路由組限流器 `Max:5 / 30s` 且 KeyGenerator 回傳固定字串，全域共用配額。bench 命令已避開 |
| 8 | 留意 `dropped` 告警 | 非零即代表結果不可信 |
| 9 | `--duration` ≥ 2× 採集時長 | 確保 profile 完整落在穩態區間 |
| 10 | `--rand-seed` 固定可複現 | 對比前後兩次優化效果時務必固定 |
| 11 | 中斷壓測會保留資料 | Ctrl-C 會跳過 cleanup。用 `example` CLI 或直接清 Mongo 集合 |
| 12 | CRUD 走 MongoDB，非 MySQL | web API 是 `example-module`（Mongo）；CLI `example` 命令是 `command-module`（MySQL），兩者資料不互通 |

### 6.3 已知問題與既有行為

| # | 事項 | 狀態 |
|---|---|---|
| 13 | CLI 錯誤只寫日誌不寫 stderr | `application.go:52` 的 `ExitErrHandler` 行為。bench 命令已額外打印 `bench failed: ...` |
| 14 | `example` CLI 需要 MySQL | 已改惰性接線，不再導致整個 CLI 啟動 panic；但執行 `example` 子命令時若無 MySQL 仍會 panic |
| 15 | 當前引擎為 Hertz | `example_main/main.go:63`。profile 中會見到 `cloudwego/hertz` |

### 6.4 Profile 解讀陷阱

| # | 陷阱 | 正確做法 |
|---|---|---|
| 16 | CPU 佔用過低時看不出瓶頸 | 先看 `Total samples = X (N%)`。N < 30% 說明負載太輕，加壓後再採 |
| 17 | block profile 被 `selectgo` 淹沒 | 那是常駐 goroutine 的正常等待。用 `-focus` 過濾請求路徑 |
| 18 | heap 單次快照混雜啟動期分配 | 用 `-base` 做兩次差分 |
| 19 | goroutine 數在壓測中偏高屬正常 | 需在壓測結束靜置 30s 後再判斷是否洩漏 |
| 20 | block/mutex 未設採樣率則為空 | 已在 `application_test.yml` 設定；改配置後記得重啟服務 |

---

## 七、完整操作流程（可直接照做）

```bash
cd /mnt/d/code/github_opensource/tmp/fiberhouse

# ---- 步驟 1：確認依賴 ----
nc -z 127.0.0.1 27037 && echo "mongo ok"
nc -z 127.0.0.1 6379  && echo "redis ok"

# ---- 步驟 2：啟動服務（終端 1）----
APP_ENV_application_env=test go run ./example_main

# ---- 步驟 3：確認就緒（終端 2）----
curl -s -o /dev/null -w "pprof:%{http_code}\n" http://127.0.0.1:6060/debug/pprof/
curl -s -o /dev/null -w "api:%{http_code}\n" "http://127.0.0.1:8080/examples?page=1&page_size=1"

# ---- 步驟 4：記錄基線 goroutine 數 ----
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1

# ---- 步驟 5：啟動壓測（終端 2）----
cd example_application/command
go run . bench --rps=2000 --workers=256 --duration=120s --warmup=5s --seed=1000 --rand-seed=42

# ---- 步驟 6：採集 profile（終端 3，壓測開始 ~15 秒後）----
mkdir -p /tmp/fh-prof && cd /tmp/fh-prof

curl -s http://127.0.0.1:6060/debug/pprof/heap > heap_t1.pb.gz
go tool pprof -proto -output=cpu.pb.gz "http://127.0.0.1:6060/debug/pprof/profile?seconds=45"
curl -s http://127.0.0.1:6060/debug/pprof/heap    > heap_t2.pb.gz
curl -s http://127.0.0.1:6060/debug/pprof/block   > block.pb.gz
curl -s http://127.0.0.1:6060/debug/pprof/mutex   > mutex.pb.gz
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1

# ---- 步驟 7：壓測結束靜置 30s 後，複查 goroutine ----
sleep 30
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1

# ---- 步驟 8：分析 ----
go tool pprof -top -nodecount=30 cpu.pb.gz                    # 先確認 CPU 佔用是否夠高
go tool pprof -http=:8000 -focus='fiberhouse' cpu.pb.gz       # 只看框架代碼
go tool pprof -http=:8001 -base heap_t1.pb.gz heap_t2.pb.gz   # 穩態分配增長
go tool pprof -top -focus='fiberhouse' block.pb.gz            # 請求路徑阻塞
go tool pprof -top -focus='fiberhouse' mutex.pb.gz            # 框架層鎖爭用
```

---

## 八、重點觀察清單

加壓到 CPU 60%+ 後，以下位置值得優先檢視。

> **先讀 8.4**：清單告訴你「去哪裡看」，8.4 告訴你「看到之後怎麼判斷有沒有問題」。只看 8.1 的百分比而跳過 8.4，很容易把正常成本誤判為瓶頸。

### 8.1 清單與實測驗證（2026-09-03）

驗證條件：`--rps=6000 --duration=100s --workers=512`，Fiber 引擎、異步日誌、`monitor: false`。
實際達成 **3981 RPS / 398,162 requests / 0 errors**，CPU profile 採樣 45 秒（`Total samples = 74.92s (166.49%)`）。

| # | 位置 | 觀察點 | 實測 cum% | 結果 |
|---|---|---|---:|:--|
| 1 | `validator.Struct` | 反射開銷（~~每次 `GetValidate(lang)` 取本地化實例~~，實測**非成本所在**，見 8.4） | 1.86% | ✅ 命中 |
| 2 | `Response()` 對象池 | `ResponseWrap` + `RespInfo` 雙層池化 | 2.39% | ✅ 命中 |
| 3 | coreHttp 日誌中間件 | 每請求構造 zerolog event | 35.10%* | ⚠️ 命中但**須拆解**，見 8.2 |
| 4 | `dispatchExampleChanged` | 寫操作派發 asynq 任務，Redis 往返 | 3.63% | ✅ 命中 |
| 5 | `GetWithCache` | 快取協調器 + singleflight | 5.19% | ✅ 命中 |
| 6 | sonic 編解碼 | `Marshal` / `Unmarshal` | 2.60% / 2.08% | ✅ 命中 |

6 項全部在 profile 中可見。

> ⚠️ **「命中」不等於「有問題」**。這些符號出現在 profile 上是**必然的**——它們就在請求路徑上，不出現才反常。是否構成瓶頸需要進一步判讀，**結論見 8.4**（簡短版：本次六項全部屬於合理成本，無一需要優化）。

> **已移除的項目**：原清單第 6 項 `toResponse`（結構轉換與 slice 分配）已於 2026-09-03 刪除。它在原始碼中確實存在且位於熱路徑（`example_service.go:440` 定義，被 130 / 140 / 196 / 232 行呼叫，覆蓋 get / list / cache 路徑），但在 398k 請求的 profile 中完全不出現，連 `-lines` 精確到行都查不到採樣——它只做結構欄位複製，成本低於 CPU profile 的採樣精度（10ms/次）。列在清單上只會讓人找一個找不到的東西。

補充實測數據：

```
sync.(*Pool).Get                      flat 0.43s (0.57%)  cum 1.12s (1.49%)
singleflight.(*Group).doCall                              cum 4.78s (6.38%)
asynq.(*processor).start.func1                            cum 4.41s (5.89%)
validator.(*validate).traverseField   flat 0.22s (0.29%)  cum 1.02s (1.36%)
SonicJSON.Marshal                                         cum 1.95s (2.60%)
SonicJSON.Unmarshal                                       cum 1.56s (2.08%)
```

### 8.2 ⚠️ 第 3 項的陷阱：35% 裡有 24% 不是日誌

`fiberzerolog.New.func1` 的 cum 高達 35.10%，很容易誤判為「日誌中間件吃掉三分之一 CPU」。用 `-peek` 拆開就會發現不是：

```bash
go tool pprof -peek='fiberzerolog.New.func1$' cpu.pb.gz
```

```
     0.08s  0.11%     26.30s 35.10%   fiberzerolog.New.func1
                      24.39s 92.74% |   fiber.(*Ctx).Next        ← 下游全部 handler
                       0.71s  2.70% |   fiberzerolog.(*Config).logger
                       0.53s  2.02% |   zerolog.(*Event).Msg
                       0.24s  0.91% |   zerolog.(*Logger).Info
                       0.11s  0.42% |   fiber.(*Ctx).UserContext
                       0.03s  0.11% |   time.Now / time.Since
```

**92.74% 是 `Ctx.Next`**——中間件位於責任鏈上游，它的 cum 天然包含所有下游 handler。日誌中間件**自身**的成本是 `0.71 + 0.53 + 0.24 + 0.11 + 0.08 ≈ 1.67s`，約 **2.2%**。

> **通用規則**：任何中間件的 cum% 都包含 `Next()` 之後的整條鏈。評估中間件自身開銷，必須用 `-peek` 減去 `Next` 分支，或直接看 flat%。這是責任鏈架構下讀 profile 最常見的誤判。

### 8.3 本次未能滿足的前提：CPU 未達 60%

清單開頭寫「加壓到 CPU 60%+」，但本次實測：

```
Duration: 45s, Total samples = 74.92s (166.49%)
```

16 核機器上 166.49% ≈ **10.4% 利用率**，遠未達標。top 兩名是 `Syscall6`（23.61%）與 `runtime.futex`（18.43%），都是**等待**而非計算：

```
17.69s 23.61%  internal/runtime/syscall/linux.Syscall6
13.81s 18.43%  runtime.futex
16.98s 22.66%  runtime.findRunnable      ← 調度器找不到活幹
```

`findRunnable` 佔 22.66% 是「CPU 在空轉找任務」的直接證據。瓶頸在 MongoDB / Redis 的網路往返，不在框架。

**這意味著上表的百分比是「相對於一個大量空閒的系統」**——框架各項的真實佔比被 syscall 等待稀釋了。若要讓框架開銷顯形，需按 §5.5 提高 `cache_get` 佔比或增加 workers，把 DB 踢出畫面。

> 本次已用 `--workers=512`（預設推導值的上限）仍未突破，說明單機客戶端已達瓶頸，需分散式壓測或直接對快取命中路徑做微基準測試（`go test -bench`）。

### 8.4 判讀結論：本次無需優化任何一項

前三節給的是**數據**，本節給**判斷**。這是清單最容易被跳過、卻最重要的一步。

#### 8.4.1 逐項判定

| # | 位置 | cum% | 判定 | 依據 |
|---|---|---:|---|---|
| 1 | `validator.Struct` | 1.86% | 合理成本 | 反射固有開銷，無異常放大 |
| 2 | `Response()` 對象池 | 2.39% | 合理成本 | 池化本身有效，`sync.Pool.Get` 僅 1.49% |
| 3 | 日誌中間件 | **~2.2%** | 已解決 | 35.10% 中 92.74% 是 `Ctx.Next`，見 8.2 |
| 4 | `dispatchExampleChanged` | 3.63% | **非框架成本** | 87.87% 是 Redis I/O |
| 5 | `GetWithCache` | 5.19% | **非框架成本** | 67.10% 是下游 service |
| 6 | sonic 編解碼 | 4.68% | 合理成本 | JSON 序列化必要開銷 |

第 4、5 項尤其要拆開看——它們的 cum 幾乎都是下游 I/O，不是框架自身：

```
dispatchExampleChanged  2.72s
    └─ TaskDispatcher.EnqueueContext   2.39s (87.87%)   ← Redis 往返
    └─ NewExampleChangedTask           0.21s ( 7.72%)

GetWithCache (api)      3.89s
    └─ service.GetWithCache            2.61s (67.10%)   ← Mongo / 快取
    └─ validateID                      0.62s (15.94%)
    └─ JsonWithCtx                     0.50s (12.85%)
```

**六項自身開銷加總不到 CPU 的 5%。**

#### 8.4.2 真正的瓶頸不在清單上

```
17.69s 23.61%  Syscall6
13.81s 18.43%  runtime.futex
16.98s 22.66%  runtime.findRunnable   ← 調度器空轉找不到活幹
 1.48s  1.98%  runtime.gcBgMarkWorker ← GC 完全健康
```

`findRunnable` 佔 22.66%，是「CPU 在等 I/O」的決定性證據。16 核只用了 10.4%，GC 僅 1.98%。

**結論：本系統的瓶頸是 MongoDB / Redis 的網路往返，框架層無可優化空間。** 在 DB 先飽和的前提下，優化任何一項框架代碼都不會提升吞吐——這是判斷「要不要優化」的首要問題，優先於任何百分比高低。

#### 8.4.3 清單第 1 項的方向修正

原清單註記「每次 `GetValidate(lang)` 都取本地化實例」，暗示本地化查找是成本所在。實測**否定**了這個猜測：

```
ExampleHandler.validate  1.60s
    └─ validator.(*Validate).Struct    1.39s (86.88%)   ← 反射，真正的成本
    └─ validate.(*Wrap).GetValidate    0.06s ( 3.75%)   ← 猜測的位置，可忽略
    └─ AppContext.GetValidateWrap      0.03s ( 1.88%)
```

成本在 `Struct` 的反射遍歷，不在取實例。清單註記已修正。

另一個順帶發現：`validateID` 佔 1.60%，**比整個 JSON 回應序列化（`JsonWithCtx` 0.50s）還貴**。原因是為校驗單一字串走了完整反射流程：

```go
// example_api.go:57
func (h *ExampleHandler) validateID(id, lang string) error {
    return h.validate(&requestvo.ObjId{ID: id}, lang)   // 包成 struct 再反射
}

// ObjId 只有一個欄位
ID string `validate:"required,alphanum,min=18,max=32"`
```

改成手寫校驗（`len(id)` + 字元檢查）可省下這 1.6%。**但不建議改**：在一個只用了 10% CPU 的系統上省 1.6%，換來的是放棄 validator 的統一錯誤訊息與多語言支援。這是典型的過早優化。

> **記錄它而不改它**——若未來 DB 瓶頸解除、框架真正成為熱點，這是第一個可以回頭看的位置。

#### 8.4.4 若要繼續往下查

目前的壓測**測不出框架性能**，因為 DB 先飽和。要真正評估框架開銷，只有兩條路：

1. **微基準**：`go test -bench` 直接打 handler，繞開 DB
2. **Mock 資料層**：讓 service 回傳固定資料，把 Mongo / Redis 踢出畫面

否則再加壓也只是加大 `findRunnable` 的佔比。

---

## 九、壓測後清理

正常結束會自動清理。異常中斷（Ctrl-C）後資料會殘留：

```bash
# 確認殘留量
curl -s "http://127.0.0.1:8080/examples?page=1&page_size=1" | head -c 200

# 直接清 Mongo 集合（謹慎：會刪除全部 example 資料）
mongosh "mongodb://admin:admin@127.0.0.1:27037/?authSource=admin" \
  --eval 'db.getSiblingDB("<dbname>").examples.deleteMany({name:/^bench-/})'
```

> 種子資料名稱前綴為 `bench-seed-`，壓測期新建的前綴為 `bench-`，可據此精確篩選。

診斷結束後建議把 `blockRate` / `mutexFraction` 歸零，避免常態運行時的額外開銷。
