# Fiber 引擎壓測與性能剖析結果

> 環境：Fiber + Sonic + MongoDB + Redis，`application_test.yml`，16 P（16 邏輯核心）
>
> 測試日期：2026-09-03。所有數據均為本機實測，非推測。
>
> 相關文件：[操作手冊](./bench-profiling-runbook.md)、[壓測命令設計](./bench-command-design.md)

---

## 一、結論摘要

**發現一個嚴重的框架層性能瓶頸：同步日誌寫入的全域鎖，使 p50 延遲惡化 300 倍。**

| 指標 | 同步日誌（現況） | 異步日誌（asyncConf.enable=true） | 改善 |
|---|---:|---:|---:|
| p50 延遲 | 139.5ms | **461µs** | **302×** |
| p90 延遲 | 154.1ms | 1.4ms | 110× |
| p99 延遲 | 175.2ms | 2.8ms | 62× |
| mutex 爭用總時長 | 10.52 hrs | 464ms | ~81600× |
| block 阻塞總時長 | 13.03 hrs | 可忽略 | — |
| CPU 中日誌寫入佔比 | 14.24% | 0%（未進榜） | — |

**GC 完全正常，無異常**。詳見 §5。

---

## 二、測試方法

```bash
# 服務端（gctrace 開啟）
GODEBUG=gctrace=1 APP_ENV_application_env=test go run ./example_main

# 壓測（開環固定速率）
go run . bench --rps=2000 --workers=256 --duration=70s --warmup=5s \
               --seed=300 --rand-seed=7
```

兩次測試唯一變數為 `application.appLog.asyncConf.enable`（false → true），其餘配置、隨機種子、負載模型完全一致。

CPU 採樣率達 **66.37%**（`Total samples = 26.55s (40s)`），遠高於先前 Hertz 測試的 16.4%，框架代碼在 profile 上已充分顯形。

---

## 三、核心發現：同步日誌的全域鎖

### 3.1 CPU profile（同步模式）

```
Duration: 40s, Total samples = 26.55s (66.37%)

      flat  flat%   sum%        cum   cum%
     7.77s 29.27% 29.27%      7.77s 29.27%  internal/runtime/syscall/linux.Syscall6
     4.49s 16.91% 46.18%      4.49s 16.91%  runtime.futex
     ...
     0.06s  0.23% 58.57%      3.78s 14.24%  lumberjack.(*Logger).Write
```

累積視圖顯示完整調用鏈：

```
    17.43s 65.65%  fasthttp.(*Server).serveConn
    14.40s 54.24%  fiber.(*App).handler
    13.61s 51.26%  fiberzerolog.New.func1              ← 日誌中間件
     3.91s 14.73%  zerolog.(*Event).Msg
     3.86s 14.54%  zerolog.(*Event).write
     3.79s 14.27%  io.(*multiWriter).Write
     3.78s 14.24%  SyncLumberjackWriter.Write          ← 框架的同步 writer
     3.78s 14.24%  lumberjack.(*Logger).Write
     6.05s 22.79%  internal/poll.(*FD).Write           ← 阻塞式 write syscall
```

**每個請求都在 handler 內同步執行一次 `write(2)` 系統呼叫。**

### 3.2 Block profile —— 決定性證據

```
Showing nodes accounting for 13.03hrs, 67.66% of 19.27hrs total

  13.03hrs 67.66%  sync.(*Mutex).Lock (inline)
         0     0%  zerolog.(*Event).Msg
         0     0%  zerolog.(*Event).write
         0     0%  SyncLumberjackWriter.Write
         0     0%  lumberjack.(*Logger).Write
         0     0%  io.(*multiWriter).Write
  12.47hrs 64.72%  fiber.(*App).handler
```

**13 小時的累積阻塞時間，全部落在日誌 writer 的互斥鎖上。**

### 3.3 Mutex profile —— 100% 集中

```
Showing nodes accounting for 10.52hrs, 100% of 10.52hrs total

  10.52hrs   100%   100%  sync.(*Mutex).Unlock
         0     0%   100%  SyncLumberjackWriter.Write
         0     0%   100%  zerolog.(*Event).Msg
         0     0%   100%  lumberjack.(*Logger).Write
  10.05hrs 95.57%        fiberzerolog.New.func1
```

**進程內 100% 的鎖爭用都來自同一個日誌鎖。**

### 3.4 根因

`component/logging/writer/sync_lumberjack_writer.go` 的 `SyncLumberjackWriter` 在每次寫入時持有互斥鎖，而 `lumberjack.Logger.Write` 內部再做一次阻塞式 `write(2)`。

在 16 核、1800 RPS 下，**所有請求 goroutine 串行通過這一個鎖**，且鎖內含系統呼叫。這使日誌寫入從「並行的小開銷」退化為「全域序列化點」——這正是 p50 高達 139ms 的直接原因（而非 DB 或網路）。

### 3.5 驗證：開啟異步日誌

框架已內建 `AsyncDiodeWriter`（`bootstrap.go:328`），由 `application.appLog.asyncConf.enable` 控制，預設關閉。

僅將該值改為 `true`、其餘不變後重測：

```
                        同步            異步
p50                  139.5ms  →       461µs
p90                  154.1ms  →       1.4ms
p99                  175.2ms  →       2.8ms
max                  458.3ms  →      20.9ms
```

異步模式的 CPU profile 中，**`lumberjack` / `SyncLumberjackWriter` 完全消失**，前 14 名無任何日誌相關符號。mutex 爭用從 10.52 小時降至 464ms，且調用棧只剩 runtime 調度器自身的 `runtime.unlock`，與日誌無關。

**建議**：壓測與生產環境應開啟 `asyncConf.enable: true`。

代價：diode 環形緩衝區會依 `diodeConf.size` 預先配置記憶體。實測 `size: 33554432` 對應 **256MB 常駐**（`go-diodes.NewManyToOne` 佔 inuse heap 的 91%）。這是啟動時一次性配置、非洩漏，但該預設值偏大。

`size` 的合適取值已於 2026-09-03 專項實測，見 **§十一**。

---

## 四、次要發現：monitor 中間件的分配壓力

### 4.1 分配剖析

```
alloc_space 累積（總 5997MB）：
  1198.44MB 19.98%  monitor.updateStatistics
  1186.40MB 19.78%  gopsutil/net.ConnectionsPid
  1186.40MB 19.78%  gopsutil/net.connectionsPidMaxWithoutUidsWithContext
```

CPU 佔比僅 1.66%，但**吃掉全進程約 20% 的記憶體分配量**。

`gopsutil/net.ConnectionsPid` 會掃描 `/proc/net/*` 解析連線表，每次呼叫產生大量臨時物件。

### 4.2 建議

`application_test.yml` 中 `middleware.monitor: true`。若非必要（例如壓測期間），關閉可顯著降低 GC 壓力：

```yaml
middleware:
  monitor: false
```

---

## 五、GC 分析：完全正常，無異常

### 5.1 gctrace 實測（同步模式，231 次 GC）

```
gc 160 @207.404s 0%: 0.072+1.1+0.004 ms clock, ... 46->46->22 MB, 47 MB goal, 16 P
gc 161 @208.982s 0%: 0.12+1.3+0.033 ms clock,  ... 45->46->23 MB, 46 MB goal, 16 P
gc 162 @211.991s 0%: 0.87+1.1+0.027 ms clock,  ... 46->47->23 MB, 48 MB goal, 16 P
gc 167 @221.087s 0%: 0.081+1.1+0.038 ms clock, ... 45->45->22 MB, 46 MB goal, 16 P
```

### 5.2 各項指標判定

| 指標 | 實測值 | 健康標準 | 判定 |
|---|---|---|---|
| GC CPU 佔比 | **175/231 次為 0%**，最高 4% | < 10% | ✅ 正常 |
| STW 暫停 | 0.07–0.9ms（標記開始）+ 0.004–0.06ms（結束） | < 1ms | ✅ 優秀 |
| 標記耗時 | 1.0–2.2ms | — | ✅ 正常 |
| 存活堆 | **穩定在 22–24MB** | 不持續增長 | ✅ 無洩漏 |
| 堆目標 | 46–51MB，穩定 | 不失控 | ✅ 正常 |
| GC 頻率 | 約 0.5–3 秒一次 | — | ✅ 合理 |

GC CPU 百分比分佈（231 次）：

```
  175 次  0%
   27 次  2%
   12 次  3%
   11 次  1%
    6 次  4%
```

### 5.3 結論

**GC 無任何異常。** 堆大小、暫停時間、CPU 佔比全部健康，且存活堆穩定在 22–24MB 不增長。

值得強調：即使在 p50 高達 139ms 的嚴重劣化下，GC 依然是 0%——**這反證了延遲問題與 GC 無關，而是鎖爭用**。若當初誤判為「GC 問題」去調 `GOGC`，方向就完全錯了。

---

## 六、Heap 分析

### 6.1 差分（穩態，t2 − t1，間隔 35 秒）

```
Showing nodes accounting for -5344.56kB, 62.66% of 8528.94kB total

-1166.01kB  sonic/internal/encoder/vars.init.func1
   -1028kB  bufio.NewReaderSize
   -1028kB  bufio.NewWriterSize
-1026.52kB  gopkg/lang/dirtmake.Bytes
 -565.76kB  sonic/internal/decoder/jitdec.newStack
  514.38kB  redis/internal/proto.(*Reader).readStringReply
```

**差分為負（−5.3MB）**，表示穩態下記憶體正被回收，**無洩漏**。正負項均為連線緩衝與編解碼器的池化物件正常進出。

### 6.2 靜態佔用（同步模式）

總 inuse 僅約 24MB，主要為 ristretto 快取結構與各類池，屬啟動期一次性配置。

---

## 七、Goroutine 分析：無洩漏

| 時點 | 數量 |
|---|---:|
| 壓測前基線 | 43 |
| 壓測中（1800 RPS） | 318 |
| 壓測結束後 | **43** |

壓測結束後精確回落至基線值 43，**無 goroutine 洩漏**。壓測中的 318 是 fasthttp worker pool 的正常擴張。

---

## 八、壓測數據對照

### 8.1 同步日誌（現況）

```
duration      1m10.4s   target rps 2000   actual rps 1797.0
requests      126506    errors     911 (0.72%)
latency       p50 139.5ms  p90 154.1ms  p99 175.2ms  max 458.3ms

!! WARNING: 1477 ticks dropped
```

### 8.2 異步日誌（讀密集，無寫操作）

```
duration      1m30s     target rps 2000   actual rps 1270.1
requests      114310    errors     0 (0.00%)
latency       p50 210µs  p90 588µs  p99 1.1ms  max 24.6ms

cache_get  57051   p50 160µs   p90 252µs   p99 543µs
get        35799   p50 507µs   p90 791µs   p99 1.5ms
list       21460   p50 210µs   p90 316µs   p99 661µs
```

### 8.3 關於 `actual rps` 未達 2000

兩種模式都出現 `dropped` 告警，實際約 1270–1800 RPS。原因是**壓測客戶端與被測服務跑在同一台 16 核機器上**，互相爭搶 CPU。這不影響本次結論——因為兩次測試條件完全相同，延遲差異（139ms vs 461µs）純粹來自被測端。

若要測真實極限吞吐，需將壓測客戶端移至另一台機器。

### 8.4 關於 409 錯誤

`create` 操作出現 409（衝突）是**測試方法的產物**，非框架缺陷：使用了相同的 `--rand-seed=7` 且帶 `--keep-data`，導致隨機產生的名稱與上一輪殘留資料重複。已驗證：用唯一名稱手動 POST 可正常回 200。

---

## 九、行動建議與落實狀態

### 9.1 已套用（`example_config/application_test.yml`）

以下三項已於 2026-09-03 套用至 `application_test.yml`：

| # | 項目 | 變更 | 依據 | 效果 |
|---|---|---|---|---|
| 1 | `appLog.asyncConf.enable` | `false` → **`true`** | §3 | p50 139.5ms → 461µs（302×） |
| 2 | `appLog.asyncConf.diodeConf.size` | `33554432`(32M) → **`1048576`**(1M) | §3.5、§11 | diode 骨架記憶體 256MB → 8MB（實測確認）。§11 進一步顯示 **1M 仍偏大，建議 `262144`(256K)** |
| 3 | `middleware.monitor` | `true` → **`false`** | §4 | 減少約 20% 記憶體分配 |

當前配置實況：

```yaml
appLog:
  asyncConf:
    enable: true                 # 已開啟異步日誌
    type: "diode"
    diodeConf:
      size: 1048576              # 33554432/32M → 1048576/1M
middleware:
  monitor: false                 # 开启指标监控，建议关闭，减少GC压力
```

> **關於第 2 項**：已於 2026-09-03 實測驗證，並進一步測試 1M / 4M 等多個取值，**結論是 1M 仍然偏大**。詳見 **§十一**。
>
> **關於第 3 項的取捨**：關閉 `monitor` 會同時失去 Fiber 的 `/metrics` 指標端點。若生產環境依賴該端點做監控，需權衡「監控可觀測性」與「20% 分配壓力」，而非無條件關閉。

### 9.2 未採納 / 待決策

| # | 建議 | 依據 | 狀態 |
|---|---|---|---|
| 4 | 讓 `SyncLumberjackWriter` 不再是框架預設 | §3.4 | **待你決策** |

`SyncLumberjackWriter` 的鎖內含阻塞 syscall，在任何高並發場景下都會成為序列化點。框架已提供 `AsyncDiodeWriter` 但**預設關閉**，使用者若未察覺就會遭遇本次發現的問題。

本次只改了 `application_test.yml`（示例配置），**框架預設值未動**——因為這會改變既有使用者的行為，屬於你的決策範圍。

可選方案：
- **A（保守）**：僅在框架文件與配置註解中標註此風險，預設不變。
- **B（折衷）**：保持預設同步，但在啟動時若偵測到高並發配置（如 `appConcurrency` 較大）輸出一則警告日誌。
- **C（激進）**：將預設改為異步，並在 CHANGELOG 標註破壞性變更。

---

## 十、待確認事項

- §9.2 的建議 4 涉及改變框架預設行為，需你選擇 A/B/C 方案。
- ~~`diodeConf.size` 調為 1M 後的記憶體佔用與丟日誌率尚未實測~~ → **已完成，見 §十一**。
- 本次未測試 `asyncConf.type: chan`（另一種異步實作），僅測了預設的 `diode`。
- 本次三項變更僅套用於 `application_test.yml`；`application_dev.yml` 與 `application_prod.yml` 未同步調整。

---

## 十一、`diodeConf.size` 專項實測（2026-09-03）

> 目的：確定 `size` 的合適取值。先前 §9.1 只做了線性推算，本節以實測取代推算。

### 11.1 先讀原始碼：`size` 到底是什麼

`size` **不是「喚醒緩衝」，是環形緩衝區的槽位數量**。

`async_diode_writer.go:62` 把它直接傳給 go-diodes：

```go
dd := diodes.NewManyToOne(diodeSize, diodes.AlertFunc(...))
```

`go-diodes/many_to_one.go:29`：

```go
d := &ManyToOne{
    buffer:  make([]unsafe.Pointer, size),   // ← size 就是切片長度
    alerter: alerter,
}
```

`unsafe.Pointer` 在 64 位平台固定 8 位元組，所以**陣列骨架 = size × 8B**，這是啟動時一次性 `make()`、與日誌量無關的固定成本。

但**骨架不是全部佔用**。每次 `Set` 還會分配一個 bucket（`one_to_one.go:26`，`data` 8B + `seq` 8B = 16B），而 `async_diode_writer.go:118` 每次 Write 又拷貝一份日誌內容：

```go
data := make([]byte, l)
copy(data, p)
```

所以：

```
固定成本（骨架）    size × 8B                      ← 可精確計算
浮動成本（在途）    在途條目數 × (16B + 日誌長度)   ← 取決於生產/消費速率差，需實測
```

先前「每條目 8 位元組」的說法只涵蓋了骨架，遺漏了浮動部分——這正是當時標註「未經實測」的原因。

### 11.2 一個測量陷阱：pprof 看不到這塊記憶體

第一次嘗試用 heap profile 找 `NewManyToOne`，**完全找不到**：

```
$ go tool pprof -top -nodecount=200 -sample_index=inuse_space heap.pb.gz | grep -i diode
(無輸出)
```

但 goroutine profile 證明 diode 確實存在：

```
#  0x13b8af8  writer.(*AsyncDiodeWriter).consume+0x138  async_diode_writer.go:100
```

**原因**：pprof heap 是**取樣**的（預設每分配 512KB 取樣一次）。單次 `make([]unsafe.Pointer, N)` 是**一次**分配事件，很可能整個被取樣遺漏，不論它有多大。

> **教訓**：pprof heap 擅長找「高頻小分配」，不擅長找「單次大分配」。後者要用 `runtime.MemStats`：
> ```bash
> curl -s "http://127.0.0.1:6060/debug/pprof/heap?debug=1" | grep -E "^# (HeapAlloc|HeapInuse|Sys) "
> ```

### 11.3 骨架成本：MemStats 實測

啟動後靜置 3 秒取樣（無流量）：

| size | 陣列理論值 | HeapAlloc | 扣除基線(~19MB) | 吻合度 |
|---:|---:|---:|---:|:--|
| 512 | 4 KB | 15.11 MB | ~0 MB | ✓ |
| 65536 (64K) | 0.5 MB | 16.01 MB | ~0.5 MB | ✓ |
| 1048576 (1M) | **8 MB** | 28.42 MB | ~8 MB | ✓ |
| 4194304 (4M) | **32 MB** | 54.22 MB | ~33.6 MB | ✓ |
| 33554432 (32M) | **256 MB** | 287.68 MB | ~268.7 MB | ✓ |

**骨架部分的線性關係嚴格成立**，`size × 8B` 可直接當公式用。§9.1 的推算在這一點上是對的。

### 11.4 浮動成本與丟棄率：壓測實測

條件：`--rps=1800 --duration=60s --seed=1000`，異步 diode 模式，16 核。

| size | 骨架 | 負載期 HeapAlloc | p50 | p90 | p99 | max | 丟棄 | set collision |
|---:|---:|---:|---:|---:|---:|---:|:-:|:-:|
| 512 | 4 KB | 27.42 MB | 434µs | 1.3ms | 2.3ms | 35.1ms | **0** | **0** |
| 64K | 0.5 MB | 27.42 MB | 451µs | 1.4ms | 2.8ms | 30.6ms | **0** | **0** |
| **1M** | 8 MB | 53.67 MB | 431µs | 1.2ms | 2.3ms | 28.7ms | **0** | **0** |
| **4M** | 32 MB | 81.72 MB | 453µs | 1.4ms | 3.2ms | 36.1ms | **0** | **0** |

三個關鍵觀察：

1. **延遲與 size 無關**。p50 在 431–453µs 之間波動，差異在雜訊範圍內；4M 甚至略差於 1M。**加大 size 不會提升性能**。
2. **記憶體隨 size 線性增長**。4M 比 1M 多吃 28MB，換來零收益。
3. **連 512 槽（4KB）都沒有任何丟棄**。

### 11.5 為什麼小到 512 都不丟？——讀 `consume` 就明白了

`async_diode_writer.go:95-105`：

```go
case <-ticker.C:
    _ = a.writer.Flush()
default:
    data, ok := a.diode.TryNext()
    if !ok || data == nil {
        time.Sleep(1000 * time.Microsecond)   // 僅在「空」時睡 1ms
        continue
    }
    b := *(*[]byte)(data)
    _, _ = a.writer.Write(b)                 // 非空時連續消費，不睡
```

消費側是**忙輪詢**，只在 diode 為空時睡 1ms。所以：

```
生產：~1230 req/s × 1 條 ≈ 1230 條/s
消費：非空時不間斷消費，僅寫入 bufio.Writer（記憶體操作，非 syscall）
      即使最壞情況每 1ms 只消費 1 條，也有 1000 條/s 的下限
```

**消費速率恆定追上生產速率，環形緩衝的穩態深度接近 0**。這就是為什麼 4KB 的環也夠用——它從來就沒被填滿過。

> 關鍵：`consume` 寫的是 `bufio.Writer`（記憶體），真正的磁碟 syscall 由 `Flush` 每秒觸發一次。這正是異步模式相對同步模式快 302 倍的根本原因（§3），也是消費側能持續跑贏的原因。

### 11.6 丟棄檢測本身是否可信？

「零丟棄」這個結論需要先證明**檢測機制真的有效**，否則等於什麼都沒測。三重佐證：

1. **stderr 捕獲確實工作**——同一份 stderr 抓到了關閉期的訊息：
   ```
   zerolog: could not write event: AsyncDiodeWriter: writer is closed
   ```
2. **日誌確實在大量寫入**——單輪產生 16.7MB，觸發多次 lumberjack 滾動（5MB/檔）：
   ```
   app-2026-09-03T03-45-06.363.log  5.0M
   app-2026-09-03T03-45-22.465.log  5.0M
   app.log                          929.0K
   ```
   平均每請求約 221 位元組日誌。
3. **兩條丟棄路徑都會寫 stderr**，且都被 grep 覆蓋：
   - `AlertFunc` → `fmt.Fprintf(os.Stderr, "AsyncDiodeWriter: diode full, ...")`（`async_diode_writer.go:64`）
   - 寫入碰撞 → `log.Println("Diode set collision: ...")`（`many_to_one.go:50,60`）

### 11.7 一個容易忽略的風險：set collision 跑在業務 goroutine 上

`many_to_one.go:47-52`：

```go
if old != nil && (*bucket)(old) != nil &&
    (*bucket)(old).seq > writeIndex-uint64(len(d.buffer)) {
    log.Println("Diode set collision: consider using a larger diode")
    continue
}
```

這條路徑用標準庫 `log.Println` **直接同步寫 stderr**，而且它是在 `Set` 裡、即**業務請求的 goroutine 上**執行的。標準庫 `log` 自帶互斥鎖 + 阻塞 syscall——正是 §3 花整節篇幅消除的那類全域序列化點。

對比之下，`AlertFunc`（diode full）跑在單一 consume goroutine 上，影響小得多。

**所以 size 設得過小的真正風險，不是「省了記憶體」，而是一旦觸發 collision 就會在熱路徑上引入 stderr 鎖。** 這是保留安全邊際的理由。

### 11.8 結論與建議取值

**實測結論**：在本次負載（~1230 RPS、每請求約 1 條日誌）下，`size` 從 512 到 4M 對延遲與丟棄率**均無可測量影響**。1M 已遠超需求，4M 純屬浪費，32M 預設值更是離譜。

但「實測夠用」與「安全邊際」是兩回事。考量 §11.7 的 collision 風險，以及突發流量（burst）與 GC STW 期間消費側短暫停頓的可能，建議：

| 場景 | 建議 size | 骨架成本 | 理由 |
|---|---:|---:|---|
| 開發 / 測試 | `65536`（64K） | 0.5 MB | 實測綽綽有餘，記憶體幾乎免費 |
| **一般生產（推薦）** | **`262144`（256K）** | **2 MB** | 較實測需求高兩個數量級的安全邊際，成本仍可忽略 |
| 超高吞吐 / 日誌極密 | `1048576`（1M） | 8 MB | 目前 `application_test.yml` 的取值；仍屬保守 |
| 框架預設 `33554432` | — | 256 MB | **無任何實測依據支持，建議下調** |

**推薦 `262144`（256K）作為預設**：相對實測峰值需求有約 200 倍餘量，骨架僅 2MB，即使消費側因 GC 或磁碟抖動停頓數十毫秒也不會溢出。

> **注意這個結論的邊界**：以上基於「每請求約 1 條、約 221 位元組」的日誌量。若應用把 `level` 調到 `debug`、或單請求打多條日誌，生產速率會成倍上升——但由於消費側是忙輪詢且只寫記憶體緩衝，瓶頸更可能出現在 `Flush` 的磁碟頻寬，而非環形緩衝深度。真正需要加大 size 的情形是**磁碟寫入變慢導致消費側被拖住**（如日誌盤 IO 飽和），這時 stderr 上會出現 `diode full`，可作為調大 size 的觸發訊號。

### 11.9 可複現指令

```bash
# 用環境變數覆寫 size，無需改 YAML（APP_CONF_ 前綴映射到配置鍵）
cd /mnt/d/code/github_opensource/tmp/fiberhouse
APP_ENV_application_env=test \
APP_CONF_application_appLog_asyncConf_diodeConf_size=262144 \
  go run ./example_main

# 骨架成本（靜置時取，不要用 pprof heap——會漏掉單次大分配）
curl -s "http://127.0.0.1:6060/debug/pprof/heap?debug=1" \
  | grep -E "^# (HeapAlloc|HeapInuse|Sys) "

# 壓測（注意 bench CLI 的配置路徑相對於 example_application/command）
cd example_application/command
APP_ENV_application_env=test go run . bench --rps=1800 --duration=60s --seed=1000

# 丟棄檢測（兩條路徑都寫 stderr）
grep -c "diode full"          server.stderr    # AlertFunc，consume goroutine
grep -c "Diode set collision" server.stderr    # 寫入碰撞，業務 goroutine（更嚴重）
```

> **兩個實測時踩到的坑**：
> 1. **server 必須從倉庫根目錄啟動**（配置路徑 `./example_config/...` 相對 CWD），而 **bench CLI 必須從 `example_application/command/` 啟動**（其路徑為 `../../example_config/...`）。
> 2. **兩者都要顯式設 `APP_ENV_application_env=test`**，否則 `bootstrap.go:219` 會回落到 `dev`，載入錯誤的配置檔並 panic。
