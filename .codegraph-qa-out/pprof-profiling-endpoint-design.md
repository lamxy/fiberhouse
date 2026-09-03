# pprof 性能剖析端點設計方案

> 目標：為 fiberhouse 增加**可通過 YAML 配置開關**的 pprof 性能監控服務，支持 cpu / heap / goroutine / block / mutex 五類 profile。
>
> 分析日期：2026-09-02。基於 codegraph + rg 對主線代碼（排除 `.worktrees/`、`.superpowers/`）的實測。

---

## 一、現狀盤點（實測結論）

### 1.1 框架層完全沒有 pprof

`rg` 全庫掃描（主線代碼）結果：**框架本體零 pprof 引用**。唯一的 pprof 出現在示例應用：

`example_application/providers/middleware/fiber_module_middleware.go:15`

```go
// Pprof中间件 /monitor/debug/pprof/
app.Use(pprof.New(pprof.Config{
    Next: func(c *fiber.Ctx) bool {
        // todo 定义条件
        return false          // ← 恆為 false，即「永不跳過」
    },
    Prefix: "/uuid",
}))
```

存在四個實際問題：

| # | 問題 | 說明 |
|---|---|---|
| 1 | **無條件常開** | `Next` 恆返回 `false`，pprof 在所有環境（含 prod）暴露，無任何配置開關 |
| 2 | **僅 Fiber** | Gin / Hertz 核心完全沒有等價實現，能力不對等 |
| 3 | **註釋與代碼不符** | 註釋寫 `/monitor/debug/pprof/`，實際 `Prefix` 是 `/uuid` |
| 4 | **與業務端口同進程同監聽** | pprof 路由掛在業務 app 上，無法做網絡層隔離 |

### 1.2 block / mutex profile 未被啟用

Go 的 `block` 與 `mutex` profile **必須顯式開啟採樣率**，否則 `/debug/pprof/block`、`/debug/pprof/mutex` 返回空數據：

```go
runtime.SetBlockProfileRate(rate)     // 默認 0 = 關閉
runtime.SetMutexProfileFraction(rate) // 默認 0 = 關閉
```

全庫掃描：**`SetBlockProfileRate` / `SetMutexProfileFraction` 零引用**。這意味著即便掛上 pprof 端點，用戶請求 block/mutex 也只會拿到空 profile。這是本次設計必須覆蓋的關鍵點，也是現有實現最容易被忽略的缺陷。

### 1.3 現有配置慣例

`example_config/application_dev.yml` 的既有開關風格：

```yaml
application:
  swagger:
    type: 1
    enable: true          # 布爾開關 + 子配置，本設計沿用此形態
  middleware:
    monitor: true         # GetMiddlewareSwitch("monitor") 讀取
  task:
    enableServer: true
```

讀取方式為 koanf 路徑字符串：`cfg.Bool("application.swagger.enable")`、`cfg.String(prefix+"host", "0.0.0.0")`、`cfg.Int(...)`、`cfg.Duration(...)`。

### 1.4 Provider / Location / Type 三元體系的可用擴展點

- **類型**：`ProviderTypeDefault()` 提供 `GroupProviderAutoRun`（自動運行一次）、`GroupDefaultManagerType`、`GroupExtendReplace` 等。`ProviderTypeGen().MustCustom(name)` 可註冊自定義類型（ID 64–255）。
- **位點**：`ProviderLocationDefault()` 提供 `LocationServerRunBefore`、`LocationServerShutdownAfter` 等；`ProviderLocationGen().MustCustom(name)` 可註冊自定義位點。
- **統一分派**：`LoadProviderManagersAtLocation(managers, location, dependency)`（`starter_manager_loader.go:21`）是框架導出的位點分派函數，Fiber / Gin / Hertz 三核心均基於它實現，支持 `GroupExtendReplace` 替代語義。

### 1.5 ⚠️ 關鍵約束：ServerRunBefore 位點的執行邏輯有缺陷

`boot.go:365-375`：

```go
runBeforeManagers := ProviderLocationDefault().LocationServerRunBefore.GetManagers()
if len(runBeforeManagers) > 0 {
    for _, m := range runBeforeManagers {
        if m.IsUnique() {          // ← 只執行 IsUnique() 的管理器
            _, _ = m.LoadProvider(...)
            break                   // ← 且只執行第一個就 break
        }
    }
}
```

**只有 `IsUnique()` 為 true 的管理器會被執行，且執行第一個後即 `break`。** 非唯一管理器在此位點會被靜默跳過。

同樣的模式也出現在 `LocationBootStrapConfig`（`boot.go:299-309`），且那裡的 `break` 位置在 `if` 之外，是更明顯的邏輯問題（無論是否 unique 都在第一輪迭代後跳出）。

**這直接決定了方案選型**：任何依賴 `ServerRunBefore` 的設計，其管理器必須是 unique 的，否則會與應用自身的其他 ServerRunBefore 擴展互斥（先註冊者勝）。

此外 `LocationServerRunAfter` 與 `LocationServerShutdown` 在框架默認實現中**未被使用**（見 `provider_location.go:175,177` 註釋）。可用的關閉位點是 `LocationServerShutdownBefore` 與 `LocationServerShutdownAfter`，二者經 `resolveRunAndShutdownManagers()` 合併後傳入 `coordinateServerRun`，由核心的 `Shutdown()` 通過 `LoadProviderManagersAtLocation` 正常分派（無 IsUnique 限制）。

---

## 二、設計目標

1. **配置驅動**：YAML 開關，默認關閉（安全默認）。
2. **引擎無關**：Fiber / Gin / Hertz 一致行為，不重複三份實現。
3. **覆蓋五類 profile**：cpu、heap、goroutine、block、mutex —— 且 block/mutex 採樣率可配。
4. **生產安全**：獨立端口 + 綁定回環 + 可選鑑權，避免公網暴露。
5. **符合框架慣例**：Provider 模式接入，遵循既有位點與類型語義。
6. **優雅關閉**：隨主應用生命週期一同關閉。

---

## 三、方案候選

### 候選 A：獨立監聽端口 + AutoRun Provider（**推薦**）

**核心思路**：pprof 不掛在業務引擎上，而是**另起一個 `net/http` Server 監聽獨立端口**，由一個 Provider 在 `ServerRunBefore` 位點啟動、在 `ServerShutdownAfter` 位點關閉。

```
業務引擎 (Fiber/Gin/Hertz)  :8080  ← 業務流量
pprof 專用 http.Server      :6060  ← 僅回環，診斷流量
```

**實現骨架**（框架層，新增 `pprof_provider.go` 等）：

```go
// 配置結構
type ConfPprofBase struct {
    Enable        bool          // 總開關，默認 false
    Host          string        // 默認 "127.0.0.1"（安全默認，不對外）
    Port          int           // 默認 6060
    Prefix        string        // 默認 "/debug/pprof"
    BlockRate     int           // SetBlockProfileRate，默認 0（關閉）
    MutexFraction int           // SetMutexProfileFraction，默認 0（關閉）
    ReadTimeout   time.Duration
    WriteTimeout  time.Duration // 需 > CPU profile 的 seconds 參數
    Username      string        // 可選 BasicAuth
    Password      string
}

type PprofProvider struct {
    fiberhouse.IProvider
    server *http.Server
    mu     sync.Mutex
}

func (p *PprofProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
    appCtx := ctx.(fiberhouse.IApplicationContext)
    cfg := appCtx.GetConfig()

    if !cfg.Bool("application.pprof.enable") {
        return nil, nil   // 未啟用：零開銷，不註冊任何東西
    }

    // 關鍵：block / mutex 必須顯式開採樣率，否則 profile 為空
    if rate := cfg.Int("application.pprof.blockRate", 0); rate > 0 {
        runtime.SetBlockProfileRate(rate)
    }
    if f := cfg.Int("application.pprof.mutexFraction", 0); f > 0 {
        runtime.SetMutexProfileFraction(f)
    }

    mux := http.NewServeMux()
    prefix := cfg.String("application.pprof.prefix", "/debug/pprof")
    mux.HandleFunc(prefix+"/", pprof.Index)          // 含 heap/goroutine/block/mutex/allocs/threadcreate
    mux.HandleFunc(prefix+"/cmdline", pprof.Cmdline)
    mux.HandleFunc(prefix+"/profile", pprof.Profile) // CPU
    mux.HandleFunc(prefix+"/symbol", pprof.Symbol)
    mux.HandleFunc(prefix+"/trace", pprof.Trace)

    handler := withOptionalBasicAuth(mux, cfg)       // 可選鑑權

    p.server = &http.Server{
        Addr:         net.JoinHostPort(host, strconv.Itoa(port)),
        Handler:      handler,
        ReadTimeout:  ...,
        WriteTimeout: ...,   // 必須 > CPU profile seconds，否則採樣被截斷
    }
    go func() {
        if err := p.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            // 用框架 logger 記錄，不 panic —— 診斷服務不應拖垮業務
        }
    }()
    return nil, nil
}
```

配套一個 `PprofShutdownProvider` 綁定到 `LocationServerShutdownAfter`，調用 `p.server.Shutdown(ctx)`。

**YAML**：

```yaml
application:
  pprof:
    enable: false            # 總開關，默認關閉
    host: "127.0.0.1"        # 安全默認：僅回環
    port: 6060
    prefix: "/debug/pprof"
    blockRate: 0             # >0 才採 block profile，如 10000 (納秒)
    mutexFraction: 0         # >0 才採 mutex profile，如 100
    readTimeout: 10
    writeTimeout: 120        # 需大於 CPU profile 的 seconds
    auth:
      enable: false
      username: ""
      password: ""
```

**優點**
- 引擎無關：一份實現覆蓋 Fiber / Gin / Hertz，無需三套適配。
- 生產安全性最好：獨立端口 + 回環綁定，業務端口不暴露診斷面；即使業務引擎的中間件鏈（鑑權、限流、CORS）有誤配也影響不到、也不被影響。
- 診斷可用性高：業務引擎 goroutine 飽和或中間件死鎖時，獨立 Server 仍可響應，這正是最需要 profile 的時刻。
- 不進業務中間件鏈 → 對正常請求路徑**零性能開銷**。
- 完整覆蓋五類 profile，block/mutex 採樣率一併配置化。

**缺點**
- 多佔一個端口，容器編排需相應暴露（但這通常正是期望行為 —— 診斷端口只在內網開放）。
- 需自行管理該 Server 的生命週期（已由 shutdown provider 覆蓋）。

---

### 候選 B：掛載到業務引擎路由（每引擎適配）

沿用現有 `fiber_module_middleware.go` 思路，但改為配置驅動，並補齊 Gin / Hertz。

```go
if !cfg.Bool("application.pprof.enable") { return }
// Fiber: app.Use(pprof.New(pprof.Config{Prefix: prefix}))
// Gin:   pprof.Register(engine, prefix)  // github.com/gin-contrib/pprof
// Hertz: 需自行 wrap net/http/pprof handler
```

**優點**
- 不佔用額外端口，部署形態不變。
- 復用業務引擎既有中間件（鑑權、日誌、requestId）。

**缺點**
- **需要三份引擎適配**，與框架「引擎可插拔」的成本模型衝突，新增引擎就要再寫一份。
- Fiber 需引入 `gofiber/fiber/v2/middleware/pprof`，Gin 需引入 `gin-contrib/pprof`，Hertz 無現成方案需手寫 adapter —— 依賴面擴大。
- **診斷可用性差**：引擎 goroutine 池耗盡或中間件死鎖時，pprof 端點同樣不可達 —— 恰恰在最需要它的場景失效。
- 診斷面暴露在業務端口，公網服務誤配風險高。
- pprof 路由會進入業務中間件鏈，可能被 CSRF / 限流 / 鑑權中間件干擾（現有代碼中 pprof 恰好在 csrf 之前註冊，順序敏感）。

---

### 候選 C：A + B 混合（模式可選）

配置增加 `mode: standalone | attached`，兩種實現都保留，由配置選擇。

**優點**：靈活，覆蓋「不能多開端口」的受限環境。

**缺點**：實現與測試成本近似 A+B 之和，兩條路徑都要維護；對一個診斷輔助功能而言收益不成比例。**除非有明確的受限部署需求，否則不建議首發即做。**

---

## 四、方案對比

| 維度 | A 獨立端口 | B 掛業務引擎 | C 混合 |
|---|---|---|---|
| 引擎適配成本 | **1 份** | 3 份（且隨引擎增長） | 4 份 |
| 新增依賴 | 無（純標準庫 `net/http/pprof`） | Fiber/Gin pprof 中間件 + Hertz 手寫 | 同 B |
| 業務路徑開銷 | **零** | 進中間件鏈 | 取決於模式 |
| 引擎飽和時可診斷 | **是** | 否 | 取決於模式 |
| 生產暴露風險 | **低**（回環+獨立端口） | 中高（業務端口） | 中 |
| 額外端口 | 需要 | 不需要 | 可選 |
| 實現複雜度 | 低 | 中 | 高 |

---

## 五、推薦結論

**首選候選 A（獨立端口 + AutoRun Provider）**，理由按權重排序：

1. **引擎無關是決定性因素**。框架的核心賣點是 Fiber/Gin/Hertz 可插拔；候選 B 讓一個診斷功能的成本隨引擎數線性增長，違背框架設計取向。
2. **診斷可用性**。pprof 存在的意義就是排查 goroutine 洩漏、鎖競爭、CPU 打滿 —— 這些場景下業務引擎往往已經不能正常響應。掛在業務引擎上的 pprof 在最需要時失效。
3. **安全默認**。`enable: false` + `host: 127.0.0.1`，生產默認不暴露；需要時通過 SSH 隧道或內網訪問。
4. **零依賴**。只用標準庫 `net/http/pprof`。

**若後續出現「容器只允許單端口」的部署訴求，再增量演進到候選 C**，此時候選 A 的代碼可原樣保留為 `standalone` 模式。

---

## 六、實施要點（落地時必須注意）

1. **block / mutex 採樣率必須顯式設置**，否則端點返回空 profile。這是最容易遺漏的一點（現有代碼零引用）。建議：
   - `blockRate`：納秒閾值，典型值 `10000`（即 10µs）。設為 `1` 會記錄所有阻塞事件，開銷極大，僅短期診斷用。
   - `mutexFraction`：採樣分母，典型值 `100`（1/100 的爭用事件）。
   - 二者均有**運行時性能開銷**，默認 `0`（關閉），YAML 註釋需明確標註風險。

2. **`WriteTimeout` 必須大於 CPU profile 的 `seconds` 參數**（默認 30s）。否則 `go tool pprof http://host:6060/debug/pprof/profile?seconds=60` 會在寫超時處被截斷。建議默認 `120`，或設為 `0`（不限制）並在註釋說明。

3. **Provider 類型與位點選擇**：
   - 類型用 `ProviderTypeDefault().GroupProviderAutoRun`（語義即「自動運行一次」），或 `ProviderTypeGen().MustCustom("PprofService")`。
   - 啟動位點 `LocationServerRunBefore`：⚠️ **注意 §1.5 的約束** —— 該位點只執行 `IsUnique()` 管理器且執行首個後 `break`。若 pprof 管理器設為 unique，會與應用自有的 ServerRunBefore unique 管理器互斥。
   - **更穩妥的替代**：用 `LocationGlobalInit` 或 `LocationCoreEngineInit`（經 `LoadProviderManagersAtLocation` 正常分派，無 IsUnique 限制），或**在框架層 `RunServer` 內直接以獨立分支啟動**（不走 provider 位點），因為 pprof 是框架內建能力而非用戶擴展。
   - 關閉位點用 `LocationServerShutdownAfter`（正常分派，無限制）。

4. **順帶修復現有問題**（建議同期處理，範圍可控）：
   - `example_application/providers/middleware/fiber_module_middleware.go` 中常開的 pprof 應移除或改為讀配置 —— 目前它在示例應用中無條件暴露 `/uuid/debug/pprof/`。
   - 該文件註釋 `/monitor/debug/pprof/` 與實際 `Prefix: "/uuid"` 不符，需訂正。

5. **鑑權**：即便綁定回環，仍建議提供可選 BasicAuth，用於需要綁 `0.0.0.0` 的內網場景。

6. **測試建議**：
   - `enable: false` 時端口未監聽（否定路徑最易回歸）。
   - `enable: true` 時五類 profile 端點均返回非空（block/mutex 需先設採樣率）。
   - 優雅關閉後端口釋放。

---

## 七、實施結果（已完成）

決策：採用**候選 A**，pprof 置於**框架層**，並移除 `ServerRunBefore` 位點的 `IsUnique()` 限制。

### 7.1 新增與修改的文件

| 文件 | 說明 |
|---|---|
| `pprof_config.go` | `PprofConfig` 結構、默認值、`LoadPprofConfig()` 配置讀取、前綴規範化 |
| `pprof_server.go` | `PprofServer`：獨立 `http.Server` 生命週期、採樣率設置、BasicAuth |
| `pprof_providers_and_manager.go` | 啟動/關閉 Provider 與 Manager，接入位點體系 |
| `boot.go` | `ServerRunBefore` 位點改用 `LoadProviderManagersAtLocation` 統一分派 |
| `default.go` | 兩個 Provider 與兩個 Manager 註冊進默認集合 |
| `example_config/application_{dev,test,prod}.yml` | 新增 `application.pprof` 配置塊 |
| `example_application/providers/middleware/fiber_module_middleware.go` | 移除常開的硬編碼 pprof |
| `pprof_server_test.go`、`pprof_config_test.go`、`pprof_lifecycle_test.go` | 19 個測試用例 |

### 7.2 關鍵實現決策

**1. 啟動與關閉必須使用獨立的 Provider 類型**

框架的 `resolveManagerWithProviders`（`boot.go:577-596`）按「類型 ID 相等」把 Provider 分派給 Manager，且**首個匹配即 `break`**。若啟動與關閉 Provider 共用 `GroupProviderAutoRun`，二者會被歸入同一個 Manager，導致關閉 Provider 在啟動位點被執行。

因此通過 `ProviderTypeGen().MustCustom()` 註冊了兩個獨立自定義類型 `PprofStartType` / `PprofShutdownType`，並由 `TestPprofProviderRoutedToCorrectManager` 覆蓋此分派行為。

**2. `ServerRunBefore` 位點的修改**

原邏輯只執行 `IsUnique()` 管理器且首個後 `break`。已改為：

```go
if _, _, err := LoadProviderManagersAtLocation(
    ProviderLocationDefault().LocationServerRunBefore.GetManagers(),
    ProviderLocationDefault().LocationServerRunBefore,
    appStarter,
); err != nil { /* 記錄日誌 */ }
```

現在該位點：支持註冊多個管理器並全部執行；獲得與其他位點一致的 `GroupExtendReplace` 替代語義。由 `TestServerRunBeforeExecutesAllManagers` 覆蓋。

**3. `Serve` goroutine 的空指針修復**

初版在 goroutine 內讀取 `p.server` 字段，而 `Shutdown` 會將其置 nil，快速啟停時觸發 SIGSEGV（實測復現）。已改為在啟動前捕獲局部變量：

```go
server := &http.Server{...}
p.server = server
go func() { server.Serve(listener) }()   // 捕獲局部變量，不讀字段
```

**4. 監聽同步化**

`net.Listen` 在 `Start()` 中同步執行，端口衝突等錯誤可同步返回，不會被 goroutine 靜默吞掉。

### 7.3 驗證結果

- `go build ./...`、`go vet ./...`：通過
- pprof 相關 19 個測試：全部通過（含 `-race`）
- 全倉庫測試套件（39 個包）：全部通過（含 `-race`），無回歸

覆蓋的行為：關閉時不監聽端口、五類 profile 均返回非空、CPU profile 不被寫超時截斷、BasicAuth 三種路徑、關閉後端口釋放、重複啟動冪等、自定義前綴、根路徑重定向、配置默認值與讀取、Provider 分派正確性、位點綁定、端到端啟停全流程。

### 7.4 使用方式

```yaml
application:
  pprof:
    enable: true
    host: "127.0.0.1"
    port: 6060
    blockRate: 10000        # 開啟 block profile
    mutexFraction: 100      # 開啟 mutex profile
```

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30  # CPU
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
go tool pprof http://127.0.0.1:6060/debug/pprof/goroutine
go tool pprof http://127.0.0.1:6060/debug/pprof/block
go tool pprof http://127.0.0.1:6060/debug/pprof/mutex
```

### 7.5 遺留事項

- ~~`.worktrees/cache-stabilization/` 分支存在同構代碼，若該分支仍活躍，改動需同步。~~
  已核實該工作樹為過期殘留（`caa38d2` 已合併進 main，無獨有提交與未追蹤檔案），工作樹已刪除，
  分支 `codex/cache-stabilization` 暫予保留。本次 pprof 改動無需同步。
- `boot.go:299-309` 的 `LocationBootStrapConfig` 位點存在同類問題，且 `break` 寫在 `if` 之外（無論是否 unique 都在第一輪迭代後跳出）。本次未改動該位點，因其超出當前任務範圍；如需修正建議單獨評估。
