# Component 分层命名空间

## 决策

`component/codec`、`component/task`、`component/logging` 是不包含 Go 文件的纯领域命名空间。可导入 API 位于叶子 package；父目录不提供 facade、registry 或 re-export。

| 旧路径 | 当前路径 | package | 调用 selector |
|---|---|---|---|
| `component/jsoncodec` | `component/codec/json` | `jsoncodec` | `jsoncodec` |
| `component/tasklog` | `component/task/logadaptor` | `logadaptor` | `logadaptor` |
| `component/writer` | `component/logging/writer` | `writer` | `writer` |

JSON 路径末段与 package 名有意不同，用于避免与标准库及 Gin 的 `json` package 混淆。`logadaptor` 沿用仓库现有 `adaptor/` 目录词汇；writer 使用 Go 惯用的单数名称。

## 依赖边界

- FiberHouse root 可以导入不反向依赖 root 的 `component/codec/json`。
- `bootstrap` 导入 `component/logging/writer`；writer 依赖 `appconfig`。
- `component/task/logadaptor` 依赖 FiberHouse root，因此只能由应用装配层接入，root `task.go` 不得反向导入它。
- `component/jsonconvert` 不属于 codec backend，本次保持原位。

## 迁移边界

迁移改变公开 import path 和 Go package identity，但不重命名导出 API、不保留兼容 shim，也不修改运行行为、配置、依赖版本或既有测试逻辑。完整测试仍按 `docs/reference/known-test-failures.md` 中记录的六项既有问题分类。
