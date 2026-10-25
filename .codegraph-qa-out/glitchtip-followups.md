# GlitchTip 接入的后续事项（第 6、8 点）

日期：2026-10-26。框架侧的第 1–5、7 点已在本地 `main` 完成，见 `glitchtip-error-field-verification.md`。
以下两点不涉及框架代码，记录评估与建议，均**未执行**。

## 第 6 点：分支合并与版本引用

现状：`feature/logger-extra-writers` 已合并进本地 `main`（`99b68ac`），之后的修正都在 `main` 上；用户指示「暂不推送」，
因此 `origin` 仍停在合并前，backend-dev 的 fiberhouse.md 规则还在引用提交 `39cba6c`。

建议（用户决定推送时按序执行）：

```sh
# 1. 推送前在本地再跑一遍
CGO_ENABLED=1 go test -race ./...

# 2. 推送 main（不 force）
git push origin main

# 3. 打版本 tag。v1.x 的公共 API 有行为变更（日志级别跟随 HTTP 状态、panic 状态码统一、
#    访问日志 5xx 降为 WARN），按 semver 取下一个 minor 版本
git tag -a v1.<next-minor>.0 -m "error tracking: level by HTTP status, error field, traceId, single 5xx report"
git push origin v1.<next-minor>.0

# 4. 把 backend-dev 的 fiberhouse.md 从提交 39cba6c 改为引用该版本号（经 GOPROXY 可正常解析）
```

- 远端 `feature/logger-extra-writers` 已完整并入 `main`，推送后可删除该远端分支（同样由用户决定）。
- `<next-minor>` 需先查 `git tag --sort=-v:refname | head` 确认当前最新版本。

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
