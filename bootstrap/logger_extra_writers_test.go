package bootstrap

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// recordingWriter 记录写入内容与 Close 次数；err 非空时 Write 返回该错误。
type recordingWriter struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	err      error
	closeErr error
	closed   int
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	return w.buf.Write(p)
}

func (w *recordingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed++
	return w.closeErr
}

func (w *recordingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// plainWriter 只实现 io.Writer，用于确认未实现 io.Closer 的附加 writer 不会被关闭。
type plainWriter struct{ bytes.Buffer }

func TestLoggerExtraWriters_ReceiveSameLinesAndHonorLevel(t *testing.T) {
	state := isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_CONF_application_appLog_level", "info")

	extra := &recordingWriter{}
	plain := &plainWriter{}
	logger := NewLoggerOnceWithWriters(NewConfigOnce(dir), []io.Writer{nil, extra, plain}, dir)
	logger.Debug().Msg("debug suppressed")
	logger.Error().Str("traceId", "t-1").Msg("error emitted")
	state.closeLogger(t)

	file := readFile(t, filepath.Join(dir, "test.log"))
	if got := extra.String(); got != file {
		t.Fatalf("extra writer content differs from file:\nextra: %q\nfile:  %q", got, file)
	}
	if plain.String() != file {
		t.Fatalf("plain writer content differs from file: %q", plain.String())
	}
	if strings.Contains(file, "debug suppressed") || !strings.Contains(file, `"traceId":"t-1"`) {
		t.Fatalf("unexpected log content: %q", file)
	}
	if extra.closed != 1 {
		t.Fatalf("extra writer closed %d times, want 1", extra.closed)
	}
}

func TestLoggerExtraWriters_CloseOnceAndJoinErrors(t *testing.T) {
	state := isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")

	errA, errB := errors.New("close a"), errors.New("close b")
	a := &recordingWriter{closeErr: errA}
	b := &recordingWriter{closeErr: errB}
	logger := NewLoggerOnceWithWriters(NewConfigOnce(dir), []io.Writer{a, b}, dir)

	err := logger.Close()
	state.loggerClosed = true
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("Close error = %v, want both close errors joined", err)
	}
	if again := logger.Close(); again == nil || again.Error() != err.Error() {
		t.Fatalf("second Close = %v, want replay of %v", again, err)
	}
	if a.closed != 1 || b.closed != 1 {
		t.Fatalf("close counts a=%d b=%d, want 1 each", a.closed, b.closed)
	}
}

func TestLoggerExtraWriters_FailureDoesNotBlockBuiltInOutputs(t *testing.T) {
	state := isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")

	broken := &recordingWriter{err: errors.New("sink down")}
	logger := NewLoggerOnceWithWriters(NewConfigOnce(dir), []io.Writer{broken}, dir)
	logger.Error().Msg("still written to file")
	state.closeLogger(t)

	if file := readFile(t, filepath.Join(dir, "test.log")); !strings.Contains(file, "still written to file") {
		t.Fatalf("file output missing after extra writer failure: %q", file)
	}
}

func TestLoggerExtraWriters_IgnoredAfterNewLoggerOnce(t *testing.T) {
	state := isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")

	cfg := NewConfigOnce(dir)
	first := NewLoggerOnce(cfg, dir)
	late := &recordingWriter{}
	second := NewLoggerOnceWithWriters(cfg, []io.Writer{late}, dir)
	second.Error().Msg("after first init")
	state.closeLogger(t)

	if first != second {
		t.Fatal("NewLoggerOnceWithWriters did not return the existing singleton")
	}
	if late.String() != "" || late.closed != 0 {
		t.Fatalf("late extra writer was used: content=%q closed=%d", late.String(), late.closed)
	}
}
