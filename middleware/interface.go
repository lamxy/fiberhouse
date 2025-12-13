package middleware

import (
	"github.com/lamxy/fiberhouse"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
)

type IRecover interface {
	DefaultStackTraceHandler(providerCtx.ContextProvider, interface{})
	ErrorHandler(providerCtx.ContextProvider, error) error
	GetContext() fiberhouse.IApplicationContext
}
