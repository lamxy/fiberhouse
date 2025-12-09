# Package bootstrap 提供应用程序启动时的核心初始化功能，包括应用配置管理和日志系统的初始化。

该包是整个应用框架的引导模块，负责在应用启动时最先完成的必要的初始化工作，包括：
- 应用配置的加载和管理（支持多环境配置）
- 全局日志系统的初始化（支持同步/异步日志）
- 环境变量的解析和配置覆盖

## 配置管理

支持基于环境的多配置文件加载机制：
- 通过环境变量 APP_ENV_application_appType 设置应用类型（web/cmd）
- 通过环境变量 APP_ENV_application_env 设置运行环境（dev/test/prod）
- 配置文件命名规则：application_[appType]_[env].yml
- 支持环境变量覆盖配置文件：APP_CONF_[配置文件的配置路径] = value，将覆盖配置文件配置路径中的对应值

## 日志系统

提供统一的日志管理接口和多种写入模式：
- 支持控制台输出和文件输出
- 支持同步和异步日志写入
- 异步模式支持 channel 和 diode 两种实现
- 集成日志轮转功能（基于 lumberjack）
- 提供结构化日志和来源标识

## 使用示例

	// 初始化配置（使用默认配置路径 ./config）
	config := bootstrap.NewConfigOnce()

	// 初始化配置（指定配置路径）
	config := bootstrap.NewConfigOnce("/app/config")

	// 初始化日志系统（使用默认日志路径 ./logs）
	logger := bootstrap.NewLoggerOnce(config)

	// 初始化日志系统（指定日志路径）
	logger := bootstrap.NewLoggerOnce(config, "/app/logs")

	// 使用日志记录
	logger.Info().Msg("应用启动成功")
	logger.ErrorWith(appconfig.LogOriginFrame).Err(err).Msg("初始化失败")

## 环境变量配置

引导环境变量（APP_ENV_ 前缀）：

	APP_ENV_application_appType=web    # 设置应用类型
	APP_ENV_application_env=prod       # 设置运行环境

配置覆盖环境变量（APP_CONF_ 前缀）：

	APP_CONF_application_appLog_level=error           # 覆盖日志级别
	APP_CONF_application_server_port=9090             # 覆盖服务端口
	APP_CONF_application_appLog_asyncConf_type=chan   # 覆盖异步日志类型

## 线程安全

所有全局初始化函数（NewConfigOnce、NewLoggerOnce）都使用 sync.Once 确保线程安全，
即使在并发环境下多次调用也只会执行一次初始化。