# CodeGraph Q&A: 代碼庫入口分析

## 問題

使用 codegraph mcp，分析當前代碼庫的有那些入口？

## 回答

使用 CodeGraph MCP 分析後，這個 Go 代碼庫的主要入口可以分成以下幾類。

### 可執行入口

- `example_main/main.go:28`
  - HTTP/Web 範例應用入口。
  - 建立 `fiberhouse.New(&BootConfig{...})`，設定 `CoreTypeWithFiber`、config/log 路徑、providers/managers，最後呼叫 `RunServer()` 啟動服務。

- `example_application/command/main.go:11`
  - CLI 範例入口。
  - 初始化 config/logger/command context，建立 command application，最後用 `commandstarter.RunCommandStarter(...)` 跑 `urfave/cli` 命令列應用。

### 框架 API 入口

- `boot.go:127`
  - `fiberhouse.New()` 是框架主建構入口。
  - 建立全局 container、config、logger、application context。

- `boot.go:286`
  - `(*FiberHouse).RunServer()` 是 Web server 啟動流程入口。
  - 載入 provider、建立 frame/core starter、註冊 globals/middleware/routes/swagger/task，最後呼叫 core 的 `AppCoreRun()`。

- `default.go:30`
  - `DefaultProviders()` 是預設 provider 集合入口。
  - 包含 Fiber/Gin core starter、JSON codec、recover middleware、protobuf/msgpack 等預設能力。

### Web Core 入口

- `core_fiber_starter_impl.go:54`
  - Fiber core 初始化入口。
  - 建立 `fiber.App`、JSON codec、error handler、核心 middleware。

- `core_fiber_starter_impl.go:269`
  - Fiber runtime 入口。
  - 從 config 讀 `application.server.host/port`，呼叫 `app.coreApp.Listen(...)`，並處理 shutdown。

- `core_gin_starter_impl.go:51`
  - Gin core 初始化入口。
  - 建立 `gin.Engine` 與 `http.Server`。

- `core_gin_starter_impl.go:317`
  - Gin runtime 入口。
  - 呼叫 `httpServer.ListenAndServe()`，監聽 SIGINT/SIGTERM 做 graceful shutdown。

### 路由入口

- `example_application/module/module_impl.go:35`
  - 模組路由註冊總入口。
  - 透過 route register provider manager 載入對應核心框架的路由註冊器。

- `example_application/module/example-module/api/register_api_router.go:10`
  - Fiber 路由入口。
  - 註冊 `/example`、`/health`、`/common` 路由。

- `example_application/module/example-ginapi-module/api/register_api_router.go:8`
  - Gin 路由入口。
  - 註冊 `/gin/example`、`/gin/common` 路由。

### CLI 入口

- `commandstarter/frame_cmd_application.go:14`
  - `RunCommandStarter()` 是 CLI 啟動流程入口。
  - 初始化 core app、註冊錯誤處理、commands、global options，最後執行 CLI。

- `commandstarter/core_cmd_application.go:51`
  - CLI core 初始化入口。
  - 建立 `cli.App`。

- `commandstarter/core_cmd_application.go:93`
  - CLI runtime 入口。
  - 呼叫 `coreApp.Run(os.Args)`。

- `example_application/command/application/application.go:92`
  - CLI 命令註冊入口。
  - 目前註冊了 `commands.NewTestOrmCMD(...)`。

- `example_application/command/application/commands/test_orm_command.go:23`
  - 具體 CLI command 入口。
  - 命令名是 `test-orm`，alias 是 `orm`。

### 異步任務入口

- `example_application/module/task_impl.go:20`
  - `NewTaskAsync()` 建立 task registrar。

- `example_application/module/task_impl.go:43`
  - `GetTaskHandlerMap()` 統一收集 task handlers。

- `example_application/module/example-module/task/handler/mount.go:10`
  - example-module 的任務 handler 掛載入口。

- `task.go:35`
  - `NewTaskWorker()` 建立 Asynq worker。

- `task.go:88`
  - `TaskWorker.RunSync()` / `RunServer()` 是 worker runtime 入口。

### 初始化與生成類入口

- `example_main/docs/docs.go:303`
  - Swagger docs 的 `init()` 入口。
  - 註冊 `SwaggerInfo`。

- `rpc/protosrc/resp_info.pb.go:122`
  - protobuf generated code 的 `init()` 入口。
  - 初始化 proto descriptor。

## 補充觀察

CodeGraph 顯示 `core_starter_gin_provider.go:25` 的 Gin provider 在沒有 `initFunc` 時回傳 `NewCoreWithFiber(...)`。這看起來像 Gin/Fiber provider fallback 寫錯；後續若要整理入口或修 core 選擇邏輯，這裡值得檢查。
