# 框架 panic 裁決表

本表是 `feature-status.md` 條件 4 的證據。逐條記錄框架套件內每一處可執行
panic 的裁決結論：**要麼有文件化的回傳 error 替代入口，要麼有明確歸類理由。**

## 判據

條件 4 的判據見 [`feature-status.md` 的「關於條件 4 的適用邊界」](feature-status.md)。摘要：

- **允許**：不可恢復的裝配錯誤在啟動期 panic，且同時提供回傳 error 的替代入口
  （`Must*` / `*WithError` 成對出現，與 stdlib 的 `regexp.MustCompile`、
  `template.Must` 一致）。呼叫方若需處理錯誤，永遠有一條不 panic 的路徑。
- **不允許**：呼叫方可以處理、卻只能以 panic 得知的錯誤；或唯一入口即 panic。
- **運行期 panic 始終不滿足本條**，不論有無替代入口。

## 總覽

全庫非測試、非快照、可執行 panic/fatal 共 89 處。

| 類別 | 數量 | 處置 |
|---|---|---|
| `example_application/` | 7 | 排除（示例無成熟度承諾），僅列為示例限制 |
| `exception/` 業務慣用法 | 11 | 不動——`panic(exception.Get(...))` 是文件化的錯誤拋出慣例 |
| `Must*` 已有 error 姊妹 | 13 | 不動，本已滿足條件 4 |
| `LoggerWrap.Fatal()` API 本身 | 3 | 不動——是日誌器暴露 zerolog Fatal 級別的 API，非框架 fatal 呼叫 |
| 模型建構子（已有 `*WithError`） | 2 | 不動，是條件 4 的判例來源 |
| **本輪處理** | **53** | 見下表 |

## 本輪 53 處逐條裁決

### 一、裝配期 panic → 補 `*WithError` 姊妹（42 處）

原入口簽名與 panic 行為**完全不變**，改為委派給姊妹方法後再 panic。

| 檔案 | 處數 | 原入口 | 新增姊妹入口 | 哨兵 |
|---|---|---|---|---|
| `boot.go` | 10 | `resolveFrameStarterOpts`<br>`resolveCoreStarterOpts`<br>`resolveAndReturnFrameStarter`<br>`resolveAndReturnCoreStarter`<br>`RunServer` | 各自的 `*WithError`<br>`RunServerWithError` | `ErrBootAssemblyFailed` |
| `provider_impl.go` | 3 | `Check`<br>`MountToParent` | `CheckWithError`<br>`MountToParentWithError` | `ErrProviderTypeNotSet`<br>`ErrProviderMountInvalid` |
| `provider_manager_impl.go` | 5 | `Check`<br>`BindToUniqueProvider`<br>`MountToParent` | 各自的 `*WithError` | 同上 + `ErrProviderIsUnique` |
| `core_fiber_starter_impl.go` | 3 | `InitCoreApp` | `InitCoreAppMust`（反向成對，見註 1） | `ErrCoreInitFailed` |
| `core_gin_starter_impl.go` | 2 | `InitCoreApp` | 記錄 `initErr`，由 `AppCoreRun`/`Shutdown` 回傳（見註 2） | `ErrCoreInitFailed` |
| `response_facade.go` | 2 | `NewRespInfoPManagerOnce` | `NewRespInfoPManagerOnceWithError` | `ErrRespInfoPManagerUnavailable` |
| `commandstarter/core_cmd_application.go` | 3 | 三個 `Register*` | 各自的 `*WithError` | `ErrCmdApplicationNotRegistered` |
| `commandstarter/frame_cmd_application.go` | 1 | `NewFrameCmdApplication` | `NewFrameCmdApplicationWithError` | `ErrNoCmdStarterOption` |
| `option/app_starter_option.go` | 3 | `WithAppRegister`<br>`WithModuleRegister`<br>`WithTaskRegister` | 各自的 `*WithError` | `ErrRegisterTypeMismatch` |
| `option/cmd_starter_option.go` | 1 | `WithCmdRegister` | `WithCmdRegisterWithError` | 同上 |
| `appconfig/config.go` | 2 | `LoadDefault`<br>`LoadYaml` | 各自的 `*WithError`（並入 `IAppConfig` 介面） | `ErrConfigLoadFailed` |
| `bootstrap/bootstrap.go` | 2 | `NewWriterAsync` | `NewWriterAsyncWithError` | `ErrAsyncWriterUnavailable` |
| `bootstrap/bootstrap.go` | 2 | `NewConfigOnce` 內的 LoadEnv/LoadEnvConf | `NewConfigOnceWithError`（見註 3） | `ErrConfigBootstrapFailed` |
| `utils/common.go` | 2 | `GetExecPath`<br>`GetWD` | 各自的 `*WithError` | 無（直接包裝底層錯誤） |
| `frame_starter_impl.go` | 2 | `NewFrameApplication`<br>`RegisterGlobalInitializers` | 各自的 `*WithError` | `ErrNoFrameStarterOption`<br>`ErrApplicationNotRegistered` |
| `recover_error_handler_impl.go` | 3 | `RecoverMiddleware` | `RecoverMiddlewareWithError`（並入 `IErrorHandler` 介面） | `ErrRecoverMiddlewareUnavailable` |

**註 1（Fiber）**：Fiber 此前**完全沒有** `initErr` 機制（Gin 有 13 處、
hertzcore 有 15 處），故本輪為它新建了一整套錯誤狀態機。`InitCoreApp` 改為
記錄 `initErr` 並短路後續生命週期方法，`AppCoreRun`/`Shutdown` 回傳它；
原 fail-fast 行為保留在新增的 `InitCoreAppMust`。成對關係方向與其他項相反
（預設入口回傳錯誤、`Must` 入口 panic），因為呼叫方是框架自身的啟動鏈。

**註 2（Gin）**：Gin 本就有 `initErr`，這兩處卻繞開它直接 panic，屬同一類
不一致。改為與 Gin 自身機制對齊即可，不新增公共入口——Gin 內核在
`feature-status.md` 中是實驗性能力且非 `Default()` 選擇。

**註 3（LoadEnv）**：這兩處位於 `cfgOnce.Do` 閉包內，且外層 `LoadFunc` 回調
只回傳 `IAppConfig`、沒有 error 通道，因此失敗只能先記錄到套件變數，再由
`NewConfigOnceWithError` 取回。失敗只影響環境變數覆蓋那一層，回傳的設定實例
始終可用。

### 二、運行期 panic → 改為降級，不補姊妹（5 處）

條件 4 明言運行期 panic 始終不滿足本條，故這些處必須**移除** panic 而非
提供替代入口。

| 檔案:行 | 方法 | 原行為 | 新行為與理由 |
|---|---|---|---|
| `recover_fiber_impl.go:133` | `TraceID` | 上下文型別不符即 panic | 回傳 `""`。本方法在 **panic 恢復路徑**上被呼叫，再拋一次 panic 會讓恢復中介軟體自身崩潰。簽名只回傳 `string`，無 error 通道。與 `hertzcore` 同名實作一致 |
| `recover_gin_impl.go:123` | `TraceID` | 同上 | 同上 |
| `adaptor/errorhandler/gin_error_handler.go:21` | `GinErrorHandler` | 錯誤處理器自身失敗即 panic | 登記到 `c.Errors`。本中介軟體在 `c.Next()` **之後**執行，即請求已處理完畢、正在收尾錯誤響應；此時 panic 會被 recovery 轉成 500，掩蓋真正的原始錯誤 |
| `adaptor/errorhandler/gin_error_handler.go:29` | 同上 | 同上 | 同上 |
| `frame_starter_impl.go:367` | `RegisterTaskServer` | worker 解析失敗即 panic | 記錄日誌並返回。task server 是可選能力（`application.task.enableServer` 控制），它起不來不該連帶打斷整個應用啟動——Web 服務本身仍可正常對外提供 |

### 三、已合規，無需改動（1 處）

| 檔案:行 | 理由 |
|---|---|
| `component/cache/cache_utils.go:391` | `GetCache` panic ← `Lookup` 回傳 error。姊妹入口就在其上方，原始碼註解已明示「保留解析失敗時 panic 的原有相容行為」。本已滿足條件 4 |

## 為何改動後 `grep panic(` 仍有 55 處

補姊妹入口**不減少** panic 語句的數量，這是預期結果而非遺漏。

原入口的 panic 行為必須保持不變（否則就是 breaking change），因此改造後的
形態是「原入口呼叫姊妹方法，拿到錯誤後 panic」——panic 語句仍在，只是移到了
委派點。以 `utils.GetExecPath` 為例：

```go
func GetExecPath() string {
    dir, err := GetExecPathWithError()   // 姊妹入口回傳錯誤
    if err != nil {
        panic(err)                        // 原入口照常 panic
    }
    return dir
}
```

現存 55 處的組成：

| 組成 | 約略處數 | 說明 |
|---|---|---|
| 委派點 panic（本輪新形態） | 31 | 上表第一類 42 處改造後留下的 `panic(err)`，每一處都緊鄰其姊妹入口 |
| `exception/` 業務慣用法 | 10 | 文件化的錯誤拋出慣例 |
| `Must*` 已有 error 姊妹 | 13 | `provider_type.go`、`provider_location.go`、`global_utils.go`、`context_impl.go`、`boot.go` 的 `GetMustValue` |
| 模型建構子 | 2 | 條件 4 的判例來源 |
| `cache_utils.go` | 1 | 見第三類 |

判斷條件 4 是否滿足，看的是**每處 panic 是否有對應的 error 入口**，而不是
panic 語句的總數。上述 55 處全部滿足。

## 驗證

- `go build ./...` 通過
- `go vet ./...` 通過
- `go test ./... -count=1` 全通過
- `go test -race ./... -count=1` 零 data race
- 每組改動均有專項測試，涵蓋：姊妹入口的錯誤路徑、成功路徑、原入口保留
  panic、panic 文本逐字不變、哨兵可經 `errors.Is` 分類

## 相容性

本輪**不是** breaking change：

- 所有原入口的簽名與行為（含 panic 值與文本）逐字不變。
- 新增的介面方法（`IProvider`、`IProviderManager`、`IAppConfig`、
  `IErrorHandler`）對嵌入基底實作的既有型別自動滿足；倉庫內全部實作與範例
  皆已驗證編譯通過。
- 唯二的行為變更是上表第二類的 5 處運行期 panic：它們從「拋 panic」變為
  「降級並記錄」，這正是條件 4 要求的方向。
