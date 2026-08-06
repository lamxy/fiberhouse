# hertzcore —— 第三方核心引擎接入参考

本目录是 [cloudwego/hertz](https://github.com/cloudwego/hertz) 作为 FiberHouse 可切换核心引擎的完整接入实现。

**它的价值不只是「多支持一个引擎」，更是「如何在不修改框架的前提下接入任意 HTTP 引擎」的可运行模板。**

> 完整教程见 [docs/guides/custom-core-starter.md](../../docs/guides/custom-core-starter.md)。
> 本文只做目录导航与最小上手。

---

## 目录结构

| 目录 | 内容 | 对应的框架契约 |
|---|---|---|
| `constant/` | `CoreTypeWithHertz = "hertz"` 等标识常量 | `BootConfig.CoreType` 与各 Provider 的 `Target()` |
| `adaptor/` | `HertzContext` —— 原生上下文适配 + 对象池 | `adaptor/context.ICoreContext` |
| `recovery/` | `HertzRecovery` —— panic 恢复与请求数据提取 | `fiberhouse.IRecover` |
| `starter/` | `CoreWithHertz` —— 引擎生命周期九方法 | `fiberhouse.CoreStarter` |
| `providers/` | 六类 Provider | 各 `Group*` Provider 类型 |

传输层（handler 与路由）位于 `../module/example-hertzapi-module/api/`，与 Fiber/Gin 的模块目录并列。

---

## 切换到 Hertz

在 `example_main/main.go` 中：

```go
CoreType: hertzconst.CoreTypeWithHertz,   // 替换 constant.CoreTypeWithFiber
```

并确保七个 Provider 已加入 `WithProviders`（已在 `example_main/main.go` 中接线，可直接参考）。

配置位于 `example_config/application_*.yml` 的 `application.plugins.engine.servers.hertz`。

---

## 三个关键设计点

1. **复用框架的位点分派**
   每个生命周期方法都调用 `fiberhouse.LoadProviderManagersAtLocation(...)`，
   而非自行复制「Provider 分派 + 扩展替代」逻辑。

2. **错误处理委派框架处理器**
   Hertz 没有 Gin 的 `c.Error()` 错误链。正确做法是委派
   `fiberhouse.NewErrorHandlerOnce(appCtx).ErrorHandler(...)`，
   它已实现 `fiber.Error → ValidateException → Exception → 未知错误` 的完整分类。
   自行实现容易漏掉校验错误分支，导致本该 400 的响应变成 500。

3. **信号处理交给框架**
   使用 `Engine.Run()` 而非 `Hertz.Spin()`——后者会重复注册信号监听，
   与框架 `RunServer` 的统一优雅关闭冲突。

---

## 验证状态

已在真实依赖环境（MongoDB / Redis / MySQL 容器）下端到端验证：

- 完整 CRUD：`POST` 201 → `GET` 200 → `PUT` 200 → `LIST` 200 → `DELETE` 204
- 领域错误映射：不存在资源 → 404
- 参数校验：非法 ID → 400，含字段级错误信息
- panic 恢复：以框架统一信封响应，进程不退出
- Swagger UI：`/swagger/index.html` 与 `/swagger/doc.json` 均 200
- 响应格式：全部为框架统一信封 `{"code":..,"msg":..,"data":..}`

单元测试：`go test ./example_application/hertzcore/...`
