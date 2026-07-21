# FiberHouse 🏠

FiberHouse 是一个 Go Web 装配式框架：把配置、日志、HTTP 内核、Provider 扩展、业务注册器这些启动一个服务前要打理好的环节，用一条统一的启动链串起来。框架使用全面接口化设计，默认使用 Fiber 作爲核心， 支持 Gin 核心随时切换，以及通过实现相应接口来定制核心和扩展功能；数据库、缓存、任务等组件是否接入，完全由应用显式决定。

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Build & Test](https://github.com/lamxy/fiberhouse/actions/workflows/go1.yml/badge.svg)](https://github.com/lamxy/fiberhouse/actions/workflows/go1.yml)

## FiberHouse 解决什么问题

搭一个 Go Web 服务，配置怎么加载、日志往哪写、中间件按什么顺序装、共享资源怎么让全局都能访问到——这些接线工作每个项目都要做一遍，也很容易做得零散。FiberHouse 不是帮你生成业务目录的脚手架，而是把这条接线路径固定下来：一套启动链（`FiberHouse` → `FrameStarter` → `CoreStarter`）、一套可插拔的扩展机制（Provider / Manager / Location）、一套跨组件访问共享实例的方式（`GlobalManager`）。

真正的业务逻辑——路由、中间件、连接管理、关闭顺序——仍然由应用自己实现。框架不会扫描目录，也不会因为某个配置项存在就偷偷启用功能，所有接入都是显式的。
框架提供了默认的框架级启动器（FrameStarter），核心启动器（CoreStarter），都可以自由定制、扩展和随时切换，样例应用(example_application)提供了统一规范的上层应用的业务结构和接入模板。

## 目前是什么状态

FiberHouse 仍在逐步迭代和完善中，**尚未达到保证直接用于生产环境的稳定性**。Web 主链（Fiber/Gin、配置日志、统一响应、参数校验）、`GlobalManager`，以及 MySQL/MongoDB/Redis/缓存/asynq 任务/CLI 这些可选组件都已经可以接入使用，具体见下面的[核心能力](#核心能力)。

部分错误处理、并发场景和资源关闭路径还在打磨中，接口后续也可能调整。想了解某个能力具体成熟到什么程度、有哪些已知限制，看[功能状态](docs/reference/feature-status.md)——这是全仓库最新、最细的事实来源。`example_main`/`example_config`/`example_application` 只用来展示调用路径，不要当生产模板直接使用。

## 核心能力

- Fiber 和 Gin 并支持扩展多种 HTTP 内核，在同一套启动模型下切换，支持上层统一的样例应用结构模板。
- Provider / Manager / Location：能力如何被发现、加载、在什么时机执行，都可拆可换：[Provider 系统](docs/concepts/provider-system.md)。
- `ApplicationRegister`、`ModuleRegister`、`TaskRegister` 三类注册器，把应用能力接入 Starter。
- YAML + 环境变量的配置装配，zerolog 输出到 console 或轮转文件。
- `GlobalManager` + `Context` + `Locator`，跨组件访问已注册的共享实例。
- 统一 `{code,msg,data}` 响应、panic recovery、参数校验，可选 MsgPack/Protobuf body。
- MySQL、MongoDB、Redis、本地/二级缓存、asynq 后台任务、基于 urfave/cli 的命令行子应用，按需接入。

## 环境要求

- Go `1.25.0`（以 [`go.mod`](go.mod) 为准）。
- 只是阅读代码或编译框架本身，不需要数据库和 Redis。
- 想原样跑完整 Web 示例，需要 Docker + Docker Compose，或者自行准备好 MongoDB、Redis、MySQL。
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

另开一个终端确认服务是否正常：

```bash
curl http://localhost:8080/health/livez
```

示例启动时会连接 MongoDB、Redis、MySQL 和两个 Sonic codec 实例——只注册 initializer 是不够的，`ConfigRequiredGlobalKeys()` 会主动读取它们。用完记得停掉容器：

```bash
docker compose -f docs/docker_compose_db_redis_yaml/docker-compose.yml down
```

配置怎么选、Gin 怎么切、常见问题有哪些，看[入门指南](docs/getting-started.md)。

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

`newApplicationRegister`、`newModuleRegister` 这几个函数代表应用必须自己实现接口的部分，示例里省略是为了篇幅，不代表可以省略这些职责。完整接线可参照 [`example_main/main.go`](example_main/main.go)，但不要把 example package 当作稳定依赖直接 import。`Default()` 也不会替你调用 `DefaultProviders()`/`DefaultPManagers(ctx)`，这一步需要自己完成。

## 想深入了解框架内部

启动链如何串起来、Provider 系统如何设计、请求从进来到出去经过哪些环节——这些架构细节 README 不展开，直接看对应文档：

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

两种内核共享同一套启动抽象，但路由、绑定、handler 仍各自使用原生 API。示例默认运行 Fiber，`/health/livez` 也只注册在 Fiber 路由里。详细差异见[Web 运行时](docs/guides/web-runtime.md)。

## 示例目录

- [`example_main/`](example_main/)：Web 可执行入口，将 Provider/Manager 全部合并运行。
- [`example_config/`](example_config/)：dev/test/prod 三套配置形状，以及环境变量如何覆盖。
- [`example_application/`](example_application/)：应用、模块、任务、Fiber/Gin 路由、CLI 的完整接线示范。

示例里包含写死的凭据、调试开关、未接完的分支和不完整的关闭流程——把它当源码导航，不要当部署模板直接使用。细节见[示例目录说明](docs/reference/examples.md)。

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

当前工作树上 `go test ./... -count=1` 和 `go test -race ./... -count=1` 应该都能通过，`go vet ./...` 是独立的静态检查门禁。这个结论只对当前提交负责，不代表历史发布标签同样适用——发布版本以对应的 Git tag 和 release note 为准。

CI 在 GitHub Actions 上拆成三个 job：`quality` 跑普通测试、覆盖率和 vet，`race` 单独跑 race 检测，`smoke` 拉起真实的 Redis/MongoDB/MySQL 启动完整示例、检查一条 HTTP 路径，并定向执行带 `liveintegration` 标签的测试（覆盖 Redis、asynq、MySQL、MongoDB 各自的创建-读写-关闭路径，不含并发或故障注入场景）。这三个 job 是否会阻止合并，取决于仓库分支保护是否将它们设为必需检查。

`Makefile` 也提供 `build`、`lint` 和交叉编译目标，使用前请先核对目标路径与本机工具是否匹配。

## 贡献

改动范围保持清晰，说明行为、兼容性与验证结果；涉及启动、并发或资源生命周期的修改，请补充相应测试和文档。问题和建议欢迎提交 [GitHub Issues](https://github.com/lamxy/fiberhouse/issues)，安全问题请使用维护者公布的私密联系方式，不要通过公开 issue 报告。

## 许可证

MIT License，见 [LICENSE](LICENSE)。
