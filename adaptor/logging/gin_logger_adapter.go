package logging

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
)

const ginLogComponent = "Gin"

// GinLoggerAdapter maps Gin's diagnostic output to the framework logger.
type GinLoggerAdapter struct {
	logger bootstrap.LoggerWrapper
	origin appconfig.LogOrigin
}

// NewGinLoggerAdapter creates a structured logging adapter for Gin.
func NewGinLoggerAdapter(
	logger bootstrap.LoggerWrapper,
	origin appconfig.LogOrigin,
) *GinLoggerAdapter {
	return &GinLoggerAdapter{
		logger: logger,
		origin: origin,
	}
}

// DebugPrint records a formatted Gin diagnostic message at Debug level.
func (a *GinLoggerAdapter) DebugPrint(format string, values ...any) {
	message := strings.TrimRight(fmt.Sprintf(format, values...), "\r\n")
	if message == "" {
		return
	}

	a.event(zerolog.DebugLevel, "debug").Msg(message)
}

// DebugPrintRoute records a Gin route registration as structured fields.
func (a *GinLoggerAdapter) DebugPrintRoute(
	httpMethod string,
	absolutePath string,
	handlerName string,
	handlerCount int,
) {
	a.event(zerolog.DebugLevel, "debug").
		Str("method", httpMethod).
		Str("path", absolutePath).
		Str("handler", handlerName).
		Int("handlerCount", handlerCount).
		Msg("Gin route registered")
}

// InfoWriter returns a writer for Gin's informational output.
func (a *GinLoggerAdapter) InfoWriter() io.Writer {
	return a.writerFor(zerolog.InfoLevel, "writer")
}

// ErrorWriter returns a writer for Gin's error output.
func (a *GinLoggerAdapter) ErrorWriter() io.Writer {
	return a.writerFor(zerolog.ErrorLevel, "error")
}

// HTTPServerErrorLogger returns an unprefixed logger for net/http server errors.
func (a *GinLoggerAdapter) HTTPServerErrorLogger() *log.Logger {
	return log.New(a.writerFor(zerolog.ErrorLevel, "server"), "", 0)
}

func (a *GinLoggerAdapter) event(
	level zerolog.Level,
	channel string,
) *zerolog.Event {
	var event *zerolog.Event
	switch level {
	case zerolog.DebugLevel:
		event = a.logger.DebugWith(a.origin)
	case zerolog.ErrorLevel:
		event = a.logger.ErrorWith(a.origin)
	default:
		event = a.logger.InfoWith(a.origin)
	}

	return event.
		Str("Component", ginLogComponent).
		Str("Channel", channel)
}

func (a *GinLoggerAdapter) writerFor(
	level zerolog.Level,
	channel string,
) io.Writer {
	return &ginLogWriter{
		adapter: a,
		level:   level,
		channel: channel,
	}
}

type ginLogWriter struct {
	adapter *GinLoggerAdapter
	level   zerolog.Level
	channel string
}

func (w *ginLogWriter) Write(p []byte) (int, error) {
	length := len(p)
	message := strings.TrimRight(string(p), "\r\n")
	if message == "" {
		return length, nil
	}

	w.adapter.event(w.level, w.channel).Msg(message)
	return length, nil
}
