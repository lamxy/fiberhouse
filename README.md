# FiberHouse Framework

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://go.dev/)
[![Build & Test](https://github.com/lamxy/fiberhouse/actions/workflows/go1.yml/badge.svg)](https://github.com/lamxy/fiberhouse/actions/workflows/go1.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Fiber Version](https://img.shields.io/badge/fiber-v2.x-green.svg)](https://github.com/gofiber/fiber)
[![Gin Version](https://img.shields.io/badge/gin-v1.x-green.svg)](https://github.com/gin-gonic/gin)
[![Hertz Version](https://img.shields.io/badge/hertz-v0.10.x-green.svg)](https://github.com/cloudwego/hertz)

📖 中文 | [English]()

## 🏠 關於 FiberHouse 

FiberHouse 是一个面向 Go Web 应用的装配式框架。它将配置、日志、HTTP 内核、Provider 扩展和业务注册统一纳入清晰的启动生命周期，使服务的创建、运行与关闭遵循一致的组织方式。框架默认使用 Fiber，也支持切换 Gin；数据库、缓存、任务等组件均由应用显式选择和装配。

框架的重点不是替代业务架构，而是提供可组合的启动器、可定位的扩展执行点，以及可复用的共享实例管理。当前实现进一步完善了 Provider 初始化幂等、扩展替代逻辑、运行错误传递和优雅关闭链，使 Fiber 与 Gin 能够在同一套生命周期模型下运行。


## 🎯 FiberHouse 解决什么问题

Go Web 服务通常需要分别处理配置加载、日志输出、中间件顺序、共享资源和关闭流程。这些基础能力如果缺少统一边界，往往会分散在入口、组件和业务代码中。FiberHouse 不负责生成业务目录，而是提供一条明确的装配路径：一套启动链（`FiberHouse` → `FrameStarter` → `CoreStarter`）、一套可插拔的扩展机制（Provider / Manager / Location），以及一套跨组件访问共享实例的方式（`GlobalManager`）。

路由、中间件、连接管理和资源所有权仍由应用负责。框架不会扫描目录，也不会根据配置自动启用未声明的组件，所有能力都通过显式注册进入启动链。

框架提供默认的框架级启动器（FrameStarter）和核心启动器（CoreStarter），两者都可以定制或替换。样例应用（`example_application`）展示了上层业务结构与接入方式，便于从可运行路径理解各类扩展点。

## 📊 目前是什么状态

**15 项公共 API 能力已全部达到「稳定公共能力」**：创建、运行、失败、关闭四段生命周期契约明确，单元/契约测试与 race 检测进入 CI，涉及外部依赖的能力有可重复的 live integration 覆盖，调用方本可处理的错误不以 panic 表达，且文档示例确实执行对应路径。逐项证据见[功能状态](docs/reference/feature-status.md)。

具体而言：跨组件资源所有权由 `ClosableProvider` + `IProviderManager.CloseProviders` 形成统一关闭契约，关闭链按注册逆序回收；应用生命周期由 `AppPhase` 有序状态机承载；框架装配期的 panic 已逐条裁决，每一处都有返回 error 的替代入口（见 [panic 裁决表](docs/reference/panic-adjudication.md)）；外部依赖的验证由 Redis、MySQL、MongoDB、asynq 与二级缓存的 live integration 用例覆盖，含并发读写、故障注入与关闭排空。

需要明确的边界：**「稳定」指该能力的生命周期契约、验证证据与文档已闭合，不等于导出符号已冻结。** 兼容承诺将在 v2.0.0 统一给出，在此之前仍可能发生 breaking rename 或签名调整，此类改动会在 CHANGELOG 与功能状态页同步记录。下一个大版本 v2.0.0 将在 v1.x.x 达到生产稳定后重新设计，丢弃 v1 的历史包袱，并引入插件扩展机制以降低应用的启动配置复杂度（方向见 [plugins](plugins/README.md)）。另有 4 项内部工具（Dig 容器、jsonconvert、mongodecimal、logging writer 与 adaptor）按设计不参与晋级，它们是框架内部实现工具而非对外承诺的能力。

`example_main`/`example_config`/`example_application` 用于展示调用路径，包含固定凭据与调试开关，不应直接作为生产模板。

## ✨ 核心能力

- Fiber 与 Gin 共享同一套启动和关闭模型，并保留扩展其他 HTTP 内核的接口边界。
- Provider / Manager / Location：按类型和执行位点组织扩展，支持初始化状态缓存、同位点替代和重复加载保护：[Provider 系统](docs/concepts/provider-system.md)。
- `ApplicationRegister`、`ModuleRegister`、`TaskRegister` 三类注册器，把应用能力接入 Starter。
- YAML 与环境变量共同完成配置装配，zerolog 可输出到 console 或轮转文件。
- `GlobalManager` + `Context` + `Locator`，用于跨组件访问已注册的共享实例。
- 统一 `{code,msg,data}` 响应、panic recovery、参数校验，可选 MsgPack/Protobuf body。
- MySQL、MongoDB、Redis、本地/二级缓存、asynq 后台任务、基于 urfave/cli 的命令行子应用，按需接入；缓存的稳定 API、所有权边界和迁移路径见[缓存指南](docs/guides/cache.md)。

## 🧰 环境要求

- Go `1.25.0`（以 [`go.mod`](go.mod) 为准）。
- 阅读代码或编译框架本身不需要数据库和 Redis。
- 运行完整 Web 示例需要 Docker + Docker Compose，或者自行准备 MongoDB、Redis、MySQL。
- 示例的 MySQL DSN 指向 `test` 库，Compose 只启动 MySQL 服务本身，不会自动建库。

## 🚀 五分钟跑起来

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

## 🧩 接入你自己的应用长什么样

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

## 🔍 想深入了解框架内部

启动链、Provider 系统和请求处理流程的完整设计由专题文档说明：

- [架构总览](docs/concepts/architecture.md)：`BootConfig`、`Context`、`FrameStarter`/`CoreStarter` 各自的职责边界。
- [Provider 系统](docs/concepts/provider-system.md)：能力怎么被发现、加载、执行。
- [Context 与 Locator](docs/concepts/context-and-locators.md)：跨组件访问共享实例的方式。
- [Web 启动生命周期](docs/concepts/startup-lifecycle.md)：`RunServer` 从配置加载到监听关闭的完整阶段。
- [响应与序列化](docs/guides/response-and-serialization.md)、[错误与恢复](docs/guides/errors-and-recovery.md)：统一响应、JSON codec 选择、MsgPack/Protobuf 协商。

## ⚡ Fiber 还是 Gin

| 维度 | Fiber | Gin |
|---|---|---|
| `CoreType` | `constant.CoreTypeWithFiber` | `constant.CoreTypeWithGin` |
| Handler 签名 | `func(*fiber.Ctx) error` | `func(*gin.Context)` |
| 普通错误怎么报 | 直接 `return err` | 调用 `c.Error(err)` |
| JSON codec 作用域 | 单个 `fiber.App` 内 | Gin package 级，全局生效 |
| 备注 | 默认内核，`example_main` 实际跑的路径 | 可切换；TLS 证书加载与真实握手都有回归测试覆盖 |

两种内核共享启动、运行错误传递和信号关闭抽象，但路由、绑定与 handler 仍使用各自的原生 API。示例默认运行 Fiber，`/health/livez` 也只注册在 Fiber 路由中。详细差异见[Web 运行时](docs/guides/web-runtime.md)。

### 还可以接第三方内核

`CoreType` 是普通字符串而非枚举，所有分派点都按 `provider.Target() == CoreType` 匹配，因此**应用侧可以在不修改框架代码的前提下接入新的 HTTP 引擎**。

`example_application/hertzcore/` 是一个已端到端验证的完整参考（cloudwego/hertz），可作为接入模板。做法见[自定义核心启动器](docs/guides/custom-core-starter.md)。

## 📂 示例目录

- [`example_main/`](example_main/)：Web 可执行入口，将 Provider/Manager 全部合并运行。
- [`example_config/`](example_config/)：dev/test/prod 三套配置形状，以及环境变量如何覆盖。
- [`example_application/`](example_application/)：应用、模块、任务、Fiber/Gin 路由、CLI 的完整接线示范。
- [`example_application/hertzcore/`](example_application/hertzcore/)：在框架之外接入第三方内核（Hertz）的完整参考实现。

示例中包含固定凭据、调试开关和用于展示边界的简化实现，适合作为源码导航，不适合作为部署模板直接使用。细节见[示例目录说明](docs/reference/examples.md)。

## 📚 更多文档

- [完整文档索引](docs/README.md)
- [入门指南](docs/getting-started.md)
- [组件目录](docs/reference/components.md)
- [功能状态](docs/reference/feature-status.md)

## 🛠️ 开发与验证

仓库根目录下常用命令：

```bash
go mod download
go build ./...
go test ./...
go test -race ./... -count=1
go vet ./...
```

当前提交以 `go test ./... -count=1`、`go test -race ./... -count=1` 和 `go vet ./...` 作为基础验证，其中 vet 是独立的静态检查门禁。验证结果只对应当前提交，历史版本应以相应的 Git tag 和 release note 为准。

CI 在 GitHub Actions 上拆成三个 job：`quality` 跑普通测试、覆盖率和 vet，`race` 单独跑 race 检测，`smoke` 拉起真实的 Redis/MongoDB/MySQL 启动完整示例、检查健康路径与二进制响应协商（断言 MsgPack、Protobuf 与 JSON 回退三条路径的 Content-Type 及 `data` 完整性），并定向执行带 `liveintegration` 标签的测试。live 覆盖包含 Redis、asynq、MySQL、MongoDB 与二级缓存各自的创建-读写-关闭路径，以及并发读写叠加健康检查、handler 持续失败的重试-归档、就绪探针的依赖故障与恢复、关闭前排空在途异步写等并发与故障注入场景。这三个 job 是否会阻止合并，取决于仓库分支保护是否将它们设为必需检查。

`Makefile` 同时提供 `build`、`lint` 和交叉编译目标，执行前应确认目标路径与本机工具链一致。

## 🤝 贡献

提交改动时应保持范围清晰，并说明行为、兼容性和验证结果；涉及启动、并发或资源生命周期的修改，需要补充相应测试与文档。问题和建议可提交至 [GitHub Issues](https://github.com/lamxy/fiberhouse/issues)，安全问题请使用维护者公布的私密联系方式，不要通过公开 issue 报告。

## 📄 许可证

MIT License，见 [LICENSE](LICENSE)。
