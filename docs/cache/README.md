# Package cache 提供高性能的多级缓存系统，支持本地缓存、远程缓存和二级缓存架构，内置多种缓存保护机制。

该包是应用框架的缓存模块，提供统一的缓存接口和灵活的配置选项，包括：
- 多级缓存架构：本地缓存、远程缓存（Redis）、二级缓存
- 灵活的同步策略：同步/异步双写、仅写远程等
- 完善的保护机制：单飞保护、布隆过滤器、熔断器
- 智能 TTL 管理：固定 TTL、随机 TTL、百分比随机
- 高性能对象池：减少内存分配和 GC 压力
- 泛型支持：类型安全的缓存操作

## 缓存架构

支持三种缓存级别：
- Local：仅使用本地内存缓存（最快）
- Remote：仅使用远程 Redis 缓存（持久化）
- Level2：二级缓存，本地 + 远程组合使用（最优）

## 同步策略

二级缓存支持四种同步策略：
- WriteBoth：同步双写本地和远程缓存
- WriteRemoteOnly：仅同步写入远程缓存
- AsyncWriteBoth：异步双写本地和远程缓存
- AsyncWriteRemoteOnly：仅异步写入远程缓存

## 保护机制

内置三种缓存保护机制：
- SingleFlight：防止缓存击穿，相同 key 并发请求合并
- BloomFilter：防止缓存穿透，快速判断 key 是否存在
- CircuitBreaker：防止缓存雪崩，服务降级保护

## 基本使用示例

	// 从对象池获取配置选项（推荐方式）
	co := cache.OptionPoolGet(s.GetContext())
	defer cache.OptionPoolPut(co)

	// 配置二级缓存选项
	option := co.SetCacheKey("user:123").
		Level2().
		SetLocalTTLRandomPercent(10*time.Second, 0.1). // 本地缓存 9-11 秒随机
		SetRemoteTTLWithRandom(5*time.Minute, 30*time.Second). // 远程缓存 4.5-5.5 分钟随机
		SetSyncStrategyWriteRemoteOnly().
		EnableProtectionAll() // 启用所有保护机制

	// 使用泛型方法获取缓存数据
	result, err := cache.GetCached[*User](option, func(ctx context.Context) (*User, error) {
		// 缓存未命中时的数据获取逻辑
		return userRepo.GetByID(123)
	})

## 配置选项详解

	// 创建新的配置选项
	co := cache.NewCacheOption(s.GetContext())

	// 缓存级别设置
	co.Local()     // 仅本地缓存
	co.Remote()    // 仅远程缓存
	co.Level2()    // 二级缓存

	// TTL 配置方式
	co.SetLocalTTL(5 * time.Minute)                           // 固定 TTL
	co.SetLocalTTLWithRandom(5*time.Minute, 1*time.Minute)    // 绝对随机范围
	co.SetLocalTTLRandomPercent(5*time.Minute, 0.2)           // 百分比随机（±20%）

	// 同步策略设置
	co.SetSyncStrategyWriteBoth()              // 同步双写
	co.SetSyncStrategyWriteRemoteOnly()        // 同步仅写远程
	co.SetSyncStrategyAsyncWriteBoth()         // 异步双写
	co.SetSyncStrategyAsyncWriteRemoteOnly()   // 异步仅写远程

	// 保护机制设置
	co.EnableSingleFlight()     // 击穿保护
	co.EnableBloomFilter()      // 穿透保护
	co.EnableCircuitBreaker()   // 雪崩保护
	co.EnableProtectionAll()    // 启用所有保护

## 高级使用示例

	// 复杂业务场景：用户列表分页缓存
	func (s *UserService) GetUserList(page, size int) ([]*User, error) {
		co := cache.OptionPoolGet(s.GetContext())
		defer cache.OptionPoolPut(co)

		// 构建缓存 key
		cacheKey := fmt.Sprintf("user:list:page:%d:size:%d", page, size)

		// 配置缓存选项
		option := co.SetCacheKey(cacheKey).
			Level2().                                              // 二级缓存
			SetLocalTTLRandomPercent(30*time.Second, 0.1).         // 本地缓存随机 TTL
			SetRemoteTTLRandomPercent(10*time.Minute, 0.2).        // 远程缓存随机 TTL
			SetSyncStrategyAsyncWriteRemoteOnly().                 // 异步写远程
			EnableSingleFlight().                                  // 防击穿
			SetContextCtx(context.WithTimeout(context.Background(), 5*time.Second))

		// 获取缓存数据
		return cache.GetCached[[]*User](option, func(ctx context.Context) ([]*User, error) {
			return s.userRepo.FindUsers(page, size)
		})
	}

## 缓存操作 API

	// 获取缓存（泛型方法）
	data, err := cache.GetCached[T](option, dataLoader)

	// TODO 设置缓存
	err := cache.SetCache(option, key, data)

	// TODO 删除缓存
	err := cache.DeleteCache(option, key)

	// TODO 检查缓存是否存在
	exists, err := cache.ExistsCache(option, key)

## 对象池优化

为减少内存分配和 GC 压力，推荐使用对象池：

	// 推荐方式：使用对象池
	co := cache.OptionPoolGet(ctx)
	defer cache.OptionPoolPut(co) // 重要：使用完必须归还

	// 或者使用 Clone 方法复制配置
	coNew := co.Clone(newContext)
	defer cache.OptionPoolPut(coNew)

	// 不推荐：频繁创建新实例
	// co := cache.NewCacheOption(ctx) // 会增加 GC 压力

## TTL 随机化策略

支持多种 TTL 随机化方式防止缓存雪崩：

	// 1. 固定 TTL（无随机）
	co.SetLocalTTL(5 * time.Minute)

	// 2. 绝对时间范围随机
	co.SetLocalTTLWithRandom(5*time.Minute, 1*time.Minute) // 4-6 分钟随机

	// 3. 百分比范围随机（推荐）
	co.SetLocalTTLRandomPercent(5*time.Minute, 0.2) // ±20% 随机，即 4-6 分钟

	// 获取实际 TTL
	actualTTL := co.GetLocalTTL()    // 每次调用返回新的随机值
	baseTTL := co.GetLocalBaseTTL()  // 返回基础 TTL，不含随机

## 配置验证

	// 验证配置是否有效
	if err := option.Valid(); err != nil {
		return fmt.Errorf("invalid cache option: %w", err)
	}

	// 获取 TTL 配置信息（调试用）
	ttlInfo := option.GetTTLInfo()
	log.Printf("TTL Info: %+v", ttlInfo)

## 性能考虑

缓存性能优化建议：
- 优先使用对象池获取 CacheOption 实例
- 合理设置 TTL 随机范围，避免缓存雪崩
- 根据业务场景选择合适的缓存级别和同步策略
- 为热点数据启用单飞保护
- 为可能不存在的 key 启用布隆过滤器
- 在缓存服务不稳定时启用熔断器

## 错误处理

缓存操作的错误处理模式：
- 缓存获取失败时，执行数据加载函数
- 缓存设置失败时，记录错误但不影响业务逻辑
- 缓存服务不可用时，通过熔断器降级到直接调用数据源

## 并发安全性

CacheOption 实例在配置阶段（设置参数）时不是并发安全的，但在使用阶段（读取参数）是安全的。
推荐使用模式：
- 单个 goroutine 中完成 CacheOption 的配置
- 配置完成后可以在多个 goroutine 中安全使用
- 使用对象池时，每个 goroutine 获取独立的实例

## 扩展和自定义

支持自定义缓存实例：
- 通过 SetDefaultInstanceKey 指定自定义缓存实例
- 支持自定义 JSON 序列化器：SetJsonWrapper
- 支持自定义上下文：SetContextCtx

## 监控和调试

提供丰富的状态查询方法：
- GetTTLInfo()：获取 TTL 配置详情
- GetSingleFlightState()：检查单飞保护状态
- GetBloomFilterState()：检查布隆过滤器状态
- GetCircuitBreakerState()：检查熔断器状态

## 子包说明

- cache2/：二级缓存实现
- cachelocal/：本地内存缓存实现
- cacheremote/：远程 Redis 缓存实现