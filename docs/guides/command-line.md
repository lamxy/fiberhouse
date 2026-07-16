# 命令行应用

FiberHouse 的命令行形态基于 urfave/cli v2。它复用配置、日志、`GlobalManager` 和应用实例 key 契约，但不经过 Web 的 `FiberHouse.RunServer`、Provider/Manager/Location 或 HTTP 生命周期。当前 CLI 已有可达实现，错误返回、健康检查和资源关闭仍需应用补齐，因此属于实验性能力。

## 三层组成

| 层 | 当前实现 | 责任 |
|---|---|---|
| Frame | `FrameCmdStarter` / `FrameCmdApplication` | 持有 `ICommandContext` 与 `ApplicationCmdRegister`，注册 Origin 子日志器、应用全局对象，并按配置执行一次健康扫描 |
| Core | `CoreCmdStarter` / `CoreCmdCli` | 创建或接收 `*cli.App`，委托注册错误处理器、命令、全局 flags/action，最后调用 `cli.App.Run(os.Args)` |
| Application | `CommandStarter` / `CMDLineApplication` | 组合 Frame 与 Core 两个接口，作为 `RunCommandStarter` 的输入 |

`ApplicationCmdRegister` 同时嵌入 `IRegister` 与 `IApplication`。因此命令应用除了实现 `RegisterCommands` 等 CLI 回调，还必须实现数据库、缓存、codec、任务等实例 key getter；即使某个命令没有使用这些资源，接口方法仍不能省略。CLI 与 Web 可以复用同一个具体应用 key 约定，但两条启动链不会自动互相注册对象。

## 建立 ICommandContext

CLI 没有类似 `fiberhouse.New(BootConfig)` 的总入口。调用方先自行完成 bootstrap，再创建进程级命令上下文：

```go
cfg := bootstrap.NewConfigOnce("./config")
logger := bootstrap.NewLoggerOnce(cfg, "./logs")
ctx := fiberhouse.NewCmdContextOnce(cfg, logger)

app := newCommandApplication(ctx) // 实现 ApplicationCmdRegister
starter := &commandstarter.CMDLineApplication{
	FrameCmdStarter: commandstarter.NewFrameCmdApplication(
		ctx,
		option.WithCmdRegister(app),
	),
	CoreCmdStarter: commandstarter.NewCoreCmdCli(ctx),
}

// RunCommandStarter 当前不会自动回写；需要 Starter 回指的代码必须显式注册。
ctx.RegisterStarterApp(starter)
commandstarter.RunCommandStarter(starter)
```

`NewCmdContextOnce` 保存配置、日志器、进程级 `GlobalManager` 和进程级 Dig 容器。它不保存可用 validator：`CmdContext.GetValidateWrap()` 当前固定返回 nil，需要校验时由 CLI 自己构造并持有 `validate.Wrap`。配置、日志、Context、GlobalManager 和 Dig 都有单例边界；同进程重复执行多套命令装配不具备隔离保证。

配置路径和日志路径都相对进程当前工作目录，而不是相对 Go 源文件。仓库示例使用 `./../../example_config`，只有在匹配的目录运行时才能找到文件；正式命令应从明确工作目录、绝对路径或自身 flags 得到路径。

## RunCommandStarter 的实际顺序

[`RunCommandStarter`](../../commandstarter/frame_cmd_application.go) 固定执行：

```text
1. InitCoreApp
2. RegisterGlobalErrHandler(Frame)
3. RegisterCommands(Frame)
4. RegisterCoreGlobalOptional(Frame)
5. RegisterApplicationGlobals
6. AppCoreRun
```

顺序带来几个直接结果：

- `InitCoreApp` 从 `command.name`、`command.usage`、`command.version` 创建 `cli.App`；`Suggest`、bash completion 和短选项处理默认开启。
- `command.sortFlagsByName` / `sortCommandsByName` 在 commands 与 flags 注册之前执行。默认新建 app 时排序的是空切片，后续应用赋值不会再次排序；不能仅靠这两个配置保证最终顺序。
- Frame 必须先通过 `option.WithCmdRegister` 注入应用。没有 Frame option 时构造器走 fatal 日志；应用为 nil 时三个 Core 注册阶段会 panic。
- `RegisterApplicationGlobals` 在解析和执行具体 command 之前运行。因此即使只查看 help，应用回调仍可能注册或预初始化外部资源。
- `AppCoreRun()` 的 error 会由 `CoreCmdCli` 记录并返回，但 `RunCommandStarter` 明确丢弃该返回值。

`RunCommandStarter` 也不会调用 `ctx.RegisterStarterApp(starter)`。如果 model、cache option 或其他代码通过 `IContext.GetStarter().GetApplication()` 取实例 key，入口必须像上例一样先回写；仓库 CLI 示例当前遗漏了这一步，不能把它作为完整模板。

## 注册命令、全局 flag 与 action

`RegisterGlobalErrHandler`、`RegisterCommands`、`RegisterCoreGlobalOptional` 三个 `ApplicationCmdRegister` 回调接收的 `core` 静态类型是 `interface{}`；`RegisterApplicationGlobals` 没有参数。当前 `CoreCmdCli` 向前三个回调传入 `*cli.App`，应用应检查类型而不是直接信任示例中的强制断言：

```go
func (a *CommandApplication) RegisterCommands(core interface{}) {
	app, ok := core.(*cli.App)
	if !ok {
		panic("command core is not *cli.App")
	}
	app.Commands = []*cli.Command{
		{
			Name: "migrate",
			Action: func(c *cli.Context) error {
				return a.runMigration(c.Context)
			},
		},
	}
}

func (a *CommandApplication) RegisterCoreGlobalOptional(core interface{}) {
	app := core.(*cli.App)
	app.Flags = []cli.Flag{/* 全局 flags */}
	app.Action = func(c *cli.Context) error {
		return cli.ShowAppHelp(c)
	}
}
```

也可以用 `CommandGetter.GetCommand() interface{}` 分散构造命令，再在 `RegisterCommands` 汇总为 `*cli.Command`。这个接口没有类型约束，应用必须在汇总处检查返回值；重复名称、alias、flag 冲突和命令顺序均由 urfave/cli 与应用负责。

全局 `Action` 只在没有匹配子命令时执行，不是每个 command 的前置钩子。命令级 flags/action 应放在对应 `cli.Command`；全局 flags/action 放在 `RegisterCoreGlobalOptional`。

## 全局对象与一次健康扫描

Frame 先把配置中的 `LogOrigin` 子日志器 initializer 注册到 `GlobalManager`，再调用应用自己的 `RegisterApplicationGlobals()`。CLI 不使用 Web 的 `ConfigGlobalInitializers` / `ConfigRequiredGlobalKeys`，应用回调必须自行：

1. 调用 `Register` / `Registers` 注册 initializer；
2. 检查重复注册结果；
3. 对命令启动必需的 key 调用 `Get` 并决定是否失败退出；
4. 为成功创建的数据库、缓存、client 和 writer 指定关闭所有者。

当 `application.globalManage.keepAlive=true` 时，CLI 只在命令运行前同步遍历容器一次，调用 `CheckHealth` 并尝试 `Rebuild`；它没有 ticker，也没有后台保活周期。尚未初始化的 lazy entry 会被视为健康。重建失败后当前代码仍会追加一条 success 日志，因此不能只依赖该文本判断结果。完整容器限制见[《GlobalManager》](global-manager.md)。

## 错误、退出码与清理

`RegisterGlobalErrHandler` 由应用给 `cli.App.ExitErrHandler` 赋值。仓库示例会记录错误；若错误实现 `cli.ExitCoder`，使用其 exit code，否则使用 1，并通过 `cli.OsExiter` 退出进程。这是一种应用策略，不是框架自动策略。

需要可测试的退出码和确定清理时，不应把所有回收工作放在 `defer` 后再让深层 `ExitErrHandler` 调用 `os.Exit`：`os.Exit` 不执行 defer。仅仅绕过 `RunCommandStarter` 还不够；应用必须让 `RegisterGlobalErrHandler` 安装一个只记录、不调用 `cli.OsExiter` / `os.Exit` 的 handler，或者在自定义启动流程中跳过该注册阶段。这样 `cli.App.Run` 的 error 才能经 `AppCoreRun()` 返回顶层。

严格入口可按同一阶段顺序显式驱动：

```go
starter.InitCoreApp()
starter.RegisterGlobalErrHandler(starter.GetFrameCmdApp()) // 应用实现必须禁止深层退出
starter.RegisterCommands(starter.GetFrameCmdApp())
starter.RegisterCoreGlobalOptional(starter.GetFrameCmdApp())
starter.RegisterApplicationGlobals()

runErr := starter.AppCoreRun()
closeErr := closeApplicationResources()
exitCode := chooseExitCode(runErr, closeErr) // 可识别 cli.ExitCoder
os.Exit(exitCode) // 所有清理完成后，且只在最外层退出
```

若应用不能保证 `RegisterGlobalErrHandler` 的实现不退出，就不要调用该阶段，改由顶层处理 `AppCoreRun` 返回值。命令 action 可以返回带 exit code 的 error；顶层统一记录运行与关闭错误，再决定最终 exit code。

CLI 当前没有统一 shutdown 阶段，也不会自动调用 GlobalManager 中实例的 `Close`。安全顺序应由入口建立：停止新工作，等待命令启动的 goroutine，关闭数据库/缓存/client，关闭日志 writer，最后决定退出码。`GlobalManager.ClearAll(true)` 只删除引用，不能替代逐项关闭。

## `test-orm` 示例边界

[`example_application/command`](../../example_application/command/) 只用于说明装配。其 `test-orm` / `orm` 命令：

- 把 `ICommandContext`、MySQL model 和 service 构造器 `Provide` 到单例 Dig 容器；
- 检查 `Provide` 错误，再用 `github.com/lamxy/fiberhouse/component/container` 包的泛型 `container.Invoke` 取得 service；
- 每次执行都会先 `AutoMigrate`，随后按 `--method ok|orm` 运行演示操作；
- `--operation` 和 `--id` 只在 `--method orm` 分支读取。

它不是 migration 工具或生产数据命令。重复执行会向同一个 Dig 容器再次注册构造器，示例又预初始化 MongoDB、Redis、两个 Sonic codec 和 MySQL，即使当前命令只需要 MySQL 也可能连接其他服务。示例中的 MongoDB command service、cron wrapper 和更多 CRUD 方法没有可达命令入口。

采用 CLI 形态前至少检查：配置工作目录、Starter 回写、所需实例 key、重复 Dig provider、错误到退出码的映射，以及所有外部资源的关闭顺序。源码入口为 [`command_interface.go`](../../command_interface.go)、[`context_impl.go`](../../context_impl.go) 和 [`commandstarter`](../../commandstarter/)。
