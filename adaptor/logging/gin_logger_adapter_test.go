package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func newTestAdapter(t *testing.T) (*GinLoggerAdapter, *bytes.Buffer) {
	t.Helper()

	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.DebugLevel)

	return NewGinLoggerAdapter(
		bootstrap.NewLoggerWrap(&logger),
		appconfig.LogOrigin("Frame"),
	), &output
}

func decodeRecords(t *testing.T, output *bytes.Buffer) []map[string]any {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		records = append(records, record)
	}

	return records
}

func TestGinLoggerAdapter_DebugPrintEmitsStructuredDebugRecord(t *testing.T) {
	adapter, output := newTestAdapter(t)

	adapter.DebugPrint("registered %s\n", "route")

	records := decodeRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "debug", records[0]["level"])
	require.Equal(t, "Frame", records[0]["Origin"])
	require.Equal(t, "Gin", records[0]["Component"])
	require.Equal(t, "debug", records[0]["Channel"])
	require.Equal(t, "registered route", records[0]["message"])
}

func TestGinLoggerAdapter_DebugPrintRouteEmitsStructuredRouteFields(t *testing.T) {
	adapter, output := newTestAdapter(t)

	adapter.DebugPrintRoute("GET", "/users/:id", "handler", 6)

	records := decodeRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "debug", records[0]["level"])
	require.Equal(t, "Frame", records[0]["Origin"])
	require.Equal(t, "Gin", records[0]["Component"])
	require.Equal(t, "debug", records[0]["Channel"])
	require.Equal(t, "GET", records[0]["method"])
	require.Equal(t, "/users/:id", records[0]["path"])
	require.Equal(t, "handler", records[0]["handler"])
	require.Equal(t, float64(6), records[0]["handlerCount"])
	require.Equal(t, "Gin route registered", records[0]["message"])
}

func TestGinLoggerAdapter_WritersEmitAtMappedLevels(t *testing.T) {
	tests := []struct {
		name            string
		writer          func(*GinLoggerAdapter) io.Writer
		message         string
		expectedMessage string
		expectedLevel   string
		expectedChannel string
	}{
		{
			name: "info",
			writer: func(adapter *GinLoggerAdapter) io.Writer {
				return adapter.InfoWriter()
			},
			message:         "native access\n",
			expectedMessage: "native access",
			expectedLevel:   "info",
			expectedChannel: "writer",
		},
		{
			name: "error",
			writer: func(adapter *GinLoggerAdapter) io.Writer {
				return adapter.ErrorWriter()
			},
			message:         "native error\r\n",
			expectedMessage: "native error",
			expectedLevel:   "error",
			expectedChannel: "error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, output := newTestAdapter(t)

			n, err := test.writer(adapter).Write([]byte(test.message))

			require.Equal(t, len(test.message), n)
			require.NoError(t, err)
			records := decodeRecords(t, output)
			require.Len(t, records, 1)
			require.Equal(t, test.expectedLevel, records[0]["level"])
			require.Equal(t, "Frame", records[0]["Origin"])
			require.Equal(t, "Gin", records[0]["Component"])
			require.Equal(t, test.expectedChannel, records[0]["Channel"])
			require.Equal(t, test.expectedMessage, records[0]["message"])
		})
	}
}

func TestGinLoggerAdapter_WriterIgnoresEmptyLineEndings(t *testing.T) {
	adapter, output := newTestAdapter(t)
	message := "\r\n\r\n"

	n, err := adapter.InfoWriter().Write([]byte(message))

	require.Equal(t, len(message), n)
	require.NoError(t, err)
	require.Empty(t, decodeRecords(t, output))
}

func TestGinLoggerAdapter_WriterPreservesMultiLineMessageAsOneRecord(t *testing.T) {
	adapter, output := newTestAdapter(t)
	message := "first line\nsecond line\r\n"

	n, err := adapter.InfoWriter().Write([]byte(message))

	require.Equal(t, len(message), n)
	require.NoError(t, err)
	records := decodeRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "first line\nsecond line", records[0]["message"])
}

func TestGinLoggerAdapter_HTTPServerErrorLoggerEmitsUnprefixedError(t *testing.T) {
	adapter, output := newTestAdapter(t)

	adapter.HTTPServerErrorLogger().Print("accept failed")

	records := decodeRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "error", records[0]["level"])
	require.Equal(t, "Frame", records[0]["Origin"])
	require.Equal(t, "Gin", records[0]["Component"])
	require.Equal(t, "server", records[0]["Channel"])
	require.Equal(t, "accept failed", records[0]["message"])
}
