# 配置与引导

FiberHouse 的配置分成两层：`BootConfig` 决定框架怎样开始引导，`AppConfig` 保存从环境变量和 YAML 合并出的应用配置。`New` 在 `RunServer` 之前依次创建进程级配置、日志器和 `AppContext`，因此配置装载失败会直接阻断构造；完整时序见[《Web 启动生命周期》](../concepts/startup-lifecycle.md)。

## BootConfig 与 AppConfig

`BootConfig` 位于根 package，负责框架类型、HTTP 流量 codec、配置目录、日志目录以及应用 ID、名称、版本等启动参数。`Default(opts...)` 当前构造以下默认值，再调用 `New(cfg)`：

| 字段 | `Default()` 的值 |
|---|---|
| `AppId` | 空字符串 |
| `AppName` | `FiberHouse Application` |
| `Version` | `0.0.1` |
| `Date` | 空字符串 |
| `FrameType` | `DefaultFrameStarter` 对应常量 |
| `CoreType` | `fiber` 对应常量 |
| `TrafficCodec` | `sonic_json_codec` 对应常量 |
| `EnableBinaryProtocolSupport` | `false` |
| `ConfigPath` | `./config` |
| `LogPath` | `./logs` |

`New(&BootConfig{...})` 不会补齐这些值，也不会检查 nil。直接使用 `New` 时应完整设置所需字段；如果传入空 `ConfigPath`，当前路径拼接会查找 `/application_dev.yml` 形式的文件，而不是回退到 `./config`。这是当前源码的静态限制，不应依赖空值获得默认目录。

`BootConfig.WithCustom` 可在启动期写入额外键值，`Finally` 会封闭后续写入；`GetValue` 返回错误，`GetMustValue` 在键缺失时 panic。这组存储只属于启动配置，不会合并到 koanf 配置树。

`AppConfig` 包装 koanf，并持有应用基础信息、日志、recover、trace、日志 Origin 与中间件开关的派生视图。`NewConfigOnce` 完成所有来源合并后调用 `AppConfig.Initialize`，从最终配置树生成这些视图。随后 `New` 会用非空的 `BootConfig.AppId`、`AppName`、`Version` 覆盖对应的 typed getter；该覆盖只修改派生字段，不会回写 `cfg.String("application.appId")` 等原始 koanf 键。

不要把 [`example_config/application_dev.yml`](../../example_config/application_dev.yml) 的值当作框架默认。它只展示仓库示例选择的一组配置；真正的框架默认必须以 `Default`、`NewAppConfig` 和各消费方 getter 的显式 fallback 为准。

## 配置加载顺序

`bootstrap.NewConfigOnce` 的精确顺序是：

```text
APP_ENV_
  → 选择 application_<env>.yml
  → 加载该 YAML
  → 将选中的 env 回写 application.env
  → APP_CONF_
  → AppConfig.Initialize
```

展开后有以下语义：

1. 先装载所有 `APP_ENV_` 变量。
2. 读取 `application.env`；缺失或空字符串时使用 `dev`。
3. 从配置目录加载 `application_<env>.yml`。文件内容后加载，因此会覆盖除最终回写项外的同名 `APP_ENV_` 键。
4. 把第 2 步选中的值重新写入 `application.env`，避免文件内的不同值改变已经完成的文件选择。
5. 最后装载 `APP_CONF_`，覆盖前面所有同名键。
6. 调用 `Initialize`，建立 typed 配置视图、日志 Origin map 和中间件开关 map。

YAML 文件缺失、不可读或解析失败时，`LoadYaml` 会 panic；环境 provider 或 map 装载失败也以 panic 结束引导。当前入口不把这些错误包装成 `New` 的返回值。

`NewAppConfig()` 自身的配置目录是 `./config`，`LoadYaml()` 无参数时的文件名是 `application.yml`。但框架的 `New` 总会把 `BootConfig.ConfigPath` 作为参数传给 `NewConfigOnce`，标准引导实际加载的是该目录下按环境选择的 `application_<env>.yml`。路径会转换为 slash 并去掉尾部 `/`。

## APP_ENV_ 选择环境文件

环境变量名在去掉 `APP_ENV_` 后，把每一个 `_` 替换成 `.`；代码不会自动转为小写。因此键映射大小写敏感：

```text
APP_ENV_application_env=prod
                 ↓
application.env=prod
                 ↓
<ConfigPath>/application_prod.yml
```

例如 `application_appType` 映射到 `application.appType`，不能写成 `application_apptype` 来代替。变量值含空格时，provider 会把它按空格拆成字符串切片。由于每个下划线都会变成分隔符，这种映射也不能表达键名中的字面下划线。

`APP_ENV_` 的主要职责是选择环境文件。虽然它能写入其他键，但这些键随后可能被 YAML 覆盖；业务覆盖应使用最后加载的 `APP_CONF_`。

## APP_CONF_ 覆盖配置

`APP_CONF_` 使用同一套大小写敏感映射，并在 YAML 与 `application.env` 回写之后加载。例如：

```text
APP_CONF_application_appName=orders
APP_CONF_application_appLog_level=debug
```

分别映射到 `application.appName` 与 `application.appLog.level`。`APP_CONF_application_env` 也能覆盖最终可读的 `application.env`，但不会回头重新选择 YAML 文件：文件选择已在它加载之前完成。

环境 provider 产生字符串或字符串切片，后续由 koanf typed getter 转换。部署时应使用目标 getter 可解析的表示形式，并在启动验证中读取关键值；框架不会集中校验所有业务配置。

## 常用配置分组

下表描述当前源码消费的主要分组，不为缺失键虚构默认值：

| 分组 | 作用与边界 |
|---|---|
| `application.env` | 选择配置文件；引导默认是 `dev` |
| `application.appId`、`appName`、`version` | `Initialize` 建立应用基础视图；非空 `BootConfig` 值随后覆盖 typed 视图 |
| `application.appLog` | console/file、level、`logOriginEnum`、轮转与异步 writer；详见[《日志》](logging.md) |
| `application.plugins.engine.servers.gin` | Gin mode、监听地址、timeout、header 限制与 TLS；详见[《Web 运行时》](web-runtime.md) |
| `application.recover` | debug、堆栈打印和请求调试标识 |
| `application.trace.requestID` | trace 请求 ID 键；`Initialize` 的源码 fallback 为 `requestId` |
| `application.middleware` | 初始化时复制到中间件开关 map |
| `application.globalManage` | `keepAlive` 与健康扫描 `interval`；详见[《GlobalManager》](global-manager.md) |
| `application.task.enableServer` | 是否在 Web 启动链中启动任务 worker |
| `application.swagger.enable` | 是否进入模块 Swagger 注册 |

布尔键缺失时读取为 `false`。未显式提供 fallback 的数值和字符串按 koanf 的零值读取；这不代表示例文件中的数值是框架默认。

## Gin mode 与 recovery debug

Gin mode 的规范键是 `application.plugins.engine.servers.gin.mode`，可设置为 Gin 接受的 `debug`、`release` 或 `test`。解析时第一个非空值优先：先读取规范键；规范键缺失或为空时读取旧的 `application.plugins.server.gin.mode` 作为兼容 fallback；两者都为空时使用 `release`。旧键不应再用于新配置。最终的非空值交给 `gin.SetMode` 验证，不受支持的非空值保持 Gin 既有的失败行为。

`application.recover.debugMode` 不再改变 Gin mode。它只控制 recovery 的响应细节与堆栈行为；开发和测试示例分别显式使用 `mode: debug`，生产示例显式使用 `mode: release`。从旧行为迁移时，应设置规范 mode 键，而不是依赖 recovery debug 间接切换 Gin。

## 读取与启动期修改

`IAppConfig` 提供 `String`、`Strings`、`Int`、`Int64`、`Float64`、`Bool`、`Duration` 和 `GetBytes`。除 `Bool` 外的 getter 可传一个 fallback；当读取结果是空字符串、空切片、零数值或零时长时也会采用 fallback，因此调用者无法借此区分“键缺失”和“显式配置为零”。

`GetApplication`、`GetAppLog`、`GetRecover`、`GetTrace` 返回 `Initialize` 时建立的结构体副本。引导完成后再直接修改 koanf，不会自动刷新这些副本、日志 Origin map 或中间件 map。

稳定的使用边界是：

- 启动期写：装载来源、应用 `BootConfig` 覆盖、注册自定义 Origin，以及完成其他组件装配。
- 运行期读：通过 typed getter 或只读视图读取已经冻结的配置。
- 停止期：先停止请求、任务和其他生产者，再关闭依赖配置创建的资源。

`SafeGet` / `SafeSet` 只为传入回调持有 `AppConfig` 自己的读写锁；直接 koanf getter、`GetLogOriginMap` 返回的 map 和其他注册表并不会自动加入同一事务。它们不构成热重载协议。`RegisterLogOrigin` 也直接写 map，只应在并发服务开始前调用。

## 单例与测试隔离限制

`NewConfigOnce` 和 `NewLoggerOnce` 各由 package 级 `sync.Once` 控制。进程内第一次调用固定配置目录、当时的环境变量和日志装配；后续 `New` 即使传入不同路径或修改 `APP_ENV_` / `APP_CONF_`，也会复用第一次的对象。默认 `AppContext` 与 `GlobalManager` 同样带进程级单例语义。

因此测试不应把多组环境、配置目录或日志方案放在同一进程内并假设相互隔离，也不应并行修改环境后竞争第一次初始化。可采用独立测试进程，或直接构造 `NewAppConfig` 并只测试局部配置逻辑；后者仍会连接进程级 `GlobalManager`，不能等同于完整应用沙箱。

当前源码还没有公开的配置/日志单例 reset。运行期重新调用装载函数、替换配置树或并发写 `BootConfig` 的自定义存储，都不属于受支持的应用生命周期。源码入口见 [`boot.go`](../../boot.go)、[`bootstrap/bootstrap.go`](../../bootstrap/bootstrap.go) 与 [`appconfig/config.go`](../../appconfig/config.go)。
