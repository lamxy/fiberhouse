package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/rs/zerolog"
)

// TestLoggerWrapCallerPointsToCallSite 校验 caller 字段指向调用 LoggerWrap 方法的代码行，
// 而不是 LoggerWrap 方法本身所在的 bootstrap.go。
func TestLoggerWrapCallerPointsToCallSite(t *testing.T) {
	var buf bytes.Buffer
	zl := zerolog.New(&buf).Level(zerolog.TraceLevel)
	lw := NewLoggerWrap(&zl)
	origin := appconfig.LogOrigin("test")

	check := func(name string, line int) {
		t.Helper()
		var entry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("%s: decode log entry %q: %v", name, buf.String(), err)
		}
		buf.Reset()
		_, file, _, _ := runtime.Caller(0)
		want := fmt.Sprintf("%s:%d", file, line)
		if entry["caller"] != want {
			t.Errorf("%s: caller = %v, want %s", name, entry["caller"], want)
		}
	}

	// 带 Caller 的级别：Debug、Error、Err（Fatal、Panic 会退出或 panic，不在此调用）。
	var l int
	_, _, l, _ = runtime.Caller(0)
	lw.Debug().Msg("x")
	check("Debug", l+1)
	_, _, l, _ = runtime.Caller(0)
	lw.Debug(origin).Msg("x")
	check("Debug(origin)", l+1)
	_, _, l, _ = runtime.Caller(0)
	lw.DebugWith(origin).Msg("x")
	check("DebugWith", l+1)
	_, _, l, _ = runtime.Caller(0)
	lw.Error().Msg("x")
	check("Error", l+1)
	_, _, l, _ = runtime.Caller(0)
	lw.Error(origin).Msg("x")
	check("Error(origin)", l+1)
	_, _, l, _ = runtime.Caller(0)
	lw.ErrorWith(origin).Msg("x")
	check("ErrorWith", l+1)
	_, _, l, _ = runtime.Caller(0)
	lw.Err(fmt.Errorf("boom")).Msg("x")
	check("Err", l+1)
}
