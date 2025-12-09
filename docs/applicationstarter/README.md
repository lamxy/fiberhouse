# Package applicationstarter 提供基于 Fiber 框架的应用启动器实现，负责应用的完整生命周期管理和启动流程编排。

该包是应用框架的启动引擎，提供标准化的应用启动流程和生命周期管理，包括：
- 应用启动流程的标准化编排和执行
- 全局对象容器的初始化和管理
- 中间件、路由、钩子函数的注册管理
- 异步任务服务器的启动和管理
- 应用健康检测和保活机制
- 优雅关闭和资源清理

## 启动流程

应用启动按以下顺序执行，确保依赖关系正确：
1. RegisterToCtx：注册启动器到上下文
2. RegisterApplicationGlobals：注册全局对象和验证器
3. InitCoreApp：初始化 Fiber 核心应用
4. RegisterAppHooks：注册应用生命周期钩子
5. RegisterAppMiddleware：注册应用级中间件
6. RegisterModuleInitialize：注册模块级组件
7. RegisterModuleSwagger：注册 Swagger 文档
8. RegisterTaskServer：启动异步任务服务器
9. RegisterGlobalsKeepalive：启动健康检测
10. AppCoreRun：启动应用监听

## 基本使用示例

	// 创建应用上下文
	ctx := bootstrap.NewAppContextOnce(appConfig, logger) // 依赖引导包的应用配置和日志器

	// 创建注册器实例
	appRegister := application.NewApplication(ctx)
	moduleRegister := module.NewModule(ctx)
	taskRegister := task.NewTask(ctx)

	// 创建应用启动器
	starter := applicationstarter.NewFrameApplication(ctx,
		appRegister,
		moduleRegister,
		taskRegister,
	)

	// 执行应用启动流程
	applicationstarter.RunApplicationStarter(starter)

## 默认配置或传入自定义配置启动

	// 使用默认 Fiber 配置
	defaultConfig := fiber.Config{
		AppName:      "xxx",
		Concurrency:  256 * 1024,
		...
	}

	// 启动时传入自定义配置
	applicationstarter.RunApplicationStarter(starter, customConfig)

## 应用注册器 实现 ApplicationRegister 接口，并注册绑定到启动器的应用属性上

## 模块注册器 实现 ModuleRegister 接口，并注册绑定到启动器的模块属性上

## 任务注册器 实现 TaskRegister 接口，并注册绑定到启动器的任务属性上

## 参考的项目结构示例

		project/
		├── application/                    # 应用层（业务实现）
		│   ├── application.go              # 应用注册器实现
		│   ├── constant.go                # 应用常量定义
		│   ├── api-vo/                    # API 数据传输对象
		│   │   ├── commonvo/              # 通用 VO
		│   │   └── example/               # 示例模块 VO
		│   ├── command/                   # 命令行工具
		│   │   ├── main.go
		│   │   └── application/           # 命令行应用逻辑
		│   ├── exceptions/                # 异常定义
		│   │   └── example-module/        # 模块异常
		│   ├── middleware/                # 中间件注册
		│   │   └── register_app_middleware.go
		│   ├── module/                    # 业务模块
		│   │   ├── example-module/        # example模块/子系统
		│   │   ├── module.go              # 模块注册器
		│   │   ├── route_register.go      # 路由注册
		│   │   ├── swagger.go             # Swagger 注册
		│   │   ├── task.go                # 任务注册器
		│   │   └── api/                   # API 模块
		│   ├── utils/                     # 工具函数
		│   └── validatecustom/            # 自定义验证器
		│       ├── tag_register.go
		│       ├── tags/                  # 自定义标签
		│       └── validators/            # 多语言验证器
		│
		├── frame/                         # 框架核心（基础设施层）
		│   ├── applicationstarter/        # 应用启动器
		│   │   └── frame_application.go   # 启动器实现
	    ...

## 配置文件示例

应用配置文件 application_web_dev.yml：

	application:
	  appName: "ExampleWebApp"
	  server:
	    host: "0.0.0.0"
	    port: "8080"

## 生命周期管理

应用启动器提供完整的生命周期管理：

	// 启动阶段
	- 全局对象初始化（数据库、缓存、Redis 等）
	- 验证器配置（多语言支持、自定义标签）
	- 应用级中间件注册（CORS、压缩、认证等）
	- 模块级中间件和路由注册
	- Swagger 文档注册
	- 异步任务服务器启动
	- 健康检测启动

	// 运行阶段
	- HTTP 服务监听和请求处理
	- 异步任务执行
	- 全局对象健康检测和自动重建
	- 错误恢复和日志记录

	// 关闭阶段
	- 接收关闭信号（SIGINT、SIGTERM）
	- 停止接收新请求
	- 等待现有请求完成
	- 清理全局资源
	- 关闭数据库连接
	- 关闭日志器

## 健康检测机制

内置健康检测功能，定期检查全局对象状态：
- 自动检测数据库连接健康状态
- 自动检测 Redis 连接状态
- 自动检测缓存实例状态
- 不健康对象自动重建
- 可配置检测间隔
- 异常情况结构化日志记录

## 错误处理和恢复

提供多层次的错误处理机制：
- 全局 Panic 恢复中间件
- 结构化错误响应格式
- 业务异常分类处理
- 调用堆栈追踪
- 调试模式支持

## 扩展和自定义

支持多种扩展方式：
- 自定义全局对象初始化器
- 自定义验证器标签和多语言支持
- 自定义中间件注册逻辑
- 自定义路由分组和处理器
- 自定义异常类型和处理
- 自定义任务类型和处理器