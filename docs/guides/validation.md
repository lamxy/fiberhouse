# 参数校验

FiberHouse 的 [`component/validate`](../../component/validate/) 包装 go-playground/validator，为每种启用语言保存独立 validator 与 translator，并把 `validator.ValidationErrors` 转成框架的 `ValidateException`。Web `AppContext` 创建时会初始化并持有包装器，应用随后可在 Web 启动阶段追加语言、tag 和 translation；当前 `CmdContext.GetValidateWrap()` 固定返回 nil，CLI 若需要校验，必须自行调用 `validate.NewWrap(cfg)` 并管理注册与引用。

它不是通用 i18n 系统，也不会自动绑定请求或选择语言。handler 仍要负责解析输入、确定语言、执行校验并把错误送入所选 HTTP 内核的错误通路。

## 内建语言与初始化

`validate.NewWrap(cfg)` 读取 `application.validate.langFlags`：

- 配置为空时，只注册默认 `en`。
- 非空时逐项转为小写，只识别 `en`、`zh-cn`、`zh-tw`；因此 YAML 中的 `zh-CN`、`zh-TW` 可以使用。
- 不认识的 flag 被静默忽略，不会自动回退或返回配置错误。

每种语言构造独立的 `validator.Validate`，启用 `validator.WithRequiredStructEnabled()` 并注册官方默认 translation。`DefaultLang` 固定为 `en`；`GetValidate(lang)`/`GetTranslator(lang)` 在请求语言未注册时回退到 en。

显式语言列表若漏掉 `en`，这个回退槽也不存在，getter 可能返回 nil，后续 `Struct` 或 `Translate` 会 panic。生产配置应始终包含 `en`，并在启动检查 `GetValidate(validate.LangEn) != nil` 及所有业务语言均已注册。

## 应用启动期扩展

`ApplicationRegister` 提供两个扩展入口：

```go
ConfigCustomValidateInitializers() []validate.ValidateInitializer
ConfigValidatorCustomTags() []validate.RegisterValidatorTagFunc
```

`FrameApplication.RegisterApplicationGlobals` 先注册普通 global initializer，再依次：

1. 调用每个语言 initializer，取得实现 `ValidateRegister` 的对象；
2. 调用其 `RegisterToWrap(*validate.Wrap)` 添加 validator、translator 和 lang flag；
3. 调用每个 custom tag 函数，由函数向所需语言的 validator 注册 validation/translation。

语言 initializer 的 `RegisterToWrap` 没有 error 返回值；内部失败只能自行 panic 或采用其他记录方式。custom tag 函数可返回 error，但 FrameStarter 只汇总并记录日志，不会让启动失败。若某个 tag 是业务启动的硬要求，应用需要在自己的入口执行可失败检查，而不是只依赖内建日志。

仓库的 [`validatecustom`](../../example_application/providers/validatecustom/) 展示日语、韩语和自定义 tag 的接线，但其中语言字符串和 translation 分支仍有不完善处，只适合阅读接口关系。它不扩展框架的内建语言承诺。

## handler 中执行校验

结构体校验的基本路径是：

```go
vw := appCtx.GetValidateWrap()
lang := validate.LangZhCN

if err := vw.GetValidate(lang).Struct(&req); err != nil {
	var fieldErrs validator.ValidationErrors
	if errors.As(err, &fieldErrs) {
		return vw.Errors(fieldErrs, lang, true)
	}
	return err
}
```

`Errors(..., true)` 把结构体字段名变为 snake_case；未传 true 时使用 camelCase。它返回 `exception.VeGet("InputParamError").RespData(map)`，因此应用必须向 GlobalManager 注册框架异常表和 `InputParamError`；缺失时异常 lookup 会 panic。

变量校验可用 validator 的 `Var` 后调用 `ErrorsVar`，动态 map 可用 `ValidateMap` 后调用 `ErrorsMap`。`ErrorsMap` 只处理值类型为 `validator.ValidationErrors` 的条目，其他错误值不会进入输出 map；调用方应先确认完整错误形状。

语言选择由业务完成，例如从 header 读取后映射到允许列表。不要把任意 header 直接作为 map key；虽然 getter 会 lower-case 并回退 en，显式 allowlist 更便于 API 契约和监控。

## 进入错误与响应通路

`ValidateException` 同时实现 `error` 和 `response.IResponse`。不同内核的交接方式是：

- Fiber handler 可直接 `return vw.Errors(...)`，随后进入 Fiber ErrorHandler。
- Gin handler 应 `_ = c.Error(vw.Errors(...))` 后立即 return；只从 handler 返回不会自动进入统一链。
- 业务也可以 panic 该异常，由当前内核 recovery 捕获，但普通输入错误通常无需借助 panic。

统一错误处理把 `ValidateException` 映射为 HTTP 400，并保留其业务 code/msg/data；字段错误 map 位于 data。业务 code 来自应用异常表，与 HTTP status 是两个维度。完整分类与 Gin/Fiber 差异见[《错误与恢复》](errors-and-recovery.md)，响应所有权见[《响应与序列化》](response-and-serialization.md)。

validator 返回的非 `ValidationErrors` 错误不应强制断言；应作为普通 error 传播，否则会丢失配置或类型错误。Gin `c.Error` 和 Fiber 返回 error 的最终发送错误语义也不完全对称。

## 并发与可变状态

`Wrap` 明确不是并发读写安全对象。内部包含 validator map、translator map 和 lang slice，注册方法没有锁；`GetValidators`、`GetTranslators`、`GetLangList` 还直接暴露内部引用。稳定生命周期是：

- Web `AppContext` 构造：注册配置选择的内建语言；CLI 自建 wrapper 时由 CLI 入口完成同一步骤。
- Web 启动期：完成所有应用语言、tag 和 translation 注册，并检查结果。
- 请求/任务运行期：只调用 validator 与 translator，不修改 map、slice 或规则。
- 关闭期：纯校验对象没有 `Close`，随 Context 生命周期结束。

不要在 handler 的 `RegisterValidationWrap`/`RegisterTranslationWrap` 之类方法里动态修改共享 wrapper。示例 request VO 中这两个方法为空，也注明推荐启动期统一注册；它们不是框架自动调用的稳定扩展链。

`AppContext` 持有 Web wrapper；`CmdContext` 当前不持有可用 wrapper，CLI 自建实例也不会自动进入 Web 的 `FrameApplication` 自定义语言/tag 注册链。两类入口仍可能共享配置、GlobalManager 与异常表的进程级对象。测试多语言或自定义 tag 时，应避免并行修改同一 wrapper，并为异常表初始化提供隔离。

## 启动检查清单

- `application.validate.langFlags` 至少包含 `en`，并只使用 `en`、`zh-CN`、`zh-TW` 或已注册的应用语言。
- 检查每个业务语言的 validator 和 translator 非 nil。
- 在并发服务启动前注册所有 custom tag 与 translation；把必需注册错误升级为启动失败。
- 注册异常表及 `InputParamError`，再测试 Fiber 返回 error 与 Gin `c.Error` 两条路径。
- 对字段命名策略、翻译文本和 HTTP 400 + 业务 code 编写 API 合约测试。
- 运行期只读，不把 wrapper 的内部 map/slice 暴露给会修改它们的业务代码。

源码入口：[`component/validate/validate_wrapper.go`](../../component/validate/validate_wrapper.go)、[`component/validate/en.go`](../../component/validate/en.go)、[`component/validate/zh_cn.go`](../../component/validate/zh_cn.go)、[`component/validate/zh_tw.go`](../../component/validate/zh_tw.go) 与 [`frame_starter_impl.go`](../../frame_starter_impl.go)。
