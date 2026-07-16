# 已知测试失败

本文记录当前仓库中与组件目录迁移无关、但会使 `go test ./...` 返回非零状态的已知测试问题。记录基于 2026-07-16 在迁移前基线 `971b332` 和迁移分支上的重复运行结果。

分层命名空间迁移只把 `component/writer/` 原样移动到 `component/logging/writer/`，没有修改 writer 生产逻辑或测试逻辑。以下问题的名称和归因保持不变，仅 package 路径随迁移更新。

## 问题清单

| 包与测试 | 修改前基线 | 当前症状 | 已知边界 |
|---|---|---|---|
| `bootstrap.Test_Config_EnvOverrideAndSingleton` | 已失败 | 加载临时目录中的 `application_dev.yml` 时文件不存在并 panic | 环境覆盖、配置文件名和进程级 singleton 状态之间的测试隔离问题尚待单独诊断 |
| `component/logging/writer.TestAsyncDiodeWriter_Write` | 已失败 | 固定等待约 100ms 后读取 `test.log`，文件尚未创建 | 异步 writer 默认在约 1000ms 周期执行 flush，测试没有等待可观察的刷盘条件 |
| `component/logging/writer.TestAsyncDiodeWriter_MultipleWrites` | 已失败 | 固定等待不足一个 flush 周期后读取 `test.log`，文件尚未创建 | 与 `Write` 相同，测试依赖固定睡眠而不是完成条件 |
| `component/logging/writer.TestAsyncDiodeWriter_ConcurrentWrites` | 基线单次未失败；后续可重复失败 | 等待恰好 1000ms 后读取 `test.log`，可能先于首次 flush | 等待时间与 flush 周期同为 1000ms，属于调度边界上的既有时序 flaky；除路径与 package identity 外，迁移分支未修改 writer 生产或测试内容 |
| `component/logging/writer.TestAsyncDiodeWriter_Close` | 已失败 | 等待约 1000ms 后读取文件时文件仍可能不存在 | 测试中的 `Close` 调用被注释，随后仍依赖固定睡眠观察异步刷盘 |
| `component/logging/writer.TestAsyncDiodeWriter_WriteAfterClose` | 已失败 | `Write` 返回 `writer is closed` 且写入长度为 0，测试却期望写入成功 | 当前实现明确拒绝关闭后的写入，测试期望与实现契约不一致 |

## 验证与归因

- 迁移前完整测试已经复现除 `ConcurrentWrites` 外的五项失败。
- `ConcurrentWrites` 曾在相同代码下通过，也曾在完整套件和聚焦运行中失败；失败均发生在约 1 秒边界。
- 除路径和 package identity 外，组件迁移分支没有修改 writer 的生产代码或测试内容；`bootstrap/` 也没有跟踪文件差异。
- 迁移后的八个目标包可以被 `go list` 正确解析，三组聚焦测试通过，完整构建通过。

因此，这六项应作为独立测试债务处理，不应在没有对应源码差异的情况下归因为组件目录或 import path 迁移回归。

## 后续处理方向

1. 为 bootstrap 测试隔离环境变量、临时配置文件和 singleton 重置顺序，并先建立稳定复现。
2. writer 测试使用 `Close`、可观察的 flush 完成条件或带超时的条件轮询，避免用固定睡眠推断异步刷盘完成。
3. 明确并测试 `WriteAfterClose` 的公开契约：保持返回错误，或调整实现；不能继续让测试与实现持有相反预期。
