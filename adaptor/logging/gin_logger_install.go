// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package logging

import (
	"errors"
	"io"
	"sync"

	"github.com/gin-gonic/gin"
)

var ErrGinLoggerAlreadyInstalled = errors.New(
	"gin framework logger is already installed",
)

var ginLoggerInstallation struct {
	sync.Mutex
	active *GinLoggerLease
}

// GinLoggerLease owns the process-level Gin logging hooks until Release.
type GinLoggerLease struct {
	once sync.Once

	previousDebugPrint      func(string, ...any)
	previousDebugPrintRoute func(string, string, string, int)
	previousWriter          io.Writer
	previousErrorWriter     io.Writer
}

// InstallGinLogger routes Gin's process-level logging hooks through adapter.
func InstallGinLogger(adapter *GinLoggerAdapter) (*GinLoggerLease, error) {
	if adapter == nil {
		return nil, errors.New("gin logger adapter is nil")
	}
	if adapter.logger == nil {
		return nil, errors.New("gin logger adapter has no framework logger")
	}

	ginLoggerInstallation.Lock()
	defer ginLoggerInstallation.Unlock()

	if ginLoggerInstallation.active != nil {
		return nil, ErrGinLoggerAlreadyInstalled
	}

	lease := &GinLoggerLease{
		previousDebugPrint:      gin.DebugPrintFunc,
		previousDebugPrintRoute: gin.DebugPrintRouteFunc,
		previousWriter:          gin.DefaultWriter,
		previousErrorWriter:     gin.DefaultErrorWriter,
	}

	gin.DebugPrintFunc = adapter.DebugPrint
	gin.DebugPrintRouteFunc = adapter.DebugPrintRoute
	gin.DefaultWriter = adapter.InfoWriter()
	gin.DefaultErrorWriter = adapter.ErrorWriter()
	ginLoggerInstallation.active = lease

	return lease, nil
}

// Release restores the Gin logging hooks captured during installation.
func (l *GinLoggerLease) Release() {
	if l == nil {
		return
	}

	l.once.Do(func() {
		ginLoggerInstallation.Lock()
		defer ginLoggerInstallation.Unlock()

		if ginLoggerInstallation.active != l {
			return
		}

		gin.DebugPrintFunc = l.previousDebugPrint
		gin.DebugPrintRouteFunc = l.previousDebugPrintRoute
		gin.DefaultWriter = l.previousWriter
		gin.DefaultErrorWriter = l.previousErrorWriter
		ginLoggerInstallation.active = nil
	})
}
