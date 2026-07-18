# 后续局部补丁优化清单

> 记录日期：2026-07-18
> 收口基线：`main@5faeabf`
> 状态：P0 已完成并合入 main；P1 已确认既有契约并决定不修改代码；当前没有自动承接的局部代码任务。
> 约束：优先局部补丁；不新增公共运行入口，不建立统一生命周期或关闭架构，不迁移公共 API。功能代码使用独立 Git 工作树和子代理执行，审核前不回合 main。

## 已撤销或排除

- [x] 撤销“Gin TLS 缺少证书路径必须 fail-stop”的建议。`tls.enable=true` 不能在没有明确契约时直接解释为“用户要求必须以 HTTPS 启动”；现有 HTTP 回退不作为缺陷候选。
- [x] 排除 MsgPack/Protobuf 内容协商调查与修改。
- [x] 排除 `Run(ctx context.Context) error` 或等价入口、统一关闭链、生命周期协调器、公共接口迁移及跨组件重构。

## P0：CLI 健康检查错误后仍记录成功（已完成）

- [x] `5faeabf` 在现有 `startHealthCheck` 内修复日志控制流：`Rebuild` 返回错误时记录失败并继续下一个条目，不再记录本条目的成功日志。
- [x] `commandstarter/lifecycle_test.go` 已增加真实 GlobalManager 失败路径回归测试，并按 RED→GREEN 验证。
- [x] focused 测试与 `go test ./commandstarter -count=1` 在分支及合入后的 main 上通过；补丁保持方法签名、检查频率、CLI 生命周期和其他日志不变。

修复前证据：`startHealthCheck` 在 `gm.Rebuild(name)` 返回错误后记录 `rebuild failed`，但随后仍无条件记录 `rebuild success`，同一函数内的日志语义直接矛盾。

## P1：L2 自定义 local cache 的 metrics 边界（已调查，不修改代码）

- [x] 已确认 `cache.Cache` 只承载通用缓存行为，metrics 方法调用的是内置 `*cachelocal.LocalCache` 的具体 `GetMetrics` 能力。
- [x] 保留当前具体类型断言；安全断言返回 nil 会掩盖不支持的调用，新增 metrics 接口则超出局部补丁范围。
- [x] 缓存指南明确该入口不属于通用 `cache.Cache` 契约；自定义 local 实现不得调用这些 metrics 入口。

结论：该具体类型断言是既有能力边界，不作为缺陷候选，也不新增代码补丁。

## 伴随文档检查

- [x] 已在 `main@5faeabf` 快速检查 README、`docs/reference/feature-status.md` 和对应指南；仅对齐 Fiber/Gin 状态维度与 L2 metrics 具体能力边界。
- [x] 本次纯文档变化不运行全仓 Go 测试，不改写相邻章节。

## 仅建议、待单独审核的有限重构

- [ ] L2 `Wait` 完整 flush 语义：若未来明确要求等待 ants pool 中已提交的异步写任务，再单独设计内部任务计数、提交失败、关闭竞争和超时策略。当前只保留为已知限制。

## 暂不进入优化队列

- `RunCommandStarter` 丢弃 `AppCoreRun` error：当前公共入口返回 void，改变传播方式会修改入口语义。
- CLI keepalive 只检查一次：尚无契约证明必须周期运行；增加 ticker 会引入新的停止生命周期。
- Fiber/Gin 在监听返回后设置 `AppState=true`：状态语义尚未定义，不能局部猜测修改。
- GlobalManager 非 `Closable` 的 `Release`：当前行为已文档化，没有证据要求重置。
- task worker/dispatcher 错误传播与统一关闭：需要跨组件生命周期设计。

## 推荐执行顺序

1. [x] 执行、审核并合入 P0。
2. [x] 单独调查 P1 契约。
3. [x] P1 结论为不修改代码，仅澄清对应指南。
