# 响应与序列化

FiberHouse 的统一业务响应是 `response.IResponse`，默认 JSON 形状为：

```json
{
  "code": 0,
  "msg": "ok",
  "data": null
}
```

`code` 是业务 code，`msg` 是业务消息，`data` 是业务数据；三者都不决定 HTTP status。HTTP status 由 `JsonWithCtx`/`SendWithCtx` 的可选 `status` 参数决定，未传时为 `200 OK`。

## HTTP status 与业务 code

这两个维度必须分别设计和监控：

| 场景 | HTTP status | 典型业务 code |
|---|---:|---:|
| `SuccessWithData`，未显式传 status | 200 | 0 |
| `ErrorCustom(code, msg)`，未显式传 status | 200 | 调用方给定值 |
| recovery 处理 `ValidateException` | 400 | 注册的验证业务码 |
| recovery 处理 `Exception` | 400 | 注册的业务码，不按数字前缀推断 status |
| 未分类的 `error` / runtime panic | 500 | `UnknownErrCode` |

例如，示例异常表中的 `NotFoundDocument` 是业务 code `400002`，实际 recovery 响应仍是 HTTP 400，不是 404；`InternalError` 是业务 code `500001`，只要类型是 `*Exception`，实际也映射为 HTTP 400。示例 handler 的 Swagger 注释同时声明了 404/500，因此不能用这些注释反推当前运行时映射。错误分类详见[《错误与恢复》](errors-and-recovery.md)。

## 构造器、接口与 facade

[`response.IResponse`](../../response/response_interface.go) 提供字段读取、`Reset`、`From`、成功/失败修改、JSON/字节发送和 `Release`。主要入口分两层：

- `response.NewRespInfo`、`response.SuccessWithData`、`response.ErrorCustom` 返回池化 `*RespInfo`；`NewRespInfoWithoutPool`、`SuccessWithoutPool`、`ErrorWithoutPool` 创建时不从池取对象。不过后者仍继承 `RespInfo.Release/JsonWithCtx`，一旦释放或发送，仍会被放入 `respPool`；“WithoutPool”只描述构造动作。
- 根 package 的 `fiberhouse.Response()` 返回池化 `*ResponseWrap`。它代理 `Reset`、`SuccessWithData`、`ErrorCustom`、`From`，并在自己的 `SendWithCtx` 中决定 JSON 或二进制格式。
- `fiberhouse.RespInfo()`、`RespProto()`、`RespMsgpack()` 可直接取得具体响应实现；直接使用时，调用方仍需遵守各实现的释放语义。
- `exception.Exception` 与 `ValidateException` 复用相同字段和 `IResponse` 方法，但用于错误分类，不应只凭 `code` 猜测其类型。

典型 handler 把原生 Context 包成最小 adaptor 后发送：

```go
return fiberhouse.Response().
	SuccessWithData(result).
	SendWithCtx(adaptorctx.WithFiberContext(c), http.StatusOK)
```

Gin handler 没有 `error` 返回值，应检查发送结果，并按应用策略交给 `c.Error(err)`；不要丢弃序列化或 socket 写入错误。`ICoreContext` 的边界与生命周期见[《Web 运行时》](web-runtime.md)。

## 默认 JSON 路径

`RespInfo.JsonWithCtx` 始终把结构体交给 `ICoreContext.JSON`，并在返回时释放 `RespInfo`。JSON 的 `data` 即使为 nil 也会编码为 `null`。引擎使用哪个 JSON 库，取决于启动期选择的 std/Sonic JSON codec：Fiber 把 encoder 放在 app 配置中，Gin 修改包级 `gin/codec/json.API`。

JSON 写出错误的接口语义并不对称：`FiberContext.JSON` 直接返回 `fiber.Ctx.JSON` 的错误，调用链可以收到 Fiber 的编码/写出失败；`GinContext.JSON` 调用 `gin.Context.JSON` 后无条件返回 nil，因此通过 `IResponse.JsonWithCtx`/JSON 分支 `SendWithCtx` 无法观察 Gin JSON 写出的失败。两边的原始字节 `Send` 都返回底层发送/`Writer.Write` 错误。

启动期 JSON codec 与响应协商是正交机制：

- `BootConfig.TrafficCodec` 选择引擎 JSON 编解码实现。
- `BootConfig.EnableBinaryProtocolSupport` 控制 `ResponseWrap.SendWithCtx` 是否尝试 MsgPack/Protobuf。

开启二进制支持不会把 `TrafficCodec` 改成 Protobuf，也不会改变请求绑定所用的 JSON codec。

## `SendWithCtx` 的实际协商顺序

只有通过 `ResponseWrap.SendWithCtx` 且 `EnableBinaryProtocolSupport=true` 才会进入协商。当前算法是：

1. 先读取请求 `Content-Type`；只有该值为空才读取 `Accept`。
2. 逗号分隔时只取第一项，再删除第一个分号后的参数并 trim 空格。
3. `application/json`、`application/*`、`*/*` 直接走 JSON。
4. 其他 MIME 以完整字符串查找 `RespInfoPManager` Provider。
5. 找到并初始化成功时设置响应 `Content-Type`，复制 `{code,msg,data}` 后调用对应实现。
6. MIME 未知、Provider 缺失、初始化失败或类型不匹配时回退 JSON；不会返回 `406 Not Acceptable`。

“第一项”不等于按 q 值排序。例如 `text/plain;q=0.1, application/msgpack;q=1` 仍先尝试 `text/plain`，然后回退 JSON。请求 `Content-Type` 的优先级又高于 `Accept`，因此 `Content-Type: application/json` 与 `Accept: application/msgpack` 会返回 JSON。调用方若依赖二进制响应，应按当前约定只发送一个受支持 MIME，并理解这不是完整的 RFC 内容协商器。

## MsgPack 与 Protobuf

默认注册两个响应 Provider：

| MIME | 实现 | 当前结构 |
|---|---|---|
| `application/msgpack` | `RespInfoMagPack` | map 中始终有 `code`、`msg`，`data != nil` 时才加入 `data` |
| `application/x-protobuf` | `RespInfoPB` | `response/pb.RespInfoProto{code,msg,data}`，data 通过 `structpb.Value` 包进 `Any` |

MsgPack 和 Protobuf 只改变响应 body 编码，不改变 HTTP status 或业务 code。Protobuf 的 `Reset` 在 `structpb.NewValue` / `anypb.New` 失败时不会向调用方返回转换错误，而是留下 nil/旧值边界；复杂自定义 Go 类型不应假定都能无损转成 `structpb.Value`。MsgPack 客户端解析 helper 对字段具体类型做直接断言，面对不受信任或不同 schema 的数据可能 panic，调用方需要额外校验。

## 对象池所有权与传播风险

`RespInfo`、`ResponseWrap`、两个二进制响应以及两个 Context adaptor 都使用 `sync.Pool`。池化对象只属于一次同步请求/发送链：发送或 `Release` 后不得保存指针、跨 goroutine 使用、再次读取或再次释放；放入 `data` 的可变对象仍由业务代码负责并发安全。

当前 facade 存在以下源码静态风险：

- `ResponseWrap.SendWithCtx` 自身 `defer r.Release()`；JSON 分支又调用会自行释放的 `RespInfo.JsonWithCtx`，使同一个内层响应可能被重复放回池。
- 二进制分支的 `resp.From(r.IResponse, true)` 已释放源 `RespInfo`，facade 返回时又会通过 wrapper 再释放源对象。
- Protobuf `SendWithCtx` 会释放自身；MsgPack `SendWithCtx` 不会释放自身，因此两种二进制实现的所有权不对称。
- `ResponseWrap` 没有覆盖 `JsonWithCtx`。经 facade 构造后若直接调用提升的 `JsonWithCtx`，内层 `RespInfo` 会归还，但外层 wrapper 没有对应归还动作。
- `ICoreContext.JSON/Send` 会归还 adaptor；只读 header 或没有发送动作的路径不会。

这些结论来自逐行的池所有权与控制流分析，未通过竞态测试、压力测试或对象复用故障复现。文档因此把它们标为重复释放、过早复用或未归还的风险，而不宣称每次请求都会产生可见错误。修复前应避免手动追加释放，并尽量让一次响应只经过一个明确发送入口。

## 错误传播

所有发送方法的签名都返回 `error`，但并非所有引擎路径都能产生非 nil 值：Fiber JSON 会返回引擎错误，Gin JSON adaptor 固定返回 nil；MsgPack/Protobuf 的编码错误会在调用 Context 前返回，二进制写出则由两边的 `Send` 返回。上层处理仍不一致：panic recovery 忽略发送结果；Gin 错误 adaptor 只会在统一处理器实际返回非 nil 时再次 panic，Gin JSON 写出失败不会经该返回值触发；Fiber adaptor 在成功写出统一错误响应后仍可能把原始 handler error 返回给 Fiber。最后一种是“已写响应后继续传播原错误”的静态风险，尚未核对 Fiber 对最终 socket 的每一种处理结果。

## 测试边界

当前 [`response/response_impl_test.go`](../../response/response_impl_test.go) 覆盖 `RespInfo` 的构造、Reset/Release、基础并发池使用和标准库 JSON 序列化。它没有覆盖：

- `ResponseWrap` facade 与协商分支；
- `Content-Type`/`Accept` 多值、q 值及未知 MIME fallback；
- MsgPack/Protobuf 发送、转换失败和 Content-Type；
- Context adaptor 的池所有权；
- error/recovery 与响应 facade 的组合路径。

因此基础 `RespInfo` 测试通过不能证明二进制协商或对象池组合链安全。

源码入口：[`response_facade.go`](../../response_facade.go)、[`response`](../../response/)、[`response_providers_and_manager.go`](../../response_providers_and_manager.go)、[`adaptor/context`](../../adaptor/context/) 与 [`exception`](../../exception/)。
