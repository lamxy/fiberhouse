# Provider 系统

FiberHouse 的扩展链由 `Type`、`Target`、Provider、Manager 和 Location 共同组成：Provider 描述“可做什么”，Manager 描述“同组对象如何选择和执行”，Location 描述“哪个生命周期入口会找到这个 Manager”。当前机制已接入 Web 启动链，但错误传播、执行顺序和部分 Location 接线存在限制。

## 五个核心概念

| 概念 | 当前契约 | 关键边界 |
|---|---|---|
| `IProviderType`（Type） | ID + 名称；Provider 与 Manager 以 `GetTypeID()` 匹配 | 默认 ID 为 0–63，自定义 ID 为 64–255；名称注册重复或区间耗尽时返回错误，`Must*` 版本 panic |
| `Target` | Provider 上的字符串选择维度，常用 `fiber` / `gin` | 框架不统一解释；具体 Manager 决定是否以及如何匹配 |
| `IProvider` | 名称、版本、Type、Target、状态和 `Initialize` | Type 必须在初始化前设置；基类写操作只适合启动期 |
| `IProviderManager` | 收集同一 Type 的 Provider，决定选择、排序语义和 `LoadProvider` 行为 | Provider 保存在 map 中，默认没有稳定遍历顺序；自定义 Manager 必须实现加载策略 |
| `IProviderLocation`（Location） | ID + 名称 + 绑定的 Manager 列表 | 只标识执行位置；没有入口读取该 Location 时不会自动运行 |

`Name` 是 Manager map 内的唯一键，`Version` 和 `Target` 是可选的选择条件。以 JSON codec 为例，`JsonCodecPManager` 同时检查 Type、`BootConfig.TrafficCodec` 对应的 Version 以及 `BootConfig.CoreType` 对应的 Target；默认 fallback Manager 则只对 `GroupProviderAutoRun` 无条件执行，其他 Provider 只按 Target 等于 `CoreType` 执行。

`ProviderTypeDefault()` 预定义 Choose、Type、AutoRun、Unique 等命名组，但这些后缀只是内置约定，不会自动生成加载算法。真正的选择逻辑仍在各 Manager 的 `LoadProvider` 中。

## 注册、分发与加载

完整路径是：

```text
WithProviders → RunServer 按 Type ID 分发 → Manager.Register
              → Manager 所属 Location 被生命周期入口读取
              → Manager.LoadProvider → Provider.Initialize(IContext, ...)
```

`RunServer` 对每个 Provider 只注册到第一个 Type ID 匹配的 Manager。若找不到匹配 Manager，则注册到 fallback `DefaultPManager`。这个 fallback 来自 `RunServer` 的第一个可变参数；没有参数时由框架新建。`WithPManagers(...)` 中即使已经含有默认 Manager，也不会自动成为这个参数。

没有 Location 的 Manager 初始属于 `ZeroLocation`。`RunServer` 在创建 Starter 之前会加载所有 zero-location Manager；随后若 fallback 有 Provider，又会再加载一次 fallback。自定义初始化若非幂等，应避免依赖这条重复调用路径。

绑定到非零 Location 的 Manager 不在这一步加载。它只有在对应入口读取 Location 并显式调用 `LoadProvider` 时才执行。内置启动链的真实消费范围见[《Web 启动生命周期》](startup-lifecycle.md)。自定义 Location 没有自动调度器。

## 状态

源码声明四个状态：

| 状态 | ID | 含义 |
|---|---:|---|
| `StatePending` | 0 | 待处理，也是 `NewProvider()` 的默认值 |
| `StateLoaded` | 1 | 声明为已加载 |
| `StateSkipped` | 2 | 声明为已跳过 |
| `StateFailed` | 3 | 声明为失败 |

当前 Manager 和 `RunServer` 不会在成功、跳过或失败时自动更新状态；`SetStatus` 也只通过 `sync.Once` 接受一次赋值。因此这些值目前是可用的状态词汇，不是可靠的运行时观测系统。不要根据 `Status()` 推断 Provider 已执行，也不要在请求期修改状态。

## 基类组合与 parent mounting

`Provider` 与 `ProviderManager` 是通过组合模拟可覆写行为的基类。它们分别保存 `sonProvider` / `sonManager`，基类方法再动态分派给外层具体类型。因此自定义构造器必须完成 parent mounting：

- Provider 外层实例调用 `MountToParent(outerProvider)` 后，基类 `Initialize` / `RegisterTo` 才能回到外层实现。
- Manager 外层实例调用 `MountToParent(outerManager)` 后，基类 `LoadProvider` 才能回到外层实现。
- 未挂载时，相关基类方法返回错误；把父对象自身作为 son 也会报错或 panic。

若构造器在 mounting 之前调用 `SetOrBindToLocation(location, true)`，Location 中保存的是内部基类 Manager；只要随后正确 mounting，调用基类 `LoadProvider` 仍会分派到外层实现。更易读的自定义写法是先 mounting，再绑定，并检查直接调用 `location.Bind` 时返回的错误。

## unique-provider 模式

`BindToUniqueProvider` 把 Manager 切换为“只允许一个 Provider”：

- 空 Manager 直接注册该 Provider 并标记 unique；
- 已经是同一个名称、同一个实例时视为成功；
- 已有不同 Provider，或已有多个 Provider 时 panic；
- unique Manager 之后再 `Register` 会返回错误。

`RunServer` 在 bootstrap 位置只检查 Manager 列表第一项，且仅当它 `IsUnique()` 时执行；before-run、after-run 会寻找并执行第一个 unique Manager。unique 因而既是数量约束，也是这些入口是否会执行的条件。它不等于进程级单例；调用方仍需保证构造和集合只发生一次。

## 默认集合不是自动装配

`DefaultProviders()` 当前收集 Frame、Fiber/Gin Core、Std/Sonic JSON、Fiber/Gin recovery、Protobuf/MsgPack response Provider。`DefaultPManagers(ctx)` 收集 fallback、Frame、Core、JSON codec、recovery 和 response Manager。

二者都是进程级单例集合：

- `List()` 返回当前切片的副本；
- `AndMore(...)` 返回默认项与自定义项的新切片；
- `Add` / `Except` 会修改后续调用看到的默认集合；
- `Default()` 不会读取这两个集合，应用必须显式传给 `WithProviders` / `WithPManagers`。

集合中的 Manager 在构造时可能已经绑定进进程级 Location。测试、重复启动或多应用同进程场景会共享这些对象和绑定状态，不能假设每次 `DefaultPManagers(ctx)` 都会用新 Context 构造一套隔离 Manager。

## 最小自定义骨架

下面的 Provider/Manager 使用一个自定义 Type，Manager 保持默认 `ZeroLocation`，所以在已有完整应用装配中会于 Starter 创建前被加载。所有符号与当前接口匹配；它只展示扩展协议，不包含业务资源生命周期。

```go
package extension

import fh "github.com/lamxy/fiberhouse"

var warmupType = fh.ProviderTypeGen().MustCustom("Warmup")

type WarmupProvider struct {
	fh.IProvider
}

func NewWarmupProvider() *WarmupProvider {
	p := &WarmupProvider{
		IProvider: fh.NewProvider().
			SetName("WarmupProvider").
			SetVersion("v1").
			SetType(warmupType),
	}
	p.MountToParent(p)
	return p
}

func (p *WarmupProvider) Initialize(
	ctx fh.IContext,
	initFuncs ...fh.ProviderInitFunc,
) (any, error) {
	p.Check()
	// 只在启动期执行初始化；ctx 提供配置、日志与容器。
	return nil, nil
}

type WarmupManager struct {
	fh.IProviderManager
}

func NewWarmupManager(ctx fh.IApplicationContext) *WarmupManager {
	m := &WarmupManager{
		IProviderManager: fh.NewProviderManager(ctx).
			SetName("WarmupManager").
			SetType(warmupType),
	}
	m.MountToParent(m)
	return m
}

func (m *WarmupManager) LoadProvider(
	loadFuncs ...fh.ProviderLoadFunc,
) (any, error) {
	m.Check()
	for _, provider := range m.List() {
		if _, err := provider.Initialize(m.GetContext()); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// AddWarmup 只追加扩展；house 仍须由调用方完成默认 Provider、
// Manager、ApplicationRegister 与 ModuleRegister 的完整装配。
func AddWarmup(house *fh.FiberHouse) {
	house.WithProviders(NewWarmupProvider()).
		WithPManagers(NewWarmupManager(house.AppCtx))
}
```

若要设置 Target，应在 Provider 上调用 `SetTarget`，并由 `LoadProvider` 明确比较它；框架不会替自定义 Manager 推断选择规则。若要绑定自定义 Location，还必须实现并调用读取它的生命周期消费者。

## 错误、顺序与并发边界

- 同一个 Manager 中 Provider 名称重复时，`Register` 返回 `ErrProviderAlreadyExists`。`RunServer` 记录错误后继续，而且仍把该 Provider 视为“已匹配”，不会转入 fallback。
- 未匹配 Provider 注册 fallback 失败时只记录日志。fallback 加载仅执行 AutoRun 或 Target 等于 `BootConfig.CoreType` 的 Provider，其余项静默跳过。
- `DefaultPManager.LoadProvider` 当前把每次 Initialize 的 error（包括 nil）都追加到切片，只要尝试过 Provider 就会构造汇总 error；而 `RunServer` 又忽略该阶段错误。这是静态分析观察，调用方不能依赖其返回值准确反映成功。
- Provider 或 Manager 未设置 Type 时，`Check()` panic。创建 Frame/Core 所需 Manager 缺失或返回类型不符时，`RunServer` 使用 fatal 日志。
- Manager 用 map 保存 Provider，`List()` 顺序不稳定。若存在优先级，应由具体 Manager 排序或按唯一选择条件确定。
- Location 用锁保护 Manager 切片并返回副本；但当前 `Bind` 以 Location ID 检查重复，会拒绝同一 Location 的后续 Manager，`SetOrBindToLocation` 又忽略该错误。不要承诺同一位置可可靠执行多个 Manager。
- Provider/Manager 基类没有保护其名称、Type、Target、map 等全部启动写入；默认集合虽然对切片加锁，也不使内部对象可在运行期安全重配。推荐模式是启动期注册、运行期只读。
- `Unregister` 当前为空操作；不要用它实现热卸载。Provider 状态也未形成自动转换与日志链。

源码入口为 [`provider_interface.go`](../../provider_interface.go)、[`provider_impl.go`](../../provider_impl.go)、[`provider_manager_impl.go`](../../provider_manager_impl.go)、[`provider_type.go`](../../provider_type.go)、[`provider_location.go`](../../provider_location.go) 和 [`default.go`](../../default.go)。
