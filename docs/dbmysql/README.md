# Package dbmysql 提供基于 MySQL 的数据库连接和 GORM ORM 操作功能。

该包是应用框架的 MySQL 数据库组件，基于 GORM v2 提供完整的 MySQL 数据库操作抽象层，
包括连接池管理、日志集成、健康检查、连接重建等核心功能。

## 核心功能

- MySQL 数据库连接和连接池管理
- GORM ORM 框架集成和配置
- 框架日志器与 GORM 日志的适配
- 数据库连接健康检查和自动重连
- 连接池参数优化和性能调优
- 配置文件驱动的连接管理
- 并发安全的连接操作

## 快速开始

	// 创建 MySQL 连接（使用默认配置）
	mysqlDb, err := dbmysql.NewMysqlDb(appCtx)
	if err != nil {
		log.Fatal("Failed to connect to MySQL:", err)
	}

	// 使用自定义配置路径
	mysqlDb, err := dbmysql.NewMysqlDb(appCtx, "custom.mysql")
	if err != nil {
		log.Fatal("Failed to connect to MySQL:", err)
	}

	// 获取 GORM 实例进行数据库操作
	db := mysqlDb.Client
	var users []User
	result := db.Find(&users)

## 配置文件示例

配置文件结构（xxx.yaml）：

	mysql:
	 dsn: "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
	 gorm:
	   maxIdleConns: 20                       # 最大空闲连接数
	   maxOpenConns: 200                      # 最大打开连接数
	   connMaxLifetime: 7200                  # 连接最大生命周期，单位秒
	   connMaxIdleTime: 300                   # 连接最大空闲时间，单位秒
	   logger:
	       level: info                        # 日志级别: silent、error、warn、info
	       slowThreshold: 200 * time.Millisecond # 慢SQL阈值，建议 200 * time.Millisecond，根据实际业务调整
	       colorful: false                    # 是否彩色输出
	       enable: true                       # 是否启用日志记录
	       skipDefaultFields: true            # 跳过默认字段
	 pingTry: false

## GORM 日志集成

该包提供了 GormLoggerAdapter 适配器，将框架的统一日志系统与 GORM 日志无缝集成：

- 自动识别 GORM 日志级别并映射到框架日志级别
- 支持慢查询日志记录和分析
- 统一的日志格式和输出目标
- 可配置的日志详细程度和过滤规则

日志适配特性：

	// GORM 日志会自动适配到框架日志器
	mysqlDb.Client.Debug().Find(&users)  // 调试日志
	mysqlDb.Client.Find(&users)          // 普通查询日志

## 连接池优化

该包提供了完整的连接池配置支持：

	连接池参数说明：
	- MaxIdleConns：    最大空闲连接数，建议设置为 MaxOpenConns 的 10-20%
	- MaxOpenConns：    最大打开连接数，根据应用并发量和数据库承载能力设置
	- ConnMaxLifetime： 连接最大生存时间，建议 1-2 小时
	- ConnMaxIdleTime： 连接最大空闲时间，建议 5-10 分钟

## 健康检查和重连

该包提供了完善的连接健康检查和自动重连机制：

	// 检查数据库连接健康状态
	if !mysqlDb.IsHealthy() {
		log.Println("Database connection is unhealthy")

		// 尝试重建连接
		newDb, err := mysqlDb.Rebuild()
		if err != nil {
			log.Printf("Failed to rebuild connection: %v", err)
		} else {
			log.Println("Database connection rebuilt successfully")
		}
	}

	// 使用新配置重建连接
	newDb, err := mysqlDb.Rebuild("backup.mysql")
	if err != nil {
		log.Printf("Failed to rebuild with new config: %v", err)
	}

## 错误处理策略

该包采用分层错误处理机制：

- 连接级错误：DSN 配置错误、网络连接失败等关键错误会返回 error
- 配置级错误：缺少必要配置参数时返回详细的错误信息
- 运行时错误：连接池耗尽、查询超时等运行时问题通过日志记录
- 业务级错误：GORM 操作错误由调用方处理

## 并发安全性

该包的并发安全特性：

- GORM 实例本身是线程安全的，可以在多个 goroutine 中安全使用
- 连接池自动管理并发连接，无需额外同步
- 连接重建操作使用读写锁保护，确保并发安全
- 建议为每个请求创建独立的 context.Context

## 性能优化建议

1. 连接池调优
    - 监控连接池使用情况，调整 maxOpenConns 和 maxIdleConns
    - 设置合适的连接生存时间，避免长时间占用连接

2. 查询优化
    - 使用 GORM 的预编译语句功能（PrepareStmt: true）
    - 合理使用索引，避免全表扫描
    - 使用批量操作代替单条记录操作

3. 日志优化
    - 生产环境建议设置日志级别为 warn 或 error
    - 启用慢查询日志分析（slowThreshold）
    - 关闭不必要的详细日志（skipDefaultFields: true）

4. 事务管理
    - 合理使用事务，避免长时间锁定
    - 使用 GORM 的 SavePoint 功能处理嵌套事务