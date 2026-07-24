// Package commands 承载 example 应用的 CLI 命令定义。每个命令自行接线其
// service/repository/model 栈（见 NewExampleCommand），把 CLI 标志转换为 service
// 调用，并将结果以 JSON 写入命令的 writer。
package commands

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/model"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/command-module/service"
	"github.com/urfave/cli/v2"
)

// exampleCommand 为「example」CLI 命令组实现 fiberhouse.CommandGetter，
// 将全部业务逻辑委托给 service.ExampleUseCase。
type exampleCommand struct {
	useCase service.ExampleUseCase
}

// NewExampleCommand 从 ctx 接线完整的 command-module 栈（model、repository、
// service），并返回构建出的「example」CLI 命令。
func NewExampleCommand(ctx fiberhouse.ICommandContext) fiberhouse.CommandGetter {
	mysqlModel := model.NewExampleMysqlModel(ctx)
	repo := repository.NewExampleRepository(mysqlModel)
	useCase := service.NewExampleMysqlService(repo)
	return newExampleCommand(useCase)
}

// newExampleCommand 围绕一个已构造好的 use case 构建 exampleCommand，
// 使测试可以注入 fake/mock 而无需触碰 ctx 接线。
func newExampleCommand(useCase service.ExampleUseCase) fiberhouse.CommandGetter {
	return &exampleCommand{useCase: useCase}
}

// GetCommand 返回「example」CLI 命令组的 urfave/cli 命令树
// （migrate/create/get/list/update/delete 子命令）。
func (c *exampleCommand) GetCommand() interface{} {
	return &cli.Command{
		Name:  "example",
		Usage: "manage example records in MySQL",
		Subcommands: []*cli.Command{
			c.migrateCommand(),
			c.createCommand(),
			c.getCommand(),
			c.listCommand(),
			c.updateCommand(),
			c.deleteCommand(),
		},
	}
}

func (c *exampleCommand) migrateCommand() *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "create or update the example_records table",
		Action: func(cliCtx *cli.Context) error {
			if err := c.useCase.Migrate(cliCtx.Context); err != nil {
				return err
			}
			return writeJSON(cliCtx.App.Writer, map[string]string{"status": "ok"})
		},
	}
}

func (c *exampleCommand) createCommand() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "create an example record",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Required: true},
			&cli.StringFlag{Name: "description"},
			&cli.StringFlag{Name: "status", Value: "active"},
		},
		Action: func(cliCtx *cli.Context) error {
			status := cliCtx.String("status")
			if !validCLIStatus(status) {
				return fmt.Errorf("status must be active or archived")
			}
			record, err := c.useCase.Create(cliCtx.Context, service.CreateInput{
				Name:        cliCtx.String("name"),
				Description: cliCtx.String("description"),
				Status:      status,
			})
			if err != nil {
				return err
			}
			return writeJSON(cliCtx.App.Writer, record)
		},
	}
}

func (c *exampleCommand) getCommand() *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "get an example record",
		Flags: []cli.Flag{&cli.Uint64Flag{Name: "id", Required: true}},
		Action: func(cliCtx *cli.Context) error {
			id, err := positiveID(cliCtx)
			if err != nil {
				return err
			}
			record, err := c.useCase.Get(cliCtx.Context, id)
			if err != nil {
				return err
			}
			return writeJSON(cliCtx.App.Writer, record)
		},
	}
}

func (c *exampleCommand) listCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "list example records",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "page", Value: 1},
			&cli.IntFlag{Name: "page-size", Value: 20},
			&cli.StringFlag{Name: "status"},
		},
		Action: func(cliCtx *cli.Context) error {
			page, pageSize, status := cliCtx.Int("page"), cliCtx.Int("page-size"), cliCtx.String("status")
			if page < 1 {
				return fmt.Errorf("page must be greater than zero")
			}
			if pageSize < 1 || pageSize > 100 {
				return fmt.Errorf("page-size must be between 1 and 100")
			}
			if status != "" && !validCLIStatus(status) {
				return fmt.Errorf("status must be active or archived")
			}
			result, err := c.useCase.List(cliCtx.Context, service.ListInput{
				Page: page, PageSize: pageSize, Status: status,
			})
			if err != nil {
				return err
			}
			return writeJSON(cliCtx.App.Writer, result)
		},
	}
}

func (c *exampleCommand) updateCommand() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "update an example record",
		Flags: []cli.Flag{
			&cli.Uint64Flag{Name: "id", Required: true},
			&cli.StringFlag{Name: "name"},
			&cli.StringFlag{Name: "description"},
			&cli.StringFlag{Name: "status"},
		},
		Action: func(cliCtx *cli.Context) error {
			id, err := positiveID(cliCtx)
			if err != nil {
				return err
			}
			var input service.UpdateInput
			if cliCtx.IsSet("name") {
				value := cliCtx.String("name")
				input.Name = &value
			}
			if cliCtx.IsSet("description") {
				value := cliCtx.String("description")
				input.Description = &value
			}
			if cliCtx.IsSet("status") {
				value := cliCtx.String("status")
				if !validCLIStatus(value) {
					return fmt.Errorf("status must be active or archived")
				}
				input.Status = &value
			}
			if input.Name == nil && input.Description == nil && input.Status == nil {
				return fmt.Errorf("at least one update flag is required")
			}
			record, err := c.useCase.Update(cliCtx.Context, id, input)
			if err != nil {
				return err
			}
			return writeJSON(cliCtx.App.Writer, record)
		},
	}
}

func (c *exampleCommand) deleteCommand() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "hard-delete an example record",
		Flags: []cli.Flag{&cli.Uint64Flag{Name: "id", Required: true}},
		Action: func(cliCtx *cli.Context) error {
			id, err := positiveID(cliCtx)
			if err != nil {
				return err
			}
			if err := c.useCase.Delete(cliCtx.Context, id); err != nil {
				return err
			}
			return writeJSON(cliCtx.App.Writer, map[string]string{"status": "ok"})
		},
	}
}

// positiveID 读取必填的 --id 标志并拒绝零值。
func positiveID(cliCtx *cli.Context) (uint64, error) {
	id := cliCtx.Uint64("id")
	if id == 0 {
		return 0, fmt.Errorf("id must be greater than zero")
	}
	return id, nil
}

// validCLIStatus 报告 status 是否为两个允许值之一，与 service.validStatus 保持
// 一致，用于 CLI 侧的预校验。
func validCLIStatus(status string) bool {
	return status == "active" || status == "archived"
}

// writeJSON 将 value 以 JSON 编码写入 writer（不做 HTML 转义），用于所有命令输出。
func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
