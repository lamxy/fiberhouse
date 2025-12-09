# Package appconfig 提供应用配置管理功能，支持多格式配置文件加载、线程安全的配置读写操作和日志源管理。

该包是应用框架的配置管理核心，提供以下主要功能：
- YAML 配置文件的加载和解析
- 线程安全的配置项读写操作
- 多层级配置合并和覆盖机制
- 日志源分类管理和自定义注册
- 全局对象容器集成
- 中间件开关状态管理

# 基本使用示例

	// 创建配置对象
	config := appconfig.NewAppConfig()

	// 加载 YAML 配置文件
	config.LoadYaml("application.yml")

	// 设置应用基本信息
	config.SetAppName("MyApp")
	config.SetVersion("1.0.0")

	// 初始化配置属性
	config.Initialize()

	// 获取配置值
	dbHost := config.String("database.host", "localhost")
	dbPort := config.Int("database.port", 3306)
	enableCache := config.Bool("cache.enable")

# 多配置文件加载

	// 按优先级加载多个配置文件（后加载覆盖先加载的配置）
	config.LoadYaml("application.yml").
		LoadYaml("application_dev.yml").
		LoadDefault(map[string]interface{}{
			"debug": true,
			"server.port": 8080,
		})

# 线程安全的配置操作

	// 安全读取配置
	value, err := config.SafeGet("database.connection",
		func(key string, conf appconfig.IAppConfig) (interface{}, error) {
			return conf.String(key), nil
		})

	// 安全设置配置
	err := config.SafeSet("runtime.status", "running",
		func(key string, val interface{}, conf appconfig.IAppConfig) error {
			// 自定义设置逻辑
			return nil
		})

# 日志源管理

	// 使用预定义日志源
	webLogOrigin := config.LogOriginWeb()
	dbLogOrigin := config.LogOriginDatabase()
	cacheLogOrigin := config.LogOriginCache()

	// 注册自定义日志源
	customOrigin := appconfig.LogOrigin("payment")
	err := config.RegisterLogOrigin("payment", customOrigin)
	if err != nil {
		log.Fatal("Failed to register log origin:", err)
	}

	// 获取日志源的实例键（用于全局容器）
	instanceKey := customOrigin.InstanceKey()

	// 获取所有日志源
	originMap := config.GetLogOriginMap()

# 配置文件结构示例

	application:
	  appName: "ExampleApp"
	  version: "1.0.0"
	  server:
	    host: "0.0.0.0"
	  appLog:
	    level: "info"
	    logOriginEnum:
	      web: "WEB"
	      api: "API"
	      payment: "PAYMENT"
	  recover:
	    debugMode: true
	cache:
	  redis:
	    host: "localhost"
	    port: 6379
	    db: 0

# 配置结构体获取

	// 获取应用基础配置
	appConf := config.GetApplication()
	fmt.Printf("App: %s, Version: %s\n", appConf.AppName, appConf.Version)

	// 获取恢复配置
	recoverConf := config.GetRecover()
	if recoverConf.debugMode {
		fmt.Println("debugMode is true")
	}

# 泛型配置获取

	// 获取底层配置对象的泛型方法
	koanfConfig, err := appconfig.GetCoreWithConfig[*koanf.Koanf](config)
	if err != nil {
		log.Fatal("Failed to get core config:", err)
	}

	// 使用底层配置对象的高级功能
	exists := koanfConfig.Exists("database.host")
	rawData := koanfConfig.Raw()

# 中间件开关管理

	// 检查中间件开关状态
	corsEnabled := config.GetMiddlewareSwitch("cors")
	compressEnabled := config.GetMiddlewareSwitch("compress")
	recoverEnabled := config.GetMiddlewareSwitch("recover")

	// 根据开关状态注册中间件
	if corsEnabled {
		app.Use(cors.New())
	}
	if compressEnabled {
		app.Use(compress.New())
	}

# 配置路径管理

	// 设置自定义配置文件目录
	config.SetConfPath("/etc/myapp/config")

	// 获取当前配置路径
	confPath := config.GetConfPath()

	// 从自定义路径加载配置
	config.LoadYaml("custom_config.yml")

# 配置类型转换

	// 支持多种数据类型的配置获取
	stringVal := config.String("app.name", "default-app")
	stringSlice := config.Strings("app.tags", []string{"default"})
	intVal := config.Int("server.port", 8080)
	int64Val := config.Int64("server.maxConnections", 1000)
	floatVal := config.Float64("server.timeout", 30.0)
	boolVal := config.Bool("debug.enabled")
	durationVal := config.Duration("server.readTimeout", 30*time.Second)
	bytesVal := config.GetBytes("security.key", []byte("default-key"))

# 预定义日志源

框架提供以下预定义日志源，用于不同组件的日志分类：
- LogOriginCoreHttp：HTTP 请求相关日志
- LogOriginFrame：框架自身日志
- LogOriginRecover：错误恢复日志
- LogOriginWeb：Web 业务日志
- LogOriginCMD：命令行日志
- LogOriginTask：异步任务日志
- LogOriginCache：缓存操作日志
- LogOriginDatabase：数据库操作日志
- LogOriginMq：消息队列日志
- LogOriginMongodb：MongoDB 日志
- LogOriginMysql：MySQL 日志
- LogOriginTest：测试相关日志

# 并发安全性

AppConfig 在初始化阶段非并发安全，在运行时阶段是只读安全的：
- 应用启动阶段：可以使用 Load* 方法和 Set* 方法
- 运行时阶段：建议只使用 Get* 方法进行只读访问
- 运行时写操作：必须使用 SafeGet 和 SafeSet 方法
- 全局容器：本身提供并发安全保障

# 配置加载顺序

配置加载遵循"后加载覆盖先加载"的原则：
1. LoadDefault：加载默认 map 配置
2. LoadYaml：加载 YAML 文件配置
3. LoadFunc：自定义加载逻辑
4. 环境变量：可覆盖配置文件中的值

# TODO 优化

1. 将配置按模块分组，使用层级结构组织
2. 为所有配置项提供合理的默认值
3. 使用类型安全的获取方法，避免类型转换错误
4. 在应用启动阶段完成所有配置加载和初始化
5. 运行时优先使用只读方法访问配置
6. 合理使用日志源分类，便于日志分析和问题定位
7. 利用全局容器管理配置相关的单例对象
8. 使用 SafeGet/SafeSet 处理运行时配置变更需求