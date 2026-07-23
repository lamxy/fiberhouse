# FiberHouse 🏠

FiberHouse 是一个面向 Go Web 应用的装配式框架。它将配置、日志、HTTP 内核、Provider 扩展和业务注册统一纳入清晰的启动生命周期，使服务的创建、运行与关闭遵循一致的组织方式。框架默认使用 Fiber，也支持切换 Gin；数据库、缓存、任务等组件均由应用显式选择和装配。

框架的重点不是替代业务架构，而是提供可组合的启动器、可定位的扩展执行点，以及可复用的共享实例管理。当前实现进一步完善了 Provider 初始化幂等、扩展替代逻辑、运行错误传递和优雅关闭链，使 Fiber 与 Gin 能够在同一套生命周期模型下运行。

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Build & Test](https://github.com/lamxy/fiberhouse/actions/workflows/go1.yml/badge.svg)](https://github.com/lamxy/fiberhouse/actions/workflows/go1.yml)

## FiberHouse 解决什么问题

Go Web 服务通常需要分别处理配置加载、日志输出、中间件顺序、共享资源和关闭流程。这些基础能力如果缺少统一边界，往往会分散在入口、组件和业务代码中。FiberHouse 不负责生成业务目录，而是提供一条明确的装配路径：一套启动链（`FiberHouse` → `FrameStarter` → `CoreStarter`）、一套可插拔的扩展机制（Provider / Manager / Location），以及一套跨组件访问共享实例的方式（`GlobalManager`）。

路由、中间件、连接管理和资源所有权仍由应用负责。框架不会扫描目录，也不会根据配置自动启用未声明的组件，所有能力都通过显式注册进入启动链。

框架提供默认的框架级启动器（FrameStarter）和核心启动器（CoreStarter），两者都可以定制或替换。样例应用（`example_application`）展示了上层业务结构与接入方式，便于从可运行路径理解各类扩展点。

## 目前是什么状态

FiberHouse 仍在持续迭代，**目前尚未承诺可直接用于生产环境的稳定性**。Web 主链已经覆盖 Fiber/Gin、配置与日志、统一响应、参数校验、Provider 幂等初始化和信号关闭；`GlobalManager` 以及 MySQL、MongoDB、Redis、缓存、asynq 任务、CLI 等组件也已有明确接入路径，具体见下面的[核心能力](#核心能力)。

启动运行与关闭链已经具备专项回归测试，但统一资源所有权、完整并发契约、可恢复错误边界和外部依赖验证仍需继续完善，公开接口也可能调整。各项能力的成熟度和限制以[功能状态](docs/reference/feature-status.md)为准；`example_main`/`example_config`/`example_application` 用于展示调用路径，不应直接作为生产模板。

## 核心能力

- Fiber 与 Gin 共享同一套启动和关闭模型，并保留扩展其他 HTTP 内核的接口边界。
- Provider / Manager / Location：按类型和执行位点组织扩展，支持初始化状态缓存、同位点替代和重复加载保护：[Provider 系统](docs/concepts/provider-system.md)。
- `ApplicationRegister`、`ModuleRegister`、`TaskRegister` 三类注册器，把应用能力接入 Starter。
- YAML 与环境变量共同完成配置装配，zerolog 可输出到 console 或轮转文件。
- `GlobalManager` + `Context` + `Locator`，用于跨组件访问已注册的共享实例。
- 统一 `{code,msg,data}` 响应、panic recovery、参数校验，可选 MsgPack/Protobuf body。
- MySQL、MongoDB、Redis、本地/二级缓存、asynq 后台任务、基于 urfave/cli 的命令行子应用，按需接入。

## 环境要求

- Go `1.25.0`（以 [`go.mod`](go.mod) 为准）。
- 阅读代码或编译框架本身不需要数据库和 Redis。
- 运行完整 Web 示例需要 Docker + Docker Compose，或者自行准备 MongoDB、Redis、MySQL。
- 示例的 MySQL DSN 指向 `test` 库，Compose 只启动 MySQL 服务本身，不会自动建库。

## 五分钟跑起来

在仓库根目录执行以下命令，拉起完整的 Fiber 示例：

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

在另一个终端验证服务：

```bash
curl http://localhost:8080/health/livez
```

示例启动时会连接 MongoDB、Redis、MySQL 和两个 Sonic codec 实例。仅注册 initializer 并不足以跳过这些依赖，因为 `ConfigRequiredGlobalKeys()` 会主动读取它们。验证完成后停止容器：

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml down
```

配置选择、Gin 切换和常见问题见[入门指南](docs/getting-started.md)。

## 接入你自己的应用长什么样

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

	frameOptions := []fh.FrameStarterOption{
		option.WithAppRegister(newApplicationRegister(house.AppCtx)),
		option.WithModuleRegister(newModuleRegister(house.AppCtx)),
	}
	providers := fh.DefaultProviders().AndMore(newApplicationProviders(house.AppCtx)...)
	managers := fh.DefaultPManagers(house.AppCtx).AndMore(newApplicationManagers(house.AppCtx)...)

	house.WithFrameStarterOptions(frameOptions...).
		WithProviders(providers...).
		WithPManagers(managers...).
		RunServer()
}
```

`newApplicationRegister`、`newModuleRegister` 代表应用必须自行实现的接口职责，示例为控制篇幅省略了具体实现。完整装配可参照 [`example_main/main.go`](example_main/main.go)，但不应将 example package 作为稳定依赖直接 import。`Default()` 不会自动调用 `DefaultProviders()`/`DefaultPManagers(ctx)`，默认集合仍需由应用显式加入。

## 想深入了解框架内部

启动链、Provider 系统和请求处理流程的完整设计由专题文档说明：

- [架构总览](docs/concepts/architecture.md)：`BootConfig`、`Context`、`FrameStarter`/`CoreStarter` 各自的职责边界。
- [Provider 系统](docs/concepts/provider-system.md)：能力怎么被发现、加载、执行。
- [Context 与 Locator](docs/concepts/context-and-locators.md)：跨组件访问共享实例的方式。
- [Web 启动生命周期](docs/concepts/startup-lifecycle.md)：`RunServer` 从配置加载到监听关闭的完整阶段。
- [响应与序列化](docs/guides/response-and-serialization.md)、[错误与恢复](docs/guides/errors-and-recovery.md)：统一响应、JSON codec 选择、MsgPack/Protobuf 协商。

## Fiber 还是 Gin

| 维度 | Fiber | Gin |
|---|---|---|
| `CoreType` | `constant.CoreTypeWithFiber` | `constant.CoreTypeWithGin` |
| Handler 签名 | `func(*fiber.Ctx) error` | `func(*gin.Context)` |
| 普通错误怎么报 | 直接 `return err` | 调用 `c.Error(err)` |
| JSON codec 作用域 | 单个 `fiber.App` 内 | Gin package 级，全局生效 |
| 备注 | 默认内核，`example_main` 实际跑的路径 | 可切换；TLS 证书加载与真实握手都有回归测试覆盖 |

两种内核共享启动、运行错误传递和信号关闭抽象，但路由、绑定与 handler 仍使用各自的原生 API。示例默认运行 Fiber，`/health/livez` 也只注册在 Fiber 路由中。详细差异见[Web 运行时](docs/guides/web-runtime.md)。

## 示例目录

- [`example_main/`](example_main/)：Web 可执行入口，将 Provider/Manager 全部合并运行。
- [`example_config/`](example_config/)：dev/test/prod 三套配置形状，以及环境变量如何覆盖。
- [`example_application/`](example_application/)：应用、模块、任务、Fiber/Gin 路由、CLI 的完整接线示范。

示例中包含固定凭据、调试开关和用于展示边界的简化实现，适合作为源码导航，不适合作为部署模板直接使用。细节见[示例目录说明](docs/reference/examples.md)。

## 更多文档

- [完整文档索引](docs/README.md)
- [入门指南](docs/getting-started.md)
- [组件目录](docs/reference/components.md)
- [功能状态](docs/reference/feature-status.md)

## 开发与验证

仓库根目录下常用命令：

```bash
go mod download
go build ./...
go test ./...
go test -race ./... -count=1
go vet ./...
```

当前提交以 `go test ./... -count=1`、`go test -race ./... -count=1` 和 `go vet ./...` 作为基础验证，其中 vet 是独立的静态检查门禁。验证结果只对应当前提交，历史版本应以相应的 Git tag 和 release note 为准。

CI 在 GitHub Actions 上拆成三个 job：`quality` 跑普通测试、覆盖率和 vet，`race` 单独跑 race 检测，`smoke` 拉起真实的 Redis/MongoDB/MySQL 启动完整示例、检查一条 HTTP 路径，并定向执行带 `liveintegration` 标签的测试（覆盖 Redis、asynq、MySQL、MongoDB 各自的创建-读写-关闭路径，不含并发或故障注入场景）。这三个 job 是否会阻止合并，取决于仓库分支保护是否将它们设为必需检查。

`Makefile` 同时提供 `build`、`lint` 和交叉编译目标，执行前应确认目标路径与本机工具链一致。

## 贡献

提交改动时应保持范围清晰，并说明行为、兼容性和验证结果；涉及启动、并发或资源生命周期的修改，需要补充相应测试与文档。问题和建议可提交至 [GitHub Issues](https://github.com/lamxy/fiberhouse/issues)，安全问题请使用维护者公布的私密联系方式，不要通过公开 issue 报告。

## 许可证

MIT License，见 [LICENSE](LICENSE)。
