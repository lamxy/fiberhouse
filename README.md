# FiberHouse Framework

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![Fiber Version](https://img.shields.io/badge/fiber-v2.x-green.svg)](https://github.com/gofiber/fiber)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
<img src="https://img.shields.io/github/issues/lamxy/fiberhouse.svg" alt="GitHub Issues"></img>


📖 [中文](README.md) | [English](./docs/README_en.md)

## 🏠 关于 FiberHouse

FiberHouse 是基于 Fiber 的高性能、可装配的 Go Web 框架，内置全局管理器、配置器、统一日志器、验证包装器以及数据库、缓存、中间件、统一异常处理等框架组件，开箱即用。

- 提供了强大的全局管理容器，支持自定义组件一次注册到处使用的能力，方便开发者按需替换和功能扩展，
- 在框架层面约定了应用启动器、全局上下文、业务分层等接口以及内置默认实现，支持自定义实现和模块化开发，
- 使得 FiberHouse 像装配"家具"的"房子"一样可以按需构建灵活的、完整的 Go Web 应用。

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

## 🏗️ 架构说明

```
fiberhouse/                              # FiberHouse 框架核心
├── 接口定义层
│   ├── application_interface.go    # 应用启动器接口定义
│   ├── command_interface.go        # 命令行应用接口定义  
│   ├── context_interface.go        # 全局上下文接口定义
│   ├── json_wraper_interface.go    # JSON 包装器接口定义
│   ├── locator_interface.go        # 服务定位器接口定义
│   └── model_interface.go          # 数据模型接口定义
├── 应用启动层
│   ├── applicationstarter/         # Web 应用启动器实现
│   │   └── web_starter.go          # 基于 Fiber 的应用启动器
│   ├── commandstarter/             # 命令行应用启动器实现
│   │   └── cmdline_starter.go      # 命令行应用启动器
│   └── bootstrap/                  # 应用引导程序
│       └── bootstrap.go            # 统一引导入口
├── 配置管理层
│   └── appconfig/                  # 应用配置管理
│       └── config.go               # 多格式配置文件加载和管理
├── 全局管理层
│   ├── globalmanager/              # 全局对象容器管理
│   │   ├── interface.go            # 全局管理器接口
│   │   ├── manager.go              # 全局管理器实现
│   │   └── types.go                # 全局管理器类型定义
│   └── global_utility.go           # 全局工具函数
├── 数据访问层
│   └── database/                   # 数据库驱动支持
│       ├── dbmysql/                # MySQL 数据库组件
│       │   ├── interface.go        # MySQL 接口定义
│       │   ├── mysql.go            # MySQL 连接实现
│       │   └── mysql_model.go      # MySQL 模型基类
│       └── dbmongo/                # MongoDB 数据库组件
│           ├── interface.go        # MongoDB 接口定义
│           ├── mongo.go            # MongoDB 连接实现
│           └── mongo_model.go      # MongoDB 模型基类
├── 缓存系统层
│   └── cache/                      # 高性能缓存组件
│       ├── cache_interface.go      # 缓存接口定义
│       ├── cache_option.go         # 缓存配置选项
│       ├── cache_utility.go        # 缓存工具函数
│       ├── cache_errors.go         # 缓存错误定义
│       ├── helper.go               # 缓存助手函数
│       ├── cache2/                 # 二级缓存实现
│       │   └── level2_cache.go     # 本地+远程二级缓存
│       ├── cachelocal/             # 本地缓存实现
│       │   ├── local_cache.go      # 内存缓存实现
│       │   └── type.go             # 本地缓存类型
│       └── cacheremote/            # 远程缓存实现
│           ├── cache_model.go      # 远程缓存模型基类
│           └── redis_cache.go      # Redis 缓存实现
├── 组件库层
│   └── component/                  # 框架核心组件
│       ├── dig_container.go        # 基于dig依赖注入容器包装
│       ├── jsoncodec/              # JSON 编解码器
│       │   └── sonicjson.go        # 基于 Sonic 的高性能 JSON编解码器
│       ├── jsonconvert/            # JSON 转换工具
│       │   └── convert.go          # 转换核心实现
│       ├── mongodecimal/           # MongoDB 十进制处理
│       │   └── mongo_decimal.go    # MongoDB Decimal128 支持
│       ├── validate/               # 参数验证组件
│       │   ├── type_interface.go   # 验证器接口定义
│       │   ├── validate_wrapper.go # 验证器包装实现
│       │   ├── en.go               # 英文验证器实现
│       │   ├── zh_cn.go            # 简体中文验证器实现
│       │   ├── zh_tw.go            # 繁体中文验证器实现
│       │   └── example/            # 注册示例
│       ├── tasklog/                # 任务日志组件
│       │   └── logger_adapter.go   # 日志适配器
│       └── writer/                 # 日志写入器
│           ├── async_channel_writer.go     # 异步通道写入器
│           ├── async_diode_writer.go       # 异步二极管写入器
│           ├── async_diode_writer_test.go  # 异步写入器测试
│           └── sync_lumberjack_writer.go   # 同步滚动日志写入器
├── 中间件层
│   └── middleware/                 # HTTP 中间件
│       └── recover/                # 异常恢复中间件
│           ├── config.go           # 恢复中间件配置
│           └── recover.go          # 恢复中间件实现
├── 响应处理层
│   └── response/                   # 统一响应处理
│       └── response.go             # 响应对象池和序列化
├── 异常处理层
│   └── exception/                  # 统一异常处理
│       ├── types.go                # 异常类型定义
│       └── exception_error.go      # 异常错误实现
├── 工具层
│   ├── utils/                      # 通用工具函数
│   │   └── common.go               # 通用工具实现
│   └── constant/                   # 框架常量
│       ├── constant.go             # 全局常量定义
│       └── exception.go            # 异常常量定义
├── 业务分层
│   ├── api.go                      # API 层接口定义
│   ├── service.go                  # 服务层接口定义
│   ├── repository.go               # 仓储层接口定义
│   └── task.go                     # 任务层接口定义
└── 占位模块
    ├── mq/                         # 消息队列（待实现）
    ├── plugins/                    # 插件支持（待实现）
    └── component/
        ├── i18n/                   # 国际化（待实现）
        └── rpc/                    # RPC 支持（待实现）
        
```

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
	"github.com/lamxy/fiberhouse/example_application"
	"github.com/lamxy/fiberhouse/example_application/module"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/applicationstarter"
	"github.com/lamxy/fiberhouse/bootstrap"
)

func main() {
	// bootstrap 初始化启动配置(全局配置、全局日志器)，配置目录默认为当前工作目录"."下的`example_config/`
	// 可以指定绝对路径或基于工作目录的相对路径
	cfg := bootstrap.NewConfigOnce("./example_config")
	
	// 日志目录默认为当前工作目录"."下的`example_main/logs`
	// 可以指定绝对路径或基于工作目录的相对路径
	logger := bootstrap.NewLoggerOnce(cfg, "./example_main/logs")

	// 初始化全局应用上下文
	appContext := fiberhouse.NewAppContextOnce(cfg, logger)

	// 初始化应用注册器、模块/子系统注册器和任务注册器对象，注入到应用启动器
	appRegister := example_application.NewApplication(appContext)  // 需实现应用注册器接口，见fiberhouse.ApplicationRegisterer接口定义，参考example_application/application.go样例实现
	moduleRegister := module.NewModule(appContext)  // 需实现模块注册器接口，见样例模块module/module.go的实现
	taskRegister := module.NewTaskAsync(appContext)  // 需实现任务注册器接口，见样例任务module/task.go的实现

        // 实例化 Web 应用启动器
        webStarter := &applicationstarter.WebApplication{
            // 实例化框架启动器
            FrameStarter: applicationstarter.NewFrameApplication(appContext,
              option.WithAppRegister(appRegister),
              option.WithModuleRegister(moduleRegister),
              option.WithTaskRegister(taskRegister),
            ),
            // 实例化核心应用启动器
            CoreStarter: applicationstarter.NewCoreFiber(appContext),
        }
	
	// 运行框架应用启动器
	applicationstarter.RunApplicationStarter(webStarter)
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

# 设置cmd应用的环境变量，windows环境，将读取application_cmd_dev.yml配置文件
set APP_ENV_application_appType=cmd
set APP_ENV_application_env=dev

# Linux or MacOS 环境
# export APP_ENV_application_appType=cmd
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

## 📖 使用指南

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
│   ├── application.go                  # 应用注册器实现
│   ├── constant.go                     # 应用级常量定义
│   └── customizer_interface.go         # 应用定制器接口
├── API 接口层
│   └── api-vo/                         # API 值对象定义
│       ├── commonvo/                   # 通用 VO
│       │   └── vo.go                   # 通用值对象
│       └── example/                    # 示例模块 VO
│           ├── api_interface.go        # API 接口定义
│           ├── requestvo/              # 请求 VO
│           │   └── example_reqvo.go    # 示例请求对象
│           └── responsevo/             # 响应 VO
│               └── example_respvo.go   # 示例响应对象
├── 命令行框架应用层
│   └── command/                        # 命令行程序
│       ├── main.go                     # 命令行main入口
│       ├── README_go_build.md          # 构建说明
│       ├── application/                
│       │   ├── application.go          # 命令应用配置和逻辑
│       │   ├── constants.go            # 命令常量
│       │   ├── functions.go            # 命令工具函数
│       │   └── commands/               # 具体命令脚本实现
│       │       ├── test_orm_command.go # ORM 测试命令
│       │       └── test_other_command.go # 其他更多开发的命令脚本...
│       ├── component/                  # 命令行组件
│       │   ├── cron.go                 # 定时任务组件
│       │   └── readme.md               # 组件说明
│       └── target/                     # 构建产物
│           └── cmdstarter.exe          # 命令行可执行文件
├── 异常处理层
│   ├── get_exceptions.go               # 异常获取器
│   └── example-module/                 # 示例模块异常，其他模块异常，每个模块独立目录
│       └── exceptions.go               # 模块异常汇总
├── 中间件层
│   └── middleware/                     # 应用级中间件
│       └── register_app_middleware.go  # 应用中间件注册器
├── 模块(子系统)层
│   └── module/                         # 业务模块
│       ├── module.go                   # 模块注册器
│       ├── route_register.go           # 路由注册器
│       ├── swagger.go                  # Swagger 文档配置
│       ├── task.go                     # 异步任务注册器
│       ├── api/                        # 模块级 API 中间件
│       │   └── register_module_middleware.go
│       ├── command-module/             # 命令行脚本专用的业务模块
│       │   ├── entity/                 # 实体定义
│       │   │   └── mysql_types.go      # MySQL 类型定义
│       │   ├── model/                  # 数据模型
│       │   │   ├── mongodb_model.go    # MongoDB 模型
│       │   │   └── mysql_model.go      # MySQL 模型
│       │   └── service/                # 业务服务
│       │       ├── example_mysql_service.go  # MySQL 服务示例
│       │       └── mongodb_service.go        # MongoDB 服务示例
│       ├── common-module/           # 通用模块
│       │   ├── attrs/                  # 属性定义
│       │   │   └── attr1.go            # 属性示例
│       │   ├── command/                # 通用命令
│       │   ├── fields/                 # 通用字段
│       │   │   └── timestamps.go       # 时间戳字段
│       │   ├── model/                  # 通用模型
│       │   ├── repository/             # 通用仓储
│       │   ├── service/                # 通用服务
│       │   └── vars/                   # 通用变量
│       │       └── vars.go             # 变量定义
│       ├── constant/                # 常量定义
│       │   └── constants.go            # 模块常量
│       └── example-module/          # 用于展示的核心样例模块
│           ├── api/                    # API 控制器层
│           │   ├── api_provider_wire_gen.go    # Wire 依赖注入生成文件
│           │   ├── api_provider.go             # API 提供者，提供依赖关系
│           │   ├── common_api.go               # 通用 API 控制器
│           │   ├── example_api.go              # 示例 API 控制器
│           │   ├── health_api.go               # 健康检查 API 控制器
│           │   ├── README_wire_gen.md          # Wire 生成说明
│           │   └── register_api_router.go      # API 路由注册
│           ├── dto/                    # 数据传输对象
│           ├── entity/                 # 实体层
│           │   └── types.go            # 类型定义
│           ├── model/                  # 模型层
│           │   ├── example_model.go            # 示例模型
│           │   ├── example_mysql_model.go      # MySQL 示例模型
│           │   └── model_wireset.go            # 模型 Wire 集合
│           ├── repository/             # 仓储层
│           │   ├── example_repository.go       # 示例仓储
│           │   ├── health_repository.go        # 健康检查仓储
│           │   └── repository_wireset.go       # 仓储 Wire 集合
│           ├── service/                # 服务层
│           │   ├── example_service.go          # 示例服务
│           │   ├── health_service.go           # 健康检查服务
│           │   ├── service_wireset.go          # 服务 Wire 集合
│           │   └── test_service.go             # 测试服务
│           └── task/                   # 任务层
│               ├── names.go            # 任务名称定义
│               ├── task.go             # 任务注册器
│               └── handler/            # 任务处理器
│                   ├── handle.go       # 任务处理逻辑
│                   └── mount.go        # 任务挂载器
├── 工具层
│   └── utils/                          # 应用工具
│       └── common.go                   # 通用工具函数
└── 自定义验证器层
    └── validatecustom/                 # 自定义验证器
        ├── tag_register.go             # 标签注册器
        ├── validate_initializer.go     # 验证器初始化
        ├── tags/                       # 自定义标签
        │   ├── new_tag_hascourses.go   # 课程验证标签
        │   └── tag_startswith.go       # 前缀验证标签
        └── validators/                 # 多语言验证器
            ├── ja.go                   # 日语验证器
            ├── ko.go                   # 韩语验证器
            └── langs_const.go          # 语言常量
```

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
	return response.RespSuccess(resp).JsonWithCtx(c)
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
		exception.GetInternalError().RespError(err.Error()).Panic() // 直接panic
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
		exception.GetInputError().RespError(err.Error()).Panic()
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
配置文件格式: application_[应用类型]_[环境].yml
应用类型: web | cmd
环境类型: dev | test | prod

示例文件:
- application_web_dev.yml     # Web应用开发环境
- application_web_test.yml    # Web应用测试环境  
- application_web_prod.yml    # Web应用生产环境
- application_cmd_test.yml    # 命令行应用测试环境

```
- 环境变量配置

```
# 引导环境变量 (APP_ENV_ 前缀):
APP_ENV_application_appType=web    # 设置应用类型: web/cmd
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
  appType: "web"                  # 应用类型: web/cmd
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
  - 测试环境配置: [example_config/application_web_test.yml](./example_config/application_web_test.yml)
  - 命令行测试环境配置: [application_cmd_test.yml](./example_config/application_cmd_test.yml)


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