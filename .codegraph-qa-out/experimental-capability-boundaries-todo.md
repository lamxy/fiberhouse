# 实验性能力边界优化代办

> 记录日期：2026-07-17
> 来源：根目录 README“当前状态”澄清，以及 `docs/reference/feature-status.md` 的现状盘点。
> 目的：保留当前已知问题现象，后续按专题验证和优化；本文不表示这些问题已修复或已有兼容性承诺。

## 状态与责任边界

“实验性”表示仓库中已有可运行实现和调用路径，但完整能力在生命周期、错误处理、并发安全或外部依赖边界上尚未充分收敛。它不表示能力只是占位，也不表示所有缺口都应由业务代码实现。

- 使用方应用的装配层负责选择和注册 provider/manager、提供配置与外部服务、创建并持有资源，以及把资源关闭纳入自身生命周期。
- 框架实现应负责支持路径上的错误可观察性、并发状态安全、关闭与重建语义，以及避免由内部缺口导致的 panic、泄漏或静默失败。
- 当前文档所说“由应用显式管理风险”，是指采用前评估限制并进行必要规避或托管，不是把框架内部缺陷永久转嫁给业务应用。

## 已知问题现象

### Gin HTTP 与二进制响应

- [x] Gin TLS 有效证书加载与 TLS serve 路径已接通；无效非空证书保持 fail-stop。缺失路径仍会记录错误并保留 HTTP 路径，且尚无真实 listener/握手集成验证，因此 Gin HTTP 内核仍维持实验性。
- [ ] MsgPack/Protobuf 内容协商只处理首个媒体类型，未命中或加载失败时回退 JSON；当前实现不是通用 RPC 或完整内容协商方案。该项仅保留为能力边界，已排除为后续调查或实施候选。

### GlobalManager 与扩展生命周期

- [x] `Release` 已改用原子指针清空实例，初始化失败可在后续 `Get` 重试，默认 keepalive 已具备取消、等待与内置 Fiber/Gin 关闭协调。
- [ ] `ClearAll` 仍是 deletion-only；`Rebuild` 旧实例退役、调用方引用存活期、非 `Closable` 的 `Release` 语义和统一资源所有权仍未闭合。
- [ ] `ServerShutdownBefore`、`ServerShutdownAfter` 等扩展位点只有声明或部分消费路径；内置 Fiber/Gin 会先停止并等待默认 keepalive，再执行 deletion-only 清空，但扩展位点消费、关闭顺序和资源回收责任仍未闭合。

### L2 缓存与 Redis 保护机制

- [ ] singleflight 尚未形成完整 loader 合并路径，Bloom filter 与 circuit breaker 的 miss 语义不一致。
- [x] L2 `Close` 已具备原子幂等、关闭后拒绝操作、子缓存单次关闭和错误聚合。
- [ ] L2 的 ants pool 排队/flush 语义、local/remote 共享所有权和构造失败传播仍需明确。

### 异步任务与 CLI

- [ ] 异步任务启动的部分内部错误只记录日志，未完整向调用方传播；worker、dispatcher 与 Redis 依赖缺少统一关闭和回收编排。
- [ ] `RunCommandStarter` 丢弃 `AppCoreRun` 返回值，CLI 健康检查只执行一次，命令结束时尚无统一资源回收链。

### MySQL 与 MongoDB

- [ ] client 重建时不会关闭旧连接；读侧没有与 client 替换锁配套，并发读取、替换和关闭边界需要专题验证。
- [ ] 连接失败会使依赖该资源的启动装配失败；需要明确错误传播、重试、降级和强制初始化策略。

## 后续专题建议

- [ ] 按“HTTP/TLS”“GlobalManager 生命周期”“缓存一致性与关闭”“任务与 CLI 生命周期”“数据库 client 重建”拆分专题，避免在一次修改中混合不同资源所有权模型。
- [ ] 每个专题先补失败路径、并发与关闭测试，再确定状态机和兼容性约束。
- [ ] 完成后同步更新 `docs/reference/feature-status.md`、对应主指南及 README 状态；只有创建、运行、报错和关闭链均有明确保证时，才考虑从“实验性”调整为“已接入”。
