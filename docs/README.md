# FiberHouse 文档

这里保存 FiberHouse 的设计、运行机制、组件边界和示例说明。根目录 README 用于快速判断与首次体验；需要确认默认值、装配条件、错误语义、并发边界或已知限制时，从本索引进入专题页。

## 阅读路径

1. **首次运行**：从[入门与首次运行](#入门与首次运行)完成依赖准备、配置选择和示例健康检查。
2. **理解架构**：按[设计与运行模型](#设计与运行模型)依次理解 Context、Starter、Provider 和生命周期。
3. **使用组件**：在[组件与专题指南](#组件与专题指南)选择配置、Web、缓存、数据库、任务等能力，并先检查各自生命周期限制。
4. **扩展与维护**：阅读[扩展与维护](#扩展与维护)，再用参考页确认能力成熟度、组件调用者与示例边界。

## 入门与首次运行

- [入门指南](getting-started.md)：环境要求、完整 Web 示例、配置覆盖、Fiber/Gin 切换、外部项目装配和常见失败。

## 设计与运行模型

- [架构总览](concepts/architecture.md)：框架职责、Web/CLI 形态、核心对象和依赖方向。
- [Web 启动生命周期](concepts/startup-lifecycle.md)：从 `New` 到监听、信号与停止的真实执行顺序。
- [Provider 系统](concepts/provider-system.md)：Type、Target、Provider、Manager、Location 的选择与执行契约。
- [Context 与 Locator](concepts/context-and-locators.md)：应用 Context、全局容器、运行期定位和 Wire 的差异。

## 组件与专题指南

- [配置与引导](guides/configuration.md)：`BootConfig`、YAML、`APP_ENV_`、`APP_CONF_` 与进程级单例。
- [日志](guides/logging.md)：zerolog、Origin、轮转、异步 writer、丢弃指标与关闭顺序。
- [GlobalManager](guides/global-manager.md)：initializer、延迟单例、健康检查、重建、释放和清理限制。
- [Web 运行时](guides/web-runtime.md)：Fiber/Gin 装配、codec、请求 Context、监听和 TLS 边界。
- [响应与序列化](guides/response-and-serialization.md)：业务响应、HTTP status、JSON 与二进制协商、对象池所有权。
- [错误与恢复](guides/errors-and-recovery.md)：普通 error、panic、异常分类、trace、脱敏和生产配置。
- [参数校验](guides/validation.md)：内建语言、自定义 tag、错误响应与启动写/运行读边界。
- [缓存](guides/cache.md)：本地、Redis、L2、read-through、保护机制和关闭限制。
- [数据库](guides/database.md)：MySQL、MongoDB、GlobalManager 注册、model locator 和 client 生命周期。
- [后台任务](guides/background-tasks.md)：asynq worker/dispatcher、handler context、启动和资源所有权。
- [命令行应用](guides/command-line.md)：CLI Context、urfave/cli 启动顺序、退出码和清理。

## 扩展与维护

- [扩展 FiberHouse](guides/extending-fiberhouse.md)：自定义 Provider/Manager/Location、codec、响应协议和 CoreStarter。
- [功能状态](reference/feature-status.md)：定义“已接入”“实验性”“内部工具”“预留/占位”，并逐项说明默认注册和应用启用条件。
- [组件目录](reference/components.md)：`component/`、数据库辅助和内部工具的调用者、并发与生命周期索引。
- [示例目录](reference/examples.md)：`example_main`、`example_config`、`example_application` 的可借鉴部分与不完善处。

## 文档约定

- 当前源码是行为依据；配置键、默认值和错误路径以可达实现为准。
- “默认集合”不等于自动启用，应用仍需显式传入 Provider/Manager 并提供业务注册器。
- 示例只用于理解接线，不构成生产模板、稳定 API 或成熟度承诺。
- 源码静态分析发现的问题会标注为限制或风险；没有运行复现时不会写成确定故障。
- 文档中的相对命令默认从仓库根目录执行，另有说明的 CLI 示例除外。
