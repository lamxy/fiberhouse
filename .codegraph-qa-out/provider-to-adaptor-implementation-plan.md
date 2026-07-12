# Provider-to-Adaptor Rename Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 將根目錄 `provider/` 破壞式搬移為 `adaptor/`，並將原 `provider/adaptor/` 對齊為 `adaptor/errorhandler/`，完整更新本倉庫產品碼 import、alias 與 selector。

**Architecture:** 先原子化搬移 context package 並更新全部 19 個舊 context import，使倉庫在第一個提交後仍可編譯；再搬移 error handler package 並更新兩個 starter，使第二個提交完成最終目錄結構。公開函式 `FiberErrorHandler`、`GinErrorHandler` 和 context 公開符號全部保留原名，只改 package identity 與 error handler package clause。

**Tech Stack:** Go 1.25、Git worktree、CodeGraph CLI、ast-grep 0.42.2、ripgrep、gofmt。

## Global Constraints

- 只在 `/mnt/d/code/github_opensource/tmp/fiberhouse/.worktrees/adaptor-directory-rename`、分支 `refactor/adaptor-directory-rename` 修改。
- 目標路徑必須是 `provider/context/ -> adaptor/context/` 和 `provider/adaptor/ -> adaptor/errorhandler/`。
- 不保留 `provider/context`、`provider/adaptor` compatibility shim。
- 不修改 `README.md` 或 `docs/*.md`。
- 不修改 root package 的 provider 架構名稱，也不修改 `example_application/providers/`、`ctx_*_adaptor_provider.go` 或 `CoreCtx*Provider` 名稱。
- context import alias 統一為 `adaptorctx`。
- starter 的 error handler import alias 統一為 `adaptorerrorhandler`。
- starter 必須使用 `adaptorerrorhandler.FiberErrorHandler` 與 `adaptorerrorhandler.GinErrorHandler`。
- `adaptor/context` 保留 `package context`；`adaptor/errorhandler` 使用 `package errorhandler`。
- 不新增測試；驗證重點是 import 歸零、package 可發現、全倉編譯/建置，以及完整測試不增加失敗 package。
- 已知改名前 `go test ./...` 只失敗於 `bootstrap` 和 `component/writer`；實作後不得出現 package not found、undefined selector、import cycle 或其他新增失敗 package。
- 不手工修改 `.codegraph/`、`graphify-out/` 或歷史運行日誌。

---

### Task 1: 搬移 context package 並更新所有直接消費者

**Files:**

- Move: `provider/context/core_ctx_wrap_interface.go` → `adaptor/context/core_ctx_wrap_interface.go`
- Move: `provider/context/core_ctx_wrap_fiber_impl.go` → `adaptor/context/core_ctx_wrap_fiber_impl.go`
- Move: `provider/context/core_ctx_wrap_gin_impl.go` → `adaptor/context/core_ctx_wrap_gin_impl.go`
- Modify: `provider/adaptor/fiber_error_handler.go`
- Modify: `provider/adaptor/gin_error_handler.go`
- Modify: `ctx_core_adapter.go`
- Modify: `ctx_fiber_adaptor_provider.go`
- Modify: `ctx_gin_adaptor_provider.go`
- Modify: `recover_config.go`
- Modify: `recover_error_handler_impl.go`
- Modify: `recover_interface.go`
- Modify: `recover_recoveries_impl.go`
- Modify: `response.go`
- Modify: `response/response_interface.go`
- Modify: `response/response_impl.go`
- Modify: `response/response_proto_impl.go`
- Modify: `response/response_msgpack_impl.go`
- Modify: `exception/exception_error.go`
- Modify: `example_application/module/example-ginapi-module/api/common_api.go`
- Modify: `example_application/module/example-ginapi-module/api/example_api.go`
- Modify: `example_application/module/example-module/api/common_api.go`
- Modify: `example_application/module/example-module/api/example_api.go`

**Interfaces:**

- Consumes: 現有 `context.ICoreContext`、`WithFiberContext`、`WithGinContext`、`FiberContext`、`GinContext` 的原始定義與行為。
- Produces: `github.com/lamxy/fiberhouse/adaptor/context`，package name 仍為 `context`；所有倉庫內消費者以 `adaptorctx` alias 使用相同公開符號。

- [ ] **Step 1: 確認 Task 1 的精確舊 import 集合**

Run:

```bash
ast-grep run --pattern '"github.com/lamxy/fiberhouse/provider/context"' --lang go .
```

Expected: 19 個 Go 檔匹配，包括兩個 `provider/adaptor/*.go`、8 個 root package 檔、4 個 `response` 檔、1 個 `exception` 檔和 4 個範例 API 檔。

- [ ] **Step 2: 搬移 context package**

Run:

```bash
mkdir -p adaptor
git mv provider/context adaptor/context
```

Expected: 三個定義檔由 `provider/context/` 搬到 `adaptor/context/`；檔案內容和 `package context` 不變。

- [ ] **Step 3: 更新全部 context import、alias 與 selector**

每個列出的消費檔都將舊 import：

```go
providerctx "github.com/lamxy/fiberhouse/provider/context"
```

或 `providerCtx` / 無 alias 形式改成：

```go
adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
```

並在同一檔案將 selector 精確對齊：

```go
adaptorctx.ICoreContext
adaptorctx.WithFiberContext(...)
adaptorctx.WithGinContext(...)
```

不得修改 `ICoreContext` 的方法集合、wrapper pool、JSON/Send/header 行為或任何函式參數。

- [ ] **Step 4: 格式化受影響消費檔**

Run:

```bash
gofmt -w \
  provider/adaptor/fiber_error_handler.go \
  provider/adaptor/gin_error_handler.go \
  ctx_core_adapter.go \
  ctx_fiber_adaptor_provider.go \
  ctx_gin_adaptor_provider.go \
  recover_config.go \
  recover_error_handler_impl.go \
  recover_interface.go \
  recover_recoveries_impl.go \
  response.go \
  response/response_interface.go \
  response/response_impl.go \
  response/response_proto_impl.go \
  response/response_msgpack_impl.go \
  exception/exception_error.go \
  example_application/module/example-ginapi-module/api/common_api.go \
  example_application/module/example-ginapi-module/api/example_api.go \
  example_application/module/example-module/api/common_api.go \
  example_application/module/example-module/api/example_api.go
```

Expected: 只整理 19 個實際修改的消費檔；三個只搬移的 context 定義檔不產生格式噪音。

- [ ] **Step 5: 驗證 context 搬移後的原子狀態**

Run:

```bash
ast-grep run --pattern '"github.com/lamxy/fiberhouse/provider/context"' --lang go .
go test . ./adaptor/context ./provider/adaptor ./response ./exception \
  ./example_application/module/example-module/api \
  ./example_application/module/example-ginapi-module/api
git diff --check
```

Expected: ast-grep 無輸出；所有列出的 package 編譯/測試成功；`git diff --check` exit 0。

- [ ] **Step 6: 審閱並提交 context 搬移**

Run:

```bash
git diff --stat
git diff --find-renames
git add adaptor/context provider/adaptor \
  ctx_core_adapter.go ctx_fiber_adaptor_provider.go ctx_gin_adaptor_provider.go \
  recover_config.go recover_error_handler_impl.go recover_interface.go \
  recover_recoveries_impl.go response.go response exception \
  example_application/module/example-module/api/common_api.go \
  example_application/module/example-module/api/example_api.go \
  example_application/module/example-ginapi-module/api/common_api.go \
  example_application/module/example-ginapi-module/api/example_api.go
git commit -m "refactor: move core context adaptors"
```

Expected: commit 僅包含三個 context 檔搬移與 19 個舊 context import 消費檔；README 和 `docs/*.md` 不在提交中。

---

### Task 2: 搬移 error handler package 並更新 starter

**Files:**

- Move: `provider/adaptor/fiber_error_handler.go` → `adaptor/errorhandler/fiber_error_handler.go`
- Move: `provider/adaptor/gin_error_handler.go` → `adaptor/errorhandler/gin_error_handler.go`
- Modify: `core_fiber_starter_impl.go`
- Modify: `core_gin_starter_impl.go`

**Interfaces:**

- Consumes: Task 1 產生的 `adaptorctx.ICoreContext`、`adaptorctx.WithFiberContext`、`adaptorctx.WithGinContext`。
- Produces: `github.com/lamxy/fiberhouse/adaptor/errorhandler`；公開函式簽名仍為 `FiberErrorHandler(...)` 與 `GinErrorHandler(...)`。

- [ ] **Step 1: 搬移 error handler package**

Run:

```bash
git mv provider/adaptor adaptor/errorhandler
```

Expected: `provider/` 因已無檔案而消失；兩個 error handler 檔出現在 `adaptor/errorhandler/`。

- [ ] **Step 2: 對齊 error handler package clause**

兩個新檔案都將：

```go
package adaptor
```

改為：

```go
package errorhandler
```

保留函式宣告的大小寫與簽名：

```go
func FiberErrorHandler(fn func(adaptorctx.ICoreContext, error) error) fiber.ErrorHandler
func GinErrorHandler(fn func(adaptorctx.ICoreContext, error) error) gin.HandlerFunc
```

- [ ] **Step 3: 更新兩個 starter import 與 selector**

`core_fiber_starter_impl.go` 和 `core_gin_starter_impl.go` 將舊 import 改為：

```go
adaptorerrorhandler "github.com/lamxy/fiberhouse/adaptor/errorhandler"
```

Fiber starter 使用：

```go
adaptorerrorhandler.FiberErrorHandler
```

Gin starter 使用：

```go
adaptorerrorhandler.GinErrorHandler
```

- [ ] **Step 4: 格式化並驗證 error handler 搬移**

Run:

```bash
gofmt -w \
  adaptor/errorhandler/fiber_error_handler.go \
  adaptor/errorhandler/gin_error_handler.go \
  core_fiber_starter_impl.go \
  core_gin_starter_impl.go
ast-grep run --pattern '"github.com/lamxy/fiberhouse/provider/adaptor"' --lang go .
go test . ./adaptor/...
git diff --check
```

Expected: ast-grep 無輸出；root package 與兩個新 adaptor package 編譯成功；`git diff --check` exit 0。

- [ ] **Step 5: 審閱並提交 error handler 搬移**

Run:

```bash
git diff --stat
git diff --find-renames
git add adaptor/errorhandler core_fiber_starter_impl.go core_gin_starter_impl.go
git commit -m "refactor: move error handler adaptors"
```

Expected: commit 只包含兩個 error handler 檔搬移、package clause 修改和兩個 starter 更新；函式名仍是 `FiberErrorHandler` / `GinErrorHandler`。

---

### Task 3: 全倉靜態、建置與測試驗證

**Files:**

- Verify only: 全部已修改 Go 檔與 Git 提交。
- Do not modify: `README.md`、`docs/*.md`、`.codegraph/`、`graphify-out/`、歷史日誌。

**Interfaces:**

- Consumes: Task 1 與 Task 2 的最終 package 路徑與公開符號。
- Produces: 可重現的完成證據；不得以既有測試紅燈掩蓋新增編譯或 package 失敗。

- [ ] **Step 1: 驗證舊 Go import 完全歸零**

Run:

```bash
ast-grep run --pattern '"github.com/lamxy/fiberhouse/provider/context"' --lang go .
ast-grep run --pattern '"github.com/lamxy/fiberhouse/provider/adaptor"' --lang go .
rg -n 'github\.com/lamxy/fiberhouse/provider/(context|adaptor)' --glob '*.go'
```

Expected: 三個命令都無匹配。README、`docs/*.md` 和分析文檔中的歷史文字不屬於 Go import 驗證。

- [ ] **Step 2: 驗證新 package、package clause 與 selector**

Run:

```bash
go list ./adaptor/...
rg -n '^package errorhandler$' adaptor/errorhandler/*.go
rg -n 'adaptorerrorhandler\.(FiberErrorHandler|GinErrorHandler)' \
  core_fiber_starter_impl.go core_gin_starter_impl.go
test ! -d provider
```

Expected:

```text
github.com/lamxy/fiberhouse/adaptor/context
github.com/lamxy/fiberhouse/adaptor/errorhandler
```

兩個 error handler 檔都匹配 `package errorhandler`；兩個 starter 各匹配正確 selector；根目錄 `provider/` 不存在。

- [ ] **Step 3: 驗證全倉編譯與 CI 同等建置**

Run:

```bash
go build ./...
go build -o /tmp/fiberhouse-adaptor-rename-fhweb ./example_main/main.go
```

Expected: 兩個命令 exit 0；不在 worktree 留下 build artifact。

- [ ] **Step 4: 比較完整測試與既有基線**

Run:

```bash
go test ./...
```

Expected: exit 1，但失敗 package 只允許：

```text
github.com/lamxy/fiberhouse/bootstrap
github.com/lamxy/fiberhouse/component/writer
```

所有 `adaptor/...`、root、`response`、`exception` 與範例 API package 必須完成編譯；不得出現舊 import、undefined selector 或 import cycle。測試若再次產生未追蹤的 `component/writer/D:/invalid/path/test.log`，只刪除該測試產物並用 `rmdir -p` 清理空目錄，然後再次確認 Git status。

- [ ] **Step 5: 驗證提交範圍與工作區狀態**

Run:

```bash
git diff --check main...HEAD
git diff --name-status main...HEAD
git status --short --branch
```

Expected: `main...HEAD` 差異只包含 `.codegraph-qa-out/` 的分析/計畫變更，以及 24 個改名直接影響的 Go 檔；`.gitignore` worktree 安全規則已存在於兩個分支的共同基底，不會出現在此差異中。`README.md`、`docs/*.md` 和其他產品碼不在 diff；工作區乾淨。

---

### Task 4: CodeGraph 搬移後驗證

**Files:**

- Verify only: `adaptor/context/*.go`、`adaptor/errorhandler/*.go` 及其 caller paths。
- Do not modify: `.codegraph/codegraph.db*`、daemon log/pid。

**Interfaces:**

- Consumes: 已建置成功的最終檔案布局。
- Produces: CodeGraph source path/caller path 驗證結果，或明確的 watcher 索引延遲記錄。

- [ ] **Step 1: 查詢新 context 路徑與公開符號**

Run:

```bash
codegraph explore "Show the current definitions and complete caller blast radius for adaptor/context ICoreContext, WithFiberContext, and WithGinContext after the directory rename."
```

Expected: source path 指向 `adaptor/context/*.go`，callers 指向 root、response、exception 和範例 API 的新 import consumers。

- [ ] **Step 2: 查詢新 error handler 路徑與 starter caller**

Run:

```bash
codegraph explore "Show the current definitions and callers for adaptor/errorhandler FiberErrorHandler and GinErrorHandler after the directory rename."
```

Expected: source path 指向 `adaptor/errorhandler/*.go`；每個函式各有一個 root starter caller。若 CodeGraph 仍顯示舊路徑，記錄 watcher/index stale 狀態，以 ast-grep、`go list` 和 build 結果作為本次完成證據，不手工修改生成索引。
