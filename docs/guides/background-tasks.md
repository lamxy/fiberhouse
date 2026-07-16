# 后台任务

FiberHouse 用 asynq 封装 Redis 队列，提供 `TaskWorker`、`TaskDispatcher` 和应用侧 `TaskRegister` 契约。框架负责在 Web 启动链的任务位点调用注册器、挂载 handler 并选择是否启动 worker；任务类型、Redis 实例、队列配置、dispatcher/worker initializer 和资源回收都由应用决定。

这套能力已有运行路径，但统一错误传播和关闭编排尚不完整，因此当前属于实验性能力。

## 三个角色

| 角色 | 入口 | 责任 |
|---|---|---|
| `TaskRegister` | 应用实现 `application_interface.go` 中的接口 | 保存 handler map，注册 worker/dispatcher initializer，并从容器取实例 |
| `TaskWorker` | `fiberhouse.NewTaskWorker(appCtx, redisClient, asynq.Config)` | 包装 `asynq.Server`/`ServeMux`，注册 handler，运行消费者 |
| `TaskDispatcher` | `fiberhouse.NewTaskDispatcher(redisClient)` | 包装 `asynq.Client`，提供 `Enqueue`/`EnqueueContext` |

框架没有默认 `TaskRegister`，也不会根据配置自动创建 Redis client。应用必须把实现放入启动器，并让 `GetRedisKey`、`GetTaskServerKey`、`GetTaskDispatcherKey` 与 initializer 使用的 key 一致。

[`example_application/module/task_impl.go`](../../example_application/module/task_impl.go) 展示了一种接线：从应用的 Redis key 取 `cache.IRedisClient`，用同一底层 `*redis.Client` 构造 worker 和 dispatcher，再选择并发数、队列权重、日志 adapter。示例的 `Concurrency: 10`、`critical/default/low` 权重、任务名和 key 都不是框架默认。

## 启动链

`FrameApplication.RegisterApplicationGlobals` 在应用、模块和任务注册器已经挂载后执行：

1. 注册应用普通 GlobalManager initializer，并尝试初始化 required keys。
2. 注册自定义校验器。
3. 若 `TaskRegister != nil`，调用它的 `RegisterTaskServerToContainer` 和 `RegisterTaskDispatcherToContainer`。
4. 稍后的 `RegisterTaskServer` 位点读取 `application.task.enableServer`。
5. 开关为 true 且 task register 存在时，取得 worker、注册 `GetTaskHandlerMap()` 的所有 handler，再调用 `worker.RunServer()`。

`application.task.enableServer` 是框架内唯一直接读取的任务开关；它控制 Web 启动链是否运行 worker。是否也用它控制 initializer/dispatcher 注册属于 `TaskRegister` 实现自己的策略。示例两种注册方法都会检查该开关，因此关闭 server 时 dispatcher 也不会注册；别的应用可以选择“只生产、不消费”，但必须自行实现相应注册逻辑。

`TaskRegister` 的 handler map 没有并发保护。应在启动期一次性收集 handler，避免运行期增删或重复调用一个带副作用的 `GetTaskHandlerMap`。重复 pattern 的处理方式也由应用实现决定；示例只记录 warning 并保留旧 handler。

## handler context 与 payload

`NewTaskWorker` 在 `ServeMux` 上安装中间件，把创建 worker 时的 `IContext` 写入任务 `context.Context`：

```go
appCtx, ok := ctx.Value(fiberhouse.ContextKeyAppCtx).
	(fiberhouse.IApplicationContext)
if !ok || appCtx == nil {
	return errors.New("missing application context")
}
```

handler 不应像示例那样忽略类型断言结果；装配错误否则可能在随后的日志或容器访问处 panic。这个 context value 只提供应用引用，不改变 asynq 的取消、deadline 或 retry 语义。

任务 payload 可自行编码。根 package 的 `PayloadBase` 会优先从 `GetFastTrafficCodecKey()` 取 `JsonWrapper`，失败时 `GetMustJsonHandler` 记录 warning 并回退到新的 Sonic fastest 实例。生产者和消费者必须使用兼容 schema/codec；任务结构演进、幂等和重复执行不由框架自动解决。

## 同步与异步 worker

- `RunSync()` 在当前 goroutine 调用 `asynq.Server.Run`，直到 server 结束；普通错误会记录并返回。
- `RunAsync()` 启动 goroutine，内部错误只记录且不向调用方传播。
- `RunServer(true)` 选择 sync；不传或传 false 选择 async。标准 Web 启动链不传参数，因此总是 async。

两条路径都 recover panic 并记日志。`RunServer` 丢弃 sync/async 返回值，`RunAsync` 也不提供 ready、done 或 error channel；“方法已经返回”不代表 worker 已成功开始消费。若应用要求 fail-fast、readiness 或等待退出，应直接围绕 `GetServer()` 暴露的 asynq server 建立自己的监督逻辑。

## 入队

生产者先构造 `*asynq.Task`，再从应用的 `TaskRegister.GetTaskDispatcher()` 取得 dispatcher：

```go
info, err := dispatcher.EnqueueContext(
	ctx,
	task,
	asynq.MaxRetry(3),
	asynq.ProcessIn(time.Minute),
)
```

`Enqueue` 使用 client 自身调用，`EnqueueContext` 才接收调用方 context。两者原样返回 `TaskInfo` 和 asynq error；框架不重试、不转换成统一 HTTP 业务异常，也不保证任务与数据库事务原子提交。handler 返回的 error 则交给 asynq 的 retry/失败处理策略。

示例 service 在 dispatcher 或 task 构造失败后仍可能继续使用 nil 值；这只是示例的不完善分支，正式代码必须在每个 error 后停止当前路径。

## Redis 与资源所有权

worker 和 dispatcher 都依赖调用方传入的 `*redis.Client`。框架不验证该 client 属于专用任务连接还是与缓存共享，也不为它定义关闭顺序。共享可以减少连接对象，但会把缓存、生产者和消费者的健康与关闭耦合在一起。

当前 wrapper 的生命周期边界是：

- `TaskWorker` 没有自己的 `Close`/`Shutdown` 方法，但 `GetServer()` 返回原始 `*asynq.Server`；应用可使用底层 API 建立停止与等待协议。
- `TaskDispatcher` 没有 wrapper 级 `Close`，但公开 `Client *asynq.Client`；创建者仍需按 asynq/Redis 的所有权规则回收。
- 标准 Fiber/Gin shutdown 不会停止 worker 或 dispatcher，只会清空 GlobalManager；清空不调用资源关闭方法。
- async worker goroutine、GlobalManager keepalive 和日志 writer 不共享统一 cancel tree。

建议的应用关闭顺序是：先停止新请求和新入队，停止 worker 接收并等待在途 handler，关闭 dispatcher，最后按所有权关闭 Redis 与日志。若 Redis 同时供缓存使用，应在所有消费者都停止后才关闭。当前框架没有替应用执行或验证这条顺序。

## 错误与并发边界

- worker initializer 缺失、类型不符或 `GetTaskWorker` 返回错误时，标准 `RegisterTaskServer` 会 panic。
- handler map 的读写、注册器内部状态和 GlobalManager initializer 应在进入并发消费前冻结。
- `RunAsync` 的启动错误不会改变 Web server readiness；需要应用补充探针或监督状态。
- Context 注入的是共享应用对象；其配置、validator、provider 集合等仍遵守“启动期写、运行期读”。
- 示例任务 logger 只是 asynq 日志适配器，不拥有 server；其 `Fatal` 行为也不适合作为普通任务错误出口。

源码入口：[`task.go`](../../task.go)、[`application_interface.go`](../../application_interface.go)、[`frame_starter_impl.go`](../../frame_starter_impl.go) 与 [`component/task/logadaptor`](../../component/task/logadaptor/)。完整错误响应边界见[《错误与恢复》](errors-and-recovery.md)，容器清理限制见[《GlobalManager》](global-manager.md)。
