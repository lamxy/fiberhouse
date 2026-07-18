# 示例目录

三个示例目录用于展示装配方式和调用路径。完整流程依赖可选基础设施，且保留了不完整分支；它们既不是生产模板，也不是稳定 API 或 API 兼容性承诺。

## 三个目录分别演示什么

- `example_main/`：Web 可执行入口、默认 provider/manager 与应用扩展的合并，以及 Swagger 生成产物的导入。
- `example_config/`：dev/test/prod 配置样例、环境选择和覆盖键，以及 HTTP、日志、缓存、数据库、任务、验证、CLI 等配置形状。
- `example_application/`：应用、模块、任务注册器，Fiber/Gin 中间件和路由 provider，验证扩展，以及 Web/CLI 共享的数据库与缓存调用示例。

这些目录共同说明“如何接线”，不说明所有配置项背后都已有实现。尤其是 `plugins`、`mq`、`rpc` 配置节点不能作为对应运行时能力已经完成的证据。

阅读时应从可执行入口向下跟踪，而不是从最深层的 service 或 model 反推它一定会运行。当前目录中既有可达演示，也有尚未挂到入口的辅助分支。

示例把框架扩展点和业务代码放在同一仓库，是为了展示接口关系；正式应用可采用不同目录结构，只要保持注册器、provider/manager 与资源生命周期契约。

## Web 示例装配入口

入口是 `example_main/main.go`，调用链可概括为：

1. 使用 `fiberhouse.New(&fiberhouse.BootConfig{...})` 创建上下文、加载 `./example_config` 并初始化日志。
2. 通过 `DefaultProviders().AndMore(...)` 收集框架 provider，以及示例的 Fiber/Gin 中间件、Fiber hook 和两种内核的路由 provider。
3. 通过 `DefaultPManagers(fh.AppCtx).AndMore(...)` 收集默认 manager，以及示例的 option、hook、中间件和路由 manager。
4. 调用 `WithProviders(...).WithPManagers(...).RunServer()`；option provider 在启动期创建 `ApplicationRegister`、`ModuleRegister` 与 `TaskRegister`。

当前 `BootConfig.CoreType` 明确选择 Fiber。虽然集合里也放入 Gin provider，它们只用于演示 target 选择，当前这次运行不会同时启动 Gin。框架或应用 provider 缺失、类型不匹配、必需全局实例初始化失败时，路径中既有返回错误和日志，也有 fatal/panic；不要把示例的乐观类型断言直接复制到边界不可信的业务代码。

启动顺序还会经过全局实例初始化、core 初始化、hook、中间件、模块路由、Swagger、任务、keepalive、server run 与 shutdown。某个 provider 只有在类型、target 和 location 都匹配时才会参与对应阶段。

`DefaultProviders()` 和 `DefaultPManagers(ctx)` 是进程级集合。示例在启动期一次性合并它们；不要在服务已开始处理请求后继续调用 `Add` 或 `Except` 重配共享集合。

示例的 `RegisterFiberAppCoreHook` shutdown hook 只记录一条日志。清空 GlobalManager 并关闭 logger 的是 `CoreWithFiber.RegisterAppHooks` 另行注册的内建 shutdown hook：它调用 `ClearAll(true)` 后调用 logger `Close`。`ClearAll(true)` 不等于逐一调用资源的 `Close`，数据库、缓存和任务仍需要应用设计可验证的停止顺序；在该内建 hook 关闭 logger 之前也必须停止日志生产者。

## 配置示例

`example_config/application_dev.yml` 等文件展示配置形状；`APP_ENV_application_env` 选择 `application_<env>.yml`，`APP_CONF_` 前缀可覆盖具体路径。键名大小写和路径拼接必须与配置加载器规则一致。

Web 示例的 `ConfigRequiredGlobalKeys` 会在启动期请求 MongoDB、Redis、两个 Sonic codec 和 MySQL，因此按原样运行完整流程需要这些外部服务和有效凭据。`application.task.enableServer` 还决定是否注册 asynq worker/dispatcher。示例 DSN、端口、调试开关和日志路径只适合本地理解，部署前必须按环境重新设计密钥管理、超时、容量、TLS、调试输出和关闭策略。

配置及日志由进程级单例初始化；同一进程内用多个示例配置反复初始化，不能假设会得到彼此隔离的新实例。配置应在启动期完成，运行期只读。

`application.validate.langFlags` 展示 en、zh-CN 和 zh-TW，应用注册器再追加示例语言与 tag。追加动作同样应发生在启动期，避免请求并发阶段修改验证器内部 map。

配置中的 `application.plugins.engine.servers.gin.tls` 已接入证书加载和 TLS listener：有效证书/私钥会启用 HTTPS，无效的非空路径会 fail-stop；缺失任一路径时仍只记录错误并保留 HTTP 路径。配置形状不等于部署保证，正式环境还应校验证书配置并完成真实握手验证。

缓存保护、L2 异步池和 keepalive 参数也需要结合对应实现理解。示例值不能替代容量评估，开关为 true 更不代表保护和资源回收语义已经完整。

## CLI 示例入口

CLI 入口是 `example_application/command/main.go`。它从相对路径 `./../../example_config` 初始化配置，创建 `CmdContext`、命令应用注册器、`FrameCmdApplication` 与 `CoreCmdCli`，最后调用 `commandstarter.RunCommandStarter`。相对路径以进程工作目录为准，运行位置不对会导致配置加载失败。

当前注册命令主要是 `test-orm`，用 Dig 组装 MySQL model/service 并执行演示操作。CLI 应用仍会预初始化 MongoDB、Redis、Sonic codec 和 MySQL，因此查看帮助或执行单一命令也可能依赖这些服务。`RunCommandStarter` 在 core 已记录错误后忽略其返回值，且没有统一资源关闭阶段；调用方若据此构建正式 CLI，应自行保留退出码和回收语义。

`test-orm` 会先向单例 Dig 容器注册 context、model 与 service，再执行 `Invoke`。重复执行或把同一进程用于多套装配时，应处理重复 provider 和错误切片，而不是假设容器会自动重置。

命令 service 中存在比入口实际调用更多的 CRUD 方法。它们可用于阅读数据访问写法，但零调用的方法不能算成已验证的 CLI 功能。

MongoDB command service 与 cron wrapper 当前没有入口。若要采用，应用需要自行补齐命令注册、配置选择、错误传播和关闭，不应从文件存在推断其会自动执行。

## 可以借鉴的部分

- 从默认集合合并应用 provider/manager，并用 type、target 与 location 控制执行阶段。
- 用应用注册器集中声明 GlobalManager initializer 和启动必需 key，同时让业务模块只依赖接口 key。
- 将模块路由、应用中间件、自定义验证器和任务 handler 分开注册，再由启动器编排。
- 在同一接口下为 Fiber 与 Gin 提供各自适配器，并明确一次启动只选择一个 target。
- 在 CLI 启动期用 Dig 完成依赖组装，随后以已构造实例执行命令。

- 用 `ConfigRequiredGlobalKeys` 区分“已注册 initializer”和“启动必须可用的实例”，再按应用真实依赖缩减必需集合。

- 把可选基础设施放在明确开关后，并在开关启用时做启动期连通性和配置校验。

借鉴这些结构时，应保留错误返回、配置校验和资源所有权，不应复制示例中的固定地址、强制类型断言、调试配置或全局单例重置方式。

应用可保留相同接口而替换目录组织、依赖注入方式和配置来源。真正需要兼容的是当前导出接口和调用契约，而不是示例路径或文件名。

## 已知不完善处

- `CoreOptionInitProvider` 返回空 option 列表，更多 core 定制尚未在示例落地。
- Web 路径把 MySQL、MongoDB 和 Redis 都列为启动必需项；这体现调用链，不是最小应用要求。
- Gin TLS 已接通有效证书加载与 HTTPS 启动，但缺失路径仍会记录错误并保留 HTTP 路径，且尚无真实握手集成验证；示例 TLS 节点不能直接视为生产部署保证。
- CLI 的 MongoDB service、cron wrapper 和若干 command/module 目录没有可达入口，MySQL service 也保留许多未被命令调用的方法。
- `component/codec/json/gojson.go`、通用 i18n/MQ/RPC 目录以及 plugins loader/registry 没有完整实现；配置或常量名称不改变这一状态。
- 二进制响应只展示基于 MIME type 的 HTTP 响应选择，不包含 RPC server 生命周期。
- 任务异步启动、GlobalManager keepalive、日志 writer、缓存/数据库连接的停止顺序没有在示例中形成统一关闭编排。

- provider 初始化失败的处理方式并不统一：有的返回错误，有的记录日志，有的 panic 或 fatal；正式应用需要在入口统一失败策略。

- 示例中的异步 worker、keepalive 与日志 goroutine 没有共同的 context 取消树，不能据此证明进程退出时所有后台工作都已结束。

- 配置样例包含本地地址、明文凭据形状和调试选项，只能作为字段说明，不能直接进入共享或生产环境。

因此，示例适合跟踪入口、装配顺序和接口关系，不应直接作为生产模板，也不应把其中的目录布局、配置键或辅助函数当作稳定 API。
