# FiberHouse 文档体系重构设计

## 目标

重写根目录 `README.md`，并建立一套面向首次接入者、同时能够支持框架维护者深入理解实现的 `docs/` 文档体系。

完成后的文档应让读者能够：

1. 快速判断 FiberHouse 的定位、适用场景、现有能力和成熟度边界。
2. 运行仓库内示例，并理解如何把框架装配到独立应用。
3. 建立 Boot、Starter、Provider、Manager、Location、Context、GlobalManager 之间的核心心智模型。
4. 查到配置、日志、HTTP 引擎、响应、异常恢复、缓存、任务、数据库和命令行等能力的真实行为。
5. 明确区分框架实现、默认装配、示例代码、实验性能力和占位模块。

## 目标读者

根 `README.md` 第一优先服务首次接入 FiberHouse 的业务应用开发者。框架维护者和需要自定义实现的高级使用者通过 `docs/` 继续阅读。

文档使用简体中文，保留 Go 标识符、配置键、文件路径和协议名的原始英文名称。

## 设计原则

- 以可验证的当前源码为唯一事实基础；`.codegraph-qa-out/` 是分析参考，不是最终权威。
- README 是入口和导航，不复制完整接口定义、应用目录树或大段示例实现。
- 专题文档按用户要解决的问题组织，不按 Go package 机械地“一包一页”。
- 每个专题说明职责、入口、默认值、装配条件、生命周期、错误语义、并发边界和已知限制。
- 示例用于解释装配方式，不把 `example_main`、`example_config`、`example_application` 描述成生产模板或稳定 API。
- 未完整接入启动链的插件、RPC、MQ、i18n 等能力只能标为预留或占位。
- 对实现与示例声明不一致的地方，以实现为准并显式记录差异。
- 文档链接使用仓库相对路径；代码片段应来自当前可编译 API，并尽量通过构建或测试验证。

## 根 README 设计

`README.md` 保持单页快速阅读体验，章节依次为：

1. 项目定位：FiberHouse 是什么、适合解决什么问题。
2. 当前状态：稳定、实验性、占位能力的简表，并链接完整状态页。
3. 核心能力：只描述已经由源码实现或默认装配的能力。
4. 环境要求：以 `go.mod` 为准；数据库、Redis 等依赖注明仅在启用对应能力时需要。
5. 五分钟体验：验证后的仓库示例运行步骤、配置路径和预期结果。
6. 接入骨架：一段最小且真实的 `BootConfig`、Provider/Manager 收集与 `RunServer` 代码。
7. 核心模型：用一张小型关系图解释 Provider、Manager、Location 与 Starter。
8. 启动主链：从 `New`、bootstrap、Frame/Core Starter 到 middleware、route、task、server run/shutdown。
9. 请求主链：原生 handler、context adaptor、response/error/recovery 的关系。
10. Fiber 与 Gin：切换入口和不完全对称的实现边界。
11. 示例目录：三个 `example_*` 目录各自承担什么演示职责。
12. 文档导航：按“首次使用、理解设计、使用组件、扩展维护”给出阅读路径。
13. 开发、贡献、许可证与反馈入口。

README 不再包含：完整接口源码、完整业务应用目录树、CRUD 全流程代码、Wire 教程、缓存/任务/命令行的大段示例，以及整份配置文件字段罗列。这些内容分别下沉到对应专题。

## docs 信息架构

```text
docs/
├── README.md
├── getting-started.md
│
├── concepts/
│   ├── architecture.md
│   ├── startup-lifecycle.md
│   ├── provider-system.md
│   └── context-and-locators.md
│
├── guides/
│   ├── configuration.md
│   ├── logging.md
│   ├── web-runtime.md
│   ├── response-and-serialization.md
│   ├── errors-and-recovery.md
│   ├── global-manager.md
│   ├── cache.md
│   ├── command-line.md
│   ├── background-tasks.md
│   ├── database.md
│   ├── validation.md
│   └── extending-fiberhouse.md
│
└── reference/
    ├── examples.md
    ├── components.md
    └── feature-status.md
```

现有 `docs/docker_compose_db_redis_yaml/docker-compose.yml` 保留，并由入门、缓存和数据库文档按需链接。

## 页面职责

### 文档入口与入门

- `docs/README.md`：文档总索引、推荐阅读路线、状态标记说明和专题导航。
- `docs/getting-started.md`：仓库示例运行、独立应用安装、配置目录、最小装配、Fiber/Gin 选择和常见启动问题。

### 核心概念

- `docs/concepts/architecture.md`：代码结构、主要组成、依赖方向、Web 与 CLI 两种运行形态，以及框架实现与应用实现的边界。
- `docs/concepts/startup-lifecycle.md`：`FiberHouse.New`、bootstrap、Frame/Core Starter 创建、所有已接入 Location 的执行顺序，以及运行和关闭阶段。
- `docs/concepts/provider-system.md`：Provider、Manager、Location、Type、Target、状态、注册与动态选择，默认集合和扩展规则。
- `docs/concepts/context-and-locators.md`：`IContext`、`IApplicationContext`、`ICommandContext`、`AppContext`、Api/Service/Repository Locator、Starter 回指和 GlobalManager 访问关系。

### 使用指南

- `docs/guides/configuration.md`：`BootConfig` 与 `AppConfig`、配置目录、`APP_ENV_`、环境文件、`APP_CONF_` 的加载与覆盖顺序、typed getter、启动期写与运行期只读约束。
- `docs/guides/logging.md`：bootstrap logger、zerolog、日志 Origin、控制台/文件输出、lumberjack、channel/diode writer、轮转与关闭语义。
- `docs/guides/web-runtime.md`：Fiber/Gin Core Starter、HTTP server 初始化、核心中间件、context adaptor、JSON codec 选择、监听和优雅关闭差异。
- `docs/guides/response-and-serialization.md`：`IResponse`、`RespInfo`、Response facade、HTTP status 与业务 code、JSON/MsgPack/Protobuf、内容协商和对象池所有权。
- `docs/guides/errors-and-recovery.md`：Exception、ValidateException、Fiber 返回 error 与 Gin `c.Error` 的差异、panic 分类、RecoverConfig、trace/request 信息、敏感 header 脱敏和生产环境信息隐藏。
- `docs/guides/global-manager.md`：注册、延迟初始化、单例访问、健康检查、keepalive、重建、释放、清理和并发/生命周期限制。
- `docs/guides/cache.md`：Cache 接口、Local/Redis/L2、CacheOption、TTL、序列化、read-through、回填、同步策略、singleflight、Bloom filter、circuit breaker、指标和关闭语义。
- `docs/guides/command-line.md`：CommandStarter、Web/CMD 共用与独立上下文、命令注册、全局 option、运行流程、错误处理和资源回收边界。
- `docs/guides/background-tasks.md`：TaskRegister、TaskDispatcher、TaskWorker、handler map、容器注册、启动开关、运行模式和关闭边界。
- `docs/guides/database.md`：MySQL/GORM、MongoDB、连接配置、全局注册、模型基类、连接池、MongoDecimal 和资源生命周期。
- `docs/guides/validation.md`：validate wrapper、内置语言、自定义 validator/tag/translation、验证错误转换和启动期注册约束。
- `docs/guides/extending-fiberhouse.md`：新增 Provider/Manager/Location、核心引擎、JSON codec、响应协议、中间件与路由注册器的最小契约和验证清单。

### 参考与成熟度

- `docs/reference/examples.md`：`example_main`、`example_config`、`example_application` 的导航、运行入口、演示范围和已知不完善处。
- `docs/reference/components.md`：bufferpool、Dig container、jsonconvert、mongodecimal、writer、tasklog 等内部或辅助组件的用途、入口、调用方和状态。
- `docs/reference/feature-status.md`：按“已接入、实验性、内部工具、预留/占位”分类所有能力；插件、RPC、MQ、i18n、未使用 Location 和未完成选项在此说明。

## 需要明确记录的实现边界

- 默认 Web 核心为 Fiber，默认流量 JSON codec 为 Sonic；Fiber 与 Gin 的初始化和错误传播不是完全对称实现。
- JSON codec 选择属于启动期 HTTP 引擎配置；Response facade 的 JSON/MsgPack/Protobuf 协商是另一套机制。
- `ICoreContext` 只统一少量 header、JSON 和字节发送能力，路由参数和请求绑定仍使用原生 Fiber/Gin API。
- Provider 依赖 Type、Target、Manager 和 Location 共同完成匹配与执行，默认 Provider 并不表示所有能力会自动启用。
- GlobalManager、AppConfig 和 bootstrap 包含进程级单例或启动期可写语义，文档不能暗示任意运行时修改是安全的。
- Response、context adaptor 和部分实现使用对象池，文档必须说明获取、发送和释放的所有权边界。
- 示例 Swagger 状态码、示例生产配置和实际错误映射可能存在差异，文档以当前处理代码为准。
- Gin TLS、部分缓存保护策略、插件/RPC/MQ/i18n 等能力不得描述为完整生产能力。

## 证据与编写流程

每个页面编写前执行以下流程：

1. 先检查 `.codegraph-qa-out/` 中相关既有分析。
2. 使用 CodeGraph 查询入口符号、调用路径、动态派发和影响面。
3. 读取 CodeGraph 返回的当前源码；只对缺失部分使用精确文件读取或 `ast-grep`/`rg` 补充核对。
4. 对文档中的命令、路径、配置键、常量、默认值和代码片段进行二次源码核对。
5. 对涉及运行行为的关键声明，使用现有测试、定向测试、构建或最小可运行检查验证。
6. 将仍无法运行验证的结论标为源码静态分析结果，不写成已实测保证。

## 验证标准

- 根 README 和所有新页面不存在失效的仓库内相对链接。
- 文档中的源码路径在当前仓库存在；引用的公开标识符可由 CodeGraph、AST 或 Go 工具定位。
- 安装、构建和测试命令与当前 Go 版本、模块路径和目录结构一致。
- 至少验证 README 的快速体验命令和主要 Go 代码片段。
- 执行 Markdown 基础检查：标题层级、代码围栏、尾随空白和重复链接。
- 执行 `git diff --check`。
- 在可用环境中执行 `go build ./...` 和 `go test ./...`；若存在仓库既有失败，记录基线并确认文档变更没有引入新的失败。
- 搜索旧路径、已迁移名称和被删除的 README 章节内容，避免继续传播过时信息。

## 非目标

- 不修改框架运行时代码、配置语义或示例业务逻辑。
- 不修复审计中发现的缓存、对象池、Gin TLS、错误映射或生命周期问题。
- 不把示例应用补齐为生产模板。
- 不为尚未实现的插件、RPC、MQ 或 i18n 设计假想 API。
- 不复制 GoDoc 可以直接生成的逐方法 API 清单。

## 实施策略

实施按主题分批进行，而不是一次性生成全部文档：

1. 先建立文档索引、能力状态和核心概念，锁定术语与边界。
2. 重写 README 和入门指南，使入口与概念文档一致。
3. 编写 Web 运行时、响应、异常恢复、配置日志和全局管理等主路径指南。
4. 编写缓存、CLI、任务、数据库、验证与扩展指南。
5. 最后补齐示例和组件参考，执行全局交叉链接、事实与命令验证。

实施阶段使用子代理驱动模式：每个任务由独立实现代理编写，随后执行规格符合性与文档质量审查；主代理负责跨页术语、链接、重复内容和源码事实的一致性。
