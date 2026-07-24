// Package commands hosts CLI command definitions for the example
// application. Each command wires its own service/repository/model stack
// (see NewExampleCommand) and translates CLI flags into service calls,
// writing results as JSON to the command's writer.
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

// exampleCommand implements fiberhouse.CommandGetter for the "example" CLI
// command group, delegating all business logic to service.ExampleUseCase.
type exampleCommand struct {
	useCase service.ExampleUseCase
}

// NewExampleCommand wires the full command-module stack (model, repository,
// service) from ctx and returns the resulting "example" CLI command.
func NewExampleCommand(ctx fiberhouse.ICommandContext) fiberhouse.CommandGetter {
	mysqlModel := model.NewExampleMysqlModel(ctx)
	repo := repository.NewExampleRepository(mysqlModel)
	useCase := service.NewExampleMysqlService(repo)
	return newExampleCommand(useCase)
}

// newExampleCommand builds an exampleCommand around an already-constructed
// use case, letting tests inject a fake/mock without touching ctx wiring.
func newExampleCommand(useCase service.ExampleUseCase) fiberhouse.CommandGetter {
	return &exampleCommand{useCase: useCase}
}

// GetCommand returns the urfave/cli command tree for the "example" CLI
// command group (migrate/create/get/list/update/delete subcommands).
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

// positiveID reads the required --id flag and rejects the zero value.
func positiveID(cliCtx *cli.Context) (uint64, error) {
	id := cliCtx.Uint64("id")
	if id == 0 {
		return 0, fmt.Errorf("id must be greater than zero")
	}
	return id, nil
}

// validCLIStatus reports whether status is one of the two allowed values,
// mirroring service.validStatus for CLI-side pre-validation.
func validCLIStatus(status string) bool {
	return status == "active" || status == "archived"
}

// writeJSON encodes value as JSON to writer without HTML-escaping, used for
// all command output.
func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
