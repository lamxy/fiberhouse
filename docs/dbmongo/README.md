# Package dbmongo 提供基于 MongoDB 的数据库连接和模型操作功能。

该包是应用框架的 MongoDB 数据库组件，提供完整的 MongoDB 操作抽象层，
包括连接池管理、多数据库操作、集合操作封装等核心功能。

## 核心功能

- MongoDB 客户端连接和连接池管理
- 数据库和集合的统一访问接口
- 模型层的抽象和封装
- 读写分离和连接选项配置
- 全局对象容器集成
- 异常处理和错误管理

## 快速开始

	// 创建 MongoDB 连接
	mongoDb, err := dbmongo.NewMongoDb(appCtx)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// 使用自定义配置路径
	mongoDb, err := dbmongo.NewMongoDb(appCtx, "custom.mongodb")
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

## 连接池配置

推荐的连接池配置：

	uri := "mongodb://user:pass@host:27017/dbname"
	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).                    // 最大连接数
		SetMinPoolSize(10).                     // 最小连接数
		SetMaxIdleTimeMS(300000).              // 连接空闲时间（5分钟）
		SetMaxConnIdleTime(10*time.Minute).    // 最大空闲时间
		SetServerSelectionTimeout(5*time.Second). // 服务器选择超时
		SetConnectTimeout(10*time.Second)       // 连接超时

配置文件示例（xxx.yml）：

	mongodb:
	  applyURI: mongodb://admin:admin@localhost:27037/?authSource=admin
	  maxPoolSize: 100                         # 设置连接池最大连接数
	  minPoolSize: 10                          # 设置连接池最小连接数
	  maxConnIdleTime: 600                     # 设置最大空闲连接时间
	  connectTimeout: 10                       # 接超时时间
	  clientTimeout: 5                         # 客户端套接字超时时间
	  heartbeatInterval: 10                    # 心跳间隔时间
	  pingTry: false                           # 启动时ping尝试连接

## 错误处理策略

该包采用分层错误处理机制：

- 连接级错误：数据库连接失败等关键错误会触发 panic
- 操作级错误：CRUD 操作失败返回标准 error 类型
- 参数级错误：数据库名称或集合名称为空时触发 panic
- 业务级错误：通过框架异常处理机制统一管理

错误处理示例：

	mongoDb, err := dbmongo.NewMongoDb(appCtx)
	if err != nil {
		// 处理连接错误
		log.Printf("Database connection failed: %v", err)
		return err
	}

	// 检查连接健康状态
	if !mongoDb.IsHealthy() {
		log.Println("Database connection is unhealthy")
		// 尝试重建连接
		if _, err := mongoDb.Rebuild(); err != nil {
			log.Printf("Failed to rebuild connection: %v", err)
		}
	}

## 并发安全性

MongoDB 驱动和该包的并发安全特性：

- MongoDB 官方驱动程序本身是线程安全的
- MongoModel 实例可以在多个 goroutine 中安全使用
- 连接池自动管理并发连接，无需额外同步
- 建议为每个请求创建独立的 context.Context
- 内部使用读写锁保护连接重建操作

## 性能优化指南

1. 连接池调优
    - 根据应用负载调整 maxPoolSize 和 minPoolSize
    - 监控连接池使用率，避免连接数不足或过多

2. 查询优化
    - 为频繁查询的字段创建合适的索引
    - 使用聚合管道替代复杂的多次查询
    - 合理使用投影减少网络传输数据量

3. 批量操作
    - 使用 BulkWrite 进行批量写入操作
    - 批量查询时使用 cursor 迭代大结果集

4. 事务使用
    - 仅在必要时使用事务，避免长时间锁定
    - 保持事务尽可能短小和快速

5. 监控和诊断
    - 定期检查连接池状态和查询性能
    - 使用 MongoDB 性能分析工具识别慢查询