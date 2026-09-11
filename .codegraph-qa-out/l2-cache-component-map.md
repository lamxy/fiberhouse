# L2 快取四件組的代碼級定位

本文件是 PRD 任務 9 的 I0 交付物：釐清 `Level2Cache`、`Coordinator`（singleflight）、
Bloom filter、circuit breaker 四者各自位於何處、彼此是否耦合。

**結論先行：四者互相獨立，不存在包含或依賴關係。** 它們常被一併討論，是因為同屬
「快取保護機制」這個話題，但在代碼上分屬不同套件、由不同入口裝配，可以單獨使用。

## 一、四者的位置與所屬

| 組件 | 套件 | 檔案 | 核心型別 |
|---|---|---|---|
| L2 二級快取 | `component/cache/cache2` | `level2_cache.go` | `Level2Cache` |
| 單飛協調器 | `component/cache` | `cache_utils.go:77` | `Coordinator` |
| 記憶體 Bloom | `component/cache` | `filter.go:26` | `MemoryBloomFilter` |
| 分片 Bloom | `component/cache` | `helper.go:143` | `ShardedBloomFilter` |
| Stable Bloom | `component/cache` | `helper.go:246` | `StableBloomFilter` |
| Redis Bloom | `component/cache/cacheremote` | `redis_bloom.go:33` | `RedisBloomFilter` |
| 熔斷器 | `component/cache` | `helper.go:24` | `CircuitBreakerWrap`（包裝 gobreaker） |

## 二、耦合關係

```
Level2Cache ──持有──> local  cache.Cache   （cachelocal.LocalCache）
            ──持有──> remote cache.Cache   （cacheremote.RedisDb）
            ──持有──> localPool / remotePool (*ants.Pool)

Coordinator ──持有──> cache  Cache          （任一後端，非必為 L2）
            ──持有──> flights singleflight.Group
            ──持有──> filter ExistenceFilter （可選，介面）

CircuitBreakerWrap ── 獨立，不被上述任一者持有
```

要點：

- **`Level2Cache` 不知道 `Coordinator` 的存在**，反之 `Coordinator` 接受任意
  `cache.Cache` 後端，L2 只是其中一種可能。兩者是平行的組合單元。
- **Bloom filter 經 `ExistenceFilter` 介面接入 `Coordinator`**（`cache_utils.go` 的
  `filter` 欄位），不接入 L2。四種 Bloom 實作都滿足該介面，可互換。
- **`CircuitBreakerWrap` 不被任何一方持有**，由應用自行在呼叫點包裹。

## 三、L2 的非同步寫入與排空機制（I1 的事實基礎）

`Level2Cache` 用兩個 ants pool 承載非同步寫入（`localPool`、`remotePool`），
並用一組計數器 + channel 追蹤在途任務：

```go
// level2_cache.go:491
func (l2c *Level2Cache) admit() bool {
    if !l2c.admitting { return false }      // 關閉後拒絕新任務
    if l2c.pending == 0 {
        l2c.drain = make(chan struct{})     // 由 0 轉 1：開一個新的 drain
    }
    l2c.pending++
    return true
}

// level2_cache.go:504
func (l2c *Level2Cache) complete() {
    l2c.pending--
    if l2c.pending == 0 {
        close(l2c.drain)                    // 歸零：排空完成
    }
}
```

`admit()` 在**提交前**呼叫，`complete()` 以 `defer` 寫在每個 ants 任務體內，
且提交失敗時也會補呼叫一次（`level2_cache.go:343,359,376`）。因此 `pending`
覆蓋「已受理但尚未完成」的全部任務——包含仍在佇列中、尚未被 worker 取走的那些。

## 四、I1 裁決：`Wait()` 已經等待 ants 在途任務，無需修改

PRD 的 I1 條目主張 `Wait()` 不等待 ants pool 在途任務，並建議改用
`localPool.Running()`。**實測與代碼複核表明該主張不成立。**

`Wait()`（`level2_cache.go:572`）的第一件事就是 `Flush(context.Background())`，
而 `Flush`（`:513`）阻塞在 `drain` channel 上：

```go
func (l2c *Level2Cache) Flush(ctx context.Context) error {
    drain := l2c.drain
    select {
    case <-drain:   return nil      // 只有 pending 歸零時才關閉
    case <-ctx.Done(): return ctx.Err()
    }
}
```

由第三節可知 `drain` 只在 `pending` 歸零時關閉，因此 `Wait()` 必然等到全部
已受理的非同步任務執行完畢，才會去等子快取。

**既有測試已釘住此行為**：

- `TestLevel2Wait_DrainsQueuedAsyncWorkBeforeWaitingChildren`
  （`level2_cache_test.go:187`）——構造「一個執行中 + 一個排隊中」的任務，
  斷言 `Wait()` 在兩者完成前不返回。
- `TestLevel2Cache_FlushWaitsForQueuedAndRunningWork`（`:422`）——對 `Flush`
  作同樣斷言。

**為何建議的 `Running()` 方案反而更弱**：`ants.Pool.Running()` 只回報**正在執行**
的 worker 數，不含仍在佇列中等待的任務。改用它會漏掉排隊任務——正是上述測試
特意構造的那一種。現行的 `pending` 計數器在 `Submit` 之前就遞增，覆蓋面嚴格更大。

**結論：I1 無需改動。** 本輪僅補一個測試把這條結論轉為可執行斷言
（`level2_wait_contract_test.go`），避免將來有人依 PRD 字面「修復」成更弱的版本。

## 五、關閉順序

`Shutdown` → `beginShutdown`（停止受理）→ `finishShutdown`：

```go
// level2_cache.go:552
func (l2c *Level2Cache) finishShutdown(drain chan struct{}) {
    <-drain                      // 先排空在途任務
    l2c.localPool.Release()      // 再釋放 pool
    l2c.remotePool.Release()
    if l2c.ownChildren { ... }   // 最後才關子快取
}
```

順序是必需的：先關子快取會讓排空期間的任務打在已關閉的 Redis 上。
`Level2Cache` 實作 `globalmanager.CompositeClosable` 向容器聲明這層所有權，
確保 `ReleaseAll` 先釋放 L2、再釋放其子快取各自的 key。
