# GlitchTip 接入的后续事项（第 6、8 点）

日期：2026-10-26。框架侧的第 1–5、7 点已在 `main` 完成，见 `glitchtip-error-field-verification.md`。
以下两点不涉及框架代码。第 6 点的推送与打 tag 已于 2026-10-26 执行；第 8 点仅记录评估，**未执行**。

## 第 6 点：分支合并与版本引用

状态（2026-10-26 更新）：用户授权后已按下列步骤 1–3 执行。
推送前 `go test -race ./...` 共 1121 个测试通过；`main` 已推送至 `origin`（`8446be8..2038bac`，未 force）；
附注 tag `v1.2.0` 已推送。

- 步骤 4 **未执行**：backend-dev 的 fiberhouse.md 不在本仓库，仍引用提交 `39cba6c`，需在其所在仓库改为 `v1.2.0`。
- 远端 `feature/logger-extra-writers` 按用户决定**保留**，不删除。

原建议步骤（留作记录）：

```sh
# 1. 推送前在本地再跑一遍
CGO_ENABLED=1 go test -race ./...

# 2. 推送 main（不 force）
git push origin main

# 3. 打版本 tag。v1.x 的公共 API 有行为变更（日志级别跟随 HTTP 状态、panic 状态码统一、
#    访问日志 5xx 降为 WARN），按 semver 取下一个 minor 版本
git tag -a v1.2.0 -m "error tracking: level by HTTP status, error field, traceId, single 5xx report"
git push origin v1.2.0

# 4. 把 backend-dev 的 fiberhouse.md 从提交 39cba6c 改为引用该版本号（经 GOPROXY 可正常解析）
```

- 打 tag 前查得最新 tag 为 `v1.1.1`，远端无 `v1.2.0`，故取下一个 minor `v1.2.0`。

## 第 8 点：前端错误是否也送 GlitchTip

评估：

| 方面 | 结论 |
|---|---|
| 可行性 | GlitchTip 兼容 Sentry 的协议，前端可用 `@sentry/browser` 或框架对应 SDK 上报（**未验证**：本次只验证了 Go SDK） |
| 收益 | 后端已能定位接口错误；前端独有的问题（脚本异常、白屏、浏览器兼容）只有前端 SDK 能看到 |
| 成本与风险 | 浏览器流量大、噪音多（扩展、网络中断），需要采样与过滤；source map 上传；DSN 会暴露在前端，需单独的 project 与限流 |
| 关联 | 前后端用同一个 traceId 串联最有价值：后端已修正 traceId（`71420fd`），前端在请求头带上 `X-Request-ID` 即可对上 |

建议：值得接，但作为独立任务：为前端单独建 GlitchTip project（与后端分开告警），先只上报未捕获异常并设置采样率，
验证 source map 和 traceId 串联后再扩大范围。这属于 frontend-dev、fullstack-dev 的工作，不在本仓库实施。

## 访问日志级别对齐的前提（2026-10-26 验证）

问题：Gin / Hertz 访问日志不分状态码一律 INFO，Fiber 是 4xx/5xx WARN（`e499c2e`）。对齐成 WARN 是否会让 GlitchTip 多建 issue？

- 已验证（sentry-go/zerolog v0.50.0，源码 `Options.SetDefaults` + 离线测试，BeforeSend 计数、未真正发送）：
  `Levels` 留空 → 只发 error；`Levels` 含 Warn → 404/500 的 WARN 访问日志各建一个 warning 事件。
- 未验证：应用端实际设置的 `Levels` 与锁定的 sentry-go 版本。本机没有应用端代码（`~/workspace` 下无其他 `sentry-go/zerolog` 引用）。
- 结论：应用端保持默认 Levels 时，Gin/Hertz 对齐成 4xx/5xx WARN 不会新增 GlitchTip 事件；若应用端 Levels 含 Warn，Fiber 现在就已会建 warning 事件，对齐后 Gin/Hertz 也会。
