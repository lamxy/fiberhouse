# 缓存

FiberHouse 在 [`cache`](../../cache/) 下定义统一 `Cache` 接口，并分别提供基于 Ristretto 的本地缓存、基于 go-redis 的远程缓存，以及组合二者的 L2 缓存。框架不会自动创建这些实例：应用需要在启动期为自己的实例 key 注册 initializer，并通过 `IApplication` 的 `GetLocalCacheKey`、`GetRemoteCacheKey`、`GetLevel2CacheKey` 让读缓存工具定位它们。

缓存代码已有可用路径，但 L2 生命周期和保护机制仍有明显限制。[功能状态](../reference/feature-status.md) 因此把本地/Redis 基础实现列为已接入，把 L2 与保护机制列为实验性能力。

## 接口与实现

```go
type Cache interface {
	Get(context.Context, string, *CacheOption) (string, error)
	Set(context.Context, string, any, *CacheOption) error
	Delete(context.Context, ...string) error
	Close() error
	Wait() error
	GetLevel() Level
}
```

| 实现 | 构造入口 | 主要行为 |
|---|---|---|
| `cachelocal.LocalCache` | `cachelocal.NewLocalCache(ctx, confPath...)` | Ristretto 内存缓存；异步接纳写入；可选 metrics |
| `cacheremote.RedisDb` | `cacheremote.NewRedisDb(ctx, confPath...)` | Redis 字符串读写、TTL、健康检查和可选保护器 |
| `cache2.Level2Cache` | `cache2.NewLevel2Cache(ctx, local, remote)` | 先本地后远端、远端命中回填、四种写策略和两个 ants pool |

构造器的默认配置路径分别是 `cache.local` 与 `cache.redis`。这是 package 常量，不是全局实例 key。L2 的 pool 配置固定读取 `cache.asyncPool.ants.local.*` 和 `cache.asyncPool.ants.remote.*`；构造函数不接收自定义配置前缀。

本地缓存实际从 `<local-base>` 读取以下键；未传 `confPath` 时 `<local-base>` 是 `cache.local`：

- `numCounters`、`maxCost`、`bufferItems`；
- `metrics`、`ignoreInternalCost`。

Redis 实际从 `<redis-base>` 读取连接地址、认证、DB、pool 和超时配置，包括 `host`、`port`、`password`、`db`、`poolSize`、`minIdleConns`、`dialTimeout`、`readTimeout`、`writeTimeout`、`poolTimeout`、`connMaxIdleTime`、`connMaxLifetime`。未传 `confPath` 时 `<redis-base>` 是 `cache.redis`；这些时间数值会乘以 `time.Second`。

示例 YAML 当前写有 `IgnoreInternalCost` 和 `idleTimeout`，而构造器读取的是大小写不同的 `ignoreInternalCost` 以及 `connMaxIdleTime`/`connMaxLifetime`。正式配置必须按消费方键名核对，不能把示例字段当作已生效的框架默认。

[`example_application`](../../example_application/) 中的 `Application.ConfigGlobalInitializers` 展示了把 local、Redis 和 L2 注册到 GlobalManager 的一种方式。`KEY_LOCAL_CACHE` 等名称、哪些 key 被列为启动必需项，以及远程缓存是否复用 Redis 实例，都是示例应用的选择，不是框架默认。

## `CacheOption`

每次缓存操作都携带 `*CacheOption`。新对象默认启用缓存，写策略是 `WriteRemoteOnly`，但缓存级别为零、key 为空、请求 context 也为空；直接交给 `GetCached` 会因级别或 key 校验失败。至少应明确级别、key、context 与 TTL：

```go
co := cache.OptionPoolGet(appCtx)
defer cache.OptionPoolPut(co)

co.Level2().
	SetCacheKey("customer:42").
	SetContextCtx(ctx).
	SetLocalTTLRandomPercent(30*time.Second, 0.1).
	SetRemoteTTLWithRandom(10*time.Minute, time.Minute).
	SetSyncStrategyWriteRemoteOnly()
```

`SetJsonWrapper` 可指定序列化器；未指定时，`GetJsonWrapper` 通过应用的 `GetDefaultTrafficCodecKey()` 从默认 GlobalManager 取 `fiberhouse.JsonWrapper`，取不到或类型错误会 panic。`SetDefaultInstanceKey`/`GetDefaultInstanceKey` 当前只有字段访问器，`GetCached` 并不读取它，而是按缓存级别调用应用的三个 key getter。

`OptionPoolGet` 取得的对象只属于当前同步调用链。调用方负责最终 `OptionPoolPut`/`Release`，归还后不能继续引用或跨 goroutine 使用。L2 异步写会自行 `Clone` option，并在任务结束或提交失败时释放 clone；调用方仍只释放原 option。

## TTL、随机抖动与序列化

本地和远端 TTL 相互独立。固定 TTL 由 `SetLocalTTL`/`SetRemoteTTL` 设置；随机范围可用绝对时长或百分比设置。每次调用 `GetLocalTTL`/`GetRemoteTTL` 都重新计算 `base ± range`；若结果不大于零，则回退到 base。零 TTL 的具体含义由 Ristretto/Redis 底层实现决定，应用应显式配置而不是依赖零值。

本地和 Redis 的 `Set` 对 `string`、`[]byte` 直接存储，其他类型用 option 的 JSON wrapper 序列化。`Get` 始终返回字符串；类型恢复由调用方或 `GetCached` 完成。改变 codec、结构体字段或 JSON 兼容性会影响旧缓存值的可读性，key 设计应包含必要的 schema/version 维度。

## Read-through：`GetCached`

`cache.GetCached[R]` 的顺序是：

1. 校验 option；option 通过 `DisableCache` 禁用缓存时直接调用 loader。校验发生在开关判断之前，因此禁用时仍需提供合法级别、key 和 AppCtx。
2. 按 `Local`、`Remote`、`Level2` 从应用 key 获取缓存实例。
3. 命中时把字符串 JSON 解码为 `R`。
4. 普通 miss 或其他 `Get` 错误时调用 loader，再序列化并写回；写回失败只记录日志，仍返回 loader 数据。
5. Bloom 拒绝错误直接返回，不调用 loader；circuit breaker 打开时优先调用可选 fallback，否则返回错误。

因此 `GetCached` 把网络错误、反序列化前的缓存读取错误和普通 miss 大多归入同一 loader 路径，但命中后的 JSON 解码失败会直接返回错误，不会删除坏值或调用 loader。它也没有围绕 loader 建立 singleflight：Redis 层的 singleflight 只合并同一 key 的 Redis `Get`，并不能阻止多个 miss 随后并发执行 loader。

loader 会收到 `CacheOption.GetContextCtx()`。应传请求/任务 context；不要依赖 nil 或无条件使用 `context.Background()` 来绕过取消和超时。

## L2 读取、回填与写策略

L2 `Get` 先读 local，再读 remote。远端命中后会回填 local：当策略为 `AsyncWriteBoth` 或 `AsyncWriteRemoteOnly` 时走 local pool 异步回填，其他策略同步回填。回填本地失败只记录日志，远端值仍作为命中返回。

`Set` 支持：

| 策略 | 行为 |
|---|---|
| `WriteBoth` | 两个临时 goroutine 并行写 local 与 remote，等待并汇总错误 |
| `WriteRemoteOnly` | 同步只写 remote；local 的旧值不会自动删除 |
| `AsyncWriteBoth` | 分别提交 local/remote pool，调用立即返回 |
| `AsyncWriteRemoteOnly` | 只提交 remote pool，调用立即返回 |

`Delete` 总是并行删除两级并汇总错误。使用 remote-only 写策略时，调用方必须自行设计本地失效，否则旧 local 值仍优先于新 remote 值。

`WriteBoth` 把同一个 `*CacheOption` 同时传给 local/remote goroutine。值为非 `string`/`[]byte` 且 option 尚未设置 `jsonWrapper` 时，两端都可能调用会懒写字段的 `GetJsonWrapper()`，存在数据竞争的源码静态风险。进入并行 `Set` 前应先解析 codec 并调用 `SetJsonWrapper(codec)`，让两个 goroutine 只读 option；这里是控制流观察，尚未通过 race 测试复现。

异步 local/remote 操作分别使用 1 秒和 3 秒派生超时。提交失败或后台写失败只记日志，不传播给已经返回的 `Set`。pool 的 `nonblocking`、容量和阻塞任务上限决定背压/拒绝行为，必须按业务延迟和峰值评估，不能照搬示例数值。

## Redis 保护机制

只有 `<redis-base>.protection.enable=true` 时，`NewRedisDb` 才向 GlobalManager 注册默认 `shardedBloomFilter` 和 `wrapCircuitBreaker` initializer，并按 `<redis-base>.protection.type.*.selected` 取得实现；默认 `<redis-base>` 是 `cache.redis`。随后还必须在每次 `CacheOption` 上开启对应开关，保护逻辑才会运行。

- singleflight：合并 Redis `Get`，不合并 read-through loader。
- Bloom filter：filter 判定“不存在”时仍尝试一次 Redis。未启用 breaker 时，Redis miss 先转成 `ErrRedisNil`，再转成 `ErrRejectedByBloomFilter`，使 `GetCached` 不执行 loader；新合法 key 的冷启动语义需要应用自行验证。
- circuit breaker：只包裹 Redis `Get`；`Set` 中的 breaker 分支当前被注释，写入不受保护。breaker 分支遇到 Redis miss 时保留原始 `redis.Nil`，`GetCached` 把它当普通错误并调用 loader；只有 breaker 打开/半开拒绝被转换成 `ErrCircuitBreakerOpen`，此时才使用可选 fallback。

因此 `EnableProtectionAll()` 并不保证产生 Bloom 拒绝：当 Bloom 判定 key 不存在且 breaker 同时启用时，内部 Redis miss 仍可能以 `redis.Nil` 到达 `GetCached` 并进入 loader。保护开关的组合语义需要按实际业务 miss 场景分别测试，不能把三项开关理解成简单叠加。

默认分片 Bloom 的索引用 `hash & (shardCount-1)`，但构造器只修正非正数，没有强制 shard 数是 2 的幂。保护器的 `Reset` 当前也是空实现。上述结论来自源码控制流检查，未通过故障注入或压力测试复现；采用前应补充 miss、恢复、误判和负载测试。

## Wait、metrics 与关闭

Ristretto 写入有异步接纳过程；需要“Set 后立即 Get”时调用 local `Wait()`。Redis `Wait()` 只检查未关闭并返回 nil。L2 `Wait()` 并行调用底层两个 `Wait()`，忽略二者错误，也不等待 ants pool 中的 L2 异步任务，不能作为完整 flush 屏障。

Local metrics 只有在 `<local-base>.metrics=true` 时可用；默认 `<local-base>` 是 `cache.local`。`GetMetricsInfo` 汇总 hit/miss、eviction、drop 等计数；L2 还暴露 local metrics 和 pool capacity/running/free。L2 的 local metrics 方法直接把 local 断言为 `*cachelocal.LocalCache`，替换自定义 local 实现时可能 panic。

关闭顺序应是：先停止请求与任务生产者，再阻止新缓存操作，等待应用自己追踪的工作，最后关闭 L2 或其底层资源。不要同时让 L2 和其他所有者重复关闭共享 Redis/local 实例。

当前 L2 `Close` 有以下静态限制：

- 关闭 `stopCh` 后最多等待 pool 的 running 数归零，再释放两个 pool 和两个底层缓存；排队状态与业务完成语义没有统一确认。
- 方法没有把 `closed` 置为 true；成功关闭后普通方法的前置检查仍可能通过。
- 第二次 `Close` 会再次关闭 `stopCh`，存在 panic 风险。
- 构造 pool 失败使用 fatal 日志路径，构造函数本身不返回 error。

Local `Close` 重复调用返回 nil；Redis 第二次 `Close` 返回 `ErrCacheClosed`。这些不对称意味着应用必须指定唯一所有者并只关闭一次。GlobalManager 的 `ClearAll(true)` 不会逐项调用 `Close`，`Release` 又有已知 panic 风险，不能替代显式缓存回收；详见[《GlobalManager》](global-manager.md)。

源码入口：[`cache/cache_interface.go`](../../cache/cache_interface.go)、[`cache/cache_option.go`](../../cache/cache_option.go)、[`cache/cache_utility.go`](../../cache/cache_utility.go)、[`cache/cachelocal`](../../cache/cachelocal/)、[`cache/cacheremote`](../../cache/cacheremote/) 与 [`cache/cache2`](../../cache/cache2/)。
