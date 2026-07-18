# 后续局部补丁优化清单

> 记录日期：2026-07-18
> 适用基线：`main@457f57f`
> 约束：优先局部补丁；不新增公共运行入口，不建立统一生命周期或关闭架构，不迁移公共 API。功能代码使用独立 Git 工作树和子代理执行，审核前不回合 main。

## 已撤销或排除

- [x] 撤销“Gin TLS 缺少证书路径必须 fail-stop”的建议。`tls.enable=true` 不能在没有明确契约时直接解释为“用户要求必须以 HTTPS 启动”；现有 HTTP 回退不作为缺陷候选。
- [x] 排除 MsgPack/Protobuf 内容协商调查与修改。
- [x] 排除 `Run(ctx context.Context) error` 或等价入口、统一关闭链、生命周期协调器、公共接口迁移及跨组件重构。

## P0：CLI 健康检查错误后仍记录成功

- [ ] 在 `commandstarter/frame_cmd_application.go` 的现有 `startHealthCheck` 内修复日志控制流：`Rebuild` 返回错误时记录失败并继续下一个条目，不再记录本条目的成功日志。
- [ ] 在 `commandstarter/lifecycle_test.go` 增加一个回归测试，先证明失败路径当前会产生错误的 success 文本，再以最小实现使测试通过。
- [ ] 只运行 `commandstarter` 直接相关测试和必要的补丁检查；不改方法签名、检查频率、CLI 生命周期或其他日志。

证据：当前 `startHealthCheck` 在 `gm.Rebuild(name)` 返回错误后记录 `rebuild failed`，但随后仍无条件记录 `rebuild success`，同一函数内的日志语义直接矛盾。

## P1：L2 自定义 local cache 的 metrics 边界

- [ ] 先确认 metrics 方法是否只承诺支持 `*cachelocal.LocalCache`；未确认前不修改代码。
- [ ] 若只支持内置 local cache，保留实现并仅明确文档。
- [ ] 若自定义 `cache.Cache` 也是支持路径，在现有 metrics 方法内使用安全类型断言，不匹配时返回 nil，并补局部回归测试。

该项目前是待确认契约，不直接定性为缺陷。`Level2Cache.local` 保存为 `cache.Cache`，但 `GetLocalMetrics` 与 `GetLocalMetricsInfo` 直接断言为 `*cachelocal.LocalCache`，自定义实现会 panic。

## 伴随文档检查

- [ ] 每个补丁完成后快速检查 README、`docs/reference/feature-status.md` 和对应指南，只修正与本补丁直接相关的失实句子。
- [ ] 纯文档变化不运行全仓 Go 测试，不改写相邻章节。

## 仅建议、待单独审核的有限重构

- [ ] L2 `Wait` 完整 flush 语义：若未来明确要求等待 ants pool 中已提交的异步写任务，再单独设计内部任务计数、提交失败、关闭竞争和超时策略。当前只保留为已知限制。

## 暂不进入优化队列

- `RunCommandStarter` 丢弃 `AppCoreRun` error：当前公共入口返回 void，改变传播方式会修改入口语义。
- CLI keepalive 只检查一次：尚无契约证明必须周期运行；增加 ticker 会引入新的停止生命周期。
- Fiber/Gin 在监听返回后设置 `AppState=true`：状态语义尚未定义，不能局部猜测修改。
- GlobalManager 非 `Closable` 的 `Release`：当前行为已文档化，没有证据要求重置。
- task worker/dispatcher 错误传播与统一关闭：需要跨组件生命周期设计。

## 推荐执行顺序

1. [ ] 执行并审核 P0。
2. [ ] P0 审核完成后，单独调查 P1 契约。
3. [ ] P1 结论经审核后，决定“不修改”、文档补丁或一个原函数内的最小代码补丁。
