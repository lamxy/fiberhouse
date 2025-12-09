# Package response 提供了统一的HTTP响应格式和高性能的响应对象管理功能。

该包实现了标准化的JSON响应结构，支持对象池优化，减少GC压力，提高并发性能。

主要功能：
- 统一的响应格式 (code, msg, data)
- 对象池管理，减少内存分配
- 便捷的成功/错误响应创建方法
- Fiber框架集成支持
- 异常和验证错误的专用处理

基本用法：

	// 创建成功响应
	resp := response.RespSuccess(map[string]string{"user": "john"})
	defer resp.Release() // 显示释放回对象池

	// 创建错误响应
	errResp := response.RespError(400, "参数错误")

	// 在Fiber中使用，c为 fiber.Ctx上下文对象，JsonWithCtx内部自动调用release将自己隐式放回池中
	return resp.JsonWithCtx(c, 200)

对象池使用：

	// 使用对象池（推荐，高性能）
	resp := response.GetRespInfo()
	resp.Reset(0, "success", data)
	defer resp.Release()

	// 直接创建（特殊场景）
	resp := response.NewRespInfoWithoutPool(0, "success", data)

性能特性：
- 使用sync.Pool减少内存分配
- 自动重置字段防止数据泄露
- 支持高并发场景
- 零拷贝JSON序列化

注意事项：
- 使用对象池时必须调用Release()方法
- 避免在Release()后继续使用对象
- 长时间持有对象请使用WithoutPool方法