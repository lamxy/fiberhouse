package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type ginGlobalSentinels struct {
	info synchronizedBuffer
	err  synchronizedBuffer
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *synchronizedBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer.Reset()
}

var (
	ginGlobalSentinelsOnce sync.Once
	sharedGinSentinels     = &ginGlobalSentinels{}
)

func installGinGlobalSentinels(t *testing.T) *ginGlobalSentinels {
	t.Helper()

	ginGlobalSentinelsOnce.Do(func() {
		gin.DebugPrintFunc = nil
		gin.DebugPrintRouteFunc = nil
		gin.DefaultWriter = &sharedGinSentinels.info
		gin.DefaultErrorWriter = &sharedGinSentinels.err
	})

	sharedGinSentinels.info.Reset()
	sharedGinSentinels.err.Reset()

	return sharedGinSentinels
}

type closeTrackingWriter struct {
	bytes.Buffer
	closeCalls int32
}

func (w *closeTrackingWriter) Close() error {
	atomic.AddInt32(&w.closeCalls, 1)
	return nil
}

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

func TestInstallGinLogger_NilGinDebugCallbacksPreserveFallback(t *testing.T) {
	sentinels := installGinGlobalSentinels(t)
	adapter, _ := newTestAdapter(t)
	previousMode := gin.Mode()
	gin.SetMode(gin.DebugMode)
	t.Cleanup(func() {
		gin.SetMode(previousMode)
	})

	lease, err := InstallGinLogger(adapter)
	require.NoError(t, err)
	require.NotNil(t, lease)
	lease.Release()

	require.NotPanics(t, func() {
		gin.New()
	})
	require.Contains(t, sentinels.info.String(), "[GIN-debug]")
}

func TestInstallGinLogger_RoutesAllGlobalsAndReleaseFallsBack(t *testing.T) {
	sentinels := installGinGlobalSentinels(t)
	adapter, output := newTestAdapter(t)

	lease, err := InstallGinLogger(adapter)
	require.NoError(t, err)
	require.NotNil(t, lease)
	t.Cleanup(lease.Release)
	installedDebugPrint := reflect.ValueOf(gin.DebugPrintFunc).Pointer()
	installedDebugPrintRoute := reflect.ValueOf(
		gin.DebugPrintRouteFunc,
	).Pointer()
	installedWriter := gin.DefaultWriter
	installedErrorWriter := gin.DefaultErrorWriter

	gin.DebugPrintFunc("message %d", 1)
	gin.DebugPrintRouteFunc("GET", "/route", "handler", 2)
	_, err = gin.DefaultWriter.Write([]byte("info\n"))
	require.NoError(t, err)
	_, err = gin.DefaultErrorWriter.Write([]byte("error\n"))
	require.NoError(t, err)

	records := decodeRecords(t, output)
	require.Len(t, records, 4)
	require.Equal(t, "message 1", records[0]["message"])
	require.Equal(t, "debug", records[0]["Channel"])
	require.Equal(t, "Gin route registered", records[1]["message"])
	require.Equal(t, "GET", records[1]["method"])
	require.Equal(t, "/route", records[1]["path"])
	require.Equal(t, "handler", records[1]["handler"])
	require.Equal(t, float64(2), records[1]["handlerCount"])
	require.Equal(t, "info", records[2]["message"])
	require.Equal(t, "writer", records[2]["Channel"])
	require.Equal(t, "error", records[3]["message"])
	require.Equal(t, "error", records[3]["Channel"])
	require.Empty(t, sentinels.info.String())
	require.Empty(t, sentinels.err.String())

	lease.Release()
	gin.DebugPrintFunc("sentinel")
	gin.DebugPrintRouteFunc("GET", "/sentinel", "sentinel", 1)
	_, err = gin.DefaultWriter.Write([]byte("sentinel info"))
	require.NoError(t, err)
	_, err = gin.DefaultErrorWriter.Write([]byte("sentinel error"))
	require.NoError(t, err)

	require.Contains(t, sentinels.info.String(), "[GIN-debug] sentinel\n")
	require.Contains(
		t,
		sentinels.info.String(),
		"[GIN-debug] GET    /sentinel",
	)
	require.Contains(t, sentinels.info.String(), "sentinel info")
	require.Equal(t, "sentinel error", sentinels.err.String())
	require.Equal(
		t,
		installedDebugPrint,
		reflect.ValueOf(gin.DebugPrintFunc).Pointer(),
	)
	require.Equal(
		t,
		installedDebugPrintRoute,
		reflect.ValueOf(gin.DebugPrintRouteFunc).Pointer(),
	)
	require.Same(t, installedWriter, gin.DefaultWriter)
	require.Same(t, installedErrorWriter, gin.DefaultErrorWriter)
}

func TestInstallGinLogger_RejectsSecondActiveLease(t *testing.T) {
	installGinGlobalSentinels(t)
	firstAdapter, firstOutput := newTestAdapter(t)
	secondAdapter, secondOutput := newTestAdapter(t)

	firstLease, err := InstallGinLogger(firstAdapter)
	require.NoError(t, err)
	require.NotNil(t, firstLease)
	t.Cleanup(firstLease.Release)

	secondLease, err := InstallGinLogger(secondAdapter)
	require.ErrorIs(t, err, ErrGinLoggerAlreadyInstalled)
	require.Nil(t, secondLease)

	gin.DebugPrintFunc("first owner")
	gin.DebugPrintRouteFunc("POST", "/owner", "owner", 3)
	_, err = gin.DefaultWriter.Write([]byte("first info"))
	require.NoError(t, err)
	_, err = gin.DefaultErrorWriter.Write([]byte("first error"))
	require.NoError(t, err)

	require.Len(t, decodeRecords(t, firstOutput), 4)
	require.Empty(t, decodeRecords(t, secondOutput))
}

func TestGinLoggerLease_ConcurrentReleaseIsIdempotentAndAllowsReinstall(t *testing.T) {
	sentinels := installGinGlobalSentinels(t)
	output := &closeTrackingWriter{}
	logger := zerolog.New(output).Level(zerolog.DebugLevel)
	adapter := NewGinLoggerAdapter(
		bootstrap.NewLoggerWrap(&logger, output),
		appconfig.LogOrigin("Frame"),
	)

	lease, err := InstallGinLogger(adapter)
	require.NoError(t, err)
	require.NotNil(t, lease)
	t.Cleanup(lease.Release)

	const releaseCount = 64
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(releaseCount)
	for range releaseCount {
		go func() {
			defer waitGroup.Done()
			<-start
			lease.Release()
		}()
	}

	close(start)
	waitGroup.Wait()

	gin.DebugPrintFunc("sentinel")
	gin.DebugPrintRouteFunc("GET", "/sentinel", "sentinel", 1)
	_, err = gin.DefaultWriter.Write([]byte("sentinel info"))
	require.NoError(t, err)
	_, err = gin.DefaultErrorWriter.Write([]byte("sentinel error"))
	require.NoError(t, err)
	require.Contains(t, sentinels.info.String(), "[GIN-debug] sentinel\n")
	require.Contains(
		t,
		sentinels.info.String(),
		"[GIN-debug] GET    /sentinel",
	)
	require.Contains(t, sentinels.info.String(), "sentinel info")
	require.Equal(t, "sentinel error", sentinels.err.String())
	require.Zero(t, atomic.LoadInt32(&output.closeCalls))

	nextLease, err := InstallGinLogger(adapter)
	require.NoError(t, err)
	require.NotNil(t, nextLease)
	t.Cleanup(nextLease.Release)
	nextLease.Release()
	require.Zero(t, atomic.LoadInt32(&output.closeCalls))
}

func TestGinLoggerLease_ReleaseConcurrentWithGinOutputsIsRaceFree(t *testing.T) {
	installGinGlobalSentinels(t)
	logger := zerolog.New(io.Discard).Level(zerolog.DebugLevel)
	adapter := NewGinLoggerAdapter(
		bootstrap.NewLoggerWrap(&logger),
		appconfig.LogOrigin("Frame"),
	)

	lease, err := InstallGinLogger(adapter)
	require.NoError(t, err)
	require.NotNil(t, lease)
	t.Cleanup(lease.Release)

	const emitCount = 1_000
	start := make(chan struct{})
	writeErrors := make(chan error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(5)

	go func() {
		defer waitGroup.Done()
		<-start
		for range emitCount {
			gin.DebugPrintFunc("debug")
		}
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		for range emitCount {
			gin.DebugPrintRouteFunc("GET", "/race", "handler", 1)
		}
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		for range emitCount {
			if _, writeErr := gin.DefaultWriter.Write([]byte("info")); writeErr != nil {
				writeErrors <- writeErr
				return
			}
		}
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		for range emitCount {
			if _, writeErr := gin.DefaultErrorWriter.Write(
				[]byte("error"),
			); writeErr != nil {
				writeErrors <- writeErr
				return
			}
		}
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		lease.Release()
	}()

	close(start)
	waitGroup.Wait()
	close(writeErrors)
	for writeErr := range writeErrors {
		require.NoError(t, writeErr)
	}
}

func TestInstallGinLogger_RejectsNilDependencies(t *testing.T) {
	lease, err := InstallGinLogger(nil)
	require.Error(t, err)
	require.Nil(t, lease)

	lease, err = InstallGinLogger(&GinLoggerAdapter{})
	require.Error(t, err)
	require.Nil(t, lease)

	debugPrint := reflect.ValueOf(gin.DebugPrintFunc).Pointer()
	debugPrintRoute := reflect.ValueOf(gin.DebugPrintRouteFunc).Pointer()
	writer := gin.DefaultWriter
	errorWriter := gin.DefaultErrorWriter
	var typedNilLogger *bootstrap.LoggerWrap
	lease, err = InstallGinLogger(NewGinLoggerAdapter(
		typedNilLogger,
		appconfig.LogOrigin("Frame"),
	))
	if lease != nil {
		t.Cleanup(lease.Release)
	}
	require.Error(t, err)
	require.Nil(t, lease)
	require.Equal(
		t,
		debugPrint,
		reflect.ValueOf(gin.DebugPrintFunc).Pointer(),
	)
	require.Equal(
		t,
		debugPrintRoute,
		reflect.ValueOf(gin.DebugPrintRouteFunc).Pointer(),
	)
	require.Same(t, writer, gin.DefaultWriter)
	require.Same(t, errorWriter, gin.DefaultErrorWriter)
}
