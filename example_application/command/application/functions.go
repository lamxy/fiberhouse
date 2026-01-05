package application

import (
	"github.com/urfave/cli/v2"
)

var (
	p1 string
)

// FlagsGlobalOption 全局选项
func FlagsGlobalOption() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "flag",
			Value:       "flag-value",
			Usage:       "set flag value",
			Destination: &p1,
			Required:    false,
			Action: func(context *cli.Context, s string) error {
				return nil
			},
		},
	}
}
