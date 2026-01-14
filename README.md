# FiberHouse Framework

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![Fiber Version](https://img.shields.io/badge/fiber-v2.x-green.svg)](https://github.com/gofiber/fiber)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
<img src="https://img.shields.io/github/issues/lamxy/fiberhouse.svg" alt="GitHub Issues"></img>


📖 中文 | [English](./docs/README_en.md)

## 🏠 关于 FiberHouse

FiberHouse 是默认基于 Fiber 核心的高性能、可装配、模块化设计的 Go Web & CMD 框架，内置全局管理器、配置器、统一日志器、验证包装器以及数据库、缓存、中间件、统一异常处理等框架组件，以及完整的命令行子框架的实现，开箱即用。

- 提供了强大的全局管理容器，支持自定义组件一次注册到处使用的能力，方便开发者按需替换和功能扩展，
- 在框架层面约定了应用启动器、全局上下文、业务分层等接口以及内置默认实现，支持自定义实现和模块化开发，
- 使得 FiberHouse 像装配"家具"的"房子"一样可以按需构建灵活的、完整的、可切换的 Go Web 和 CMD 应用。

### 🏆 开发方向 

提供高性能、可扩展、可定制，开箱即用的 Go Web 框架

## ✨ 功能

- **高性能**: 基于 Fiber 框架，提供极速的 HTTP 性能，支持对象池、goroutine池、缓存、异步等性能优化措施
- **模块化设计**: 清晰的分层架构设计，定义了标准的接口契约和实现，支持团队协作、扩展和模块化开发
- **全局管理器**: 全局对象管理容器，无锁设计、即时注册、延迟初始化、单例特性，支持可替代第三方依赖注入工具的依赖解决方案、以及生命周期的统一管理
- **全局配置管理**: 统一配置文件加载、解析和管理，支持多格式配置、环境变量覆盖，适应不同的应用场景
- **统一日志管理**:  高性能日志系统，支持结构化日志、同步异步写入器，以及各种日志源标识管理
- **统一异常处理**: 统一异常定义和处理机制，支持错误码模块化管理、集成参数验证器、错误追踪，以及友好的调试体验
- **参数验证**: 集成开源验证包装器，支持注册自定义语言验证器、tag标签规则和多语言翻译器
- **数据库支持**: 集成 MySQL、MongoDB 驱动组件以及对数据库模型基类的支持
- **缓存组件**: 内置高性能的本地、远程和二级缓存组件的组合使用和管理，以及对缓存模型基类的支持
- **任务队列**: 集成基于 Redis 的高性能 C/S 架构异步任务队列，支持任务调度、延时执行和失败重试等功能
- **API 文档**: 集成 swag 文档工具，支持自动生成 API 文档
- **命令行应用**: 完整的命令行应用框架支持，遵循统一的模块化设计，支持团队协作、功能扩展和模块化开发
- **样例模板**: 提供完整的Web应用和CMD应用样例模板结构，涵盖了常见场景和最佳实践，开发者稍作修改即可直接套用
- **更多**: 持续优化和更新中...

## 🏗️ 架构概览与说明
### 核心架构分层

```
fiberhouse/  # FiberHouse 框架核心
├── 核心接口定义层
│   ├── `application_interface.go`         # 应用启动器接口,定义应用生命周期管理规范
│   ├── `command_interface.go`             # 命令行应用接口,定义CLI命令注册和执行规范
│   ├── `context_interface.go`             # 全局上下文接口,定义应用上下文的统一访问规范
│   ├── `locator_interface.go`             # 服务定位器接口,定义服务查找和依赖解析规范
│   ├── `model_interface.go`               # 数据模型接口,定义数据访问层的统一规范
│   ├── `provider_interface.go`            # 提供者接口,定义组件注册和初始化规范
│   └── `recover_interface.go`             # 恢复处理器接口,定义异常捕获和恢复机制规范
├── 核心实现层
│   ├── `application_impl.go`              # 应用启动器默认实现,提供标准的应用启动流程
│   ├── `context_impl.go`                  # 全局上下文默认实现,管理配置、日志、容器等核心组件
│   ├── `provider_impl.go`                 # 提供者基类实现,提供组件注册的基础能力
│   ├── `provider_manager_impl.go`         # 提供者管理器实现,统一管理所有提供者的生命周期
│   └── `service_impl.go`                  # 服务定位器实现,提供服务查找和依赖注入能力
├── 提供者管理层
│   ├── `provider_type.go`                 # 提供者类型分组,定义各类提供者的分类和标识
│   ├── `provider_location.go`             # 提供者执行位置点,定义提供者在启动流程中的执行顺序
│   └── `providers/`                       # 内置提供者集合,框架预置的核心组件提供者
│       ├── `core_starter_fiber_provider.go`     # Fiber核心启动提供者
│       ├── `core_starter_gin_provider.go`       # Gin核心启动提供者
│       ├── `json_sonic_fiber_provider.go`       # Sonic JSON编解码器提供者
│       └── `response_providers_manager_impl.go` # 响应处理提供者管理器
├── 应用启动层
│   ├── `boot.go`                          # 统一启动引导,提供一键启动能力和启动配置
│   ├── `frame_starter_impl.go`            # 框架启动器实现,编排框架层面的启动流程
│   ├── `frame_starter_manager.go`         # 框架启动器管理器,管理多种启动器的协同工作
│   ├── `core_fiber_starter_impl.go`       # Fiber核心启动器,基于Fiber的HTTP服务启动
│   ├── `core_gin_starter_impl.go`         # Gin核心启动器,基于Gin的HTTP服务启动
│   └── `commandstarter/`                  # 命令行应用启动,CLI应用的启动和命令管理
│       ├── `cmdline_starter.go`                 # 命令行启动器,管理CLI应用的启动流程
│       └── `core_cmd_application.go`            # 核心命令应用,提供CLI框架的核心功能
├── 配置管理层
│   ├── `bootstrap/`
│   │   └── `bootstrap.go`                 # 配置和日志初始化,应用启动前的基础设施准备
│   └── `appconfig/`
│       └── `config.go`                    # 多格式配置加载,支持YAML/JSON/环境变量等多源配置
├── 全局管理层
│   ├── `globalmanager/`
│   │   ├── `interface.go`                 # 管理器接口,定义全局对象管理的统一规范
│   │   ├── `manager.go`                   # 管理器实现,提供无锁、延迟初始化的全局对象容器
│   │   └── `types.go`                     # 类型定义,管理器相关的类型和常量定义
│   └── `global_utility.go`                # 全局工具函数,提供注册、查找、命名空间等实用工具
├── 数据访问层
│   └── `database/`
│       ├── `dbmysql/`
│       │   ├── `interface.go`                   # MySQL数据库接口定义
│       │   ├── `mysql.go`                       # MySQL连接管理和配置
│       │   └── `mysql_model.go`                 # MySQL模型基类,提供GORM操作的基础能力
│       └── `dbmongo/`
│           ├── `interface.go`                   # MongoDB数据库接口定义
│           ├── `mongo.go`                       # MongoDB连接管理和配置
│           └── `mongo_model.go`                 # MongoDB模型基类,提供文档操作的基础能力
├── 缓存系统层
│   └── `cache/`
│       ├── `cache_interface.go`           # 缓存接口定义,统一的缓存操作规范
│       ├── `cache_option.go`              # 缓存选项配置,提供灵活的缓存策略配置
│       ├── `cache_utility.go`             # 缓存工具函数,提供缓存操作的便捷方法
│       ├── `helper.go`                    # 缓存辅助函数,提供缓存键生成等辅助功能
│       ├── `cache2/`
│       │   └── `level2_cache.go`                # 二级缓存实现,本地+远程的组合缓存策略
│       ├── `cachelocal/`
│       │   ├── `local_cache.go`                 # 本地缓存实现,基于Ristretto的高性能内存缓存
│       │   └── `type.go`                        # 本地缓存类型定义
│       └── `cacheremote/`
│           ├── `cache_model.go`                 # 远程缓存模型,提供缓存数据的序列化能力
│           └── `redis_cache.go`                 # Redis缓存实现,基于Redis的分布式缓存
├── 核心组件层
│   └── `component/`
│       ├── `dig_container.go`             # 依赖注入容器,基于Uber Dig的依赖管理
│       ├── `jsoncodec/`
│       │   └── `sonicjson.go`                   # Sonic JSON编解码器,高性能JSON处理
│       ├── `validate/`
│       │   ├── `type_interface.go`              # 验证器接口定义
│       │   ├── `validate_wrapper.go`            # 验证器包装器,统一的参数验证能力
│       │   ├── `en.go`                          # 英文验证消息翻译
│       │   ├── `zh_cn.go`                       # 简体中文验证消息翻译
│       │   └── `zh_tw.go`                       # 繁体中文验证消息翻译
│       ├── `writer/`
│       │   ├── `async_channel_writer.go`        # 基于Channel的异步日志写入器
│       │   ├── `async_diode_writer.go`          # 基于Diode的异步日志写入器
│       │   └── `sync_lumberjack_writer.go`      # 基于Lumberjack的同步日志轮转写入器
│       └── `tasklog/`
│           └── `logger_adapter.go`              # 任务日志适配器,为Asynq提供日志集成
├── 中间件层
│   └── `middleware/`
│       ├── `recover_config.go`            # 恢复中间件配置,panic恢复的策略配置
│       ├── `recover_error_handler_impl.go` # 恢复错误处理实现,统一的panic处理逻辑
│       └── `recover_interface.go`         # 恢复中间件接口定义
├── 响应处理层
│   └── `response/`
│       ├── `response_interface.go`        # 响应接口定义,统一的响应规范
│       ├── `response_info_impl.go`        # 标准响应实现,JSON格式的统一响应结构
│       ├── `response_proto_impl.go`       # Protobuf响应实现,二进制协议响应支持
│       ├── `response_msgpack_impl.go`     # MessagePack响应实现,高效的二进制序列化
│       └── `response.go`                  # 响应工具函数,提供快速响应的便捷方法
├── 异常处理层
│   └── `exception/`
│       ├── `types.go`                     # 异常类型定义,业务异常的分类和错误码
│       └── `exception_error.go`           # 异常错误实现,统一的异常处理和传播机制
├── 工具层
│   ├── `utils/`
│   │   └── `common.go`                    # 通用工具函数,提供字符串、时间等常用工具
│   └── `constant/`
│       ├── `constant.go`                  # 常量定义,框架级别的常量声明
│       └── `exception.go`                 # 异常常量定义,预定义的异常码和消息
└── 业务分层接口
    ├── `api_impl.go`                      # API层基类实现,提供控制器的基础能力
    ├── `service_impl.go`                  # 服务层基类实现,提供业务逻辑层的基础能力
    ├── `repository_impl.go`               # 仓储层基类实现,提供数据访问层的基础能力
    └── `task.go`                          # 任务基类定义,提供异步任务的基础结构      
```

### 架构设计理念

- **接口驱动**: 核心功能均定义接口契约，支持灵活扩展
- **提供者机制**: 通过Provider模式实现组件的注册和管理
- **分层清晰**: 严格的分层架构，职责明确
- **可插拔设计**: 支持核心框架(Fiber/Gin)和组件的自由切换

## 🚀 快速开始

### 环境要求

- Go 1.24 或更高版本，推荐升级到1.25+
- MySQL 5.7+ 或 MongoDB 4.0+
- Redis 5.0+

### docker 启动数据库、缓存容器用于框架调式

- docker compose文件，见： [docker-compose.yml](docs/docker_compose_db_redis_yaml/docker-compose.yml)
- 启动命令: `docker compose up -d`

```bash

cd  docs/docker_compose_db_redis_yaml/
docker compose up -d
```

### 安装

FiberHouse 运行需要 **Go 1.24 或更高版本**。如果您需要安装或升级 Go，请访问 [Go 官方下载页面](https://go.dev/dl/)。
要开始创建项目，请创建一个新的项目目录并进入该目录。然后，在终端中执行以下命令，使用 Go Modules 初始化您的项目：

```bash

go mod init github.com/your/repo
```
项目设置完成后，您可以使用`go get`命令安装FiberHouse框架：

```bash

go get github.com/lamxy/fiberhouse
```
### main文件示例

参考样例: [example_main/main.go](./example_main/main.go)

```go
package main

import (
  "github.com/lamxy/fiberhouse"
  "github.com/lamxy/fiberhouse/constant"
  "github.com/lamxy/fiberhouse/example_application/providers/middleware"
  "github.com/lamxy/fiberhouse/example_application/providers/module"
  "github.com/lamxy/fiberhouse/example_application/providers/optioninit"
  _ "github.com/lamxy/fiberhouse/example_main/docs" // swagger docs
)

// Version 版本信息，通过编译时 ldflags 注入
// 使用方式: go build -ldflags "-X main.Version=v1.0.0"
var (
  Version string // version
)

func main() {
	// 创建 FiberHouse 应用运行实例
	fh := fiberhouse.New(&fiberhouse.BootConfig{
		AppName:                     "Default FiberHouse Application",          // 应用名称
		Version:                     Version,                                   // 应用版本
		FrameType:                   constant.FrameTypeWithDefaultFrameStarter, // 默认提供的框架启动器标识: DefaultFrameStarter
		CoreType:                    constant.CoreTypeWithFiber,                // fiber | gin | ...
		TrafficCodec:                constant.TrafficCodecWithSonic,            // 传输流量的编解码器: sonic_json_codec|std_json_codec|go_json_codec|pb...
		EnableBinaryProtocolSupport: true,                                      // 是否启用二进制协议支持，如Protobuf等
		ConfigPath:                  "./example_config",                        // 应用全局配置路径
		LogPath:                     "./example_main/logs",                     // 日志文件路径
	})

	// 在框架默认提供者和管理器基础上添加更多自定义的提供者和管理器
	providers := fiberhouse.DefaultProviders().AndMore(
		// 框架启动器和核心启动器的选项参数初始化提供者，
		//注意：由于选项初始化管理器New时已唯一绑定对应的提供者，此处提供者可以无需新建和收集
		//见NewFrameOptionInitPManager()函数
		//optioninit.NewFrameOptionInitProvider(),
		//optioninit.NewCoreOptionInitProvider(),

		//基于Fiber的中间件注册提供者
		middleware.NewFiberAppMiddlewareProvider(),
		middleware.NewFiberModuleMiddlewareProvider(),
		// 基于Gin的中间件注册提供者
		middleware.NewGinAppMiddlewareProvider(),
		// 其他可切换的框架相关中间件提供者
		// ...

		// fiber模块路由和swagger注册提供者
		module.NewFiberRouteRegisterProvider(),
		// gin模块路由和swagger注册提供者
		module.NewGinRouteRegisterProvider(),
		// 更多基于其他核心框架的模块路由注册提供者
		// ...
	)
	managers := fiberhouse.DefaultPManagers(fh.AppCtx).AndMore(
		// 框架选项初始化管理器，获取框架启动器初始化的选项函数列表
		optioninit.NewFrameOptionInitPManager(fh.AppCtx),
		// 核心选项初始化管理器，获取核心启动器初始化的选项函数列表
		optioninit.NewCoreOptionInitPManager(fh.AppCtx).MountToParent(),
		// 应用中间件管理器，注册应用级中间件到核心应用实例
		middleware.NewAppMiddlewarePManager(fh.AppCtx),
		// 模块路由注册管理器，注册模块路由到核心应用实例
		module.NewRouteRegisterPManager(fh.AppCtx),
	)

	// 收集提供者和管理器并运行服务器
	fh.WithProviders(providers...).WithPManagers(managers...).RunServer()
}
```

### 快速体验

- web应用快速体验

```bash

# 克隆框架
git clone https://github.com/lamxy/fiberhouse.git

# 进入框架目录
cd fiberhouse

# 安装依赖
go mod tidy

# 进入example_main/
cd example_main/

# 查看README
cat README_go_build.md

# 构建应用: windows环境为例，其他环境请参考交叉编译
# 退回到应用根目录（默认工作目录），在工作目录下执行以下命令，构建应用
# 当前工作目录为 fiberhouse/，构建产物输出到 example_main/target/ 目录
cd ..
# windows环境构建产物保留.exe后缀，linux环境无需保留后缀
go build "-ldflags=-X 'main.Version=v0.0.1'" -o ./example_main/target/examplewebserver.exe ./example_main/main.go

# 运行应用
# 退回到应用根目录（默认工作目录），在工作目录下执行以下命令，启动应用
./example_main/target/examplewebserver.exe
# or Linux、 MacOS
./example_main/target/examplewebserver
```

访问hello world接口： http://127.0.0.1:8080/example/hello/world

您将收到响应: {"code":0,"msg":"ok","data":"Hello World!"}

```bash

curl -sL  "http://127.0.0.1:8080/example/hello/world"

# 响应:
{
    "code": 0,
    "msg": "ok",
    "data": "Hello World!"
}
```

- Cmd应用快速体验

```bash

# mysql数据库准备
mysqlsh root:root@localhost:3306 

# 创建一个test库
CREATE DATABASE IF NOT EXISTS test CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

# 克隆框架
git clone https://github.com/lamxy/fiberhouse.git

# 进入框架目录
cd fiberhouse

# 安装依赖
go mod tidy

# 进入example_application/command/
cd example_application/command/

# 查看README
cat README_go_build.md

# 当前工作目录： command/
# windows环境构建产物保留.exe后缀，Linux or MacOS环境无需保留后缀
go build -o ./target/cmdstarter.exe ./main.go 

# 设置cmd应用的环境变量，windows环境，将读取application_dev.yml配置文件
set APP_ENV_application_env=dev

# Linux or MacOS 环境
# export APP_ENV_application_env=dev

# 执行cmd命令脚本，查看帮助
./target/cmdstarter.exe -h 
# or 
./target/cmdstarter -h

# 执行子命令，查看控制台日志输出
./target/cmdstarter.exe test-orm -m ok
# or 
./target/cmdstarter test-orm -m ok

# 控制台输出 ok
# result:  ExampleMysqlService.TestOK: OK --from: ok

```
## ⚙️ 核心接口与关键设计

### 设计理念

FiberHouse 采用**接口驱动**和**提供者机制**的设计理念,通过清晰的接口定义和灵活的提供者模式,实现框架的高度可扩展性和可定制性。

### 核心接口体系

#### 1. 应用启动接口

##### 框架启动器接口 (FrameStarter)

**文件位置**: `application_interface.go` [跳转到文件](./application_interface.go)

```go
type FrameStarter interface {
    IStarter
    // GetContext 获取应用上下文
    // 返回全局应用上下文，提供配置、日志器、全局容器等基础设施访问
    GetContext() IApplicationContext
    
    // RegisterApplication 注册应用注册器
    // 将应用注册器实例注入到启动器中，用于后续的全局对象初始化和配置
    RegisterApplication(application ApplicationRegister)
    
    // RegisterModule 注册模块注册器
    // 将模块注册器实例注入到启动器中，用于模块级中间件、路由和Swagger的注册
    RegisterModule(module ModuleRegister)
    
    // GetModule 获取模块注册器
    // 返回已注册的模块注册器实例
    GetModule() ModuleRegister
    
    // RegisterTask 注册任务注册器
    // 将任务注册器实例注入到启动器中，用于异步任务服务器的初始化和启动
    RegisterTask(task TaskRegister)
    
    // GetTask 获取任务注册器
    // 返回已注册的任务注册器实例
    GetTask() TaskRegister
    
    // RegisterToCtx 注册启动器到上下文
    // 将启动器实例注册到应用上下文中，便于其他组件访问
    RegisterToCtx(starter ApplicationStarter)
    
    // RegisterApplicationGlobals 注册应用全局对象和必要对象的初始化
    // 注册全局对象初始化器、初始化必要的全局实例、配置验证器等
    // 包括数据库、缓存、Redis、验证器、自定义标签等的初始化
    RegisterApplicationGlobals(...IProviderManager)
    
    // RegisterLoggerWithOriginToContainer 注册带来源标识的日志器
    // 将配置文件中预定义的不同来源的子日志器初始化器注册到容器中
    // 便于获取已附加来源标记的专用日志器实例
    RegisterLoggerWithOriginToContainer()
    
    // RegisterGlobalsKeepalive 注册全局对象保活机制
    // 启动后台健康检测服务，定期检查全局对象状态并自动重建不健康的实例
    RegisterGlobalsKeepalive(...IProviderManager)
    
    // RegisterTaskServer 注册异步任务服务器
    // 根据配置启动异步任务服务器，注册任务处理器，运行后台任务worker服务并开始监听任务队列
    RegisterTaskServer(...IProviderManager)
    
    // GetFrameApp 获取框架启动器实例
    GetFrameApp() FrameStarter
}
```

**职责**: 定义框架通用的初始化流程

- 全局对象初始化和管理
- 任务服务器启动
- 应用上下文获取
- 自定义初始化逻辑注册器注册

**默认实现**: `frame_starter_impl.go` [跳转到文件](./frame_starter_impl.go)

**扩展方式**: 实现 `FrameStarter` 接口,支持自定义框架初始化流程

##### 核心启动器接口 (CoreStarter)

**文件位置**: `application_interface.go` [跳转到文件](./application_interface.go)

```go
// CoreStarter 应用核心启动器接口
type CoreStarter interface {
    // GetAppContext 获取应用上下文
    // 返回全局应用上下文，提供配置、日志器、全局容器等基础设施访问
    GetAppContext() IApplicationContext
    
    // InitCoreApp 初始化核心应用
    // 创建并配置底层HTTP服务实例（如Fiber应用）
    InitCoreApp(fs FrameStarter, managers ...IProviderManager)
    
    // RegisterAppMiddleware 注册应用级中间件
    // 注册应用级别的中间件，如错误恢复、请求日志、CORS等全局中间件
    RegisterAppMiddleware(fs FrameStarter, managers ...IProviderManager)
    
    // RegisterModuleSwagger 注册模块Swagger文档
    // 根据配置决定是否注册Swagger API文档路由
    RegisterModuleSwagger(fs FrameStarter, managers ...IProviderManager)
    
    // RegisterAppHooks 注册应用钩子函数
    // 注册应用生命周期钩子函数，如启动、关闭时的回调处理
    RegisterAppHooks(fs FrameStarter, managers ...IProviderManager)
    
    // RegisterModuleInitialize 注册模块初始化
    // 执行模块级别的初始化，包括模块中间件和路由处理器的注册
    RegisterModuleInitialize(fs FrameStarter, managers ...IProviderManager)
    
    // AppCoreRun 启动应用核心运行
    // 启动HTTP服务监听，处理优雅关闭信号
    AppCoreRun(...IProviderManager)
    
    // GetCoreApp 获取核心实例
    GetCoreApp() interface{}
}
```

**职责**: 定义底层核心框架的启动逻辑

- 核心应用实例创建 (Fiber/Gin/...)
- 中间件注册
- 路由注册
- 服务监听启动

**内置实现**:

- Fiber核心启动器: `core_fiber_starter_impl.go` [跳转到文件](./core_fiber_starter_impl.go)
- Gin核心启动器: `core_gin_starter_impl.go` [跳转到文件](./core_gin_starter_impl.go)

**扩展方式**: 实现 `CoreStarter` 接口,支持其他Web框架集成

##### 注册器接口族

**文件位置**: `application_interface.go` [跳转到文件](./application_interface.go)

**接口清单**:

- `ApplicationRegister`: 应用级初始化逻辑注册
- `ModuleRegister`: 模块级初始化逻辑注册
- `TaskRegister`: 任务级初始化逻辑注册

```go
// ApplicationRegister 应用注册器
//
// 在应用启动阶段由启动器调用，用于：
// 1. 注册应用的自定义配置、依赖与初始化逻辑；
// 2. 将注册器实例绑定到 ApplicationStarter 的 application 字段，供启动流程使用。
type ApplicationRegister interface {
	IRegister
	IApplication
	// GetContext 返回全局上下文
	GetContext() IApplicationContext

	// ConfigGlobalInitializers 配置并返回全局对象初始化器的列表映射
	ConfigGlobalInitializers() globalmanager.InitializerMap
	// ConfigRequiredGlobalKeys 配置并返回需要初始化的全局对象keyName的切片
	ConfigRequiredGlobalKeys() []globalmanager.KeyName
	// ConfigCustomValidateInitializers 配置自定义语言验证器初始化器的切片
	//见框架组件: validate.Wrap
	ConfigCustomValidateInitializers() []validate.ValidateInitializer
	// ConfigValidatorCustomTags 配置并返回需要注册的验证器自定义tag及翻译的切片(当验证tag缺乏所需语言的翻译时，可以自定义tag翻译)
	//见框架组件: validate.RegisterValidatorTagFunc
	ConfigValidatorCustomTags() []validate.RegisterValidatorTagFunc

	// RegisterAppMiddleware 注册应用级别中间件
	RegisterAppMiddleware(cs CoreStarter)

	// RegisterCoreHook 注册核心应用(coreApp)的生命周期钩子
	RegisterCoreHook(cs CoreStarter)
}

// ModuleRegister 模块注册器
//
// 用于注册应用的模块/子系统，包括中间件、路由、swagger等
// 启动器会调用模块注册器完成模块初始化
type ModuleRegister interface {
	IRegister
	// GetContext 返回全局上下文
	GetContext() IApplicationContext

	// RegisterModuleMiddleware 注册模块级别/子系统中间件
	// RegisterModuleMiddleware(cs CoreStarter)

	// RegisterModuleRouteHandlers 注册模块级别/子系统路由处理器
	RegisterModuleRouteHandlers(cs CoreStarter)
	// RegisterSwagger 注册swagger
	RegisterSwagger(cs CoreStarter)
}

// TaskRegister 任务注册器（基于 asynq）
//
// 用户需实现此接口并在应用启动阶段注册到 ApplicationStarter
// 注册后的任务注册器实例会绑定到 ApplicationStarter 的 task 属性，由启动器调用其方法完成任务组件的初始化
//
// 当全局配置开启异步任务组件时，任务注册器负责：
// 1. 集中声明并注册任务类型（asynq 任务名）与其处理函数到映射容器。
// 2. 将任务调度器（Dispatcher）与任务工作器（Worker）的初始化器注册到全局容器。
// 3. 提供获取任务调度器与工作器实例的访问方法。
type TaskRegister interface {
	IRegister
	// GetContext 返回全局上下文
	GetContext() IApplicationContext

	// GetTaskHandlerMap 返回任务处理器配置map
	//
	// 示例:
	// func myTaskHandler(ctx context.Context, t *asynq.Task) error {
	//     // 处理任务逻辑
	//     return nil // 或返回错误
	// }
	//
	// taskHandlerMap := map[string]func(context.Context, *asynq.Task) error{
	//     "task_type_1": myTaskHandler,
	//     // 更多任务类型和对应的处理器函数
	// }
	GetTaskHandlerMap() map[string]func(context.Context, *asynq.Task) error

	// AddTaskHandlerToMap 向任务处理器映射中添加一个新的任务处理器
	//
	// 示例:
	// func myTaskHandler2(ctx context.Context, t *asynq.Task) error {
	//     // 处理任务逻辑
	//     return nil // 或返回错误
	// }
	//
	// taskRegister.AddTaskHandlerToMap("task_type_2", myTaskHandler2)
	AddTaskHandlerToMap(pattern string, handler func(context.Context, *asynq.Task) error)

	// RegisterTaskServerToContainer 注册异步任务服务器初始化器到容器
	RegisterTaskServerToContainer()

	// RegisterTaskDispatcherToContainer 注册异步任务客户端初始化器到容器
	RegisterTaskDispatcherToContainer()

	// GetTaskDispatcher 获取任务客户端/调度器实例
	GetTaskDispatcher() (*TaskDispatcher, error)

	// GetTaskWorker 获取任务服务器/工作器实例
	GetTaskWorker(key string) (*TaskWorker, error)
}
```

**设计目的**: 分层管理不同级别的初始化逻辑，对应业务应用、业务模块/子应用/子系统及其他功能的分层自定义逻辑

**样例实现**: 
- 应用注册器示例: `application_impl.go` [跳转到文件](./example_application/application_impl.go)
- 模块注册器示例: `module_impl.go` [跳转到文件](./example_application/module/module_impl.go)
- 任务注册器示例: `task_impl.go` [跳转到文件](./example_application/module/task_impl.go)

#### 2. 提供者机制

##### 提供者接口 (IProvider)

**文件位置**: `provider_interface.go` [跳转到文件](./provider_interface.go)

```go
// IProvider 提供者接口
type IProvider interface {
    // Name 返回提供者名称
    Name() string
    // Version 返回提供者版本
    Version() string
    // Initialize 执行提供者初始化操作
    Initialize(IContext, ...ProviderInitFunc) (any, error)
    // RegisterTo 将提供者注册到提供者管理器中
    RegisterTo(manager IProviderManager) error
    // Status 返回提供者当前状态
    Status() IState
    // Target 返回提供者的目标框架引擎类型, e.g., "gin", "fiber",...。该字段区分不同框架引擎类型的提供者实现，也可以用区分其他维度
    Target() string
    // Type 返回提供者的类型, e.g., "middleware", "route_register", "sonic_json_codec", "std_json_codec",...
    Type() IProviderType
    // SetName 设置提供者名称
    SetName(string) IProvider
    // SetVersion 设置提供者版本
    SetVersion(string) IProvider
    // SetTarget 设置提供者目标框架
    SetTarget(string) IProvider
    // SetStatus 设置提供者状态
    SetStatus(IState) IProvider
    // SetType 设置提供者类型，仅允许设置一次
    SetType(IProviderType) IProvider
    // Check 检查提供者是否设置类型值
    Check()
    // BindToUniqueManagerIfSingleton 将提供者绑定到唯一的管理器
    // 注意：传入的管理器对象应当是一个单例实现，以确保全局唯一性
    // 该方法内部调用管理器的 BindToUniqueProvider 方法进行彼此唯一绑定
    // 返回提供者自身以支持链式调用
    // 生效条件：1. 传入的管理器对象是单例实现；2. 子类提供者重载该方法且子类实例本身调用该方法；3. 需要将子类实例反向挂载到父类属性上
    BindToUniqueManagerIfSingleton(IProviderManager) IProvider
    // MountToParent 将当前提供者挂载到父级提供者中
    MountToParent(son ...IProvider) IProvider
}
```

**职责**: 定义可扩展组件的注册契约

- 提供者名称和类型定义
- 提供者注册逻辑
- 提供者依赖关系声明

**基类实现**: `provider_impl.go` [跳转到文件](./provider_impl.go)

**使用场景**:

- 自定义中间件注册
- 自定义JSON编解码器
- 自定义核心启动器
- 任意功能扩展

**注意**: 框架提供默认的提供者基类实现，开发者直接组合/继承基类无需每次手动实现接口方法

##### 提供者管理器接口 (IProviderManager)

**文件位置**: `provider_interface.go` [跳转到文件](./provider_interface.go)

```go
// IProviderManager 提供者管理器接口
type IProviderManager interface {
    // Name 返回提供者管理器名称
    Name() string
    // SetName 设置提供者管理器名称
    SetName(string) IProviderManager
    // Type 返回提供者类型
    Type() IProviderType
    // SetType 设置提供者类型，仅允许设置一次
    SetType(IProviderType) IProviderManager
    // Location 获取管理器的执行位置点
    Location() IProviderLocation
    // SetOrBindToLocation 设置管理器的执行位置点，仅允许设置一次
    SetOrBindToLocation(IProviderLocation, ...bool) IProviderManager
    // GetContext 获取管理器关联的上下文对象
    GetContext() IContext
    // Register 注册提供者到管理器中
    Register(provider IProvider) error
    // Unregister 从管理器中注销提供者
    Unregister(name string) error
    // GetProvider 根据名称获取提供者实例
    GetProvider(name string) (IProvider, error)
    // List 列出管理器中所有注册的提供者
    List() []IProvider
    // Map 以名称为键，提供者实例为值，返回管理器中所有注册的提供者映射
    Map() map[string]IProvider
    // LoadProvider 加载提供者
    LoadProvider(loadFunc ...ProviderLoadFunc) (any, error)
    // Check 检查提供者管理器是否设置类型值
    Check()
    // BindToUniqueProvider 绑定唯一的提供者到管理器
    // 确保管理器有且仅有一个提供者注册进来
    // 如果已存在相同的提供者记录，视为注册成功
    // 如果已存在多个提供者，则 panic 错误
    // 返回管理器自身以支持链式调用
    BindToUniqueProvider(IProvider) IProviderManager
    // IsUnique 返回管理器是否处于唯一提供者模式
    IsUnique() bool
    // MountToParent 将当前管理器挂载到父级管理器中
    MountToParent(son ...IProviderManager) IProviderManager
}
```

**职责**: 提供者的集中管理和位置点挂载

- 提供者收集
- 提供者批量注册
- 执行位置点挂载: 将管理器自身绑定到特定的生命周期或自定义位置点
- 生命周期管理

**基类实现**: `provider_manager_impl.go` [跳转到文件](./provider_manager_impl.go)

**注意**: 框架提供默认的提供者管理器基类实现，开发者直接组合/继承基类无需每次手动实现接口方法

##### 提供者类型分组

**文件位置**: `provider_type.go` [跳转到文件](./provider_type.go)

**内置类型**:

```go
// DefaultPType 预定义的默认类型对象集合
//
// 提供者类型分组的默认逻辑，同一类型的提供者仅允许注册进同一类型的管理器中并加载处理
// 1. GroupXXXChoose Choose结尾，表示选择其中一个提供者执行（仅符合Target()单个提供者执行，即匹配到提供者则中断后续提供者初始化）（比如切换核心引擎、切换编解码器等只取管理器注册的提供者列表中的一个提供者）
// 2. GroupYYYType Type结尾，表示受Target、Name、Version等约束条件限制，符合条件的多个提供者都可以执行（比如多个中间件注册、多个路由组注册的提供者都应用执行）
// 3. GroupZZZAutoRun AutoRun结尾，表示自动运行，不受条件约束，所有注册的提供者均执行一次（比如全局对象注册、默认启动对象初始化的提供者）
// 4. GroupWWWUnique Unique结尾，表示有且只有一个提供者存在和执行（比如框架启动器选项初始化提供者，唯一绑定管理器，管理器将无法注册更多的提供者）
// 5. 其他自定义，由开发者自行约定和实现
type DefaultPType struct {
	ZeroType                        IProviderType // 默认零值类型
	GroupDefaultManagerType         IProviderType // 默认管理器类型组，该类型提供者都注册进默认管理器进行处理
	GroupTrafficCodecChoose         IProviderType // 传输编解码器选择组，该类型提供者中仅选择一个进行流量编解码处理
	GroupCoreEngineChoose           IProviderType // 核心引擎选择组，该类型提供者中仅选择一个进行核心引擎处理
	GroupMiddlewareRegisterType     IProviderType // 中间件注册类型组，该类型提供者都注册进中间件链进行处理
	GroupRouteRegisterType          IProviderType // 路由注册类型组，该类型提供者都注册进路由表进行处理
	GroupCoreHookChoose             IProviderType // 核心钩子选择组，该类型提供者中仅选择一个进行核心钩子处理
	GroupFrameStarterChoose         IProviderType // 框架启动器选择组，该类型提供者中仅选择一个进行框架启动处理
	GroupCoreStarterChoose          IProviderType // 核心启动器选择组，该类型提供者中仅选择一个进行核心启动处理
	GroupProviderAutoRun            IProviderType // 提供者自动运行组，该类型提供者都自动运行一次进行处理
	GroupCoreContextChoose          IProviderType // 核心上下文选择组，该类型提供者中仅选择一个进行核心上下文处理
	GroupFrameStarterOptsInitUnique IProviderType // 框架启动器选项初始化唯一组，该类型提供者中仅唯一绑定一个管理器，并由该唯一的提供者进行处理
	GroupCoreStarterOptsInitUnique  IProviderType // 核心启动器选项初始化唯一组，该类型提供者中仅唯一绑定一个管理器，并由该唯一的提供者进行处理
	GroupRecoverMiddlewareChoose    IProviderType // 恢复中间件选择组，该类型提供者中仅选择一个进行恢复中间件处理（根据核心类型选择）
	GroupResponseInfoChoose         IProviderType // 响应信息选择组，该类型提供者中仅选择一个进行响应信息处理（根据name存储的http内容类型来选择）
}
```

**扩展方式**: 调用 `ProviderTypeDefault().MustCustom("xxx")` 创建自定义类型

##### 执行位置点机制

**文件位置**: `provider_location.go` [跳转到文件](./provider_location.go)

**内置位置点**:

```go
// DefaultPLocation 预定义的默认位点对象集合
//
// 位点用于标识提供者的执行位置，相同位点的管理器会被收集并按顺序执行
// 1. LocationXXXBefore 在某个阶段之前执行
// 2. LocationXXXAfter 在某个阶段之后执行
// 3. LocationXXXInit 在某个初始化阶段执行
// 4. LocationXXXRun 在XXX运行阶段执行
// 5. LocationXXXCreate 在XXX创建阶段执行
// 6. 其他，由开发者自定义
type DefaultPLocation struct {
	ZeroLocation                   IProviderLocation // 初始化默认位点/零位点/保留为初始化状态
	LocationAdaptCoreCtxChoose     IProviderLocation // 适配核心上下文选择位点（用于统一输出响应时屏蔽不同核心引擎上下文差异）
	LocationBootStrapConfig        IProviderLocation // 引导配置阶段位点
	LocationFrameStarterOptionInit IProviderLocation // 框架启动器选项初始化位点
	LocationCoreStarterOptionInit  IProviderLocation // 核心启动器选项初始化位点
	LocationFrameStarterCreate     IProviderLocation // 创建框架启动器位点
	LocationCoreStarterCreate      IProviderLocation // 创建核心引擎启动器位点
	LocationGlobalInit             IProviderLocation // 全局初始化位点
	LocationGlobalKeepaliveInit    IProviderLocation // 全局对象保活初始化位点
	LocationCoreEngineInit         IProviderLocation // 核心引擎初始化位点
	LocationCoreHookInit           IProviderLocation // 核心引擎钩子（如有）初始化位点
	LocationAppMiddlewareInit      IProviderLocation // 注册应用中间件初始化位点
	LocationModuleMiddlewareInit   IProviderLocation // 注册模块中间件初始化位点
	LocationRouteRegisterInit      IProviderLocation // 注册路由初始化位点
	LocationTaskServerInit         IProviderLocation // 任务服务器初始化位点
	LocationModuleSwaggerInit      IProviderLocation // 注册Swagger初始化位点
	LocationServerRunBefore        IProviderLocation // 服务运行前位点
	LocationServerRun              IProviderLocation // 服务运行位点
	LocationServerRunAfter         IProviderLocation // 服务运行后位点
	LocationServerShutdownBefore   IProviderLocation // 服务关闭前位点
	LocationServerShutdown         IProviderLocation // 服务关闭位点
	LocationServerShutdownAfter    IProviderLocation // 服务关闭后位点
	LocationResponseInfoInit       IProviderLocation // 响应信息初始化位点
}
```

**工作原理**:

1. 提供者管理器通过 `SetOrBindToLocation(LocationServerRun)` 挂载到服务运行位置点
2. 框架在特定生命周期(如服务运行)触发位置点
3. 自动加载并执行对应的提供者管理器

**优势**: 精确控制组件的加载时机,实现细粒度的生命周期管理

#### 3. 全局上下文接口

##### 应用上下文接口 (IAppContext)

**文件位置**: `context_interface.go` [跳转到文件](./context_interface.go)

```go
// IContext 全局上下文接口
type IContext interface {
    // GetConfig 定义获取全局配置的方法
    GetConfig() appconfig.IAppConfig
    // GetLogger 定义获取全局日志器的方法
    GetLogger() bootstrap.LoggerWrapper
    // GetContainer 定义获取全局管理器的方法
    GetContainer() *globalmanager.GlobalManager
    // GetStarter 定义获取启动器实例的方法，用于获取IApplication实例方法
    GetStarter() IStarter
    // GetLoggerWithOrigin 定义获取附加来源的子日志器单例的方法（从全局管理器获取）
    GetLoggerWithOrigin(originFormCfg appconfig.LogOrigin) (*zerolog.Logger, error)
    // GetMustLoggerWithOrigin 定义获取附加来源的日志器实例的方法，若获取失败则panic（从全局管理器获取）
    GetMustLoggerWithOrigin(originFormCfg appconfig.LogOrigin) *zerolog.Logger
    // GetValidateWrap 定义获取全局验证器包装器的方法
    GetValidateWrap() validate.ValidateWrapper
}

// IApplicationContext 框架Web应用上下文接口
type IApplicationContext interface {
    IContext
    // RegisterStarterApp 挂载框架启动器app
    RegisterStarterApp(sApp ApplicationStarter)
    // GetStarterApp 获取框架应用启动器实例(如WebApplication)
    GetStarterApp() ApplicationStarter
    // RegisterAppState 注册应用启动状态
    RegisterAppState(bool)
    // GetAppState 获取应用启动状态
    GetAppState() bool
    // GetBootConfig 获取启动配置
    GetBootConfig() *BootConfig
    // RegisterBootConfig 注册启动配置
    RegisterBootConfig(bc *BootConfig)
}
```

**职责**: 应用全局对象访问，按需获取应用运行时的全局对象单例

- 启动配置获取
- 应用配置器获取
- 日志器获取
- 全局管理器获取
- 验证器获取
- 启动器实例获取

**默认实现**: `context_impl.go` [跳转到文件](./context_impl.go)

**注意**: 框架提供默认的全局应用上下文实例的实现，开发者可以任意组合全局应用上下文实例以按需使用

#### 4. 业务分层接口

##### 服务定位器接口族

**文件位置**: `locator_interface.go` [跳转到文件](./locator_interface.go)

**接口清单**:

- `ApiLocator`: API层定位器
- `ServiceLocator`: 服务层定位器
- `RepositoryLocator`: 仓储层定位器
- `TaskLocator`: 任务层定位器


```go
// Locator 定位器接口，定义了获取上下文、名称、实例等方法
// 以及错误恢复方法。用于分层和管理应用中的业务组件或服务实例。
// 该接口可以被具体的API、Service、Repository等定位器实现。
type Locator interface {
	// 获取全局上下文对象
	GetContext() IContext
	// 获取定位器名称空间
	GetName() string
	// 设置定位器名称空间
	SetName(string) Locator // replace interface{}
	// GetInstance 获取实例（从全局管理器获取具体的单例）
	GetInstance(string) (interface{}, error)
}

// ApiLocator Api层定位器
type ApiLocator = Locator

// ServiceLocator 服务层定位器
type ServiceLocator = Locator

// RepositoryLocator 仓储层定位器
type RepositoryLocator = Locator
```

**提供能力**:

- 获取应用上下文
- 获取配置、日志器
- 获取全局管理器实例
- 统一日志输出

**使用示例**:

```go
type ExampleService struct {
    fiberhouse.ServiceLocator
    Repo *repository.ExampleRepository
}

func (s *ExampleService) DoSomething() {
    // 直接使用定位器能力
    logger := s.GetLogger()
    config := s.GetConfig()
    instance := s.GetInstance("key")
}
```

**注意**: 框架提供默认的业务分层定位器的基类实现，开发者可参考应用样例直接组合/继承基类无需每次手动实现接口方法

#### 5. 异常处理接口

##### 错误处理器接口

**文件位置**: `recover_interface.go` [跳转到文件](./recover_interface.go)

```go
// IErrorHandler 错误处理接口，用于统一定义堆栈日志记录及错误处理器的方法
type IErrorHandler interface {
	DefaultStackTraceHandler(providerctx.ICoreContext, interface{})
	ErrorHandler(providerctx.ICoreContext, error) error
	GetContext() IApplicationContext
	RecoverMiddleware(...RecoverConfig) any
}
```

**职责**: 统一错误处理逻辑

- 异常捕获
- 错误日志记录
- 响应格式化
- 多框架适配
  - 基于Fiber错误处理器适配器: `fiber_error_handler.go` [跳转到文件](./provider/adaptor/fiber_error_handler.go)
  - 基于Gin错误处理器适配器: `gin_error_handler.go` [跳转到文件](./provider/adaptor/gin_error_handler.go)

**内置实现**:

- 统一错误处理器实现: `recover_error_handler_impl.go` [跳转到文件](./recover_error_handler_impl.go)

**注意**: 框架提供默认的统一错误处理器的实现，开发者可自行实现该接口来支持更多自定义的错误处理逻辑

##### 恢复接口

**文件位置**: `recover_interface.go` [jump to file](./recover_interface.go)

```go
// IRecover 恢复惊慌接口，用于获取不同框架的请求上下文中的参数、查询参数、获取tranceID以及定义恢复中间件方法
type IRecover interface {
	// GetParamsJson 获取路由参数的 JSON 编码字节切片
	GetParamsJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
	// GetQueriesJson 获取查询参数的 JSON 编码字节切片
	GetQueriesJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
	// GetHeadersJson 获取请求头的 JSON 编码字节切片（敏感信息脱敏）
	GetHeadersJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
	// RecoverPanic 返回恢复中间件函数，根据核心类型（如 fiber、gin）返回对应的中间件
	// 通过恢复中间件管理器依据启动配置选择相应的提供者自动返回对应的恢复中间件
	RecoverPanic(...RecoverConfig) any
	TraceID(ctx providerctx.ICoreContext, flag ...string) string
	GetHeader(ctx providerctx.ICoreContext, key string) string
}
```

**职责**: Panic恢复机制

- Panic捕获
- 堆栈跟踪
- 错误响应

**内置实现**:
- 基于Fiber的恢复实现: `FiberRecovery` [跳转到文件](./recover_recoveries_impl.go)
- 基于Gin的恢复实现: `GinRecovery` [跳转到文件](./recover_recoveries_impl.go)

#### 6. 响应处理接口

##### 响应接口 (IResponse)

**文件位置**: `response/response_interface.go`  [跳转到文件](./response/response_interface.go)

```go
type IResponse interface {
    GetCode() int
    GetMsg() string
    GetData() interface{}
    SendWithCtx(c providerctx.ICoreContext, status ...int) error
    JsonWithCtx(c providerctx.ICoreContext, status ...int) error
    Reset(code int, msg string, data interface{}) IResponse
    Release()
    From(resp IResponse, needToRelease bool) IResponse
    SuccessWithData(data ...interface{}) IResponse
    ErrorCustom(code int, msg string) IResponse
}
```

**职责**: 统一响应格式

- 响应码、消息、数据封装
- 多种序列化协议支持
- 对象池优化

**内置实现**:

- `RespInfo`: JSON响应 (对象池) [跳转到文件](./response/response_impl.go)
- `Exception`: 异常响应 (对象池) [跳转到文件](./response/response_impl.go)
- `ValidateException`: 验证异常响应 (对象池) [跳转到文件](./response/response_impl.go)
- `RespInfoProto`: Protobuf响应 (对象池) [跳转到文件](./response/response_proto_impl.go)
- `RespInfoMagPack`: MsgPack响应 (对象池) [跳转到文件](./response/response_msgpack_impl.go)
- `RespInfoProtobufProvider`: Protobuf响应提供者 [跳转到文件](./response_providers_manager_impl.go)
- `RespInfoMsgpackProvider`: MsgPack响应提供者 [跳转到文件](./response_providers_manager_impl.go)
- `RespInfoPManager`: 响应提供者管理器 [跳转到文件](./response_providers_manager_impl.go)

### 关键设计模式

#### 1. 提供者模式 (Provider Pattern)

**核心思想**: 将功能以提供者形式注册到框架

**优势**:

- 解耦: 功能与框架解耦
- 灵活: 按需加载和替换
- 扩展: 无侵入式扩展

**使用流程**:

```go
// 1. 实现提供者
// RespInfoProtobufProvider 响应信息 Protobuf 提供者
type RespInfoProtobufProvider struct {
    IProvider  // 组合基类提供者实现
}

func NewRespInfoProtobufProvider() *RespInfoProtobufProvider {
  son := &RespInfoProtobufProvider{
        IProvider: NewProvider().SetName("application/x-protobuf").SetType(ProviderTypeDefault().GroupResponseInfoChoose),
  }
  son.MountToParent(son)
  return son
}

// Initialize 初始化
func (p *RespInfoProtobufProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
    return response.GetRespInfoPB(), nil
}

// 2. 收集提供者
providers := fiberhouse.DefaultProviders().AndMore(
    NewRespInfoProtobufProvider(),
)

// 3. 创建提供者管理器
// RespInfoPManager 响应信息提供者管理器
type RespInfoPManager struct {
    IProviderManager  // 组合基类提供者管理器实现
}

func NewRespInfoPManager(ctx IContext) *RespInfoPManager {
    son := &RespInfoPManager{
        IProviderManager: NewProviderManager(ctx).
            SetName("RespInfoPManager").
            SetType(ProviderTypeDefault().GroupResponseInfoChoose),
    }
    // 挂载子实例到父属性，设置并绑定子实例（当前实例）到执行位点
    son.MountToParent(son).SetOrBindToLocation(ProviderLocationDefault().LocationResponseInfoInit, true)
    return son
}

// LoadProvider 加载提供者
func (m *RespInfoPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
    if len(loadFunc) == 0 {
        return nil, fmt.Errorf("manager '%s': no load function provided", m.Name())
    }
    anything, err := loadFunc[0](m)
    if err != nil {
        return nil, err
    }
    contentType, ok := anything.(string)
    if !ok {
        return nil, errors.New("loadFunc manager '" + m.Name() + "': expected string of http Content-Type")
    }
    return m.GetProvider(contentType)
}

// 4. 框架自动加载: RunServer()内部自动将同类型组的提供者注册进管理器
fiberhouse.New().WithProviders(providers).WithPManagers(managers).RunServer()
```

#### 2. 服务定位器模式 (Service Locator Pattern)

**核心思想**: 通过定位器接口统一获取依赖

**优势**:

- 无需显式依赖注入
- 延迟获取依赖
- 简化代码结构

**使用示例**:

```go
type MyService struct {
    fiberhouse.ServiceLocator
	repoInstanceRegisterKey string
}

func (s *MyService) Method() {
    // 通过组合定位器基类的获取实例方法获取依赖对象
    dep := s.GetInstance(s.repoInstanceRegisterKey)
}
```

#### 3. 对象池模式 (Object Pool Pattern)

**应用场景**: 响应对象、缓存选项

**优势**:

- 减少GC压力
- 提升性能
- 内存复用

**使用示例**:

```go
// 从对象池获取
resp := response.GetRespInfo() // 内部从对象池获取响应信息对象
defer resp.Release() // 归还对象池

// 缓存选项池
co := cache.OptionPoolGet(ctx)
defer cache.OptionPoolPut(co)
```

### 扩展参考说明

#### 添加新的核心框架支持

1. 实现 `CoreStarter` 接口
2. 创建对应的提供者
3. 添加到提供者集合
4. 注册到框架

#### 添加新框架的错误处理器和恢复中间件

1. 实现 `IRecover` 接口，如EchoRecover，参考`GinRecovery` [跳转到文件](./recover_recoveries_impl.go)
2. 创建对应的提供者，如`EchoRecoverProvider`，参考`GinRecoveryProvider` [跳转到文件](./recover_providers_manager_impl.go)
3. 添加到提供者集合，将新的提供者添加到框架的提供者列表中
4. 注册到框架， 如: [main.go](./example_main/main.go)
  ```go
  providers := fiberhouse.DefaultProviders().AndMore(NewEchoRecoverProvider())
  fiberhouse.New(xxx).WithProviders(providers...).WithPManagers(managers...).RunServer();
  ```
#### 添加新的响应协议

1. 实现 `IResponse` 接口
2. 实现对象池支持
3. 创建对应的提供者
4. 添加到提供者集合
5. 注册到框架

FiberHouse 通过清晰的接口定义和灵活的提供者机制,实现了:

- ✅ 高度可扩展性
- ✅ 低耦合设计
- ✅ 易于测试
- ✅ 支持团队协作
- ✅ 平滑的功能演进


## 📖 业务应用使用指南

- examples样例模板项目结构
- 依赖注入工具说明和使用
- 通过框架的全局管理器实现无需依赖注入工具来解决依赖关系
- 样例 curd API实现
- 如何添加新的模块和新的api
- task异步任务的使用样例
- 缓存组件使用样例
- cmd命令行应用的使用样例

### examples样例应用模板目录结构

- 架构概览与说明

```
example_application/                    # 样例应用根目录
├── 应用配置层
│   ├── application_impl.go            # 应用注册器实现
│   ├── constant.go                    # 应用级常量
│   └── customizer_interface.go        # 应用定制器接口
│
├── API接口层
│   └── apivo/                         # API值对象定义
│       ├── commonvo/                  # 通用VO
│       │   └── vo.go                  # 通用值对象
│       └── example/                   # 示例模块VO
│           ├── api_interface.go       # API接口定义
│           ├── requestvo/             # 请求VO
│           │   └── example_reqvo.go
│           └── responsevo/            # 响应VO
│               └── example_respvo.go
│
├── 命令行应用层
│   └── command/                       # 命令行程序
│       ├── main.go                    # 命令行入口
│       ├── README_go_build.md         # 构建说明
│       ├── application/               # 命令应用配置
│       │   ├── application.go         # 命令应用逻辑
│       │   ├── constants.go           # 命令常量
│       │   ├── functions.go           # 工具函数
│       │   └── commands/              # 命令脚本实现
│       │       ├── test_orm_command.go
│       │       └── test_other_command.go
│       ├── component/                 # 命令行组件
│       │   └── cron.go                # 定时任务
│       └── target/                    # 构建产物目录
│
├── 异常处理层
│   ├── get_exceptions.go              # 异常获取器
│   └── example-module/                # 模块异常定义
│       └── exceptions.go
│
├── 提供者层
│   └── providers/                     # 提供者集合
│       ├── middleware/                # 中间件提供者
│       │   ├── fiber_app_middleware_provider.go
│       │   ├── fiber_module_middleware_provider.go
│       │   └── gin_app_middleware_provider.go
│       ├── module/                    # 模块提供者
│       │   ├── fiber_route_register_provider.go
│       │   └── gin_route_register_provider.go
│       └── optioninit/                # 选项初始化提供者
│           ├── frame_option_init_provider.go
│           └── core_option_init_provider.go
│
├── 业务模块层
│   └── module/                        # 业务模块
│       ├── module.go                  # 模块注册器
│       ├── route_register.go          # 路由注册器
│       ├── swagger.go                 # Swagger配置
│       ├── task.go                    # 任务注册器
│       │
│       ├── command-module/            # 命令行业务模块
│       │   ├── entity/                # 实体定义
│       │   ├── model/                 # 数据模型
│       │   └── service/               # 业务服务
│       │
│       ├── common-module/             # 通用模块
│       │   ├── attrs/                 # 属性定义
│       │   ├── fields/                # 通用字段
│       │   ├── model/                 # 通用模型
│       │   ├── repository/            # 通用仓储
│       │   ├── service/               # 通用服务
│       │   └── vars/                  # 通用变量
│       │
│       ├── constant/                  # 常量定义
│       │   └── constants.go
│       │
│       └── example-module/            # 核心样例模块
│           ├── api/                # API控制器层
│           │   ├── api_provider_wire_gen.go  # Wire生成文件
│           │   ├── api_provider.go    # API提供者
│           │   ├── common_api.go      # 通用API
│           │   ├── example_api.go     # 示例API
│           │   ├── health_api.go      # 健康检查API
│           │   └── register_api_router.go    # 路由注册
│           │
│           ├── dto/                # 数据传输对象
│           │
│           ├── entity/             # 实体层
│           │   └── types.go
│           │
│           ├── model/              # 模型层
│           │   ├── example_model.go
│           │   ├── example_mysql_model.go
│           │   └── model_wireset.go
│           │
│           ├── repository/         # 仓储层
│           │   ├── example_repository.go
│           │   ├── health_repository.go
│           │   └── repository_wireset.go
│           │
│           ├── service/            # 服务层
│           │   ├── example_service.go
│           │   ├── health_service.go
│           │   ├── service_wireset.go
│           │   └── test_service.go
│           │
│           └── task/               # 任务层
│               ├── names.go           # 任务名称
│               ├── task.go            # 任务注册器
│               └── handler/           # 任务处理器
│                   ├── handle.go
│                   └── mount.go
│
├── 工具层
│   └── utils/                         # 应用工具
│       └── common.go
│
└── 自定义验证器层
    └── validatecustom/                # 自定义验证器
        ├── register_validator.go
        └── custom_rules.go
```

### 目录结构说明

#### 核心分层
- **应用配置层**: 应用级配置和常量定义
- **API接口层**: 统一的API值对象定义
- **命令行应用层**: 独立的命令行子框架
- **异常处理层**: 模块化的异常定义
- **提供者层**: 框架扩展点的提供者实现
- **业务模块层**: 按模块组织的业务逻辑

#### 业务模块内部分层（以example-module为例）
- **api/**: API控制器，处理HTTP请求
- **dto/**: 数据传输对象，用于层间数据传递
- **entity/**: 实体定义，映射数据库表结构
- **model/**: 数据模型，封装数据库操作
- **repository/**: 仓储层，实现数据持久化
- **service/**: 服务层，实现业务逻辑
- **task/**: 任务层，处理异步任务


### 依赖注入工具说明和使用

- 依赖注入工具和库
  - google wire: 依赖注入代码生成工具，官方地址 [https://github.com/google/wire](https://github.com/google/wire)
  - uber dig: 依赖注入容器，推荐仅在应用启动阶段使用，官方地址 [https://github.com/uber-go/dig](https://github.com/uber-go/dig)
- google wire使用说明和示例，参考:
  - [example_application/module/example-module/api/api_provider.go](./example_application/module/example-module/api/api_provider.go)
  - [example_application/module/example-module/api/README_wire_gen.md](./example_application/module/example-module/api/README_wire_gen.md)
- uber dig使用说明和示例，参考:
  - [component/dig_container.go](component/dig_container.go)

### 通过框架的全局管理器实现无需依赖注入工具来解决依赖关系

- 见注册路由示例： [example_application/module/example-module/api/register_api_router.go](./example_application/module/example-module/api/register_api_router.go)

```go
func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app fiber.Router) {
    // 获取exampleApi处理器
    exampleApi, _ := InjectExampleApi(ctx) // 由wire编译依赖注入生成注入函数获取ExampleApi
    
    // 获取CommonApi处理器，直接NewCommonHandler
	
	// 直接New，无需依赖注入(Wire注入)，内部依赖走全局管理器延迟获取依赖组件，
	// 见 common_api.go: api.CommonHandler
	commonApi := NewCommonHandler(ctx) 
	
    // 获取注册更多api处理器并注册相应路由...
    
    // 注册Example模块的路由
    exampleGroup := app.Group("/example")
	// hello world
    exampleGroup.Get("/hello/world", exampleApi.HelloWorld).Name("ex_get_example_test")
}
```

- 见CommonHandler通过全局管理器实现无需事先依赖注入服务组件: [example_application/module/example-module/api/common_api.go](./example_application/module/example-module/api/common_api.go)

```go
// CommonHandler 示例公共处理器，继承自 fiberhouse.ApiLocator，具备获取上下文、配置、日志、注册实例等功能
type CommonHandler struct {
	fiberhouse.ApiLocator
	KeyTestService string // 定义依赖组件的全局管理器的实例key。通过key即可由 h.GetInstance(key) 方法获取实例，或由 fiberhouse.GetMustInstance[T](key) 泛型方法获取实例，
	                      // 无需wire或其他依赖注入工具
}

// NewCommonHandler 直接New，无需依赖注入(Wire) TestService对象，内部走全局管理器获取依赖组件
func NewCommonHandler(ctx fiberhouse.IApplicationContext) *CommonHandler {
	return &CommonHandler{
		ApiLocator:     fiberhouse.NewApi(ctx).SetName(GetKeyCommonHandler()),
		
        // 注册依赖的TestService实例初始化器并返回注册实例key，通过 h.GetInstance(key) 方法获取TestService实例
		KeyTestService: service.RegisterKeyTestService(ctx), 
	}
}

// TestGetInstance 测试获取注册实例，通过 h.GetInstance(key) 方法获取TestService注册实例，无需编译阶段的wire依赖注入
func (h *CommonHandler) TestGetInstance(c *fiber.Ctx) error {
    t := c.Query("t", "test")
    
    // 通过 h.GetInstance(h.KeyTestService) 方法获取注册实例
    testService, err := h.GetInstance(h.KeyTestService)
        if err != nil {
        return err
    }
    
    if ts, ok := testService.(*service.TestService); ok {
        return response.RespSuccess(t + ":" + ts.HelloWorld()).JsonWithCtx(c)
    }
    
    return fmt.Errorf("类型断言失败")
}
```

### 样例 curd API实现

- 定义实体类型: 见[example_application/module/example-module/entity/types.go](./example_application/module/example-module/entity/types.go)

```go
// Example
type Example struct {
	ID                bson.ObjectID             `json:"id" bson:"_id,omitempty"`
	Name              string                    `json:"name" bson:"name"`
	Age               int                       `json:"age" bson:"age,minsize"` // minsize 取int32存储数据
	Courses           []string                  `json:"courses" bson:"courses,omitempty"`
	Profile           map[string]interface{}    `json:"profile" bson:"profile,omitempty"`
	fields.Timestamps `json:"-" bson:",inline"` // inline: bson文档序列化自动提升嵌入字段即自动展开继承的公共字段
}
```

- 路由注册：见 [example_application/module/example-module/api/register_api_router.go](./example_application/module/example-module/api/register_api_router.go)

```go
func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app fiber.Router) {
    // 获取exampleApi处理器
    exampleApi, _ := InjectExampleApi(ctx) // 由wire编译依赖注入获取
	
    // 注册Example模块的路由
    // Example 路由组
    exampleGroup := app.Group("/example")
	
	// hello world 路由
    exampleGroup.Get("/hello/world", exampleApi.HelloWorld).Name("ex_get_example_test")
	
	// CURD 路由
    exampleGroup.Get("/get/:id", exampleApi.GetExample).Name("ex_get_example")
    exampleGroup.Get("/on-async-task/get/:id", exampleApi.GetExampleWithTaskDispatcher).Name("ex_get_example_on_task")
    exampleGroup.Post("/create", exampleApi.CreateExample).Name("ex_create_example")
    exampleGroup.Get("/list", exampleApi.GetExamples).Name("ex_get_examples")
}
```

- 定义样例Api处理器: 见 [example_application/module/example-module/api/example_api.go](./example_application/module/example-module/api/example_api.go)

```go
// ExampleHandler 示例处理器，继承自 fiberhouse.ApiLocator，具备获取上下文、配置、日志、注册实例等功能
type ExampleHandler struct {
	fiberhouse.ApiLocator
	Service        *service.ExampleService 
	KeyTestService string                  
}

func NewExampleHandler(ctx fiberhouse.IApplicationContext, es *service.ExampleService) *ExampleHandler {
	return &ExampleHandler{
		ApiLocator:     fiberhouse.NewApi(ctx).SetName(GetKeyExampleHandler()),
		Service:        es,
		KeyTestService: service.RegisterKeyTestService(ctx),
	}
}

// GetKeyExampleHandler 定义和获取 ExampleHandler 注册到全局管理器的实例key
func GetKeyExampleHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleHandler", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// GetExample 获取样例数据
func (h *ExampleHandler) GetExample(c *fiber.Ctx) error {
	// 获取语言
	var lang = c.Get(constant.XLanguageFlag, "en")

	id := c.Params("id")

	// 构造需要验证的结构体
	var objId = &requestvo.ObjId{
		ID: id,
	}
	// 获取验证包装器对象
	vw := h.GetContext().GetValidateWrap()

	// 获取指定语言的验证器，并对结构体进行验证
	if errVw := vw.GetValidate(lang).Struct(objId); errVw != nil {
		var errs validator.ValidationErrors
		if errors.As(errVw, &errs) {
			return vw.Errors(errs, lang, true)
		}
	}

	// 从服务层获取数据
	resp, err := h.Service.GetExample(id)
	if err != nil {
		return err
	}

	// 返回成功响应
    fiberhouse.Response().SuccessWithData(resp).JsonWithCtx(providerctx.WithFiberContext(c))
}
```

- 定义样例服务: 见 [example_application/module/example-module/service/example_service.go](./example_application/module/example-module/service/example_service.go)

```go
// ExampleService 样例服务，继承 fiberhouse.ServiceLocator 服务定位器接口，具备获取上下文、配置、日志、注册实例等功能
type ExampleService struct {
	fiberhouse.ServiceLocator                               // 继承服务定位器接口
	Repo                 *repository.ExampleRepository // 依赖的组件: 样例仓库，构造参数注入。由wire工具依赖注入
}

func NewExampleService(ctx fiberhouse.IApplicationContext, repo *repository.ExampleRepository) *ExampleService {
	name := GetKeyExampleService()
	return &ExampleService{
		ServiceLocator: fiberhouse.NewService(ctx).SetName(name),
		Repo:           repo,
	}
}

// GetKeyExampleService 获取 ExampleService 注册键名
func GetKeyExampleService(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// GetExample 根据ID获取样例数据
func (s *ExampleService) GetExample(id string) (*responsevo.ExampleRespVo, error) {
    resp := responsevo.ExampleRespVo{}
	// 调用仓储层获取数据
    example, err := s.Repo.GetExampleById(id)
    if err != nil {
        return nil, err
    }
	// 处理数据
    resp.ExamName = example.Name
    resp.ExamAge = example.Age
    resp.Courses = example.Courses
    resp.Profile = example.Profile
    resp.CreatedAt = example.CreatedAt
    resp.UpdatedAt = example.UpdatedAt
	// 返回数据
    return &resp, nil
}
```

- 定义样例仓储: 见 [example_application/module/example-module/repository/example_repository.go](./example_application/module/example-module/repository/example_repository.go)

```go
// ExampleRepository Example仓库，负责Example业务的数据持久化操作，继承fiberhouse.RepositoryLocator仓库定位器接口，具备获取上下文、配置、日志、注册实例等功能
type ExampleRepository struct {
	fiberhouse.RepositoryLocator
	Model *model.ExampleModel
}

func NewExampleRepository(ctx fiberhouse.IApplicationContext, m *model.ExampleModel) *ExampleRepository {
	return &ExampleRepository{
		RepositoryLocator: fiberhouse.NewRepository(ctx).SetName(GetKeyExampleRepository()),
		Model:             m,
	}
}

// GetKeyExampleRepository 获取 ExampleRepository 注册键名
func GetKeyExampleRepository(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleRepository", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleRepository 注册 ExampleRepository 到容器（延迟初始化）并返回注册key
func RegisterKeyExampleRepository(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleRepository(ns...), func() (interface{}, error) {
		m := model.NewExampleModel(ctx)
		return NewExampleRepository(ctx, m), nil
	})
}

// GetExampleById 根据ID获取Example示例数据
func (r *ExampleRepository) GetExampleById(id string) (*entity.Example, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := r.Model.GetExampleByID(ctx, id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, exception.GetNotFoundDocument() // 返回error
		}
		exception.GetInternalError().RespData(err.Error()).Panic() // 直接panic
	}
	return result, nil
}
```

- 定义样例模型: 见 [example_application/module/example-module/model/example_model.go](./example_application/module/example-module/model/example_model.go)

```go
// ExampleModel Example模型，继承MongoLocator定位器接口，具备获取上下文、配置、日志、注册实例等功能 以及基本的mongodb操作能力
type ExampleModel struct {
	dbmongo.MongoLocator
	ctx context.Context // 可选属性
}

func NewExampleModel(ctx fiberhouse.IApplicationContext) *ExampleModel {
	return &ExampleModel{
		MongoLocator: dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).SetDbName(constant.DbNameMongo).SetTable(constant.CollExample).
			SetName(GetKeyExampleModel()).(dbmongo.MongoLocator), // 设置当前模型的配置项名(mongodb)和库名(test)
		ctx: context.Background(),
	}
}

// GetKeyExampleModel 获取模型注册key
func GetKeyExampleModel(ns ...string) string {
	return fiberhouse.RegisterKeyName("ExampleModel", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleModel 注册模型到容器（延迟初始化）并返回注册key
func RegisterKeyExampleModel(ctx fiberhouse.IApplicationContext, ns ...string) string {
	return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleModel(ns...), func() (interface{}, error) {
		return NewExampleModel(ctx), nil
	})
}

// GetExampleByID 根据ID获取样例文档
func (m *ExampleModel) GetExampleByID(ctx context.Context, oid string) (*entity.Example, error) {
	_id, err := bson.ObjectIDFromHex(oid)
	if err != nil {
		exception.GetInputError().RespData(err.Error()).Panic()
	}
	filter := bson.D{{"_id", _id}}
	opts := options.FindOne().SetProjection(bson.M{
		"_id":     0,
		"profile": 0,
	})
	var example entity.Example
	err = m.GetCollection(m.GetColl()).FindOne(ctx, filter, opts).Decode(&example)
	if err != nil {
		return nil, err
	}
	return &example, nil
}
```
- 调用链路总结: 如 获取样例数据接口 GET /example/get/:id
  - 路由注册: RegisterRouteHandlers -> exampleGroup.Get("/get/:id", exampleApi.GetExample)
  - Api处理器: ExampleHandler.GetExample -> h.Service.GetExample
  - 服务层: ExampleService.GetExample -> s.Repo.GetExampleById
  - 仓储层: ExampleRepository.GetExampleById -> r.Model.GetExampleByID
  - 模型层: ExampleModel.GetExampleByID -> m.GetCollection(m.GetColl()).FindOne(...)
  - 实体层: entity.Example
  - 响应层: e.g. response.RespSuccess(resp).JsonWithCtx(c) -> response.RespInfo

### 如何添加新的模块和新的api
- 参考样例: [example_application/module/example-module](./example_application/module/example-module)

- 复制样例模块目录：从 `example-module` 目录复制一份作为新模块的起始模板

```bash

cp -r example_application/module/example-module example_application/module/mymodule
```

- 修改模块相关文件：
  - **常量定义**：修改 `constant/constants.go` 中的模块名称常量
  - **实体类型**：修改 `entity/types.go` 中的实体结构体定义
  - **模型层**：修改 `model/` 目录下的模型文件，更新模型名称和数据库表名
  - **仓储层**：修改 `repository/` 目录下的仓储文件，更新仓储接口和实现
  - **服务层**：修改 `service/` 目录下的服务文件，更新业务逻辑
  - **API层**：修改 `api/` 目录下的API控制器文件，更新接口定义

- 注册新模块API路由：在 `module/route_register.go` 中添加新模块路由注册

```go
// 在 RegisterApiRouters 函数中添加
mymodule.RegisterRouteHandlers(ctx, app)
```

- 更新Wire依赖注入：运行 `wire` 命令重新生成依赖注入代码
```bash
# 进入新模块的api目录
cd example_application/module/mymodule/api

# 运行wire命令生成依赖注入代码，指定生成代码文件的前缀
wire gen -output_file_prefix api_provider_
```

### task异步任务的使用样例

- 定义唯一任务名称: 见 [example_application/module/example-module/task/names.go](./example_application/module/example-module/task/names.go)

```go
package task

// A list of task types. 任务名称的列表
const (
	// TypeExampleCreate 定义任务名称，异步创建一个样例数据
	TypeExampleCreate = "ex:example:create:create-an-example"
)
```

- 新建任务: 见 [example_application/module/example-module/task/task.go](./example_application/module/example-module/task/task.go)

```go
/*
Task payload list 任务负载列表
*/

// PayloadExampleCreate 样例创建负载的数据
type PayloadExampleCreate struct {
	fiberhouse.PayloadBase // 继承基础负载结构体，自动具备获取json编解码器的方法
	/**
	负载的数据
	*/
	Age int8
}

// NewExampleCreateTask 生成一个 ExampleCreate 任务，从调用处获取相关参数，并返回任务
func NewExampleCreateTask(ctx fiberhouse.IContext, age int8) (*asynq.Task, error) {
	vo := PayloadExampleCreate{
		Age: age,
	}
	// 获取json编解码器，将负载数据编码为json格式的字节切片
	payload, err := vo.GetMustJsonHandler(ctx).Marshal(&vo)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExampleCreate, payload, asynq.Retention(24*time.Hour), asynq.MaxRetry(3), asynq.ProcessIn(1*time.Minute)), nil
}
```

- 定义任务处理器: 见 [example_application/module/example-module/task/handler/handle.go](./example_application/module/example-module/task/handler/handle.go)

```go
// HandleExampleCreateTask 样例任务创建的处理器
func HandleExampleCreateTask(ctx context.Context, t *asynq.Task) error {
	// 从 context 中获取 appCtx 全局应用上下文，获取包括配置、日志、注册实例等组件
	appCtx, _ := ctx.Value(fiberhouse.ContextKeyAppCtx).(fiberhouse.IApplicationContext)

	// 声明任务负载对象
	var p task.PayloadExampleCreate

	// 解析任务负载
	if err := p.GetMustJsonHandler(appCtx).Unmarshal(t.Payload(), &p); err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginWeb()).Str("From", "HandleExampleCreateTask").Err(err).Msg("[Asynq]: Unmarshal error")
		return err
	}

	// 获取处理任务的实例，注意service.TestService需在任务挂载阶段注册到全局管理器
    // 见 task/handler/mount.go: service.RegisterKeyTestService(ctx)
	instance, err := fiberhouse.GetInstance[*service.TestService](service.GetKeyTestService())
	if err != nil {
		return err
	}

	// 将负参数传入实例的处理函数
	result, err := instance.DoAgeDoubleCreateForTaskHandle(p.Age)
	if err != nil {
		return err
	}

	// 记录结果
	appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginTask()).Msgf("HandleExampleCreateTask 执行成功，结果 Age double: %d", result)
	return nil
}

```

- 任务挂载器: 见 [example_application/module/example-module/task/handler/mount.go](./example_application/module/example-module/task/handler/mount.go)

```go
package handler

import (
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
	"github.com/lamxy/fiberhouse"
)

// RegisterTaskHandlers 统一注册任务处理函数和依赖的组件实例初始化器
func RegisterTaskHandlers(tk fiberhouse.TaskRegister) {
	// append task handler to global taskHandlerMap
	// 通过RegisterKeyXXX注册任务处理的实例初始化器，并获取注册实例的keyName

	// 统一注册全局管理实例初始化器，该实例可在任务处理函数中通过tk.GetContext().GetContainer().GetXXXService()获取，用来执行具体的任务处理逻辑
	service.RegisterKeyTestService(tk.GetContext())

	// 统一追加任务处理函数到Task注册器对象的任务名称映射的属性中
	tk.AddTaskHandlerToMap(task.TypeExampleCreate, HandleExampleCreateTask)
}
```

- 将任务推送到队列: 见 [example_application/module/example-module/api/example_api.go](./example_application/module/example-module/api/example_api.go) 
  调用了 [example_application/module/example-module/service/example_service.go](./example_application/module/example-module/service/example_service.go) 的 GetExampleWithTaskDispatcher 方法

```go
// GetExampleWithTaskDispatcher 示例方法，演示如何在服务方法中使用任务调度器异步执行任务
func (s *ExampleService) GetExampleWithTaskDispatcher(id string) (*responsevo.ExampleRespVo, error) {
	resp := responsevo.ExampleRespVo{}
	example, err := s.Repo.GetExampleById(id)
	if err != nil {
		return nil, err
	}

	// 获取带任务标记的日志器，从全局管理器获取已附加了日志源标记的日志器
	log := s.GetContext().GetMustLoggerWithOrigin(s.GetContext().GetConfig().LogOriginTask())

	// 获取样例数据成功，推送延迟任务异步执行
	dispatcher, err := s.GetContext().(fiberhouse.IApplicationContext).GetStarterApp().GetTask().GetTaskDispatcher()
	if err != nil {
		log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher GetTaskDispatcher failed")
	}
	// 创建任务对象
	task1, err := task.NewExampleCreateTask(s.GetContext(), int8(example.Age))
	if err != nil {
		log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher NewExampleCountTask failed")
	}
	// 将任务对象入队
	tInfo, err := dispatcher.Enqueue(task1, asynq.MaxRetry(constant.TaskMaxRetryDefault), asynq.ProcessIn(1*time.Minute)) // 任务入队，并将在1分钟后执行

	if err != nil {
		log.Warn().Err(err).Msg("GetExampleWithTaskDispatcher Enqueue failed")
	} else if tInfo != nil {
		log.Warn().Msgf("GetExampleWithTaskDispatcher Enqueue task info: %v", tInfo)
	}

	// 正常的业务逻辑
	resp.ExamName = example.Name
	resp.ExamAge = example.Age
	resp.Courses = example.Courses
	resp.Profile = example.Profile
	resp.CreatedAt = example.CreatedAt
	resp.UpdatedAt = example.UpdatedAt
	return &resp, nil
}
```
### 缓存组件使用样例

- 见获取样例列表接口: [example_application/module/example-module/api/example_api.go](./example_application/module/example-module/api/example_api.go) 的 GetExamples 方法
  调用样例服务的 GetExamplesWithCache 方法: [example_application/module/example-module/service/example_service.go](./example_application/module/example-module/service/example_service.go)

```go

func (s *ExampleService) GetExamples(page, size int) ([]responsevo.ExampleRespVo, error) {
	// 从缓存选项池获取缓存选项对象
	co := cache.OptionPoolGet(s.GetContext())
	// 使用完的缓存选项对象归还对象池
	defer cache.OptionPoolPut(co)

	// 设置缓存参数: 二级缓存、启用本地缓存、设置缓存key、设置本地缓存随机过期时间(10秒±10%)、设置远程缓存随机过期时间(3分钟±1分钟)、写远程缓存同步策略、设置上下文、启用缓存全部的保护措施
	co.Level2().EnableCache().SetCacheKey("key:example:list:page:"+strconv.Itoa(page)+":size:"+strconv.Itoa(size)).SetLocalTTLRandomPercent(10*time.Second, 0.1).
		SetRemoteTTLWithRandom(3*time.Minute, 1*time.Minute).SetSyncStrategyWriteRemoteOnly().SetContextCtx(context.TODO()).EnableProtectionAll()

	// 获取缓存数据，调用缓存包的 GetCached 方法，传入缓存选项对象和获取数据的回调函数
	return cache.GetCached[[]responsevo.ExampleRespVo](co, func(ctx context.Context) ([]responsevo.ExampleRespVo, error) {
		list, err := s.Repo.GetExamples(page, size)

		if err != nil {
			return nil, err
		}
		examples := make([]responsevo.ExampleRespVo, 0, len(list))
		for i := range list {
			example := responsevo.ExampleRespVo{
				ID:       list[i].ID.Hex(),
				ExamName: list[i].Name,
				ExamAge:  list[i].Age,
				Courses:  list[i].Courses,
				Profile:  list[i].Profile,
				Timestamps: commonvo.Timestamps{
					CreatedAt: list[i].CreatedAt,
					UpdatedAt: list[i].UpdatedAt,
				},
			}
			examples = append(examples, example)
		}
		return examples, nil
	})
}
```

### CMD命令行应用使用样例

- 命令行框架应用main入口 : 见 [example_application/command/main.go](./example_application/command/main.go)

```go
package main

import (
	"github.com/lamxy/fiberhouse/example_application/command/application"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/commandstarter"
)

func main() {
	// bootstrap 初始化启动配置(全局配置、全局日志器)，配置路径为当前工作目录下的"./../config"
	cfg := bootstrap.NewConfigOnce("./../../example_config")

	// 全局日志器，定义日志目录为当前工作目录下的"./logs"
	logger := bootstrap.NewLoggerOnce(cfg, "./logs")

	// 初始化命令全局上下文
	ctx := fiberhouse.NewCmdContextOnce(cfg, logger)

	// 初始化应用注册器对象，注入应用启动器
	appRegister := application.NewApplication(ctx) // 需实现框架关于命令行应用的 fiberhouse.ApplicationCmdRegister接口

        // 实例化命令行应用启动器
        cmdlineStarter := &commandstarter.CMDLineApplication{
            // 实例化框架命令启动器对象
            FrameCmdStarter: commandstarter.NewFrameCmdApplication(ctx, option.WithCmdRegister(appRegister)),
            // 实例化核心命令启动器对象
            CoreCmdStarter: commandstarter.NewCoreCmdCli(ctx),
        }
	// 运行命令行启动器
	commandstarter.RunCommandStarter(cmdlineStarter)
}
```
- 编写一个命令脚本: 见 [example_application/command/application/commands/test_orm_command.go](./example_application/command/application/commands/test_orm_command.go)

```go
// TestOrmCMD 测试go-orm库的CURD操作命令，需实现 fiberhouse.CommandGetter 接口，通过 GetCommand 方法返回命令行命令对象
type TestOrmCMD struct {
	Ctx fiberhouse.IApplicationContext
}

func NewTestOrmCMD(ctx fiberhouse.IApplicationContext) fiberhouse.CommandGetter {
	return &TestOrmCMD{
		Ctx: ctx,
	}
}

// GetCommand 获取命令行命令对象，实现 fiberhouse.CommandGetter 接口的 GetCommand方法
func (m *TestOrmCMD) GetCommand() interface{} {
	return &cli.Command{
		Name:    "test-orm",
		Aliases: []string{"orm"},
		Usage:   "测试go-orm库CURD操作",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "method",
				Aliases:  []string{"m"},
				Usage:    "测试类型(ok/orm)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "operation",
				Aliases:  []string{"o"},
				Usage:    "CURD(c创建|u更新|r读取|d删除)",
				Required: false,
			},
			&cli.UintFlag{
				Name:     "id",
				Aliases:  []string{"i"},
				Usage:    "主键ID",
				Required: true,
			},
		},
		Action: func(cCtx *cli.Context) error {
			var (
				ems  *service.ExampleMysqlService
                wrap = component.NewWrap[*service.ExampleMysqlService]()
			)

			// 使用dig注入所需依赖，通过provide连缀方法连续注入依赖组件
			dc := m.Ctx.GetDigContainer().
				Provide(func() fiberhouse.IApplicationContext { return m.Ctx }).
				Provide(model.NewExampleMysqlModel).
				Provide(service.NewExampleMysqlService)

			// 错误处理
			if dc.GetErrorCount() > 0 {
				return fmt.Errorf("dig container init error: %v", dc.GetProvideErrs())
			}

			/*
			// 通过Invoke方法获取依赖组件，在回调函数中使用依赖组件
			err := dc.Invoke(func(ems *service.ExampleMysqlService) error {
				err := ems.AutoMigrate()
				if err != nil {
					return err
				}
				// 其他操作...
				return nil
			})
			*/

			// 另一种方式，使用泛型Invoke方法获取依赖组件，通过component.Wrap辅助类型来获取依赖组件
			err := component.Invoke[*service.ExampleMysqlService](wrap)
			if err != nil {
				return err
			}

			// 获取依赖组件
			ems = wrap.Get()

			// 自动创建一次数据表
			err = ems.AutoMigrate()
			if err != nil {
				return err
			}

			// 获取命令行参数
			method := cCtx.String("method")

			// 执行测试
			if method == "ok" {
				testOk := ems.TestOk()

				fmt.Println("result: ", testOk, "--from:", method)
			} else if method == "orm" {
				// 获取更多命令行参数
				op := cCtx.String("operation")
				id := cCtx.Uint("id")

				// 执行测试orm
				err := ems.TestOrm(m.Ctx, op, id)
				if err != nil {
					return err
				}

				fmt.Println("result: testOrm OK", "--from:", method)
			} else {
				return fmt.Errorf("unknown method: %s", method)
			}

			return nil
		},
	}
}
```
- 命令行构建： 见 [example_application/command/README_go_build.md](./example_application/command/README_go_build.md)

```bash
# 构建
cd command/  # command ROOT Directory
go build -o ./target/cmdstarter.exe ./main.go 

# 执行命令帮助
cd command/    ## work dir is ~/command/, configure path base on it
./target/cmdstarter.exe -h
```

- 命令行应用使用说明
  - 编译命令行应用: `go build -o ./target/cmdstarter.exe ./main.go `
  - 运行命令行应用查看帮助: `./target/cmdstarter.exe -h`
  - 运行测试go-orm库的CURD操作命令: `./target/cmdstarter.exe test-orm --method ok` 或 `./target/cmdstarter.exe test-orm -m ok`
  - 运行测试go-orm库的CURD操作命令(创建数据): `./target/cmdstarter.exe test-orm --method orm --operation c --id 1` 或 `./target/cmdstarter.exe test-orm -m orm -o c -i 1`
  - 子命令行参数帮助说明: `./target/cmdstarter.exe test-orm -h`


## 🔧 配置说明

### 应用全局配置
FiberHouse 支持基于环境的多配置文件管理，配置文件位于 example_config/ 目录。全局配置对象位于框架上下文对象中，可通过 ctx.GetConfig() 方法获取。

- 配置文件 README： 见 [example_config/README.md](./example_config/README.md)

- 配置文件命名规则

```
配置文件格式: application_[环境].yml
环境类型: dev | test | prod

示例文件:
- application_dev.yml     # 应用开发环境
- application_test.yml    # 应用测试环境  
- application_prod.yml    # 应用生产环境

```
- 环境变量配置

```
# 引导环境变量 (APP_ENV_ 前缀):
APP_ENV_application_env=prod       # 设置运行环境: dev/test/prod

# 配置覆盖环境变量 (APP_CONF_ 前缀):
APP_CONF_application_appName=MyApp              # 覆盖应用名称
APP_CONF_application_server_port=9090           # 覆盖服务端口
APP_CONF_application_appLog_level=error         # 覆盖日志级别
APP_CONF_application_appLog_asyncConf_type=chan # 覆盖异步日志类型

```
#### 核心配置项

- 应用基础配置:
```yaml
application:
  appName: "FiberHouse"           # 应用名称
  env: "dev"                      # 运行环境: dev/test/prod
  
  server:
    host: "127.0.0.1"              # 服务主机
    port: 8080                     # 服务端口
```
- 日志系统配置:
```yaml
application:
  appLog:
    level: "info"                # 日志级别: debug/info/warn/error
    enableConsole: true          # 启用控制台输出
    consoleJSON: false           # 控制台JSON格式
    enableFile: true             # 启用文件输出
    filename: "app.log"          # 日志文件名
    
    # 异步日志配置
    asyncConf:
      enable: true              # 启用异步日志
      type: "diode"             # 异步类型: chan/diode
      
    # 日志轮转配置  
    rotateConf:
      maxSize: 5                             # megabytes
      maxBackups: 5                          # 最大备份文件数
      maxAge: 7                              # days
      compress: false                        # disabled by default
```

- 数据库配置:
```yaml
# MySQL 配置
mysql:
  dsn: "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
  gorm:
    maxIdleConns: 10                       # 最大空闲连接数
    maxOpenConns: 100                      # 最大打开连接数
    connMaxLifetime: 3600                  # 连接最大生命周期，单位秒
    connMaxIdleTime: 300                   # 连接最大空闲时间，单位秒
    logger:
      level: info                        # 日志级别: silent、error、warn、info
      slowThreshold: 200 * time.Millisecond # 慢SQL阈值，建议 200 * time.Millisecond，根据实际业务调整
      colorful: false                    # 是否彩色输出
      enable: true                       # 是否启用日志记录
      skipDefaultFields: true            # 跳过默认字段
  pingTry: false
```

- redis配置:
```yaml
redis:
  host: "127.0.0.1"
  port: 6379
  password: ""
  database: 0
  poolSize: 100                # 连接池大小
  
  # 集群配置 (可选)
  cluster:
    addrs: ["127.0.0.1:6379"]
    poolSize: 100
```
- 缓存系统配置:
```yaml
cache:
  # 本地缓存
  local:                                     # 本地缓存配置
    numCounters: 1000000                     # 100万个计数器
    maxCost: 134217728                       # 最大缓存128M
    bufferItems: 64                          # 每个缓存分区的缓冲区大小
    metrics: true                            # 是否启用缓存指标
    IgnoreInternalCost: false                # 是否忽略内部开销
      
  # 远程缓存  
  redis:                                     # remote 远程缓存配置
    host: 127.0.0.1                          # Redis 服务器地址
    port: 6379                               # Redis 服务器端口
    password: ""                             # Redis 服务器密码
  # 异步池配置
  asyncPool:                               # 启用二级缓存时的异步goroutine池配置，用于处理缓存更新和同步策略
    ants:                                  # ants异步goroutine池配置
      local:
        size: 248                          # 本地缓存异步goroutine池大小
        expiryDuration: 5                  # 单位秒，空闲goroutine超时时间
        preAlloc: false                    # 不预分配
        maxBlockingTasks: 512              # 最大阻塞任务数
        nonblocking: false                 # 允许阻塞
```

- 任务组件配置
```yaml
  task:
    enableServer: true                       # 是否启用任务调度服务组件支持
```
- 更多配置按需自定义

- 完整配置示例参考：
  - 测试环境配置: [example_config/application_test.yml](./example_config/application_test.yml)
  - 命令行测试环境配置: [application_test.yml](./example_config/application_test.yml)


## 🤝 贡献指南

### 快速开始
- Fork 仓库并 Clone
- 创建分支：git checkout -b feature/your-feature
- 开发并保持格式：go fmt ./... && golangci-lint run
- 运行测试：go test ./... -race -cover
- 提交：feat(module): 描述
- 推送并发起 PR

### 分支策略
- main：稳定发布
- develop：集成开发
- feature/*：功能
- fix/*：缺陷
- 其它分类

### PR 要求
- 标题：与提交信息一致
- 内容：背景 / 方案 / 影响 / 测试 / 关联 Issue
- CI 通过

### 安全
安全漏洞请私信：pytho5170@hotmail.com

## 📄 许可证

本项目基于 MIT 许可证开源 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙋‍♂️ 支持与反馈

- 如果您感兴趣，或者支持FiberHouse的持续开发，请在GitHub上点个星[GitHub Star](https://github.com/lamxy/fiberhouse/stargazers)
- 问题反馈: [Issues](https://github.com/lamxy/fiberhouse/issues)
- 联系邮箱: pytho5170@hotmail.com

## 🌟 致谢

感谢以下开源项目：

- [gofiber/fiber](https://github.com/gofiber/fiber) - 高性能 HTTP 内核
- [rs/zerolog](https://github.com/rs/zerolog) - 高性能结构化日志
- [knadh/koanf](https://github.com/knadh/koanf) - 灵活的多源配置管理
- [bytedance/sonic](https://github.com/bytedance/sonic) - 高性能 JSON 编解码
- [dgraph-io/ristretto](https://github.com/dgraph-io/ristretto) - 高性能本地缓存
- [hibiken/asynq](https://github.com/hibiken/asynq) - 基于 Redis 的分布式任务队列
- [go.mongodb.org/mongo-driver](https://github.com/mongodb/mongo-go-driver) - MongoDB 官方驱动
- [gorm.io/gorm](https://gorm.io) - ORM 抽象与 MySQL 支撑
- [redis/go-redis](https://github.com/redis/go-redis) - Redis 客户端
- [panjf2000/ants](https://github.com/panjf2000/ants) - 高性能 goroutine 池

同时感谢：
- [swaggo/swag](https://github.com/swaggo/swag) 提供 API 文档生成
- [google/wire](https://github.com/google/wire)、[uber-go/dig](https://github.com/uber-go/dig) 支持依赖注入模式
- 以及所有未逐一列出的优秀项目

最后感谢：GitHub Copilot 提供的资料查阅、文档整理和编码辅助能力。