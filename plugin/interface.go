package plugin

import "github.com/lamxy/fiberhouse/provider"

// TODO

type Plugin interface {
	provider.IProvider
	Start() error
	Stop() error
	Restart() error
}
