# 自定义核心启动器（以 Hertz 为例）

FiberHouse 的核心 HTTP 引擎是可切换的：框架内置 Fiber 与 Gin，应用侧可以在**不修改框架任何一行代码**的前提下接入第三方引擎。

本文以 `example_application/hertzcore/`（cloudwego/hertz）为完整参考，说明接入一个新 Core 需要做什么、为什么这样做，以及实作中容易踩空的地方。该实现已通过端到端验证（真实 MongoDB/Redis/MySQL 环境下的完整 CRUD、Swagger UI、panic 恢复与错误契约）。

> 只想看接口形状与 Provider/Manager 契约，见[扩展 FiberHouse](extending-fiberhouse.md)；本文是它的可运行落地版本。

---

## 1. 为什么不需要改框架

`BootConfig.CoreType` 是**普通 string**，不是枚举（`boot.go`）。`constant.CoreTypeWithFiber` / `CoreTypeWithGin` 只是便利常量。

所有按核心分派的管理器，一律使用同一个判定：

```go
provider.Target() == bootCfg.CoreType
```

因此**新增任意 Target 字符串即可接入新核心**。框架侧的三个管理器虽然位于框架包内，但它们只遍历自己被注册到的 Provider 列表，而 `DefaultProviders()` / `DefaultPManagers()` 提供了 `Add()` / `AndMore()`（`default.go`），应用侧可以把新 Core 的 Provider 追加进去。

### 需要按 CoreType 匹配的分派点

| 分派点 | 所在文件 | 归属 | Provider 类型 |
|---|---|---|---|
| 核心启动器选择 | `core_starter_manager.go` | 框架 | `GroupCoreStarterChoose` |
| JSON 编解码选择 | `json_codec_manager.go`（同时匹配 `Version()==TrafficCodec`） | 框架 | `GroupTrafficCodecChoose` |
| Recover 中间件选择 | `recover_providers_and_manager.go` | 框架 | `GroupRecoverMiddlewareChoose` |
| 应用中间件注册 | `example_application/providers/middleware/app_middleware_manager.go` | 示例 | `GroupMiddlewareRegisterType` |
| 路由 + Swagger 注册 | `example_application/providers/module/route_register_manager.go` | 示例 | `GroupRouteRegisterType` |
| 核心生命周期 Hook | `example_application/providers/apphook/app_hook_manager.go` | 示例 | `GroupCoreHookChoose` |

---

## 2. 最小可用实现：三个接口 + 六类 Provider

### 2.1 必须实现的三个导出接口

| 接口 | 定义位置 | 职责 |
|---|---|---|
| `fiberhouse.CoreStarter` | `application_interface.go` | 引擎生命周期（9 个方法） |
| `fiberhouse.IRecover` | `recover_interface.go` | panic 恢复与请求数据提取 |
| `adaptor/context.ICoreContext` | `adaptor/context/core_ctx_wrap_interface.go` | 原生请求上下文 → 框架统一抽象 |

三者都是**普通导出接口、无未导出方法**，所以应用侧可以自由实现。

### 2.2 必须提供的六类 Provider

要让 `CoreType="hertz"` 跑通完整启动链，缺一不可：

| # | Provider | Type | 缺失后果 |
|---|---|---|---|
| 1 | CoreStarter | `GroupCoreStarterChoose` | 无法创建核心启动器 |
| 2 | JSON codec | `GroupTrafficCodecChoose` | `InitCoreApp` 解析编解码器失败 |
| 3 | Recovery | `GroupRecoverMiddlewareChoose` | recover 中间件加载失败 |
| 4 | App middleware | `GroupMiddlewareRegisterType` | 应用级中间件不生效 |
| 5 | Route register | `GroupRouteRegisterType` | 路由与 Swagger 不注册 |
| 6 | App hook | `GroupCoreHookChoose` | 生命周期钩子不生效 |

> **注意 #2**：JSON codec 管理器同时按 `Version()==BootConfig.TrafficCodec` 与 `Target()==CoreType` 两个条件筛选，所以**每个核心都要为每种 codec 各注册一个 Provider**。Hertz 参考实现注册了 sonic 与 std 两个。

---

## 3. 目录组织

参考实现的分层（`example_application/` 之下）：

```
hertzcore/
├── constant/     CoreTypeWithHertz = "hertz" 等标识常量
├── adaptor/      HertzContext —— ICoreContext 实现 + 对象池
├── recovery/     HertzRecovery —— IRecover 实现 + 敏感头脱敏
├── starter/      CoreWithHertz —— CoreStarter 九方法
└── providers/    六类 Provider

module/example-hertzapi-module/api/    该核心的传输层 handler 与路由
```

---

## 4. 关键实现点

### 4.1 生命周期方法的统一骨架

`CoreStarter` 的每个方法都遵循同一范式，**复用框架导出的位点分派函数**：

```go
func (ch *CoreWithHertz) InitCoreApp(fs fiberhouse.FrameStarter, managers ...fiberhouse.IProviderManager) {
	// 1. 短路已运行的应用
	if ch.coreApp != nil || ch.initializationFailed() || ch.GetAppContext().GetAppState() {
		return
	}

	// 2. 分派该位点的 Provider
	_, replaced, err := fiberhouse.LoadProviderManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationCoreEngineInit, ch)
	if err != nil {
		ch.logErr(err, "InitCoreApp providers failed")
		return
	}
	if replaced {
		return // 已被 GroupExtendReplace 接管
	}

	// 3. 本核心的默认逻辑
	// ...
}
```

`LoadProviderManagersAtLocation` 是框架导出的扩展点，**不要自行复制**。

### 4.2 上下文适配器

框架的统一响应链路（`Response().SendWithCtx`）与 panic 恢复链路都走 `ICoreContext`。实现它，新核心即可复用框架既有的响应与异常能力：

```go
func (h *HertzContext) JSON(statusCode int, data interface{}) error {
	defer h.Release()
	h.Ctx.JSON(statusCode, data) // hertz 的 JSON 无返回值
	return nil                    // 统一返回 nil 以符合接口契约
}

func (h *HertzContext) GetHeader(key string) string {
	return string(h.Ctx.GetHeader(key)) // hertz 返回 []byte
}
```

额外提供 `Release()` 即可接入框架的对象池回收——框架以 `interface{ Release() }` 鸭子类型判定，不需要实现额外接口。

### 4.3 错误处理：直接委派框架处理器

**这是最容易踩空的一点。** Gin 依赖 `c.Error()` 错误链 + 错误处理中间件；Hertz 没有等价机制。

不要自己实现一套错误分类，而应直接委派给框架已导出的 `IErrorHandler.ErrorHandler`：

```go
func respondError(ctx fiberhouse.IContext, reqCtx *app.RequestContext, err error) {
	eh := fiberhouse.NewErrorHandlerOnce(ctx.(fiberhouse.IApplicationContext))
	_ = eh.ErrorHandler(hertzadaptor.WithHertzContext(reqCtx), err)
}
```

它已实现完整的分类与状态码映射：`fiber.Error → ValidateException → Exception → 未知错误`。

> 参考实现最初只解 `*fiber.Error`、其余一律 500，导致**校验错误返回 500 而非 400**。改为委派框架处理器后，错误契约与 Fiber/Gin 完全一致。

### 4.4 运行与关闭：不要用引擎自带的信号处理

框架的 `RunServer` 已统一接管系统信号与优雅关闭（`coordinateServerRun`）。若再调用引擎自带的信号循环（如 hertz 的 `Spin()`），会重复注册信号监听并自行 `Shutdown`，与框架冲突。

```go
// 正确：只负责监听，信号交给框架
if err = ch.coreApp.Run(); err != nil { ... }
```

### 4.5 配置命名空间

沿用框架既有约定：

```yaml
application.plugins.engine.servers.<coreType>
```

Hertz 使用 `servers.hertz`。配置文件属于应用资产，可自由新增。

### 4.6 Swagger

swaggo 注解在整个扫描树中按 `method + path` 去重，**多个适配器标注相同 `@Router` 会导致 `swag init` 冲突**。

约定：注解**单一来源于 Fiber handler**；其余适配器（Gin、Hertz）只写普通 Go doc 注释，不带 swaggo 标签。各适配器仍需**各自注册 Swagger UI 路由**，由 `application.swagger.enable` 控制：

```go
h.GET("/swagger/*any", hertzSwagger.WrapHandler(swaggerFiles.Handler))
```

详见 `example_application/docs/README.md`。

### 4.7 引擎日志与调试输出

引擎自身的日志（路由注册、监听地址、连接错误、优雅关闭进度）默认直接写 stderr，
**绕过框架的日志器**，因而不受 Origin、级别、轮转与异步 writer 配置管辖。
接入新 Core 时应把这条通道也接管过来。

各引擎的接管方式不同，需先确认它暴露的钩子：

| 引擎 | 接管点 | 形态 |
|---|---|---|
| Gin | `DebugPrintFunc`、`DebugPrintRouteFunc`、`DefaultWriter`、`DefaultErrorWriter` | 四个分散的包级全局量 |
| Hertz | `hlog.SetLogger(hlog.FullLogger)` | 单一集中接口（21 个方法） |

Hertz 的实现见 `example_application/hertzcore/adaptor/hertz_logger.go`，三个要点：

1. **级别映射**：hertz 有框架不具备的 `Trace` 与 `Notice`，分别归并到 `Debug` 与 `Info`。
2. **前缀剥离**：hertz 会给系统日志加 `"HERTZ: "` 前缀；该信息已由结构化字段
   `Component` 承载，转发前剥离以免污染 message。
3. **`Control` 接口置空**：`SetLevel` / `SetOutput` 实现为空操作——
   输出目标与级别由框架配置统一掌管，不允许引擎侧反向篡改。

**所有权与释放顺序**（两种引擎一致）：

接管是进程级副作用，需用 lease 表达所有权。`hlog.SetLogger` 明确声明非并发安全，
所以只在首次安装时装入一个稳定转发器，之后仅用原子指针切换转发目标：

```go
// InitCoreApp：须在创建 engine 之前安装，否则路由注册日志会漏到 stderr
lease, err := hertzadaptor.InstallHertzLogger(
    hertzadaptor.NewHertzLoggerAdapter(appCtx.GetLogger(), cfg.LogOriginFrame()))

// Shutdown：必须在 logger.Close() 之前释放，
// 否则引擎后续日志会写入已关闭的 writer
ch.releaseHertzLogger()
return ch.GetAppContext().GetLogger().Close()
```

释放后引擎日志回落到其原始日志器，而不是静默丢弃——这样应用关闭日志器之后，
引擎若仍有输出也不会丢失或写坏。`Release` 需幂等，以支持
`AppCoreRun` 与 `Shutdown` 两条路径分别释放。

> 注意区分两类日志：本节说的是**引擎自身**的日志；
> 每个请求的访问日志（method/path/status/latency）是另一条通道，
> 由 Core 自己的 HTTP 日志中间件产生，见 `CoreWithHertz.loggerMiddleware`。

---

## 5. 接线与切换

在 `example_main/main.go` 追加 Provider，并切换 `CoreType`：

```go
fh := fiberhouse.New(&fiberhouse.BootConfig{
	CoreType: hertzconst.CoreTypeWithHertz, // fiber | gin | hertz
	// ...
})

providers := fiberhouse.DefaultProviders().AndMore(
	// ... 既有 fiber/gin providers

	hertzproviders.NewCoreStarterHertzProvider(),
	hertzproviders.NewSonicJCodecHertzProvider(),
	hertzproviders.NewStdJCodecHertzProvider(),
	hertzproviders.NewHertzRecoveryProvider(),
	hertzproviders.NewHertzAppMiddlewareProvider(),
	hertzproviders.NewHertzAppHookProvider(),
	hertzproviders.NewHertzRouteRegisterProvider(),
)
```

管理器沿用既有的即可——它们都按 `Target()` 分派，无需为新核心新增管理器。

---

## 6. 引擎差异速查（Fiber / Gin / Hertz）

| 维度 | Fiber | Gin | Hertz |
|---|---|---|---|
| handler 签名 | `func(*fiber.Ctx) error` | `func(*gin.Context)` | `func(context.Context, *app.RequestContext)` |
| 响应 JSON | 返回 error | 无返回值 | 无返回值 |
| 读请求头 | `string` | `string` | `[]byte` |
| 错误传递 | 返回 error | `c.Error()` 链 | 无 —— 需委派框架处理器 |
| 内置 requestid | 有 | `gin-contrib/requestid` | 无 —— 需自行生成 |
| 生命周期钩子 | `Hooks().OnListen/OnShutdown` | 无 | `OnRun` / `OnShutdown` 切片 |
| 引擎日志接管点 | 配置 `fiber.Config` | 四个包级全局量 | `hlog.SetLogger` 单一接口 |

---

## 7. 验证清单

新 Core 落地后，逐项确认：

- [ ] `go build ./...` 与 `go vet ./...` 通过
- [ ] 编译期接口断言存在：`var _ fiberhouse.CoreStarter = (*YourCore)(nil)`
- [ ] 六类 Provider 全部注册，且 `Target()` 与 `BootConfig.CoreType` 一致
- [ ] 每种 TrafficCodec 都有对应的 codec Provider
- [ ] 实际启动并请求，确认响应体是**框架的统一信封**（`{"code":..,"msg":..,"data":..}`）而非引擎默认格式
- [ ] 校验失败返回 **400** 且带字段级信息（若返回 500，说明错误处理未委派框架处理器）
- [ ] 领域错误正确映射（如 not found → 404）
- [ ] panic 被恢复并以统一格式响应，未导致进程退出
- [ ] `Ctrl+C` 能优雅关闭，无重复信号处理
- [ ] Swagger UI 可访问（若启用）
- [ ] 引擎自身日志（路由注册、监听地址）出现在**框架日志文件**中而非 stdout/stderr，
      且带正确的 Origin 与级别；关闭流程结束后没有向已关闭 writer 写入的报错

---

## 8. 参考实现索引

| 关注点 | 文件 |
|---|---|
| 标识常量 | `example_application/hertzcore/constant/constant.go` |
| `ICoreContext` 实现 | `example_application/hertzcore/adaptor/hertz_context.go` |
| 引擎日志适配 | `example_application/hertzcore/adaptor/hertz_logger.go` |
| 日志接管与 lease | `example_application/hertzcore/adaptor/hertz_logger_install.go` |
| `IRecover` 实现 | `example_application/hertzcore/recovery/hertz_recovery.go` |
| `CoreStarter` 实现 | `example_application/hertzcore/starter/core_hertz_starter.go` |
| 六类 Provider | `example_application/hertzcore/providers/` |
| 传输层与路由 | `example_application/module/example-hertzapi-module/api/` |
| 位点分派语义测试 | `starter_manager_loader_test.go`（框架根） |
