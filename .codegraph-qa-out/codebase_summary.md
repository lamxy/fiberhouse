# FiberHouse 代碼庫總結

> **Agent 導讀**：本文件是對 `lamxy/fiberhouse` 代碼庫的人工審閱總結（2026-06-26），優先閲讀本文件以快速建立上下文，再按需查閲 `GRAPH_REPORT.md` 或 wiki 社區文件做深入導航。
>
> ⚠️ 注意：根目錄 `README.md` 及各目錄下 `docs/` 相關內容仍在撰寫和優化中，存在滯後和不準確之處，**暫不建議直接作為參考依據**。

---

## 定位

Go Web 應用框架，核心 HTTP 引擎可切換（Fiber / Gin），通過 Provider 模式實現模組化擴展。模塊 path：`github.com/lamxy/fiberhouse`。

---

## 核心架構

### 三層啟動器組合

| 層 | 接口 | 職責 |
|---|---|---|
| 框架層 | `FrameStarter` | 全局對象（DB/Cache/Redis）、任務服務、配置驗證器 |
| 核心層 | `CoreStarter` | HTTP 引擎（Fiber/Gin）、中間件、路由、生命周期鉤子 |
| 組合層 | `ApplicationStarter` | 上兩者的組合接口，由 `WebApplication` 實現 |

### Provider / Manager / Location 三元體系

- **`IProvider`**：功能封裝單元（中間件、路由、JSON 編解碼器、Swagger 等）
- **`IProviderManager`**：同類型 Provider 的容器，負責加載和執行
- **`IProviderLocation`**：生命周期執行位點，管理器綁定到位點後按序執行

#### 默認位點執行順序

```
BootStrapConfig → FrameStarterCreate → CoreStarterCreate
→ GlobalInit → CoreEngineInit → CoreHookInit
→ AppMiddlewareInit → ModuleMiddlewareInit → RouteRegisterInit
→ ModuleSwaggerInit → TaskServerInit → GlobalKeepaliveInit
→ ServerRunBefore → ServerRun → ServerRunAfter → ServerShutdown
```

### 注冊器接口（用戶實現）

| 接口 | 職責 |
|---|---|
| `ApplicationRegister` | 全局依賴初始化、自定義驗證器 |
| `ModuleRegister` | 模塊路由 + Swagger 注冊 |
| `TaskRegister` | 基於 asynq 的異步任務處理器 |

---

## 入口方式

```go
fh := fiberhouse.New(&fiberhouse.BootConfig{
    CoreType:     constant.CoreTypeWithFiber,      // "fiber" | "gin"
    TrafficCodec: constant.TrafficCodecWithSonic,  // "sonic_json_codec" | "std_json_codec" | ...
    ConfigPath:   "./config",
    LogPath:      "./logs",
})
fh.WithProviders(providers...).WithPManagers(managers...).RunServer()
```

`Default(opts...)` 可替代 `New()`，使用默認配置（Fiber + Sonic + `./config`）。

---

## 主要依賴

| 模塊 | 庫 |
|---|---|
| HTTP | Fiber v2 / Gin v1.11 |
| 配置 | koanf v2（支持 YAML/Env/File） |
| 日誌 | zerolog + lumberjack（輪轉） |
| DI | uber/dig + google/wire |
| DB | GORM + MySQL + MongoDB v2 |
| 本地緩存 | dgraph-io/ristretto v2 |
| 遠端緩存 | go-redis/v9 |
| 異步任務 | asynq（Redis 驅動）+ robfig/cron |
| 熔斷 | sony/gobreaker v2 |
| Bloom Filter | bits-and-blooms/bloom v3 |
| JSON | sonic（bytedance）/ std / goccy/go-json |
| 序列化 | google/protobuf + vmihailenco/msgpack |
| 協程池 | panjf2000/ants v2 |
| 無鎖隊列 | code.cloudfoundry.org/go-diodes |
| CLI | urfave/cli v2 |
| Swagger | swaggo/swag + gofiber/swagger + gin-swagger |
| 驗證 | go-playground/validator v10 |

---

## 關鍵設計特點

1. **位點驅動啟動流程**：每個啟動階段對應一個 `IProviderLocation`，解耦各階段邏輯，可在任意位點插入自定義行為。
2. **核心引擎可插拔**：`BootConfig.CoreType` 切換 fiber/gin，Provider 通過 `Target()` 字段聲明適配的引擎類型。
3. **全局對象統一管理**：`GlobalManager` 作為單例容器，通過 `InstanceKey` 訪問 DB/Cache 等實例，支持健康保活機制（`RegisterGlobalsKeepalive`）。
4. **二級緩存支持**：本地（ristretto）+ 遠端（Redis），通過 `GetLocalCacheKey` / `GetRemoteCacheKey` / `GetLevel2CacheKey` 訪問。
5. **CLI 支持**：除 Web 應用外，有獨立的 `ICommandContext` + `CommandStarter` 體系（`commandstarter/` 目錄）。
6. **RPC 擴展**：`rpc/` 目錄 + protobuf 依賴，具備 RPC 擴展能力。

---

## 重要目錄索引

| 路徑 | 說明 |
|---|---|
| `boot.go` | `FiberHouse` 主入口，`New()` / `Default()` / `RunServer()` |
| `application_interface.go` | 所有啟動器接口定義（`FrameStarter`、`CoreStarter`、`ApplicationRegister` 等） |
| `context_interface.go` | 上下文接口（`IContext`、`IApplicationContext`、`ICommandContext`） |
| `provider_interface.go` | `IProvider`、`IProviderManager`、`IState` 接口 |
| `provider_location.go` | `IProviderLocation` 位點體系，`ProviderLocationDefault()` |
| `provider_type.go` | Provider 類型體系 |
| `globalmanager/` | 全局對象容器實現 |
| `appconfig/` | 應用配置結構定義 |
| `bootstrap/` | 配置加載、日誌初始化 |
| `component/` | 通用組件（validate、dig 容器等） |
| `cache/` | 緩存抽象層 |
| `database/` | DB 初始化 |
| `middleware/` | 內置中間件 |
| `response/` | 統一響應結構 |
| `rpc/` | RPC 支持 |
| `commandstarter/` | CLI 應用啟動器 |
| `example_application/` | ⭐ 完整示例，理解用法的最佳入口 |
| `example_main/main.go` | ⭐ 啟動示例，展示完整組裝方式 |
| `plugins/` | 插件目錄（目前為佔位符，尚未實現） |

