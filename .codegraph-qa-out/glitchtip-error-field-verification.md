# 验证报告：recover 日志写入 error 字段（改法 A）与 GlitchTip 呈现

日期：2026-10-26　框架：main（合并 feature/logger-extra-writers 之后）　复现套件：`glitchtip-verify/`

## 背景

FiberHouse 错误经 `BootConfig.LogWriters` 附加的 sentryzerolog writer 进入 GlitchTip。sentryzerolog 的字段映射
（v0.50.0 源码核实）：`message` → issue 标题；`error` → `Exception{Value, Stacktrace: sentry.NewStacktrace()}`；
其余所有字段 → tag。`Options.Levels` 默认只收 Error / Fatal / Panic，其余级别不建 issue。

改法 A：`DefaultStackTraceHandler` 在非 error panic 值包装之后调用一次 `logEvent.Err(errorFieldOf(e))`，
把错误写入 zerolog 标准的 `error` 字段。已落地：commit `c9a3b9d`。

## 环境

- GlitchTip 6.2.6（Docker：postgres:18、valkey:9），sentry-go / sentryzerolog v0.50.0（Levels 默认）
- 日志器走框架真实路径 `bootstrap.NewLoggerOnceWithWriters`：`io.MultiWriter` 同步写入，sentryzerolog 走 `Write`（从 JSON 解析 level）
- 场景：`/panic`（nil 指针 panic，恢复中间件路径）、`/error`（handler 返回 error，ErrorHandler 路径）

## 结果

| 问题 | 现状（无 error 字段） | 改法 A | 改法 A + 应用端 BeforeSend |
|---|---|---|---|
| issue 标题 | `runtime error: …`（message 事件） | `Error: runtime error: …`（缺 type 时 GlitchTip 补 `Error`） | 同 A |
| panic 堆栈 | 无 Exception 面板，堆栈只在 `stack` tag | ✅ 顶帧 `main in panicSite at line 126`，culprit `main in panicSite` | ✅ 同 A，且只有应用帧标 in-app |
| 返回 error 的堆栈 | 同上 | ⚠️ 顶帧是 `jsonparser.ObjectEach`、`zerolog.(*Event).Msg` 等日志链路帧，culprit 显示 `github.com/buger/jsonparser in ObjectEach` | ✅ 日志链路帧被裁掉，culprit 为 `fiberhouse in (*ErrorHandler).DefaultStackTraceHandler` |
| 大段 tag（stack、reqHeaders…） | 完整存储（2316 字符，含换行，无 ingest 错误），但页面上压成一行溢出、侧栏统计约 200 字符截断，**读不了** | 同左 | ✅ 移入 `fiberhouse` 上下文区块，tag 只剩短字段 |
| 分组 | 按 message 分 | 不同错误文本各自成 issue，相同错误合并（count=2），未因堆栈相同而误合并 | 同 A |

推翻的两个先前推测：
1. 以为 tag 会像 Sentry 一样被截断到约 200 字符——GlitchTip 实际完整存储，问题只在页面呈现。
2. 担心返回 error 路径的堆栈都相同会把不同错误并成一个 issue——实测不会。

附带观察：`traceId` 恒为空字符串（待处理清单第 4 点）；`caller` 正确指向 handler 里调用 logger 的行（`2deccb4` 生效）。

## 结论与落地

- **框架**：改法 A 收益明确（panic 定位到发生处），已提交 `c9a3b9d`，可单独回滚。框架日志结构保持与厂商无关，不为 Sentry 的数据模型调整字段。
- **应用端**：返回 error 路径的 culprit 噪音和大段 tag 的可读性，根源都在 sentryzerolog 的映射方式，sentry-go v0.50.0 没有 InAppExclude 一类的配置项，只能在 `BeforeSend` 处理。方案已验证有效，保存于 `glitchtip-app-side-before-send.md` 备用。

## 未验证边界

- 只验证了 Fiber 内核；Gin、Hertz 共用同一个 `DefaultStackTraceHandler`，`error` 字段行为相同，但 GlitchTip 端未逐一实测。
- sentry-go 只验证了 v0.50.0；应用实际锁定的版本需自行核对（v0.50.0 的 `New` 签名已与 master 不同）。
- 应用端 sentryzerolog 的 `Levels` 是否包含 Warn 仍需应用侧确认（决定 4xx 业务错误是否还会建 issue）。

## 追加：traceId 为空的修正（待处理清单第 4 点）

- 根因：`DefaultStackTraceHandler` 用 `GetHeader` 读**请求**头；requestid 中间件把 ID 存进上下文（Fiber 为 `Locals`），
  只写到**响应**头，客户端不自带时日志里的 traceId 恒为空。
- 修正：commit `71420fd`（`fix(recover): read the trace id from the requestid middleware`）改为先取
  `recovery.TraceID(ctx)`，为空再退回同名请求头，保留无中间件应用的原有行为。
- 验证：验证套件加上与示例应用相同的 `requestid.New(requestid.Config{ContextKey: "traceId"})` 后，
  GlitchTip 事件（environment `traceid-fix`）的 traceId 为中间件生成的 UUID，panic 与返回 error 两条路径都有值。
