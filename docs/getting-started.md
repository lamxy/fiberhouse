# 入门指南

本页先让仓库内的完整 Fiber 示例运行起来，再说明怎样把 FiberHouse 接入自己的 Go module。示例同时装配 MongoDB、Redis、MySQL、缓存、任务、验证器和业务路由，适合观察调用链，但不是最小应用或生产模板。

## 准备环境

- Go `1.25.0`；仓库版本以 [`go.mod`](../go.mod) 为准。
- Git，用于克隆仓库；也可以下载源码归档。
- Docker 与 Docker Compose，用于按示例配置启动 MongoDB、Redis、MySQL。
- `curl` 或其他 HTTP 客户端，用于检查路由。

只构建框架 package 时不需要外部服务。按原样运行 [`example_main/main.go`](../example_main/main.go) 时，示例会把 MongoDB、Redis、两个 Sonic codec 和 MySQL 列为启动必需实例，因此三种服务都应可用。

## 获取代码或作为依赖安装

体验仓库内示例时克隆源码：

```bash
git clone https://github.com/lamxy/fiberhouse.git
cd fiberhouse
go mod download
```

在自己的项目中使用时，新建 Go module 并增加依赖：

```bash
mkdir my-service
cd my-service
go mod init example.com/my-service
go get github.com/lamxy/fiberhouse
```

仓库内示例与框架使用同一个 module，包含用于演示的业务 package；外部项目不应导入 `example_application` 来代替自己的应用实现。

## 启动示例依赖

从 FiberHouse 仓库根目录执行：

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml up -d
```

Compose 会启动：

| 服务 | 主机端口 | 示例凭据或用途 |
|---|---:|---|
| MongoDB | `27037` | `admin` / `admin` |
| Redis | `6379` | 无密码，DB 0 |
| MySQL | `3306` | `root` / `root` |

[`application_dev.yml`](../example_config/application_dev.yml) 的 MySQL DSN 指向 `test` 数据库，而 Compose 文件只设置 root 密码。启动后需要显式创建数据库：

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml \
  exec mysql mysql -uroot -proot \
  -e 'CREATE DATABASE IF NOT EXISTS test CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;'
```

这些容器与明文凭据只用于本地示例。生产环境需要重新设计镜像版本、认证、网络、TLS、持久化、迁移、容量和备份。

## 配置选择与环境覆盖

`fiberhouse.New` 在 `RunServer` 之前读取 `BootConfig.ConfigPath`。标准引导顺序是：

```text
APP_ENV_ → 选择 application_<env>.yml → YAML → 回写 application.env → APP_CONF_
```

示例 `ConfigPath` 是 `./example_config`，默认环境选择为 `dev`。在 POSIX shell 中可以显式指定：

```bash
export APP_ENV_application_env=dev
```

`APP_ENV_` 用于选择环境文件；`APP_CONF_` 在 YAML 之后加载，适合覆盖配置：

```bash
export APP_CONF_application_server_port=8081
export APP_CONF_application_appLog_level=info
```

去掉前缀后，每个 `_` 会替换为 `.`，且键名大小写敏感。例如 `application_appLog_level` 映射到 `application.appLog.level`，写成 `application_applog_level` 不是同一个键。`APP_CONF_application_env` 只会改变最终可读值，不会重新选择已经加载的 YAML 文件。

配置、日志和默认 Context 都是进程级单例：第一次初始化会固定路径和当时的环境变量。不要在同一进程中先启动一套配置，再期待修改环境变量后得到隔离的新应用。

## 启动 Web 示例

确认三个容器可用并已创建 MySQL `test` 数据库后，从仓库根目录执行：

```bash
APP_ENV_application_env=dev go run ./example_main/main.go
```

示例入口选择：

- `CoreTypeWithFiber`；
- `TrafficCodecWithSonic`；
- `ConfigPath: "./example_config"`；
- `LogPath: "./example_main/logs"`；
- 二进制响应支持开启。

示例 dev 配置默认把日志写入文件而不是 console。没有终端输出并不一定表示进程未启动，可同时检查 `example_main/logs/app.log` 和健康路由。

另开终端请求 Fiber liveness：

```bash
curl http://localhost:8080/health/livez
```

该路由返回统一成功结构。它只证明 Fiber 进程与这条 handler 链可达，不是所有数据库、缓存、worker 和后台 goroutine 的完整 readiness 证明。

停止应用后清理示例容器：

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml down
```

## 切换 Fiber 与 Gin

自己的应用通过 `BootConfig.CoreType` 选择一个 HTTP 内核：

```go
CoreType: constant.CoreTypeWithFiber
// 或
CoreType: constant.CoreTypeWithGin
```

切换 CoreType 还必须同时满足：

1. 默认或自定义 Provider 集合中存在目标 CoreStarter 与匹配的 JSON codec Provider；
2. 应用为该内核注册对应中间件和路由；
3. handler 使用该内核的原生 Context 与错误传播方式；
4. 配置中存在目标内核实际读取的监听参数。

仓库示例已经把 Fiber/Gin 的 core、codec、recovery、中间件和路由 Provider 都放入集合，因此可把 [`example_main/main.go`](../example_main/main.go) 的 `CoreType` 改为 `constant.CoreTypeWithGin` 观察选择行为。Gin 示例没有注册 `/health/livez`，可改用：

```bash
curl http://localhost:8080/gin/example/hello/world
```

Fiber handler 通过返回 `error` 进入统一错误链；Gin handler 需要调用 `c.Error(err)`。Gin 当前属于实验性能力，默认 TLS 分支没有形成可工作的 HTTPS 链路；完整差异见[Web 运行时](guides/web-runtime.md)。

## 外部项目需要完成的装配

根目录 [README 的应用装配骨架](../README.md#应用装配骨架)只展示调用关系。一个可运行应用至少要明确以下责任：

1. 创建完整 `BootConfig`。直接调用 `New` 不会补齐 `Default()` 的字段，也不会为缺失路径回退。
2. 实现 `ApplicationRegister`，声明 GlobalManager initializer、必需 key、校验扩展、应用中间件与 hook。
3. 实现 `ModuleRegister`，注册所选 HTTP 内核的模块路由，并按需注册 Swagger。
4. 需要后台任务时实现 `TaskRegister`，提供 handler map、worker/dispatcher initializer 和实例 getter；不使用任务时关闭配置并不要注入任务注册器。
5. 用 `FrameStarterOption` 把注册器交给 Frame Starter，或者实现等价的 option Provider/Manager。
6. 显式传入 `DefaultProviders()` 与 `DefaultPManagers(ctx)`，并追加应用自己的中间件、路由、hook 等 Provider/Manager。
7. 为数据库、缓存、task client、worker、日志 writer 与其他 goroutine 指定唯一所有者和停止顺序。

`ApplicationRegister`、`ModuleRegister`、`TaskRegister` 不是框架扫描目录后生成的对象，它们来自你的应用实现。仓库示例在 [`frame_option_provider.go`](../example_application/providers/optioninit/frame_option_provider.go) 中创建三种注册器，再由 option manager 把 `[]FrameStarterOption` 放入启动链；外部应用也可以直接调用 `WithFrameStarterOptions(...)`。

最小配置目录至少要包含被所选环境指向的文件，例如 `config/application_dev.yml`：

```yaml
application:
  env: dev
  server:
    host: 0.0.0.0
    port: 8080
  appLog:
    enableConsole: true
    enableFile: false
    level: info
  recover:
    debugMode: false
    enablePrintStack: false
    enableDebugFlag: false
  validate:
    langFlags: [en]
  globalManage:
    keepAlive: false
  task:
    enableServer: false
  swagger:
    enable: false
```

这只是配置加载与基础 Web 运行所需形状。业务中间件、数据库、缓存和任务的键应按实际采用的组件补充；不要复制示例中未被源码消费的字段。

## 示例依赖是注册还是立即连接

示例的 `ConfigGlobalInitializers()` 先为 MongoDB、MySQL、Redis、JSON codec、本地缓存和 L2 缓存等对象注册延迟 initializer。随后 `ConfigRequiredGlobalKeys()` 把 MongoDB、Redis、两个 Sonic codec 和 MySQL 列为启动必需 key，Frame Starter 会在启动期逐项 `Get`。

因此对这些 required key 而言，示例不是“只登记名称，等首个请求再连接”。但底层构造语义并不相同：MySQL 构造会 ping，MongoDB 构造当前不主动 ping；资源是否真正可用仍要结合对应 client 的构造与健康检查理解。required key 初始化失败目前主要记录 Error 后继续，后续使用处仍可能失败或 panic，正式应用应自行决定并实现 fail-fast。

如果你的应用不使用 MongoDB、MySQL、Redis 或任务，不需要照搬示例的 initializer 和 required key。框架本身不会因为调用 `New` 或默认 Provider 集合而创建这些资源。

## 常见启动失败

### 找不到配置文件

典型现象是 `LoadYaml` panic，并显示 `application_<env>.yml` 路径。检查：

- 命令是否从预期工作目录运行；
- `BootConfig.ConfigPath` 是否正确；
- `APP_ENV_application_env` 是否选择了实际存在的文件；
- 环境变量键名大小写是否与 YAML 一致。

### Frame/Core Starter 或 codec 创建失败

如果只调用 `Default()` 或 `New()` 后直接 `RunServer()`，默认 Provider/Manager 集合并不会自动装入。确认同时调用了：

```go
WithProviders(fiberhouse.DefaultProviders().List()...)
WithPManagers(fiberhouse.DefaultPManagers(appCtx).List()...)
```

还要提供 Frame Starter 所需的应用注册器；缺失 `ApplicationRegister` 会在全局初始化阶段 panic。自定义 `TrafficCodec` 必须同时有 Version 与 Core target 匹配的 Provider，以及 `GroupTrafficCodecChoose` Manager。

### 中间件或路由没有运行

把 Provider 放进集合只完成了 Type 分发。对应 Manager 还要绑定到一个被读取的 Location，并由 `ApplicationRegister` 或 `ModuleRegister` 调用 `LoadProvider`。自定义 Location 没有通用自动调度器。

### 外部服务连接失败

确认容器状态、端口、认证、MySQL `test` 数据库和本机端口占用。示例 required key 失败不一定立即终止整个启动链，应同时查看日志，并把正式应用的必要依赖校验升级为明确的启动失败。

### 健康 URL 不可用

- Fiber 示例使用 `http://localhost:8080/health/livez`。
- Gin 示例没有该路由，使用 `http://localhost:8080/gin/example/hello/world` 检查基础调用链。
- 若通过 `APP_CONF_application_server_port` 改了端口，请同步修改 URL。

## 下一步

- 先读[架构总览](concepts/architecture.md)理解 Starter、注册器与扩展系统。
- 用[配置与引导](guides/configuration.md)核对部署配置与环境覆盖。
- 用[功能状态](reference/feature-status.md)确认计划采用的组件是否仍有实验性限制。
- 需要跟踪示例业务层时阅读[示例目录](reference/examples.md)；数据库 CRUD 与 Wire 生成细节保留在那里和源码中，不在本入门页展开。
