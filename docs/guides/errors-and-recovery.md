# 错误与恢复

FiberHouse 有两条错误通路：handler 正常结束时传播的 `error`，以及 `panic`。两条通路最终都使用统一 `{code,msg,data}` 响应，但入口、传播方式和发送错误的处理并不相同。

```text
Fiber handler return error ─→ Fiber ErrorHandler ─┐
Gin handler c.Error / c.Set ─→ 尾部中间件 ───────┼→ ErrorHandler → Response facade
panic ─→ CoreType 对应 recovery middleware ─────┘
```

## 异常类型

[`exception`](../../exception/) 定义两种实现 `error` 和 `response.IResponse` 的类型：

- `ValidateException`：参数/结构验证失败，通常携带字段到消息的 map。
- `Exception`：业务异常，从应用注册的 `ExceptionMap` 取得业务 code、msg、data。

其他 Go `error`、`runtime.Error` 和非 error panic 值都走未知错误分支。`exception.Get/VeGet` 从进程级 `GlobalManager` 读取应用异常表；异常表未注册会 panic，key 不存在则构造 `UnknownErrCode/UnknownErrMsg`。示例异常表只是演示数据，不是框架保证的生产错误码目录。

业务可选择返回 `*Exception`，也可调用其 `Panic()`/`exception.Throw`。选择会影响进入“正常错误处理”还是 recovery，但当前 HTTP status 分类对两条路径保持一致。

## Fiber：返回 error

Fiber handler 原生签名允许 `return err`。recover 中间件执行 `return c.Next()`，它只捕获 panic，不消费普通返回错误；该错误随后进入 `fiber.Config.ErrorHandler`，由 adaptor 包成 `ICoreContext` 后调用统一 `ErrorHandler`。

统一处理器先记录错误，再用 `errors.As` 分类：`ValidateException` → HTTP 400，`Exception` → HTTP 400，其他错误 → HTTP 500。发送函数返回 nil 时，当前 Fiber adaptor 仍返回原始 `err`；因此存在“统一 body 已写出后，原错误继续交回 Fiber”的传播风险。这里是控制流静态观察，未断言 Fiber 在所有版本、连接状态下都会二次写响应。

## Gin：`c.Error` 或 Context error

Gin handler 没有 error 返回值。需要进入统一错误链时应显式记录：

```go
if err != nil {
	_ = c.Error(err)
	return
}
```

也可以 `c.Set("error", err)`。Gin 错误 adaptor 在 `c.Next()` 返回后先检查 `c.Errors`，只处理最后一个；仅当 `c.Errors` 为空时才读取 Context 的 `"error"`。单纯 `return`、记录日志或把 error 放到其他 key 都不会自动产生统一错误响应。

若统一 `ErrorHandler` 的响应发送返回错误，Gin adaptor 会 panic；由于 recovery 中间件注册在它外层，该 panic 会再次进入 panic recovery。应用 handler 不应在 `c.Error` 后继续写另一个响应。

## panic recovery 与 Provider 选择

`RecoveryPManager` 绑定到应用中间件位点，按 `BootConfig.CoreType` 选择 `FiberRecoveryProvider` 或 `GinRecoveryProvider`。没有匹配 Provider、返回值未实现 `IRecover`，或 Manager 未装配都会在中间件注册阶段失败。

两个 starter 都通过 `NewErrorHandlerOnce` 取得进程级 ErrorHandler，并用以下 `RecoverConfig` 注册 recovery：

- `AppCtx`：当前应用 Context；
- `EnableStackTrace=true`，`StackTraceHandler=DefaultStackTraceHandler`；
- `Logger`：应用 logger，`Stdout=false`；
- `JsonCodec`：当前引擎选择的 JSON marshal；
- `DebugMode`：`application.recover.debugMode`。

`RecoverConfig` 自身的零参数默认值则是 stack trace 关闭、空 handler、`Stdout=true`、`DebugMode=false`。starter 明确覆盖了其中一部分，所以不能用结构体默认值描述标准 Web 装配。`configDefault` 把最终配置写入 package 级 `ConfigConfigured`；它不是每个应用独立保存的配置，也没有并发更新协议。

Fiber recovery defer 包住一次 `c.Next()`。Gin 的 `Next` 跳过回调有一项不对称：当 `RecoverConfig.Next` 返回 true 时会先调用一次 `c.Next()`，但当前实现没有 `return`，随后仍安装 defer 并再次调用 `c.Next()`；自定义使用 `Next` 时不能把它视为可靠的单次跳过语义。标准 starter 没有设置 `Next`。

## panic 分类与对外响应

`RecoverPanicInternal` 的实际映射如下：

| panic 值 | HTTP status | `debugMode=false` | `debugMode=true` |
|---|---:|---|---|
| `*ValidateException` | 400 | 完整 code/msg/data | 同左 |
| `*Exception` | 400 | 保留 code/msg，清空 data | 保留完整 data |
| `runtime.Error` | 500 | msg 为 `NullPointerException` 或 `UnknownRTException`，隐藏原始详情 | msg 为 `RuntimeError`，data 带原始错误文本 |
| 其他 `error` | 500 | `UnknownErrMsg` | msg 带原始错误文本 |
| 其他 panic 值 | 500 | `UnknownErrMsg` | 尝试 JSON/string 化后放入 data |

正常 error 通路的映射略有不同：验证异常仍完整返回；业务异常在生产模式清空 data；未知错误在 debug 模式把 `err.Error()` 放进已注册 `UnknownError` 的 data，生产模式不附加该详情。两条路径都忽略业务 code 的数值区间，只由 Go 类型决定 HTTP status。

panic recovery 内部忽略统一响应发送的返回值。若编码或连接写入失败，当前路径没有第二个可靠错误通道。

## 日志、trace 与脱敏

`DefaultStackTraceHandler` 从 `application.recover` 与 `application.trace.requestID` 读取：

| 配置 | 作用 | 缺失时 typed 配置值 |
|---|---|---|
| `debugMode` | 对外显示详细错误；同时无条件进入详细堆栈日志分支 | `false` |
| `enablePrintStack` | 即使非 debug 也记录堆栈 | `false` |
| `enableDebugFlag` | 允许请求 header 打开本次详细堆栈日志 | `false` |
| `debugFlag` / `debugFlagValue` | header 名和值的精确比较 | 空字符串 |
| `application.trace.requestID` | 从请求 header 读取 trace 值并作为日志字段名 | `requestId` |

starter 始终提供 stack handler，但是否真正生成 `ErrorStack()` 取决于 `debugMode || enablePrintStack || (enableDebugFlag && header == debugFlagValue)`。请求 debug flag 只扩大服务端详细堆栈日志条件，不会把本次响应切换成 debug 模式；对外数据隐藏仍只由 `debugMode` 控制。若启用 `enableDebugFlag` 却把 `debugFlagValue` 留空，缺少该 header 的请求也会因空字符串相等而打开详细堆栈日志。

进入详细堆栈分支时，handler 会记录路由 params、query 和请求 headers；请求 body 当前没有启用。Fiber 与 Gin 分别从各自原生 Context 收集这些字段。header 在编码前统一脱敏：`authorization`、`cookie`、`proxy-authorization`、`x-auth-token`、`x-api-key`，以及名称包含 `token`、`secret`、`password` 的字段都会被遮盖。值的字节长度不超过 8 时替换为 `***`，更长时只保留前 4 个字节再加 `...***`。

这套脱敏只覆盖 header 名启发式规则，不覆盖 query、route param 或业务 data 中的秘密；不要把凭据放入这些位置。序列化 headers/params/query 使用应用的“fast traffic codec”实例，取得或类型断言失败时 stack handler 会记录错误并提前返回。

## 生产配置与示例差异

生产环境应设置：

```yaml
application:
  recover:
    debugMode: false
    enablePrintStack: false
    enableDebugFlag: false
```

仓库当前 [`example_config/application_prod.yml`](../../example_config/application_prod.yml) 仍把 `debugMode` 设为 `true`。该文件只是示例，照搬会向客户端暴露业务 data、runtime/普通 error 详情，并生成更详细的服务端堆栈日志。部署前必须显式覆盖为 `false`，同时使用不可猜测的 debug flag 值或彻底关闭该入口。

示例 Swagger 也不是运行时真相。以示例异常表为例：

| 异常 | 业务 code | 当前统一处理器的 HTTP status | 示例 Swagger 可能声明 |
|---|---:|---:|---:|
| `InputParamError` / `ValidateException` | 400001 | 400 | 400/422 |
| `NotFoundDocument` / `Exception` | 400002 | 400 | 404 |
| `InternalError` / `Exception` | 500001 | 400 | 500 |
| 未知 `error` | `UnknownErrCode` | 500 | 500 |

如果 API 契约要求 404、422 或业务异常 500，需要修改运行时映射并补测试；只改 Swagger 注释不会改变响应。

## 对象池与并发边界

ErrorHandler、RecoveryPManager、响应 facade 以及 Context adaptor 都含进程级单例或对象池。标准 handler 必须在同一同步调用链内使用 adaptor/响应对象；发送后不可跨 goroutine 保存。正常穿过 recovery 而没有调用 adaptor 的 `JSON/Send` 时，adaptor 没有统一 `Release`，属于源码生命周期限制。

响应 facade 还存在静态的重复释放/未归还风险，详见[《响应与序列化》](response-and-serialization.md)。当前测试没有覆盖 Fiber/Gin error adaptor、panic 分类、生产数据隐藏、敏感 header 脱敏与二进制错误响应的端到端组合，因此本页对池复用和传播问题不声称已做运行时复现。

源码入口：[`recover_error_handler_impl.go`](../../recover_error_handler_impl.go)、[`recover_config.go`](../../recover_config.go)、[`recover_providers_and_manager.go`](../../recover_providers_and_manager.go)、[`recover_fiber_impl.go`](../../recover_fiber_impl.go)、[`recover_gin_impl.go`](../../recover_gin_impl.go)、[`adaptor/errorhandler`](../../adaptor/errorhandler/) 与 [`exception`](../../exception/)。
