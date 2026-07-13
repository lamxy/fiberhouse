# 扩展 FiberHouse

FiberHouse 当前可用的扩展单位是 Provider、Manager、Location、应用/模块注册器以及 `CoreStarter`。扩展前先确认调用链中确实有消费者：类型、目录或 Location 名称存在，不代表代码会自动执行。基础选择规则见[《Provider 系统》](../concepts/provider-system.md)，真实启动位点见[《Web 启动生命周期》](../concepts/startup-lifecycle.md)。

## 先选择已有扩展点

按成本从低到高选择：

1. 只需要数据库、缓存、client 或业务对象：注册 `GlobalManager` initializer，不要创建 Provider。
2. 只需要应用中间件、模块路由或 Swagger：优先实现 `ApplicationRegister` / `ModuleRegister` 的现有回调；需要按 Core target 分组时再引入 Provider/Manager。
3. 需要切换 JSON 或响应 body 编码：复用默认 JSON/response Manager，只增加匹配类型的 Provider。
4. 需要新的启动阶段：增加 Type、Provider、Manager、Location，并同时编写读取该 Location 的消费者。
5. 需要新的 HTTP 内核：实现完整 `CoreStarter`，再补齐 codec、recovery、context adaptor、中间件和路由适配；这不是只增加一个常量。

所有扩展对象应在 `RunServer` 前构造并冻结。默认集合、Type/Location registry 与多个 Manager 都是进程级单例，不适合请求运行期热注册、卸载或切换。

## 新增 Provider 与 Manager

下面骨架展示完整协议。`SetType` 让 `RunServer` 能把 Provider 分发给 Manager；`SetTarget` 只是这里选择 Fiber 的应用约定，必须由 Manager 显式比较；`MountToParent` 让基类动态分派到外层实现。Manager 必须设置 Location；`ZeroLocation` 由 `RunServer` 遍历传入的 Manager 列表消费，不需要绑定到 Location 的 Manager 列表。

```go
package extension

import (
	"fmt"

	fh "github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/constant"
)

var warmupType = fh.ProviderTypeGen().MustCustom("Warmup")

type WarmupProvider struct{ fh.IProvider }

func NewWarmupProvider() *WarmupProvider {
	p := &WarmupProvider{IProvider: fh.NewProvider().
		SetName("WarmupProvider").
		SetVersion("v1").
		SetTarget(constant.CoreTypeWithFiber).
		SetType(warmupType)}
	p.MountToParent(p)
	return p
}

func (p *WarmupProvider) Initialize(
	ctx fh.IContext,
	initFuncs ...fh.ProviderInitFunc,
) (any, error) {
	p.Check()
	// 在启动期创建或校验对象；错误返回给 Manager。
	return nil, nil
}

type WarmupManager struct{ fh.IProviderManager }

func NewWarmupManager(ctx fh.IApplicationContext) *WarmupManager {
	m := &WarmupManager{IProviderManager: fh.NewProviderManager(ctx).
		SetName("WarmupManager").
		SetType(warmupType).
		SetOrBindToLocation(fh.ProviderLocationDefault().ZeroLocation)}
	m.MountToParent(m)
	return m
}

func (m *WarmupManager) LoadProvider(
	loadFuncs ...fh.ProviderLoadFunc,
) (any, error) {
	m.Check()
	core := m.GetContext().(fh.IApplicationContext).GetBootConfig().CoreType
	for _, p := range m.List() { // map 转 slice，顺序不稳定
		if p.Target() == core {
			if _, err := p.Initialize(m.GetContext()); err != nil {
				return nil, fmt.Errorf("initialize %s: %w", p.Name(), err)
			}
		}
	}
	return nil, nil
}
```

这个 Manager 显式处于 `ZeroLocation`；只要它进入 `WithPManagers`，`RunServer` 就会在创建 Starter 前加载它，无需也不应额外调用 `ZeroLocation.Bind`。装配时 Provider 与 Manager 两边都要进入 `FiberHouse`：

```go
providers := fh.DefaultProviders().AndMore(NewWarmupProvider())
managers := fh.DefaultPManagers(house.AppCtx).AndMore(NewWarmupManager(house.AppCtx))
house.WithProviders(providers...).WithPManagers(managers...).RunServer()
```

同一 Manager 中 `Name` 重复会返回错误；Type 未设置时 `Check` panic。Manager 内部 Provider 顺序不稳定，依赖顺序时应在自定义 Manager 中显式排序。当前 Location 的重复绑定检查还会拒绝同一 Location 的后续 Manager；不要设计成“同一位点堆叠多个 Manager”后假设都会执行。

## 新增执行 Location

自定义 Location 只有注册表和 Manager 列表，不存在通用调度器：

```go
var (
	catalogType = fh.ProviderTypeGen().MustCustom("Catalog")
	afterCatalog = fh.ProviderLocationGen().MustCustom("AfterCatalog")
)

type CatalogManager struct{ fh.IProviderManager }

func NewCatalogManager(ctx fh.IApplicationContext) *CatalogManager {
	m := &CatalogManager{IProviderManager: fh.NewProviderManager(ctx).
		SetName("CatalogManager").
		SetType(catalogType).
		SetOrBindToLocation(afterCatalog)}
	m.MountToParent(m)
	if err := afterCatalog.Bind(m); err != nil {
		panic(err)
	}
	return m
}

func (m *CatalogManager) LoadProvider(
	loadFuncs ...fh.ProviderLoadFunc,
) (any, error) {
	m.Check()
	for _, provider := range m.List() {
		if _, err := provider.Initialize(m.GetContext()); err != nil {
			return nil, fmt.Errorf("initialize %s: %w", provider.Name(), err)
		}
	}
	return nil, nil
}

func runAfterCatalog() error {
	for _, manager := range afterCatalog.GetManagers() {
		if _, err := manager.LoadProvider(); err != nil {
			return err
		}
	}
	return nil
}
```

`CatalogManager` 必须重载 `LoadProvider`；只嵌入基类方法会让基类经 `sonManager` 再分派回自身，不能形成有效加载链。应用还必须在自己的可达生命周期中调用 `runAfterCatalog`。若希望框架调用，优先绑定已经被消费的默认 Location，并核对该入口是否真的执行 Manager；例如 `RunServer` 虽把若干 Manager 传给 Starter，内建 Fiber/Gin 并不会读取所有参数。`LocationServerShutdownBefore`、`LocationServerShutdownAfter` 和 `LocationAdaptCoreCtxChoose` 当前没有标准启动消费者。

## 新增中间件或路由注册器

中间件与路由仍使用原生 Fiber/Gin API。下面是可复用的 Provider 形状；`typ` 分别使用 `GroupMiddlewareRegisterType` 或 `GroupRouteRegisterType`：

```go
type CoreRegistrationProvider struct {
	fh.IProvider
	register func(fh.CoreStarter) error
}

func NewFiberRegistrationProvider(
	name string,
	typ fh.IProviderType,
	register func(fh.CoreStarter) error,
) *CoreRegistrationProvider {
	p := &CoreRegistrationProvider{
		IProvider: fh.NewProvider().SetName(name).
			SetTarget(constant.CoreTypeWithFiber).
			SetType(typ),
		register: register,
	}
	p.MountToParent(p)
	return p
}

func (p *CoreRegistrationProvider) Initialize(
	ctx fh.IContext,
	initFuncs ...fh.ProviderInitFunc,
) (any, error) {
	p.Check()
	if len(initFuncs) == 0 {
		return nil, fmt.Errorf("provider %s requires CoreStarter", p.Name())
	}
	v, err := initFuncs[0](p)
	if err != nil {
		return nil, err
	}
	core, ok := v.(fh.CoreStarter)
	if !ok {
		return nil, fmt.Errorf("provider %s expected CoreStarter", p.Name())
	}
	return nil, p.register(core)
}
```

配套 Manager 必须 `SetType(typ)`、`MountToParent(manager)`，并绑定 `LocationAppMiddlewareInit` 或 `LocationRouteRegisterInit`；`LoadProvider` 接收注册器注入的 `CoreStarter`，只初始化 `Target()==BootConfig.CoreType` 的 Provider。然后还要有消费者：

- `ApplicationRegister.RegisterAppMiddleware` 读取 `LocationAppMiddlewareInit.GetManagers()` 并用返回 `CoreStarter` 的 `ProviderLoadFunc` 调用 Manager。
- `ModuleRegister.RegisterModuleRouteHandlers` 对 `LocationRouteRegisterInit` 做同样操作。
- 最后把 Provider/Manager 通过 `WithProviders` / `WithPManagers` 注册；只有构造文件而不装配不会运行。

仓库 `example_application/providers` 展示这种接线，但它是示例，不是可导入的稳定扩展包。若不需要 target 选择，直接在两个注册器回调中调用 `core.GetCoreApp()` 并断言原生引擎通常更简单。

## 新增 JSON codec

JSON codec Provider 复用默认 `JsonCodecPManager`。Provider 的 `Version` 必须等于 `BootConfig.TrafficCodec`，`Target` 必须等于 `BootConfig.CoreType`，Type 必须是 `GroupTrafficCodecChoose`：

```go
type MyFiberJSONProvider struct{ fh.IProvider }

func NewMyFiberJSONProvider() *MyFiberJSONProvider {
	p := &MyFiberJSONProvider{IProvider: fh.NewProvider().
		SetName("MyFiberJSONProvider").
		SetVersion("my_json_codec").
		SetTarget(constant.CoreTypeWithFiber).
		SetType(fh.ProviderTypeDefault().GroupTrafficCodecChoose)}
	p.MountToParent(p)
	return p
}

func (p *MyFiberJSONProvider) Initialize(
	ctx fh.IContext,
	_ ...fh.ProviderInitFunc,
) (any, error) {
	p.Check()
	return myJSONWrapper{}, nil // 实现 fh.JsonWrapper
}
```

把 Provider 加入 `WithProviders`，保留 `DefaultPManagers(ctx)` 中已挂载到 `LocationCoreEngineInit` 的 JSON Manager，并把 `TrafficCodec` 设置为 `my_json_codec`。Fiber 只要求 `JsonWrapper` 的 `Marshal/Unmarshal`；Gin 写入的是进程级 `gin/codec/json.API`，自定义对象还必须满足 Gin 的 encoder/decoder 接口，且会影响同进程其他 Gin 使用者。

JSON codec 负责引擎 JSON 请求/响应，不负责 MsgPack/Protobuf 协商。若 codec 需要从 `GlobalManager` 读取实例，必须先由应用 initializer 注册对应 key，并保证在 `InitCoreApp` 前可取得。

## 新增响应协议

响应协议 Provider 的 `Name` 就是完整 MIME type，Type 是 `GroupResponseInfoChoose`，`Initialize` 返回一个实现 `response.IResponse` 的对象：

```go
type CBORResponseProvider struct{ fh.IProvider }

func NewCBORResponseProvider() *CBORResponseProvider {
	p := &CBORResponseProvider{IProvider: fh.NewProvider().
		SetName("application/cbor").
		SetType(fh.ProviderTypeDefault().GroupResponseInfoChoose)}
	p.MountToParent(p)
	return p
}

func (p *CBORResponseProvider) Initialize(
	ctx fh.IContext,
	_ ...fh.ProviderInitFunc,
) (any, error) {
	p.Check()
	return getCBORResponse(), nil // 每次返回本次发送独占的 response.IResponse
}
```

这里没有 `SetTarget`：当前 `RespInfoPManager` 只按 MIME `Name` 查找，不读取 Target 或 Version，添加虚假的 target 反而会误导选择语义。默认 response Manager 已 `MountToParent` 并绑定 `LocationResponseInfoInit`；保留它并把新 Provider 加入 `WithProviders` 即完成注册。应用还必须设置 `EnableBinaryProtocolSupport=true`，客户端按当前协商规则发送该 MIME。

自定义 `IResponse` 必须完整实现字段复制、编码/发送、`Release` 与池所有权。`ResponseWrap` 会调用 `From(source, true).SendWithCtx(...)`；实现必须明确 source 与自身何时释放，避免跨 goroutine、重复归还或返回仍被池复用的引用。完整协商限制见[《响应与序列化》](response-and-serialization.md)。

## 新增 CoreStarter

新 Core 必须实现整个 `CoreStarter`，而不是只返回一个 engine：

```go
type CoreWithEcho struct {
	ctx fh.IApplicationContext
	app any
}

func (c *CoreWithEcho) GetAppContext() fh.IApplicationContext { return c.ctx }
func (c *CoreWithEcho) GetCoreApp() interface{}               { return c.app }
func (c *CoreWithEcho) InitCoreApp(fh.FrameStarter, ...fh.IProviderManager) {}
func (c *CoreWithEcho) RegisterAppMiddleware(fh.FrameStarter, ...fh.IProviderManager) {}
func (c *CoreWithEcho) RegisterModuleSwagger(fh.FrameStarter, ...fh.IProviderManager) {}
func (c *CoreWithEcho) RegisterAppHooks(fh.FrameStarter, ...fh.IProviderManager) {}
func (c *CoreWithEcho) RegisterModuleInitialize(fh.FrameStarter, ...fh.IProviderManager) {}
func (c *CoreWithEcho) AppCoreRun(...fh.IProviderManager) {}

type EchoCoreProvider struct{ fh.IProvider }

func NewEchoCoreProvider() *EchoCoreProvider {
	p := &EchoCoreProvider{IProvider: fh.NewProvider().
		SetName("CoreEchoProvider").
		SetTarget("echo").
		SetType(fh.ProviderTypeDefault().GroupCoreStarterChoose)}
	p.MountToParent(p)
	return p
}

func (p *EchoCoreProvider) Initialize(
	ctx fh.IContext,
	initFuncs ...fh.ProviderInitFunc,
) (any, error) {
	p.Check()
	var opts []fh.CoreStarterOption
	if len(initFuncs) > 0 {
		v, err := initFuncs[0](p)
		if err != nil {
			return nil, err
		}
		var ok bool
		opts, ok = v.([]fh.CoreStarterOption)
		if !ok {
			return nil, fmt.Errorf("expected []CoreStarterOption")
		}
	}
	core := &CoreWithEcho{ctx: ctx.(fh.IApplicationContext)}
	for _, opt := range opts {
		opt(core)
	}
	return core, nil
}
```

将 `BootConfig.CoreType` 设为 `echo`，把 Provider 注册进 `WithProviders`，并保留默认 `CoreStarterPManager`：该 Manager 已设置 `GroupCoreStarterChoose`、`MountToParent` 并绑定 `LocationCoreStarterCreate`，会按 Target 选择新 Provider。若替换 Manager，则必须完成同样的 Type、parent mounting、Location 绑定和 `[]CoreStarterOption` 注入契约。

上面的空方法只列出接口形状，不构成可运行 Core。真正实现还必须：创建并停止 server；安装 JSON codec、普通 error handler 与 panic recovery；定义原生 request context 到 `ICoreContext` 的适配；回调应用中间件、模块路由和 Swagger；处理监听错误、信号、超时与资源所有权。默认 recovery、JSON、中间件和路由 Provider 只覆盖 Fiber/Gin target，新 Core 必须分别提供匹配实现。

## 验证清单

- Provider 的 Name 唯一，Type 与 Manager 完全一致；需要选择时 Provider 已 `SetTarget`，Manager 明确比较 Target/Version/Name。
- 每个自定义 Provider 与 Manager 都在构造器中调用 `MountToParent`；不是只构造了内部基类。
- Zero-location Manager 已通过 `SetOrBindToLocation(ZeroLocation)` 设置位置并进入 `WithPManagers`，没有额外调用 `Bind`；`RunServer` 会按 Location ID 直接加载它。
- 非 zero Location 只有在消费者调用 `GetManagers()` 时才需要显式 `Bind`；绑定 error 已检查，且该消费者确实可达。
- Provider 与 Manager 分别进入 `WithProviders` 与 `WithPManagers`；默认集合不是自动装配。
- 初始化错误能到达可观察入口，没有被 `RunServer`、Manager 或日志分支静默忽略。
- 所有 map/registry 在启动期冻结；Manager 若需要顺序，自己排序而不依赖 map 遍历。
- goroutine、连接、对象池和 writer 有唯一所有者、停止顺序与重复关闭策略。
- 为目标 Core、缺失 Provider、重复注册、初始化失败、运行和关闭路径编写最小集成测试。

## 当前不承诺的扩展面

`plugins` 目前只有接口/占位文件，没有 loader、registry、启动与关闭链；RPC 只有响应 proto 结构，没有 client/server 生命周期；MQ 与通用 i18n 也没有运行实现。不要为这些目录设计或文档化不存在的 plugin、RPC、MQ、i18n 注册 API。

同样不能把 Gin TLS、未消费的 shutdown Location、Provider `Unregister`、Provider 状态字段或默认集合热修改描述为成熟扩展协议。二进制 HTTP 响应不是 RPC，新 Core 的 `GetCoreApp()` 也不会自动让现有 Fiber/Gin provider 兼容它。扩展应以当前接口与可达调用链为准，示例目录只用于观察装配方式。
