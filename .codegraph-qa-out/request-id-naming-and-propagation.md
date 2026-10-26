# 请求ID命名统一与传递路径（2026-10-26）

## 结论

示例配置原本把 `application.trace.requestID` 设为 `traceId`，但值来自 requestid 中间件，实际是 request id。
已统一改名为 `requestId`，与 `appconfig` 的 fallback、fiberzerolog 的 `FieldRequestID` 一致。

## 中间件行为（读模块缓存源码核对）

| 引擎 | 中间件 | 存放位置 | 响应头 |
|---|---|---|---|
| Fiber v2.52.14 | `middleware/requestid` | `c.Locals(ContextKey)`，默认键 `requestid` | 写 `X-Request-ID`（`c.Set(cfg.Header, rid)`） |
| Gin | `gin-contrib/requestid` v1.0.6 | 不存上下文；生成时补写**请求**头，`requestid.Get` 读请求头 | 写 `X-Request-ID` |
| Hertz（示例自写） | `requestIDMiddleware(key)` | `reqCtx.Set(key)` + 标准 context `RequestIDContextKey` | 写 `X-Request-Id` |

注意：Fiber 的 requestid **也会**写响应头，不只是写 Locals。

## 日志记录点

- Recover（`DefaultStackTraceHandler`）：字段名 = `requestIDKey(appCtx)`（trace 配置，兜底 `requestId`）。
  Fiber 读 `Locals(同名键)`，所以示例中间件的 `ContextKey` 改为读同一配置，不再硬编码。
- `GetParamsJson` 等辅助 warn 日志：原硬编码 `"traceId"`，已改为跟随配置。
- `coreHttp` 访问日志：原三个引擎**都不记**请求ID；现按下文「兜底顺序」取值（先响应头 `X-Request-ID`），字段名跟随配置，无值时不写。
  Fiber 通过 fiberzerolog 的 `GetLogger`（在 `c.Next()` 之后调用）注入。
- Hertz 引擎 `Ctx*` 日志：`TraceIDContextKey` 原先没有任何写入方（死路径）；改名 `RequestIDContextKey`，由示例中间件写入。

## 未改动

- `IRecover.TraceID` 方法名：属于已发布（v1.2.0）的导出接口，改名会破坏外部实现，保留。

## 兜底顺序（方案 B，已实施）

`resolveRequestID`（`recover_error_handler_impl.go`）由 Recover 与 Fiber/Gin 访问日志共用，Hertz 示例访问日志内联同一顺序：
中间件值 → 请求头 `X-Request-ID` → 与配置同名的请求头（旧行为）。

收益评估（Apple M 系列，`-count 6` 平均，Fiber 访问日志单请求）：

| 场景 | 改前 | 改后 | 说明 |
|---|---|---|---|
| 有 requestid 中间件 | 651 ns / 1219 B / 7 allocs | 634 ns / 1219 B / 7 allocs | 无差异（噪声内），闭包未逃逸 |
| 无中间件，上游带 X-Request-ID | 491 ns / 641 B / 4 allocs，**不记 ID** | 614 ns / 1154 B / 5 allocs，记 ID | 增量即写字段的成本，与有中间件路径同源 |
| 无中间件，无请求头 | 447 ns / 641 B / 4 allocs | 484 ns / 641 B / 4 allocs | +37 ns：两次请求头查找，无分配 |
