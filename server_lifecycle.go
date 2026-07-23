package fiberhouse

import (
	"fmt"
	"os"
	"runtime/debug"
)

func coordinateServerRun(
	appStarter ApplicationStarter,
	runManagers []IProviderManager,
	shutdownManagers []IProviderManager,
	stopCh <-chan os.Signal,
) (runErr error, shutdownErr error, shutdownRequested bool) {
	runResult := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("application core panic: %v\n%s", recovered, debug.Stack())
			}
			runResult <- err
		}()
		err = appStarter.AppCoreRun(runManagers...)
	}()

	select {
	case runErr = <-runResult:
		return runErr, nil, false
	case <-stopCh:
		shutdownErr = appStarter.Shutdown(shutdownManagers...)
		return nil, shutdownErr, true
	}
}
