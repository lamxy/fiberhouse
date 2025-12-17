package plugin

import (
	"github.com/lamxy/fiberhouse"
)

// TODO

type Plugin interface {
	fiberhouse.IProvider
	Start() error
	Stop() error
	Restart() error
}
