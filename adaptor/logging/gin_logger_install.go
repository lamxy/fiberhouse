// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package logging

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

var ErrGinLoggerAlreadyInstalled = errors.New(
	"gin framework logger is already installed",
)

type ginLoggerOutputs struct {
	debugPrint      func(string, ...any)
	debugPrintRoute func(string, string, string, int)
	writer          io.Writer
	errorWriter     io.Writer
}

var ginLoggerInstallation struct {
	once     sync.Once
	active   atomic.Pointer[GinLoggerLease]
	fallback ginLoggerOutputs
}

// GinLoggerLease owns the active forwarding target until Release.
type GinLoggerLease struct {
	adapter     *GinLoggerAdapter
	writer      io.Writer
	errorWriter io.Writer
}

var (
	ginInfoForwardWriter  = &ginLoggerForwardWriter{}
	ginErrorForwardWriter = &ginLoggerForwardWriter{errorChannel: true}
)

// InstallGinLogger routes Gin's process-level logging hooks through adapter.
func InstallGinLogger(adapter *GinLoggerAdapter) (*GinLoggerLease, error) {
	if adapter == nil {
		return nil, errors.New("gin logger adapter is nil")
	}
	if isNilGinFrameworkLogger(adapter.logger) {
		return nil, errors.New("gin logger adapter has no framework logger")
	}

	lease := &GinLoggerLease{
		adapter:     adapter,
		writer:      adapter.InfoWriter(),
		errorWriter: adapter.ErrorWriter(),
	}

	// Installation happens before Gin activity. The package globals remain
	// stable afterward so concurrent Gin reads never race with owner changes.
	ginLoggerInstallation.once.Do(func() {
		ginLoggerInstallation.fallback = ginLoggerOutputs{
			debugPrint:      gin.DebugPrintFunc,
			debugPrintRoute: gin.DebugPrintRouteFunc,
			writer:          gin.DefaultWriter,
			errorWriter:     gin.DefaultErrorWriter,
		}
		gin.DebugPrintFunc = forwardGinDebugPrint
		gin.DebugPrintRouteFunc = forwardGinDebugPrintRoute
		gin.DefaultWriter = ginInfoForwardWriter
		gin.DefaultErrorWriter = ginErrorForwardWriter
	})

	if !ginLoggerInstallation.active.CompareAndSwap(nil, lease) {
		return nil, ErrGinLoggerAlreadyInstalled
	}

	return lease, nil
}

// Release deactivates this owner without mutating Gin's package globals.
func (l *GinLoggerLease) Release() {
	if l == nil {
		return
	}

	ginLoggerInstallation.active.CompareAndSwap(l, nil)
}

func isNilGinFrameworkLogger(logger any) bool {
	if logger == nil {
		return true
	}

	value := reflect.ValueOf(logger)
	switch value.Kind() {
	case reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Ptr,
		reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func forwardGinDebugPrint(format string, values ...any) {
	if lease := ginLoggerInstallation.active.Load(); lease != nil {
		lease.adapter.DebugPrint(format, values...)
		return
	}

	forwardCapturedGinDebugPrint(format, values...)
}

func forwardGinDebugPrintRoute(
	httpMethod string,
	absolutePath string,
	handlerName string,
	handlerCount int,
) {
	if lease := ginLoggerInstallation.active.Load(); lease != nil {
		lease.adapter.DebugPrintRoute(
			httpMethod,
			absolutePath,
			handlerName,
			handlerCount,
		)
		return
	}

	if ginLoggerInstallation.fallback.debugPrintRoute != nil {
		ginLoggerInstallation.fallback.debugPrintRoute(
			httpMethod,
			absolutePath,
			handlerName,
			handlerCount,
		)
		return
	}

	forwardCapturedGinDebugPrint(
		"%-6s %-25s --> %s (%d handlers)\n",
		httpMethod,
		absolutePath,
		handlerName,
		handlerCount,
	)
}

func forwardCapturedGinDebugPrint(format string, values ...any) {
	if ginLoggerInstallation.fallback.debugPrint != nil {
		ginLoggerInstallation.fallback.debugPrint(format, values...)
		return
	}
	if !gin.IsDebugging() {
		return
	}

	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	_, _ = fmt.Fprintf(
		ginLoggerInstallation.fallback.writer,
		"[GIN-debug] "+format,
		values...,
	)
}

type ginLoggerForwardWriter struct {
	errorChannel bool
}

func (w *ginLoggerForwardWriter) Write(p []byte) (int, error) {
	if lease := ginLoggerInstallation.active.Load(); lease != nil {
		if w.errorChannel {
			return lease.errorWriter.Write(p)
		}
		return lease.writer.Write(p)
	}

	if w.errorChannel {
		return ginLoggerInstallation.fallback.errorWriter.Write(p)
	}
	return ginLoggerInstallation.fallback.writer.Write(p)
}
