# Package recover 提供 Fiber 框架的全局异常恢复和错误处理中间件。

该包实现了完整的 panic 恢复机制、结构化错误处理和详细的堆栈跟踪功能，
支持多种错误类型的分类处理，并提供灵活的调试模式配置。

## 核心功能

- 全局 panic 恢复和异常捕获
- 多种错误类型的分类处理（ValidateException、Exception、fiber.Error、runtime.Error）
- 可配置的堆栈跟踪和调试信息输出
- 请求上下文信息记录（参数、查询、请求体）
- 框架日志系统集成
- 调试标志头部检测
- JSON 序列化错误数据处理

## 快速开始

基本使用方式：

	// 创建恢复中间件
	app := fiber.New(fiber.Config{
		ErrorHandler: recoverCatch.ErrorHandler,
	})

	// 使用默认配置
	app.Use(recover.New())

	// 使用自定义配置
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		DebugMode: true,
		StackTraceHandler: recoverCatch.DefaultStackTraceHandler,
	}))

集成应用上下文：

	// 创建恢复处理器实例
	recoverCatch := recover.NewRecoverCatch(appCtx)

	// 配置 Fiber 全局错误处理器
	app := fiber.New(fiber.Config{
		ErrorHandler: recoverCatch.ErrorHandler,
	})

	// 添加中间件
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: recoverCatch.DefaultStackTraceHandler,
		AppContext: appCtx,
		DebugMode: false,
	}))

## 错误类型处理

该包支持以下错误类型的分类处理：

1. ValidateException - 数据验证错误
- 完整输出验证错误信息到客户端
- HTTP 状态码：400 Bad Request

2. Exception - 业务逻辑异常
- 调试模式下输出完整错误信息
- 生产模式下隐藏敏感信息
- HTTP 状态码：400 Bad Request

3. fiber.Error - Fiber 框架错误
- 保留原始 HTTP 状态码
- 调试模式下显示详细错误信息

4. runtime.Error - 运行时错误
- 自动识别空指针和内存访问错误
- 转换为友好的错误消息

5. 通用 error - 标准错误接口
- 统一处理未知类的错误类型

## 配置选项

支持的配置参数：

	type Config struct {
		// 跳过中间件的条件函数
		Next func(c *fiber.Ctx) bool

		// 是否启用堆栈跟踪
		EnableStackTrace bool

		// 自定义堆栈跟踪处理器
		StackTraceHandler func(c *fiber.Ctx, e interface{})

		// 应用上下文
		AppContext fiberhouse.ContextFramer

		// 调试模式开关
		DebugMode bool
	}

配置文件示例（xxx.yaml）：

	recover:
	  debugFlag: "X-Debug-Flag"
	  debugFlagValue: "your-secret-debug-key"
	  enablePrintStack: true
	  enableDebugFlag: true
	  debugMode: false

	trace:
	  requestID: "traceId"

## 调试功能

该包提供强大的调试功能：

1. 调试模式控制
- debugMode: 全局调试开关
- enablePrintStack: 是否打印堆栈信息
- enableDebugFlag: 是否启用调试标志头部检测

2. 调试标志头部
- 通过 HTTP 头部动态启用调试模式，服务器日志将记录详细的堆栈调式信息

3. 详细请求信息记录
- 请求参数（路径参数）
- 查询参数
- 请求体内容
- 请求追踪 ID

调试使用示例：

	// 在请求头中添加调试标志
	curl -H "X-Debug-Flag: your-secret-debug-key" \
	     -X POST http://localhost:8080/api/users \
	     -d '{"name":"test"}'

## 堆栈跟踪

提供两种堆栈跟踪方式：

1. CaptureStack() - 优化的堆栈跟踪
- 过滤无关调用栈
- 格式化输出
- 性能更好

2. StackMsg() - 完整堆栈跟踪
- 使用 debug.Stack() 获取完整信息
- 包含所有调用栈信息

## 日志集成

该包与框架日志系统深度集成：

- 支持结构化日志输出
- 自动包含请求追踪 ID
- 分级日志记录（Debug、Error、Warn）
- JSON 格式的请求数据记录

日志输出示例：

	{
		"level": "error",
		"traceId": "abc123",
		"Code": 1001,
		"Msg": "validation failed",
		"Data": {"field": "email", "error": "invalid format"},
		"reqParams": {"id": "123"},
		"reqQueries": {"page": "1"},
		"reqBody": {"email": "invalid-email"},
		"time": "2024-01-01T10:00:00Z",
		"message": "validation error occurred"
	}

## 性能考虑

1. 堆栈跟踪优化
- 默认使用轻量级堆栈跟踪
- 仅在必要时启用完整堆栈

2. JSON 序列化
- 复用框架的 JSON 编码器
- 避免重复序列化

3. 内存管理
- 使用对象池减少内存分配
- 及时释放 DataWrap 资源

## 安全注意事项

1. 调试信息泄露
- 生产环境建议关闭 debugMode
- 使用安全的调试标志密钥

2. 敏感数据保护
- 请求体中的敏感信息会被记录
- 建议配置日志过滤规则 TODO 支持热更新

3. 错误信息控制
- 区分调试模式和生产模式的错误输出
- 避免向客户端暴露内部错误详情  TODO 支持MASK接口