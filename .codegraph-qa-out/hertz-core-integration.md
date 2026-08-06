# Hertz 核心启动器接入分析（example_application 扩展验证）

> 目标：在 **不改动框架本身** 的前提下，于 `example_application/` 中把 cloudwego/hertz 接入为可切换的核心启动器（与 fiber/gin 并列），并验证框架"基于接口 + Provider 装配"的扩展性。

---

## 1. 框架的核心可切换机制（调研结论）

`BootConfig.CoreType` 是**普通 string**（`boot.go:57`），不是枚举。`constant.CoreTypeWithFiber/Gin` 只是便利常量。
所有按核心分发的管理器一律使用同一套判定：

```go
provider.Target() == bootCfg.CoreType
```

因此**新增任意 `Target()` 字符串即可接入新核心，无需改动框架代码**。

### 需要按 CoreType 匹配的分发点

| 分发点 | 文件 | 归属 | Provider 类型 |
|---|---|---|---|
| 核心启动器选择 | `core_starter_manager.go:57` | 框架 | `GroupCoreStarterChoose` |
| JSON 编解码选择 | `json_codec_manager.go:39`（同时匹配 `Version()==TrafficCodec`） | 框架 | `GroupTrafficCodecChoose` |
| Recover 中间件选择 | `recover_providers_and_manager.go:106` | 框架 | `GroupRecoverMiddlewareChoose` |
| 应用中间件注册 | `example_application/providers/middleware/app_middleware_manager.go:52` | 示例 | `GroupMiddlewareRegisterType` |
| 路由 + Swagger 注册 | `example_application/providers/module/route_register_manager.go:48` | 示例 | `GroupRouteRegisterType` |
| 核心生命周期 Hook | `example_application/providers/apphook/app_hook_manager.go:35` | 示例 | `GroupCoreHookChoose` |

> 关键点：前三个管理器在框架内，但它们只**遍历自己注册到的 provider 列表**。
> `DefaultProviders()` / `DefaultPManagers()` 是单例集合，且暴露 `Add()` / `AndMore()`（`default.go:69,105`），
> 所以 example 侧可以把 hertz 的 provider **追加**进去，框架文件一行不动。

---

## 2. 必须在 example 侧补齐的 Provider 清单

要让 `CoreType = "hertz"` 跑通完整启动链，需提供 6 类 `Target()=="hertz"` 的 provider：

1. **CoreStarter provider** → 返回实现 `fiberhouse.CoreStarter` 的 `CoreWithHertz`
2. **JSON codec provider** → `Target()="hertz"` + `Version()=TrafficCodec`（否则 `InitCoreApp` 内 `resolveJSONCodec` 找不到会 panic）
3. **Recovery provider** → 返回实现 `fiberhouse.IRecover` 的 `HertzRecovery`
4. **App middleware provider** → 注册 hertz 应用级中间件
5. **Route register provider** → 注册路由 + swagger
6. **App hook provider** → 注册 hertz 生命周期钩子

---

## 3. CoreStarter 接口契约（`application_interface.go:78`）

Hertz 实现必须覆盖 9 个方法：

```
GetAppContext() IApplicationContext
GetCoreApp() interface{}
InitCoreApp(fs FrameStarter, managers ...IProviderManager)
RegisterAppMiddleware(fs FrameStarter, managers ...IProviderManager)
RegisterAppHooks(fs FrameStarter, managers ...IProviderManager)
RegisterModuleInitialize(fs FrameStarter, managers ...IProviderManager)
RegisterModuleSwagger(fs FrameStarter, managers ...IProviderManager)
AppCoreRun(...IProviderManager) error
Shutdown(...IProviderManager) error
```

每个方法的标准骨架（参照 `core_gin_starter_impl.go`）：
1. `if cg.GetAppContext().GetAppState() { return }` 短路
2. 调 `loadProviderManagersAtLocation(managers, <对应位点>, self)`，若 `replaced` 则直接返回（这是框架给的"扩展替代"能力）
3. 执行本核心的默认逻辑

> ⚠️ `loadProviderManagersAtLocation` 是**包内私有函数**，example 包（`package hertzcore` 等）**无法调用**。
> 因此 example 侧的 `CoreWithHertz` 只能省略该步骤，或改为遍历 `managers` 自行按 Location 匹配 —— 后者是可行的，
> 因为 `IProviderManager.Location()` / `LoadProvider()` 都是导出接口。

---

## 4. 上下文适配层（`adaptor/context`）

框架的 recover / 统一响应链路全部走 `adaptorctx.ICoreContext`（`adaptor/context/core_ctx_wrap_interface.go:10`）：

```go
type ICoreContext interface {
    GetCtx() interface{}
    GetHeader(key string) string
    SetHeader(key, value string)
    JSON(statusCode int, data interface{}) error
    Send(statusCode int, body []byte) error
}
```

这是个**普通导出接口，无未导出方法** → example 侧可自由实现 `HertzContext`。
`releaseCoreContext`（`recover_fiber_impl.go:120`）通过 `interface{ Release() }` 鸭子类型回收，
所以 example 的 `HertzContext` 只要额外提供 `Release()` 就能接入对象池回收。

`recoverPanicInternal` → `Response().From(...).SendWithCtx(pCtx, status)` → `ICoreContext.JSON/Send`，
整条统一响应链因此对 hertz 天然可用。

---

## 5. 配置命名空间

`example_config/application_*.yml` 中：

```yaml
application.plugins.engine.servers.<coreType>
```

gin 用 `servers.gin`（`core_gin_starter_impl.go:184`）。hertz 沿用 `servers.hertz` 即可，
配置文件属于 example 资产，可自由新增。

---

## 6. 结论：框架扩展性验证（已实测通过）

✅ **框架无需任何修改**。核心可切换设计是真正基于接口 + 运行时字符串匹配的，
新增核心引擎的成本 = 实现 `CoreStarter` + `IRecover` + `ICoreContext` 三个导出接口 + 注册 6 类 provider。

`git status` 证实：框架根目录与 `adaptor/` 下无任何文件被修改，
变更仅限 `example_config/`、`example_main/main.go`、`go.mod`/`go.sum` 及新增的 example 目录。

~~⚠️ 唯一的扩展摩擦点：`loadProviderManagersAtLocation` 未导出~~
**已解决**：该函数已导出为 `fiberhouse.LoadProviderManagersAtLocation`
（`starter_manager_loader.go`），成为框架正式的扩展点。

- 框架内 20 处调用点（Fiber 10 + Gin 10）已同步更名；
- hertz 侧删除了本地复制的 `manager_loader.go`，改用框架导出版本；
- 5 个语义锁定测试已移入框架根的 `starter_manager_loader_test.go`，
  作为公开 API 的契约测试。

---

## 7. 实作落地清单

| 路径 | 内容 |
|---|---|
| `example_application/hertzcore/constant/` | `CoreTypeWithHertz = "hertz"` 等标识常量 |
| `example_application/hertzcore/adaptor/` | `HertzContext`（`ICoreContext` 实现 + 对象池） |
| `example_application/hertzcore/recovery/` | `HertzRecovery`（`IRecover` 实现 + 敏感头脱敏） |
| `example_application/hertzcore/starter/` | `CoreWithHertz`（`CoreStarter` 9 方法）+ `loadManagersAtLocation` |
| `example_application/hertzcore/providers/` | 6 类 provider（core/codec×2/recovery/middleware/hook/route） |
| `example_application/module/example-hertzapi-module/api/` | hertz 传输层 handler 与路由注册 |

切换方式：`main.go` 的 `BootConfig.CoreType` 设为 `hertzconst.CoreTypeWithHertz` 即可。

---

## 8. 实作中发现的关键差异点

1. **hertz handler 签名为 `func(context.Context, *app.RequestContext)`**，比 fiber/gin 多一个标准 context 参数。
2. **`RequestContext.JSON` 无返回值**、**`GetHeader` 返回 `[]byte`**，适配 `ICoreContext` 时需转换。
3. **hertz 无 gin 的 `c.Error()` 错误链**。正确做法是在 handler 内直接委派给框架的
   `IErrorHandler.ErrorHandler`（已导出），它已实现
   `fiber.Error → ValidateException → Exception → 未知错误` 的完整分类与状态码映射。
   ⚠️ 曾误以自定义逻辑只解 `*fiber.Error`，导致验证错误返回 500 而非 400；委派框架处理器后修正。
4. **`AppCoreRun` 应使用 `Engine.Run()` 而非 `Hertz.Spin()`**。
   `Spin()` 会自行注册系统信号监听并调用 `Shutdown`，与框架 `RunServer` 的
   `coordinateServerRun` 统一信号处理冲突。
5. **hertz 无内置 requestid 中间件**，需自行以 uuid 生成并写入上下文键 `traceId`，
   供 `HertzRecovery.TraceID` 与日志链路取用。

---

## 9. 端到端验证结果

以 `CoreType="hertz"` 实际启动并发出 HTTP 请求：

```
GET /common/test/get-must-instance?t=hello-
→ 200  Server: hertz   X-Request-Id: <uuid>
  {"code":0,"msg":"ok","data":"hello-Hello World!"}

GET /examples/not-a-valid-id
→ 400  {"code":400001,"msg":"Invalid request parameters",
        "data":{"id":"id can only contain alphanumeric characters"}}

GET /common/test/get-must-instance-failed
→ 500  {"code":500,"msg":"get instance failed: assertion failure ..."}
```

三点证明整条链路打通：
- `Server: hertz` → hertz 确为实际核心引擎
- `{"code":..,"msg":..,"data":..}` → 走的是**框架**统一响应格式（经 `ICoreContext`）
- 验证错误正确返回 400 且带字段级信息 → 与 fiber/gin 的错误契约完全一致

启动日志显示 8 条路由全部经 provider 链注册成功，监听 `[::]:8080`。

测试：`go test ./...` 全绿；`go vet` 无告警。
新增单元测试 15 个（context 4 + recovery 6 + 位点加载器 5，后者已移入框架根）。

---

## 10. 真实依赖环境下的完整验证（第二轮）

在本机 Docker（MongoDB `27037` / Redis `6379` / MySQL `3306`）已启动的条件下复验，
启动日志中 redis 连接错误由数百条降为 **0**：

```
POST   /examples          → 201  data.id = 6a740e1c8ce48826b497122c（真实 ObjectID）
GET    /examples/{id}     → 200  返回刚创建的记录
PUT    /examples/{id}     → 200  name 已更新为 hertz-e2e-updated
GET    /examples          → 200
GET    /examples/0000...  → 404  领域错误正确映射（此前因缺 Mongo 而返回 500）
DELETE /examples/{id}     → 204  → 复查 GET 返回 404，测试数据已清理
GET    /swagger/index.html → 200
GET    /swagger/doc.json   → 200  返回真实 spec（title: XXX Service APIs）
```

> 测试记录已删除，容器既有数据未受影响。

### Swagger 结论

- **注解层面无需变化**：swaggo 按 `method + path` 去重，多适配器标注相同 `@Router`
  会导致 `swag init` 冲突。既有约定是注解单一来源于 Fiber handler，
  Gin 不带 swaggo 标签；hertz 已遵循同一约定。
- **UI 路由层面需要补齐**：各适配器需各自挂载 Swagger UI 路由。
  已引入 `github.com/hertz-contrib/swagger` v0.1.0，在
  `RegisterHertzSwagger` 中注册 `/swagger/*any`，同样由
  `application.swagger.enable` 控制。
