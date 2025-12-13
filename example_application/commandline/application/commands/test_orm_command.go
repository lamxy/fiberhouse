package commands

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/model"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/service"
	"github.com/urfave/cli/v2"
)

type TestOrmCMD struct {
	Ctx fiberhouse.ICommandContext
}

func NewTestOrmCMD(ctx fiberhouse.ICommandContext) fiberhouse.CommandGetter {
	return &TestOrmCMD{
		Ctx: ctx,
	}
}

// GetCommand 获取命令行命令对象，实现 fiberhouse.CommandGetter 接口
func (m *TestOrmCMD) GetCommand() interface{} {
	return &cli.Command{
		Name:    "test-orm",
		Aliases: []string{"orm"},
		Usage:   "测试go-orm库CURD操作",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "method",
				Aliases:  []string{"m"},
				Usage:    "测试类型(ok/orm)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "operation",
				Aliases:  []string{"o"},
				Usage:    "CURD(c创建|u更新|r读取|d删除)",
				Required: false,
			},
			&cli.UintFlag{
				Name:     "id",
				Aliases:  []string{"i"},
				Usage:    "主键ID",
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			var (
				ems  *service.ExampleMysqlService
				warp = component.NewWrap[*service.ExampleMysqlService]()
			)

			// 使用dig依赖注入组件
			dc := m.Ctx.GetDigContainer().
				Provide(func() fiberhouse.ICommandContext { return m.Ctx }).
				Provide(model.NewExampleMysqlModel).
				Provide(service.NewExampleMysqlService)

			if dc.GetErrorCount() > 0 {
				return fmt.Errorf("dig container init error: %v", dc.GetProvideErrs())
			}

			/*err := dc.Invoke(func(ems *service.ExampleMysqlService) error {
				err := ems.AutoMigrate()
				if err != nil {
					return err
				}
				// 其他操作...
				return nil
			})*/

			err := component.Invoke[*service.ExampleMysqlService](warp)
			if err != nil {
				return err
			}

			ems = warp.Get()

			// 自动创建一次数据表
			err = ems.AutoMigrate()
			if err != nil {
				return err
			}

			// 获取命令行参数
			method := cCtx.String("method")

			// 执行测试
			if method == "ok" {
				testOk := ems.TestOk()

				fmt.Println("result: ", testOk, "--from:", method)
			} else if method == "orm" {
				// 获取更多命令行参数
				op := cCtx.String("operation")
				id := cCtx.Uint("id")

				// 执行测试orm
				err := ems.TestOrm(m.Ctx, op, id)
				if err != nil {
					return err
				}

				fmt.Println("result: testOrm OK", "--from:", method)
			} else {
				return fmt.Errorf("unknown method: %s", method)
			}

			return nil
		},
	}
}
