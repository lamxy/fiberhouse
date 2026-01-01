# Package commandstarter 提供基于 cli.v2 的命令行应用启动器实现，负责命令行应用的完整生命周期管理和启动流程编排。

该包是应用框架的命令行启动引擎，提供标准化的命令行应用启动流程和生命周期管理，包括：
- 命令行应用启动流程的标准化编排和执行
- 全局对象容器的初始化和管理
- 命令、选项、错误处理器的注册管理
- 应用启动时的全局对象健康检测
- 多来源子日志器的注册和管理
- 统一的错误处理机制

## 启动流程

命令行应用启动按以下顺序执行，确保依赖关系正确：
1. InitCoreApp：初始化 cli.App 核心应用
2. RegisterGlobalErrHandler：注册全局错误处理器
3. RegisterCommands：注册命令列表
4. RegisterCoreGlobalOptional：注册全局选项和动作
5. RegisterApplicationGlobals：注册全局对象初始化器
6. AppCoreRun：运行命令行应用

## 基本使用示例

	// 创建命令行应用上下文
	ctx := fiberhouse.NewCmdContextOnce(appConfig, logger)  // 依赖引导包的应用配置和日志器

	// 创建应用注册器实例
	appRegister := application.NewApplication(ctx)

	// 创建命令行应用启动器
	starter := commandstarter.NewCmdApplication(ctx, appRegister)

	// 执行命令行应用启动流程
	commandstarter.RunCommandStarter(starter)

## 应用注册器实现示例

	type Application struct {
		Ctx             fiberhouse.IApplicationContext
		name            string
		instanceFlagMap map[fiberhouse.InstanceKeyFlag]fiberhouse.InstanceKey  // 按需
	}

	func NewApplication(ctx fiberhouse.IApplicationContext) fiberhouse.ApplicationCmdRegister {
		return &Application{
			Ctx:             ctx,
			name:            "application",
			instanceFlagMap: make(map[fiberhouse.InstanceKeyFlag]fiberhouse.InstanceKey),
		}
	}

	// 注册全局对象初始化器
	func (app *Application) RegisterApplicationGlobals() {
		initializers := globalmanager.InitializerMap{
			// 数据库连接
			app.GetDBMysqlKey(): func() (interface{}, error) {
				// 创建并返回 MySQL 数据库连接实例
			},
			// JSON 编解码器
			app.GetFastTrafficCodecKey(): func() (interface{}, error) {
				// 创建并返回 JSON 编解码器实例
			},
		}
		app.GetContext().GetContainer().Registers(initializers)

		// 预先初始化必要的全局对象
		requiredKeys := []globalmanager.KeyName{
			app.GetDBMysqlKey(),
			app.GetFastTrafficCodecKey(),
		}
		for _, key := range requiredKeys {
			if _, err := app.GetContext().GetContainer().Get(key); err != nil {
				// 处理初始化错误
			}
		}
	}

	// 注册命令列表
	func (app *Application) RegisterCommands(core interface{}) {
		coreApp := core.(*cli.App)

		// 命令获取器列表
		cmdGetters := []fiberhouse.CommandGetter{
			// 按需添加实现了fiberhouse.CommandGetter的命令实例
		}

		// 通用命令
		commonCMDs := []*cli.Command{
			{
				Name:  "health",
				Usage: "Check application health",
				Action: func(c *cli.Context) error {
					// 健康检查逻辑
					return nil
				},
			},
		}

		// 合并命令列表
		cliCommands := make([]*cli.Command, 0, len(commonCMDs)+len(cmdGetters))
		cliCommands = append(cliCommands, commonCMDs...)

		for _, getter := range cmdGetters {
			cliCommands = append(cliCommands, getter.GetCommand())
		}

		coreApp.Commands = cliCommands
	}

	// 注册核心应用的全局逻辑，可选
	func (app *Application) RegisterCoreGlobalOptional(core interface{}) {
		coreApp := core.(*cli.App)
		// 底层核心应用的全局逻辑处理
	}

	// 注册全局错误处理器
	func (app *Application) RegisterGlobalErrHandler(core interface{}) {
		coreApp := core.(*cli.App)
		coreApp.ExitErrHandler = func(cCtx *cli.Context, err error) {
			logger := app.GetContext().GetLogger()
			logger.Error().Err(err).
				Str("command", cCtx.Command.Name).
				Strs("args", cCtx.Args().Slice()).
				Msg("Command execution failed")
			cli.HandleExitCoder(err)
		}
	}

## 命令实现参考示例

	     // test_orm_command.go

		 // TestOrmCommand 示例命令，测试 ORM 数据库操作，需要实现fiberhouse.CommandGetter接口的 GetCommand 方法
			type TestOrmCommand struct {
				Ctx fiberhouse.IApplicationContext
			}

			func NewTestOrmCommand(ctx fiberhouse.IApplicationContext) fiberhouse.CommandGetter {
				return &TestOrmCommand{Ctx: ctx}
			}

			func (t *TestOrmCommand) GetCommand() interface{} {
				return &cli.Command{
					Name:    "test-orm",
					Aliases: []string{"orm"},
					Usage:   "Test ORM database operations",
					Action:  t.execute,
				}
			}

## 命令行脚本结构参考示例

	command/
	├── main.go                        # 命令行入口
	├── readme_go_build.md             # 构建说明
	├── application/                   # 应用层
	│   ├── application.go             # 应用注册器实现
	│   ├── constants.go               # 常量定义
	│   ├── functions.go               # 公共函数
	│   └── commands/                  # 命令实现
	│       ├── test_orm_command.go    # ORM 测试命令
	│       └── test_other_command.go  # 其他测试命令
	└── component/                     # 组件层
	    ├── cron.go                    # 定时任务组件
	    └── readme.md                  # 组件说明

## 配置文件示例

	database:
	  mongodb:
	    applyURI: "mongodb://localhost:27017/test"
	  mysql:
	    dsn: "user:password@tcp(localhost:3306)/dbname"
	cache:
	  redis:
	    host: "localhost"
	    port: "6379"
	    password: ""
	    db: 0
	command:
	  name: "example-cli"
	  usage: "An example command line application"
	  version: "1.0.0"
	  sortFlagsByName: true
	  sortCommandsByName: true

## 健康检测机制

内置应用启动时的健康检测功能

## 多来源日志器支持

支持按不同来源注册子日志器：
- CMD 来源：命令行操作日志
- DB 来源：数据库操作日志
- CACHE 来源：缓存操作日志
- 自定义来源：业务自定义日志