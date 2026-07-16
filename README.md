# FiberHouse

FiberHouse 是一个面向 Go Web 应用的装配式框架：用统一的启动链连接配置、日志、HTTP 内核、Provider 扩展、业务注册器和共享资源。它默认选择 Fiber，也提供 Gin 适配；数据库、缓存、任务和业务模块由应用显式接入。

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## FiberHouse 是什么

FiberHouse 解决的是“怎样把一个 Web 应用的组成部分按确定顺序接起来”，而不是替业务生成一套固定目录。它提供：

- `FiberHouse`、`FrameStarter`、`CoreStarter` 组成的启动主链；
- Provider / Manager / Location 扩展机制；
- 应用、模块和任务注册器；
- 进程级配置、日志、Context 与 `GlobalManager`；
- Fiber/Gin、统一响应、错误恢复、校验，以及可选的数据库、缓存、后台任务和 CLI 组件。

业务应用仍负责实现注册器、声明必需依赖、注册中间件与路由，并为连接、worker、缓存和日志 writer 设计关闭顺序。框架不会自动发现业务 package，也不会因目录或配置键存在就启用功能。

## 当前状态

FiberHouse 仍在演进中。当前源码中的能力按以下成熟度理解：

| 范围 | 状态 | 说明 |
|---|---|---|
| Fiber HTTP、Provider 主链、配置与日志、JSON 响应、校验 | 已接入 | 有可达实现；仍需应用完成装配 |
| Gin、GlobalManager 生命周期、L2 缓存、任务、CLI、数据库、二进制响应 | 实验性 | 可运行，但错误、并发或关闭边界仍需应用补齐 |
| buffer pool、Dig、writer、转换器 | 内部工具 | 主要服务于框架或示例，不承诺稳定公共抽象 |
| plugins、RPC、MQ、通用 i18n | 预留/占位 | 尚无完整创建、运行和关闭链 |

完整判断依据与限制见[功能状态](docs/reference/feature-status.md)。`example_main`、`example_config`、`example_application` 只用于展示调用路径，不是生产模板或稳定 API。

## 核心能力

- 可在同一启动模型中选择 Fiber 或 Gin HTTP 内核。
- 按 Type 分发 Provider，由 Manager 决定选择和执行规则，由 Location 标识生命周期入口。
- 使用 `ApplicationRegister`、`ModuleRegister`、`TaskRegister` 把应用能力接入 Starter。
- 从 YAML 与环境变量构建应用配置，并用 zerolog 输出 console 或轮转文件日志。
- 通过 Context、Locator 与 `GlobalManager` 访问已注册的共享实例。
- 提供统一 `{code,msg,data}` 响应、panic recovery、参数校验和可选 MsgPack/Protobuf body。
- 提供 MySQL、MongoDB、Redis、本地/L2 缓存、asynq 任务及 urfave/cli 的可选集成。

## 环境要求

- Go `1.25.0`（以 [`go.mod`](go.mod) 为准）。
- 只阅读或构建框架时不要求数据库与 Redis。
- 按原样运行完整 Web 示例时需要 Docker 与 Docker Compose，或自行提供可用的 MongoDB、Redis、MySQL。
- 示例 MySQL DSN 指向 `test` 数据库；Compose 只启动 MySQL 服务，不会自动创建该数据库。

## 五分钟体验

以下命令在仓库根目录执行，启动完整 Fiber 示例：

```bash
git clone https://github.com/lamxy/fiberhouse.git
cd fiberhouse
go mod download

docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml up -d
until docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml \
  exec -T mysql mysqladmin ping -uroot -proot --silent; do sleep 2; done
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml \
  exec -T mysql mysql -uroot -proot \
  -e 'CREATE DATABASE IF NOT EXISTS test CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;'

APP_ENV_application_env=dev go run ./example_main/main.go
```

另开终端检查 Fiber liveness：

```bash
curl http://localhost:8080/health/livez
```

示例会在启动期请求 MongoDB、Redis、两个 Sonic codec 和 MySQL；这里只注册 initializer 不够，`ConfigRequiredGlobalKeys()` 会主动读取这些实例。结束服务后可停止容器：

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml down
```

配置选择、Gin 切换和常见错误见[入门指南](docs/getting-started.md)。

## 应用装配骨架

下面是结构骨架，不是可直接编译的完整程序。`newApplicationRegister`、`newModuleRegister`、`newApplicationProviders` 与 `newApplicationManagers` 代表必须由应用实现的装配；为缩短示例而省略其代码，不表示可以省略这些职责。完整接线可对照 [`example_main/main.go`](example_main/main.go)，但不要直接把示例 package 当成稳定依赖。

```go
package main

import (
	fh "github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/option"
)

func main() {
	house := fh.New(&fh.BootConfig{
		AppName:      "my-service",
		Version:      "0.1.0",
		FrameType:    constant.FrameTypeWithDefaultFrameStarter,
		CoreType:     constant.CoreTypeWithFiber,
		TrafficCodec: constant.TrafficCodecWithSonic,
		ConfigPath:   "./config",
		LogPath:      "./logs",
	})

	// 这些注册器由业务应用实现；至少要提供应用初始化与模块路由职责。
	frameOptions := []fh.FrameStarterOption{
		option.WithAppRegister(newApplicationRegister(house.AppCtx)),
		option.WithModuleRegister(newModuleRegister(house.AppCtx)),
	}

	// 应用的中间件、路由、hook 等 Provider/Manager 也要显式加入。
	providers := fh.DefaultProviders().AndMore(
		newApplicationProviders(house.AppCtx)...,
	)
	managers := fh.DefaultPManagers(house.AppCtx).AndMore(
		newApplicationManagers(house.AppCtx)...,
	)

	house.
		WithFrameStarterOptions(frameOptions...).
		WithProviders(providers...).
		WithPManagers(managers...).
		RunServer()
}
```

如果应用不采用 Provider 化的中间件或路由，可在注册器回调中直接调用原生引擎 API；无论采用哪种方式，业务注册器、路由接线和资源初始化都必须存在。`Default()` 也不会替你调用 `DefaultProviders()` 或 `DefaultPManagers(ctx)`。

## 核心模型

```text
BootConfig ──> FiberHouse.New ──> AppContext
                                  │
Provider ──Type──> Manager ──Location──> RunServer
                                  │
                FrameStarter + CoreStarter
                       │              │
        application/module/task    Fiber 或 Gin
             registrars               │
        globals / validation       middleware / routes
```

- `BootConfig` 选择 Frame、Core、JSON codec 以及配置/日志目录。
- Context 提供配置、日志、容器、校验器与 Starter 回指；它不是请求 context。
- Frame Starter 负责应用注册器、共享实例、校验、任务和 keepalive。
- Core Starter 负责 HTTP 引擎、中间件、路由、监听信号与停止。
- Provider 描述能力，Manager 定义加载算法，Location 只有被生命周期代码读取时才会执行。

设计细节见[架构总览](docs/concepts/architecture.md)、[Provider 系统](docs/concepts/provider-system.md)和[Context 与 Locator](docs/concepts/context-and-locators.md)。

## 启动主链

一次标准 Web 启动可以概括为：

```text
New：配置 → 日志 → AppContext
RunServer：分发 Provider → 取得 Options → 创建 Frame/Core Starter
         → 注册 globals → 初始化 HTTP core → hook → middleware
         → routes → Swagger → task → keepalive → listen → shutdown
```

`RunServer` 不返回 `error`，现有阶段对错误的处理可能是记录、忽略、fatal 或 panic。任务和 keepalive 还可能在 HTTP 监听前启动 goroutine。需要逐阶段行为时阅读[Web 启动生命周期](docs/concepts/startup-lifecycle.md)。

## 请求与响应主链

```text
HTTP 请求 → Fiber/Gin recovery 与错误入口 → 应用中间件 → 模块路由
          → API → Service → Repository / 外部资源
          → {code,msg,data} → JSON 或可选二进制 body
```

HTTP status 与响应中的业务 `code` 是两个维度。`BootConfig.TrafficCodec` 选择引擎 JSON codec；`EnableBinaryProtocolSupport` 只控制统一响应是否尝试 MsgPack/Protobuf 协商。详见[响应与序列化](docs/guides/response-and-serialization.md)和[错误与恢复](docs/guides/errors-and-recovery.md)。

## Fiber 与 Gin

| 维度 | Fiber | Gin |
|---|---|---|
| `CoreType` | `constant.CoreTypeWithFiber` | `constant.CoreTypeWithGin` |
| Handler | `func(*fiber.Ctx) error` | `func(*gin.Context)` |
| 普通错误 | 返回 `error` | 调用 `c.Error(err)` |
| JSON codec 作用域 | 单个 `fiber.App` | 修改 Gin package 级 codec |
| 当前状态 | 已接入 | 实验性；TLS 链路未完成 |

两种内核共享启动抽象，但路由、绑定和 handler 仍使用各自原生 API。示例默认运行 Fiber，且 `/health/livez` 只在 Fiber 路由中注册。详细差异见[Web 运行时](docs/guides/web-runtime.md)。

## 示例目录

- [`example_main/`](example_main/)：Web 可执行入口与完整 Provider/Manager 合并。
- [`example_config/`](example_config/)：dev/test/prod 配置形状与环境覆盖示例。
- [`example_application/`](example_application/)：应用、模块、任务、Fiber/Gin 路由与 CLI 接线。

示例包含固定凭据、调试选项、未接线分支和不完整关闭流程。请把它当作源码导航，不要当作可直接部署的模板；参见[示例目录说明](docs/reference/examples.md)。

## 文档导航

- [完整文档索引](docs/README.md)
- [入门指南](docs/getting-started.md)
- [架构总览](docs/concepts/architecture.md)
- [组件目录](docs/reference/components.md)
- [功能状态](docs/reference/feature-status.md)

## 开发与验证

常用命令从仓库根目录执行：

```bash
go mod download
go build ./...
go test ./...
go vet ./...
```

当前 `go test ./...` 的已知失败集中在 `bootstrap` 与 `component/logging/writer` 两个 package；分层命名空间迁移只移动 writer package，没有修改或修复这些测试。writer 的失败 case 数量会受异步写入时序影响，不宜作为固定基线。

`Makefile` 还提供 `build`、`lint` 和交叉构建目标，但使用前应先核对其目标路径与本机工具。示例需要外部服务的运行验证与纯框架构建是两件事。

## 贡献

提交变更时请保持范围清晰，说明行为、兼容性与验证结果；涉及启动、并发或资源生命周期的修改应补充相应测试和文档。问题与建议可通过 [GitHub Issues](https://github.com/lamxy/fiberhouse/issues) 反馈，安全问题请使用仓库维护者公布的私密联系方式。

## 许可证

FiberHouse 以 [MIT License](LICENSE) 发布。
