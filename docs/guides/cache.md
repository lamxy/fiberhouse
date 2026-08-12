# 缓存指南

FiberHouse 把缓存拆成两个边界：`cache.Cache` backend 负责存储、连接和关闭，
`cache.Coordinator` 负责 cache-aside 流程，包括 miss 判定、singleflight、Bloom
预检、错误策略、负缓存和回填。应用显式创建并保存两者；配置文件不会自动创建
Coordinator、Bloom filter 或 breaker。

## 快速接入 Coordinator

下面的代码只依赖当前公开 API。应用应在启动期创建一个长期存活的 Coordinator，
并为每次请求创建独立的 `CacheOption`。

```go
package users

import (
	"context"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/cache"
)

type User struct {
	ID string `json:"id"`
}

func NewUserCoordinator(backend cache.Cache) (*cache.Coordinator, error) {
	return cache.NewCoordinator(backend)
}

func LoadUser(
	ctx context.Context,
	appCtx fiberhouse.IContext,
	codec fiberhouse.JsonWrapper,
	coordinator *cache.Coordinator,
	id string,
	load func(context.Context, string) (User, error),
) (User, error) {
	request := cache.NewCacheOption(appCtx).
		Level2().
		SetDefaultInstanceKey("users").
		SetCacheKey("user:" + id).
		SetContextCtx(ctx).
		SetJsonWrapper(codec).
		SetLocalTTLRandomPercent(30*time.Second, 0.10).
		SetRemoteTTLWithRandom(10*time.Minute, time.Minute).
		SetLoadTimeout(5 * time.Second).
		SetCacheErrorPolicy(cache.ReturnCacheError)

	return cache.GetOrLoad(coordinator, request, func(loadCtx context.Context) (User, error) {
		return load(loadCtx, id)
	})
}
```

`SetDefaultInstanceKey` 只设置这次 request 的 Coordinator flight namespace；它让同一
backend 上两个业务域的相同字符串 key 不会共享 flight。`NewCoordinator` 只接收 backend
及 `CoordinatorOption`，不读取 namespace 配置。旧 `GetCached` 仍通过
`Factory.Lookup(level)` 解析 backend，并分别使用 application 的 `GetLocalCacheKey`、
`GetRemoteCacheKey` 或 `GetLevel2CacheKey`；`SetDefaultInstanceKey` 不参与 backend lookup。
旧入口内部调用 `GetOrLoad` 时，该字段仍作为 request 的 flight namespace。结果类型也属于
flight key，因此 `GetOrLoad[User]` 与 `GetOrLoad[OtherType]` 不会互相复用结果。

## Backend 构造与所有权

`Cache` 保持原有六个方法，因此旧 backend 无需实现新的生命周期接口：

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

内置 backend：

| Backend | 构造函数 | 默认配置路径 |
|---|---|---|
| Ristretto local | `cachelocal.NewLocalCache(appCtx, confPath...)` | `cache.local` |
| Redis | `cacheremote.NewRedisDb(appCtx, confPath...)` | `cache.redis` |
| L2 | `cache2.NewLevel2CacheWithError(appCtx, local, remote)` | pool 固定读取 `cache.asyncPool.ants.*` |

L2 的 error-returning 构造函数默认拥有传入的 local 和 remote。它的 `Shutdown`/`Close`
会关闭两个 child，每个最多一次。因此把 child 交给 L2 后，不要再由另一个 owner 关闭。
Coordinator 不关闭 backend；创建 Coordinator 的应用仍负责 backend 生命周期。

```go
local, err := cachelocal.NewLocalCache(appCtx)
if err != nil {
	return err
}
remote, err := cacheremote.NewRedisDb(appCtx)
if err != nil {
	_ = local.Close()
	return err
}
l2, err := cache2.NewLevel2CacheWithError(appCtx, local, remote)
if err != nil {
	_ = local.Close()
	_ = remote.Close()
	return err
}
coordinator, err := cache.NewCoordinator(l2)
```

应用也可以通过 `cache.NewFactory(appCtx).Lookup(cache.Level2)` 从已经注册的实例中取得
backend。`Lookup` 返回 error；旧的 `GetCache` 保留 panic-on-failure 行为，仅用于兼容。

## GetOrLoad 的执行语义

`GetOrLoad` 先执行一次 caller-scoped cache `Get`。命中后解码并返回；miss 后进入整个
read-through flight。flight 内会再次读 cache，避免排队期间其他调用已经完成回填，然后
才调用 loader 和 `Set`。

### Miss 与基础设施错误

- `cache.IsMiss(err)` 使用 `errors.Is(err, cache.ErrKeyNotFound)` 判断 miss。
- miss 总是允许 loader 执行。
- `ReturnCacheError` 是默认策略。网络错误、closed backend 等非 miss 错误直接返回，
  loader 不执行。
- `LoadOnCacheError` 把非 miss cache error 当作可降级 miss，并让共享 loader 执行。
- loader 成功但 cache `Set` 失败时，调用仍返回 loader 数据，并发送 `EventFillError`。
- 命中值解码失败直接返回错误，不会自动删除损坏值。

`WithObserver(func(cache.Event))` 可观察 `EventCacheHit`、`EventCacheMiss`、
`EventCacheError`、`EventFillError` 和 `EventFilterError`。Observer 在请求路径同步调用，
契约要求它不能阻塞。

### Singleflight 与 context

Coordinator 总是保护完整的 miss → loader → fill 路径，不依赖旧的
`EnableSingleFlight` 开关。每个等待者仍监听自己的 request context：等待者取消后立即
返回 `ctx.Err()`，但已经共享出去的 loader 不被该等待者取消。

共享 loader 使用首个进入 flight 的 context values，通过 `context.WithoutCancel` 脱离
caller cancellation，再施加 `SetLoadTimeout`。默认 load timeout 是 30 秒。因此 loader
必须使用收到的 `loadCtx`，不能替换成 `context.Background()`。

## Bloom filter：先建立权威集合

Bloom filter 只有在 `Ready()==true` 时才能拒绝 key。`Add` 只学习一次成功 load 的 key，
不会把 filter 变成 authoritative（可用于否定判断的完整集合）。首次 `Preload` 完整成功
前保持 fail-open；已建立 authority 后的刷新失败如何处理，取决于具体实现。

### 进程内 MemoryBloomFilter

适合单实例、每个实例都能取得完整 key 清单，或允许各实例独立维护过滤状态的服务。

```go
filter, err := cache.NewMemoryBloomFilter(1_000_000, 0.01, 16)
if err != nil {
	return err
}
if err := filter.Preload(ctx, authoritativeKeys); err != nil {
	return err
}
coordinator, err := cache.NewCoordinator(
	backend,
	cache.WithExistenceFilter(filter),
	cache.WithFilterFailureMode(cache.FilterFailOpen),
)
```

Memory preload 是事务式替换：它先在旁路构建新 shards，成功后再原子发布。首次 preload
失败或取消时 filter 仍未 ready，因此继续 fail-open；已 ready 后的 refresh 失败或取消时，
旧的 authoritative snapshot 和 `Ready()==true` 保持不变。此时旧 snapshot 仍可拒绝未包含
的 key，包括刷新期间新增但不在旧集合中的有效 key。需要显式撤销 authority 时调用
`Reset`；它清空当前 snapshot、把 `Ready` 设为 false，并恢复 fail-open。

capacity 必须在 1 到 `1<<26` 之间，error rate 必须在 0 和 1 之间。shard count 的零值
表示 1；其他值向上取整为 2 的幂。

### RedisBloomFilter

适合多个进程需要共享 filter，且 Redis 安装了 RedisBloom 模块的服务。它使用
`BF.RESERVE`、`BF.MADD`、`BF.EXISTS` 和 `BF.ADD`。

```go
filter, err := cacheremote.NewRedisBloomFilter(redisClient, cacheremote.RedisBloomConfig{
	Key: "cache:users:keys", Capacity: 1_000_000, ErrorRate: 0.01, BatchSize: 1000,
})
if err != nil {
	return err
}
if err := filter.Prepare(ctx); err != nil {
	return err
}
if err := filter.Preload(ctx, authoritativeKeys); err != nil {
	return err
}
```

Redis 不支持 `BF.*` 时错误可由 `errors.Is(err, cacheremote.ErrRedisBloomUnavailable)`
识别。`AllowExisting` 默认 false，防止意外复用同名 Redis key；显式设为 true 才接受
`BF.RESERVE` 的 BUSYKEY。`Reset` 只撤销当前进程的 authority，不删除共享远程 key。

RedisBloom 的 refresh 与 Memory 不同：`Preload` 开始时先把当前进程的 `Ready` 设为
false，再分批执行 `BF.MADD`。任何批次失败或 context 取消都会保持 unready，因此当前
进程继续 fail-open；远程 Redis key 中已有或已写入的 bits 不会回滚。只有全部批次成功后
才重新发布 authority。`Reset` 同样只把本地 readiness 转为 fail-open。

Coordinator 默认 `FilterFailOpen`：filter 自身报错时继续访问 cache/loader。
`FilterFailClosed` 将 filter error 转成 `ErrRejectedByBloomFilter`。caller cancellation 和
deadline 永远原样返回，不受 failure mode 改写。每次希望启用预检的 request 仍需调用
`EnableBloomFilter()`。

## 负缓存与 TTL jitter

loader 返回可由 `errors.Is(err, cache.ErrOriginNotFound)` 匹配的错误，且
`SetNegativeTTL(ttl)` 大于零时，Coordinator 写入内部负缓存 marker。后续命中返回
`ErrOriginNotFound`，不会再次调用 loader。普通数据库错误不会负缓存。

负缓存 TTL 按 level 写入：Local 只用 local TTL，Remote 只用 remote TTL，L2 强制
`WriteBoth` 并同时设置两级 TTL。

TTL jitter（抖动）降低大量 key 同时过期造成的尖峰：

- 固定：`SetLocalTTL`、`SetRemoteTTL`；
- 绝对范围：`SetLocalTTLWithRandom(base, range)`、`SetRemoteTTLWithRandom`；
- 百分比：`SetLocalTTLRandomPercent(base, percent)`、`SetRemoteTTLRandomPercent`，percent
  被限制到 `[0,1]`。

随机 TTL 保证为正，并防止 `time.Duration` 溢出。`Valid` 拒绝负 TTL/range、零随机
base、非正 load timeout，以及默认情况下 local base TTL 大于 remote base TTL。确有需要
时调用 `AllowLocalTTLAboveRemote()` 显式放宽最后一条。

## L2 consistency 与写策略

L2 读取顺序是 local → remote。remote 命中会回填 local。每个 key 有独立、可清理的
generation fence：与同 key 的同步 Set/Delete 交错时，旧 remote read 不会在 mutation
完成后重新污染 local；无关 key 不互相抑制回填。

| Strategy | Set 行为 | 一致性 |
|---|---|---|
| `WriteBoth` | remote 成功后写 local；remote 失败不碰 local | 同步 remote-first |
| `WriteRemoteOnly` | remote 成功后删除 local；remote 失败不碰 local | 同步 remote-first |
| `AsyncWriteBoth` | 独立提交 local 与 remote pool | eventual；错误只记录日志 |
| `AsyncWriteRemoteOnly` | 只提交 remote pool | eventual；不会同步失效 local |

`Delete` 先删 remote，成功后逐 key 删除 local。remote 删除失败时 local 保持不变。
同步策略适合读后必须立即观察新值的路径。异步策略适合能接受短暂旧值、并已设置
监控和容量上限的路径。

异步任务脱离 caller cancellation，并分别施加 1 秒 local timeout、3 秒 remote timeout。
提交后 option clone 由 L2 释放；调用方只释放自己的原 option。不要把 pooled option 跨
goroutine 传递，也不要在归还后继续引用。

## Flush、Shutdown 与关闭迁移

`Flusher` 和 `Shutdowner` 是可选接口，没有加入 `Cache`，所以旧 backend 继续编译：

```go
if flusher, ok := backend.(cache.Flusher); ok {
	if err := flusher.Flush(ctx); err != nil {
		return err
	}
}
if shutdowner, ok := backend.(cache.Shutdowner); ok {
	return shutdowner.Shutdown(ctx)
}
return backend.Close()
```

L2 `Flush(ctx)` 等待调用时快照中的 queued 和 running async work；deadline 返回
`ctx.Err()`，不会取消这些任务。`Shutdown(ctx)` 先停止 admission，再 drain、release pool
并关闭 owned children。若 caller timeout，后台 shutdown 仍继续；后续调用可等待真实完成。
`Close()` 是无 context 的兼容入口，委托同一 shutdown 状态机；重复 Close 返回 nil。
`Wait()` 先 Flush，再调用两个 child 的 `Wait()`。

推荐停止顺序：停止生产请求/任务 → `Flush` → `Shutdown`。不要先关闭 child，否则已经
admitted 的 L2 任务会访问 closed backend。

## 配置参考

内置实现实际消费这些 key：

- `cache.local`: `numCounters`, `maxCost`, `bufferItems`, `metrics`,
  `ignoreInternalCost`。
- `cache.redis`: `host`, `port`, `password`, `db`, `poolSize`, `minIdleConns`,
  `dialTimeout`, `readTimeout`, `writeTimeout`, `poolTimeout`, `connMaxIdleTime`,
  `connMaxLifetime`。duration 数值按秒解释。
- `cache.asyncPool.ants.{local,remote}`: `size`, `expiryDuration`, `preAlloc`,
  `maxBlockingTasks`, `nonblocking`。两个 size 都必须大于零。

示例 YAML 中的 `cache.coordinator` 是应用侧配置形状，仅用于说明 namespace、filter、
breaker 和 jitter policy；FiberHouse 不会自动读取或装配它。应用必须把 namespace 放到
每个 request 的 `SetDefaultInstanceKey`，把 filter 翻译成 `WithExistenceFilter`，并显式
应用 backend/wrapper breaker 设置和 TTL option。`NewCoordinator` 不消费 namespace；
legacy `GetCached` 仍按 application-level cache key 解析 backend。

`cache.redis.protection` 是旧 Redis-owned protection 配置，保留用于兼容已有部署，但已
deprecated。新代码应由应用显式构造 Coordinator/filter；不要假设 YAML 会自动开启保护。

## 兼容 API 与迁移

| 兼容入口 | 新代码 | 原因 |
|---|---|---|
| `GetCached` | retained `Coordinator` + `GetOrLoad` | Coordinator 在所有 caller 间共享完整-path flight 和 policy |
| `Factory.GetCache` | `Factory.Lookup` | 用 error 处理缺少 context/application/registry key，避免 panic |
| `cache2.NewLevel2Cache` | `NewLevel2CacheWithError` | 显式返回 pool 配置/创建错误；旧入口失败时只能返回 nil |
| `EnableSingleFlight` | 直接使用 `GetOrLoad` | Coordinator 总是执行 full-path singleflight；旧 flag 只为 option 兼容保留 |
| Redis `protection` + `EnableProtectionAll` | `NewCoordinator` + `WithExistenceFilter`，breaker 由 backend/wrapper 显式提供 | 所有权和 failure mode 可见，Bloom readiness 可验证 |
| `CacheBloomFilter` | `ExistenceFilter` / `PreloadableFilter` | 新接口携带 context、error 与 authoritative readiness |

`OptionPoolGet`/`OptionPoolPut` 仍受支持，但不是异步共享机制。简单路径优先用
`NewCacheOption`；需要降低分配时再使用 pool，并严格限制为一个同步 owner。

## 验证与排错

- loader 没执行：先用 `cache.IsMiss(err)` 区分 miss 和基础设施错误，再检查
  `ReturnCacheError`/`LoadOnCacheError`。
- Bloom 拒绝新 key：确认 filter 是否错误地在完整 preload 前报告 ready；动态 `Add` 不应
  建立 readiness。
- 相同 key 仍重复 load：确认所有 caller 复用同一个 Coordinator，并使用相同 namespace、
  key 和 result type。
- Set 后立即读不到：local backend 调用 `Wait()`；L2 async strategy 调用 `Flush(ctx)`。
- shutdown 卡住：检查 pending async backend 操作是否尊重 L2 的 1 秒/3 秒 timeout，且
  child 没有被其他 owner 提前关闭。

实现入口：[`cache_utility.go`](../../component/cache/cache_utility.go)、
[`cache_option.go`](../../component/cache/cache_option.go)、
[`filter.go`](../../component/cache/filter.go)、
[`cache2/level2_cache.go`](../../component/cache/cache2/level2_cache.go)、
[`cacheremote/redis_bloom.go`](../../component/cache/cacheremote/redis_bloom.go)。
