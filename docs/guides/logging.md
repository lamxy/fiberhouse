# 日志

FiberHouse 在 `New` 阶段调用 `bootstrap.NewLoggerOnce`，用 zerolog 建立进程级日志器。`LoggerWrapper` 暴露各级别事件、底层 `*zerolog.Logger`、带字段的 context 和 `Close`；它不是日志服务守护进程，也不会替应用编排所有后台生产者的停止顺序。

## 初始化与输出选择

日志输出由 `application.appLog` 控制：

| 配置 | 当前行为 |
|---|---|
| `enableConsole=true`、`consoleJSON=false` | 使用 `zerolog.ConsoleWriter` 写 `os.Stdout`，时间格式为 RFC3339 |
| `enableConsole=true`、`consoleJSON=true` | 直接把 zerolog JSON 写到 `os.Stdout` |
| `enableFile=true` | 创建一个同步或异步 lumberjack writer |
| console 与 file 都未启用 | 回退为直接写 `os.Stdout` 的 JSON 输出 |

console 与 file 同时启用时，`io.MultiWriter` 按顺序写两个目标。`application.appLog.level` 会转为小写后交给 `zerolog.ParseLevel`；缺失或无效时源码回退到 `zerolog.TraceLevel`，并同时调用 `zerolog.SetGlobalLevel` 修改进程级全局级别。日志时间字段格式固定为 RFC3339Nano。

文件名通过 `application.appLog.filename` 读取，源码 fallback 是 `app.log`。调用 `bootstrap.NewLoggerOnce(cfg)` 而不传路径时使用当前工作目录下的 `logs/`；但框架 `New` 总会传入 `BootConfig.LogPath`。`Default()` 将其设置为 `./logs`，而直接给 `New` 传空 `LogPath` 会拼成 `/app.log` 形式，不会回退到工作目录。这是当前路径处理的静态限制。

`NewLoggerOnce` 只执行一次。第一次调用决定输出目标、级别和底层 file writer，后续调用复用同一 `LoggerWrapper`。

## Origin 来源字段

`LoggerWrapper.Debug/Info/Warn/Error/Fatal/Panic` 可接收一个 `appconfig.LogOrigin`，并写入大小写固定的 `Origin` 字段；对应的 `*With` 方法要求显式传入 Origin。`AppConfig` 内建 frame、web、task、cache、database 等 Origin，`application.appLog.logOriginEnum` 可在 `Initialize` 时覆盖或扩展其值。

`FrameApplication.RegisterApplicationGlobals` 会遍历最终 Origin map，把带 `Origin` 字段的子 zerolog logger initializer 注册到 `GlobalManager`。key 由 `LogOrigin.InstanceKey()` 生成；随后可通过 Context 的 `GetLoggerWithOrigin` / `GetMustLoggerWithOrigin` 读取。这个注册发生在 bootstrap 日志器创建之后、HTTP 服务运行之前；运行期修改 Origin map 不会自动同步已经注册的子日志器。

Origin 子日志器只是共享底层输出的带字段 logger，不拥有独立文件或独立 `Close`。Context 与容器访问边界见[《Context 与 Locator》](../concepts/context-and-locators.md)。

## Gin 框架日志桥接

选择 Gin core 时，FiberHouse 会自动把 Gin 原生诊断和 `http.Server` 默认错误日志接入同一个 `LoggerWrapper`：

| 来源 | 框架级别 | 识别字段 |
|---|---|---|
| `gin.DebugPrintFunc` | Debug | `Component="Gin"`、`Channel="debug"` |
| `gin.DebugPrintRouteFunc` | Debug | `Component="Gin"`、`Channel="debug"`、method、path、handler、handlerCount |
| `gin.DefaultWriter` | Info | component、`Channel="writer"` |
| `gin.DefaultErrorWriter` | Error | component、`Channel="error"` |
| 默认 `http.Server.ErrorLog` | Error | component、`Channel="server"` |

Debug 事件仍由 `application.appLog.level` 决定是否输出。Gin 的 debug callback 不提供 warning 严重级别，桥接不会解析私有消息格式，因而从该 callback 到达的启动 warning 也保持 Debug。应用预先提供的 `http.Server.ErrorLog` 是显式覆盖，FiberHouse 不替换它。

桥接第一次有效安装时，在 Gin 活动开始前捕获四个 package 全局诊断变量的原值，并把这些变量固定为进程生命周期内稳定的转发入口。lease 只原子切换当前 owner；初始化失败、server 退出或 shutdown 时会停用自己的 owner，之后转发入口按行为回退到首次捕获的 Gin 函数和 writer，而不会在运行期写回全局变量。这样设计是因为 Gin 读取这些变量时没有同步，运行期写回会与并发诊断输出产生数据竞争。lease 不关闭框架日志器；后续 owner 复用同一组稳定入口。lease 活跃期间的多个 Gin engine 共享同一个框架日志器，逐 engine 的原生 debug 隔离不受支持；第二个 FiberHouse core 会收到安装冲突，而不是覆盖现有 owner。桥接安装后由外部代码改写这些全局变量仍不受支持。

桥接不改变 FiberHouse 既有请求访问日志、recovery 或尾部错误处理的所有权，也不安装 Gin 原生 Logger/Recovery 中间件。访问日志仍由 `CoreWithGin.loggerMiddleware` 产生一条 Info 记录，recovery 仍由 FiberHouse recovery provider 处理；桥接只承接 Gin 自身的诊断出口和 server error logger。

## 文件轮转

同步与两种异步 writer 都把以下值交给 lumberjack：

| 配置键 | lumberjack 字段 | 单位 |
|---|---|---|
| `application.appLog.rollConf.maxSize` | `MaxSize` | MB |
| `application.appLog.rollConf.maxBackups` | `MaxBackups` | 文件数 |
| `application.appLog.rollConf.maxAge` | `MaxAge` | 天 |
| `application.appLog.rollConf.compress` | `Compress` | bool |

FiberHouse 构造器没有为这些键提供非零 fallback；缺失时把 koanf 零值交给 lumberjack。实际零值轮转语义由当前 lumberjack 版本决定，不应从 `example_config/application_dev.yml` 的示例数值推断框架默认。

同步路径使用 `SyncLumberjackWriter`，`Write` 和 `Close` 直接委托给 lumberjack。文件目录、权限或磁盘错误可能到实际写入时才暴露。

## channel 异步 writer

设置 `application.appLog.asyncConf.enable=true` 且 `type=chan` 会选择 `AsyncChannelWriter`。它使用：

- `chanConf.bufferSize` 作为 `bufio.Writer` 缓冲大小；
- `chanConf.chanSize` 作为 channel 容量；
- 固定 1 秒 flush ticker。

`Write` 会先复制输入字节。channel 有空间时入队；持续满 1 秒时丢弃该条日志、增加原子 `droppedLogs`，并按第 1、101、201……条的节奏把计数写到 `os.Stderr`。即使超时丢弃，`Write` 仍返回 `len(p), nil`，所以普通 zerolog 调用方不能从返回值识别丢失。具体 writer 的 `DroppedLogs()` 可读取累计值。

关闭 channel 后，消费 goroutine 排空已经入队的数据、执行最终 `Flush`，然后 `Close` 等待它退出并关闭 lumberjack。后台 `Write` 错误写到 `os.Stderr`；周期和最终 flush 错误当前被忽略。

## diode 异步 writer

异步启用但 `type` 缺失时，源码默认选择 `diode`。`AsyncDiodeWriter` 使用 many-to-one diode，并提供以下构造器 fallback：

| 配置键 | 源码 fallback |
|---|---:|
| `diodeConf.size` | `33554432` |
| `diodeConf.bufferSize` | `4096` |
| `diodeConf.flushInterval` | `1000`，随后乘以 `time.Millisecond` |

`flushInterval` 的当前消费约定是数值毫秒；代码无条件把 `Duration` getter 的结果再乘 `time.Millisecond`，因此不要传已经带 `s`、`ms` 等单位的 duration 字符串来推断直观时长。

`Write` 同样复制字节并向 diode 写入。容量压力下，diode 可以覆盖未消费消息；alert 回调把 missed 数量累加到 `DroppedLogs()`，并将每次 missed 事件写到 `os.Stderr`。消费 goroutine 无数据时休眠约 1 ms，按配置周期 flush；关闭时先排空当前 diode，再最终 flush 和关闭 lumberjack。覆盖发生时 `Write` 仍返回成功，因此该路径也不能称为无损。

`NewWriterAsync` 会先尝试把 `chan` 和 `diode` initializer 注册到进程级 `GlobalManager`，再按 `type` 取出实现。未知 type、初始化失败或对象不是 `io.WriteCloser` 都会 panic。批量生命周期不能假设这里会自动替换同 key 的既有 writer，因为容器重复注册返回 false，而当前调用方没有处理该结果。

## 缓冲、指标与可观测性

两种异步 writer 都有后台 goroutine、内存缓冲和 `DroppedLogs()`；它们的背压/丢弃策略不同：channel 最多阻塞调用方 1 秒再丢一条，diode 允许覆盖并由 missed 回调计数。

`application.appLog.enableMetrics` 会进入 `AppConfig.GetAppLog()` 的配置视图，但当前 bootstrap 没有自动把 writer 的 `DroppedLogs()` 注册为监控指标，`LoggerWrapper` 接口也不暴露该方法。需要指标时，应用必须明确持有或类型断言具体 writer，并自行采集；不能仅打开此配置就假设已有 exporter。

异步 writer 对底层写入/flush 错误的传播不完整：部分错误只写 stderr，部分被忽略，最终只返回 lumberjack `Close` 的错误。日志落盘成功与 `Write` 返回成功不是同一个保证。

## Close 所有权与停止顺序

`LoggerWrap` 只持有 bootstrap 选中的 file writer：

- 只有 console 或 stdout fallback 时，`Close()` 是空操作；
- 同时有 console 和 file 时，只关闭 file writer；
- 同步 file 直接关闭 lumberjack；异步 file 等待消费 goroutine 完成排空与 flush。

安全停止顺序是：停止 HTTP、task worker 和其他日志生产者，等待它们退出，然后只调用一次全局 logger 的 `Close`。Fiber/Gin 当前受控关闭路径会清空 `GlobalManager` 并关闭日志器，但不会为所有应用资源建立统一顺序；详见[《Web 启动生命周期》](../concepts/startup-lifecycle.md)。

两种异步 `Close` 都不是幂等实现：第二次调用会再次关闭 channel；channel writer 的 `Write` 与 `Close` 之间也存在“检查 closed 后、发送前关闭 channel”的竞态窗口。diode writer 在并发关闭时可能接受已无法消费的数据。这些是源码静态限制，因此必须由资源所有者先停止生产者、再单线程关闭。

## 已知限制与测试边界

- channel 与 diode 都可能丢日志，不能描述为 lossless 或 production-ready 保证。
- 异步 writer 的关闭、flush、无效路径与关闭后写入语义仍有明显边界；当前仓库的测试基线在 `component/logging/writer` package 中存在已知失败，且 case 数量会受异步写入时序影响。本页按运行实现记录行为，不把失配的测试期望当作 API 契约。
- `AsyncChannelWriter.Write` 在超时丢弃后仍报告成功；`AsyncDiodeWriter.Write` 也不把覆盖作为调用错误返回。
- `Close` 只回收日志 file writer，不会停止 keepalive、任务或其他仍可能记录日志的 goroutine。
- 配置、日志器、异步 writer initializer 与 Origin 子日志器都连接进程级单例；同进程热切换输出或并行测试多套日志配置不受支持。
- Gin 日志桥接同样依赖 package 全局 hook；一个活跃 owner 和共享 logger 是明确限制，不能用它实现多 engine 的诊断隔离。

源码入口见 [`bootstrap/bootstrap.go`](../../bootstrap/bootstrap.go)、[`component/logging/writer`](../../component/logging/writer/) 与 [`adaptor/logging`](../../adaptor/logging/)。
