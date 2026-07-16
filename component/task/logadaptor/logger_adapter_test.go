package logadaptor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func newTaskLogTestAdapter() (*TaskLoggerAdapter, *bytes.Buffer) {
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.DebugLevel)
	ctx := fiberhouse.NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
	return NewTaskLoggerAdapter(ctx), &output
}

func TestTaskLoggerAdapter_EmitsAllSupportedLevels(t *testing.T) {
	adapter, output := newTaskLogTestAdapter()
	originalFatalExit := zerolog.FatalExitFunc
	zerolog.FatalExitFunc = func() {}
	t.Cleanup(func() { zerolog.FatalExitFunc = originalFatalExit })

	adapter.Debug("debug message")
	adapter.Info("info message")
	adapter.Warn("warn message")
	adapter.Error("error message")
	adapter.Fatal("fatal message")

	logOutput := output.String()
	for _, want := range []string{
		`"level":"debug"`, `"message":"debug message"`,
		`"level":"info"`, `"message":"info message"`,
		`"level":"warn"`, `"message":"warn message"`,
		`"level":"error"`, `"message":"error message"`,
		`"level":"fatal"`, `"message":"fatal message"`,
		`"Component":"Asynq"`,
	} {
		assert.Contains(t, logOutput, want)
	}
}

func TestTaskLoggerAdapter_NonStringAndMultipleArgsAreSafe(t *testing.T) {
	adapter, output := newTaskLogTestAdapter()
	adapter.Info(42)
	adapter.Warn("first", "second", 3)
	adapter.Error()

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	assert.Len(t, lines, 3)
	assert.NotContains(t, lines[0], `"message"`)
	assert.Contains(t, lines[1], `"message":"first"`)
	assert.NotContains(t, lines[1], "second")
	assert.NotContains(t, lines[2], `"message"`)
}
