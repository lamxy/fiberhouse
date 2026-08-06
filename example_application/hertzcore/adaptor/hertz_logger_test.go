package adaptor

import (
	"bytes"
	ctxpkg "context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func newTestHertzLogAdapter(t *testing.T) (*HertzLoggerAdapter, *bytes.Buffer) {
	t.Helper()

	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.TraceLevel)

	return NewHertzLoggerAdapter(
		bootstrap.NewLoggerWrap(&logger),
		appconfig.LogOrigin("Frame"),
	), &output
}

func decodeHertzRecords(t *testing.T, output *bytes.Buffer) []map[string]any {
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

// TestHertzLoggerAdapter_LevelMapping 验证 hertz 七级日志到框架日志器的映射。
// hertz 有框架不具备的 Trace 与 Notice 两级，需分别降级到 Debug 与 Info。
func TestHertzLoggerAdapter_LevelMapping(t *testing.T) {
	tests := []struct {
		name          string
		emit          func(*HertzLoggerAdapter)
		expectedLevel string
	}{
		{"trace", func(a *HertzLoggerAdapter) { a.Trace("m") }, "debug"},
		{"debug", func(a *HertzLoggerAdapter) { a.Debug("m") }, "debug"},
		{"info", func(a *HertzLoggerAdapter) { a.Info("m") }, "info"},
		{"notice", func(a *HertzLoggerAdapter) { a.Notice("m") }, "info"},
		{"warn", func(a *HertzLoggerAdapter) { a.Warn("m") }, "warn"},
		{"error", func(a *HertzLoggerAdapter) { a.Error("m") }, "error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, output := newTestHertzLogAdapter(t)

			test.emit(adapter)

			records := decodeHertzRecords(t, output)
			require.Len(t, records, 1)
			require.Equal(t, test.expectedLevel, records[0]["level"])
			require.Equal(t, "Frame", records[0]["Origin"])
			require.Equal(t, "Hertz", records[0]["Component"])
			require.Equal(t, "m", records[0]["message"])
		})
	}
}

// TestHertzLoggerAdapter_FormatVariants 验证 *f 系列按格式化后的内容输出
func TestHertzLoggerAdapter_FormatVariants(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)

	adapter.Infof("listening on %s:%d", "0.0.0.0", 8080)

	records := decodeHertzRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "info", records[0]["level"])
	require.Equal(t, "listening on 0.0.0.0:8080", records[0]["message"])
}

// TestHertzLoggerAdapter_StripsSystemPrefixAndNewlines 验证 hertz 的 "HERTZ: " 系统前缀
// 与行尾换行被剥离——前缀信息已由 Component 字段结构化表达，重复保留会污染 message。
func TestHertzLoggerAdapter_StripsSystemPrefixAndNewlines(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)

	adapter.Errorf("HERTZ: Error=%s, remoteAddr=%s\n", "boom", "127.0.0.1")

	records := decodeHertzRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "error", records[0]["level"])
	require.Equal(
		t,
		"Error=boom, remoteAddr=127.0.0.1",
		records[0]["message"],
	)
}

// TestHertzLoggerAdapter_SkipsEmptyMessage 验证空消息不产生日志记录
func TestHertzLoggerAdapter_SkipsEmptyMessage(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)

	adapter.Info("")
	adapter.Infof("\n")
	adapter.Infof("HERTZ: ")

	require.Empty(t, decodeHertzRecords(t, output))
}

// TestHertzLoggerAdapter_CtxVariantsAttachTraceID 验证 Ctx* 系列能从请求上下文
// 提取 traceId，使引擎日志可与业务请求链路关联。
func TestHertzLoggerAdapter_CtxVariantsAttachTraceID(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)
	ctx := ctxpkg.WithValue(ctxpkg.Background(), TraceIDContextKey, "trace-xyz")

	adapter.CtxInfof(ctx, "handling request")

	records := decodeHertzRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "info", records[0]["level"])
	require.Equal(t, "trace-xyz", records[0]["traceId"])
	require.Equal(t, "handling request", records[0]["message"])
}

// TestHertzLoggerAdapter_CtxWithoutTraceIDOmitsField 验证无 traceId 时不写入空字段
func TestHertzLoggerAdapter_CtxWithoutTraceIDOmitsField(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)

	adapter.CtxInfof(ctxpkg.Background(), "no trace")

	records := decodeHertzRecords(t, output)
	require.Len(t, records, 1)
	require.NotContains(t, records[0], "traceId")
}

// TestHertzLoggerAdapter_SetOutputIsNoop 验证 SetOutput/SetLevel 不影响框架日志器。
// hlog.Control 要求实现这两个方法，但输出目标与级别应由框架配置统一掌管，
// 不允许 hertz 侧反向篡改。
func TestHertzLoggerAdapter_SetOutputIsNoop(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)
	var hijack bytes.Buffer

	adapter.SetOutput(&hijack)
	adapter.SetLevel(hlog.LevelError)
	adapter.Info("still framework")

	require.Empty(t, hijack.String())
	records := decodeHertzRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "still framework", records[0]["message"])
}

// TestHertzLoggerAdapter_ImplementsFullLogger 编译期与运行期双重确认接口契约
func TestHertzLoggerAdapter_ImplementsFullLogger(t *testing.T) {
	adapter, _ := newTestHertzLogAdapter(t)
	var full hlog.FullLogger = adapter
	require.NotNil(t, full)
}

func TestInstallHertzLogger_RoutesEngineLogsToFrameworkLogger(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)

	lease, err := InstallHertzLogger(adapter)
	require.NoError(t, err)
	require.NotNil(t, lease)
	t.Cleanup(lease.Release)

	hlog.Infof("engine message %d", 7)

	records := decodeHertzRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(t, "info", records[0]["level"])
	require.Equal(t, "Hertz", records[0]["Component"])
	require.Equal(t, "engine message 7", records[0]["message"])
}

// TestInstallHertzLogger_SystemLoggerAlsoRouted 验证 hertz 内部使用的 SystemLogger
// （引擎启动、路由注册等输出的实际通道）同样被接管。
func TestInstallHertzLogger_SystemLoggerAlsoRouted(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)

	lease, err := InstallHertzLogger(adapter)
	require.NoError(t, err)
	t.Cleanup(lease.Release)

	hlog.SystemLogger().Infof("HTTP server listening on address=%s", "[::]:8080")

	records := decodeHertzRecords(t, output)
	require.Len(t, records, 1)
	require.Equal(
		t,
		"HTTP server listening on address=[::]:8080",
		records[0]["message"],
	)
}

// TestInstallHertzLogger_ReleaseStopsForwarding 验证释放后引擎日志不再进入
// 已被应用关闭的框架日志器，避免向已关闭 writer 写入。
func TestInstallHertzLogger_ReleaseStopsForwarding(t *testing.T) {
	adapter, output := newTestHertzLogAdapter(t)

	lease, err := InstallHertzLogger(adapter)
	require.NoError(t, err)

	lease.Release()
	hlog.Infof("after release")

	require.Empty(t, decodeHertzRecords(t, output))
}

// TestInstallHertzLogger_RejectsSecondActiveLease 验证同一时刻只允许一个所有者
func TestInstallHertzLogger_RejectsSecondActiveLease(t *testing.T) {
	firstAdapter, firstOutput := newTestHertzLogAdapter(t)
	secondAdapter, secondOutput := newTestHertzLogAdapter(t)

	firstLease, err := InstallHertzLogger(firstAdapter)
	require.NoError(t, err)
	t.Cleanup(firstLease.Release)

	secondLease, err := InstallHertzLogger(secondAdapter)
	require.ErrorIs(t, err, ErrHertzLoggerAlreadyInstalled)
	require.Nil(t, secondLease)

	hlog.Infof("first owner")

	require.Len(t, decodeHertzRecords(t, firstOutput), 1)
	require.Empty(t, decodeHertzRecords(t, secondOutput))
}

// TestHertzLoggerLease_ReleaseIsIdempotent 验证重复释放安全，
// 支持 AppCoreRun 与 Shutdown 双路径释放。
func TestHertzLoggerLease_ReleaseIsIdempotent(t *testing.T) {
	adapter, _ := newTestHertzLogAdapter(t)

	lease, err := InstallHertzLogger(adapter)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		lease.Release()
		lease.Release()
		lease.Release()
	})

	// 释放后应可重新安装
	next, err := InstallHertzLogger(adapter)
	require.NoError(t, err)
	require.NotNil(t, next)
	next.Release()
}

func TestInstallHertzLogger_RejectsNilDependencies(t *testing.T) {
	lease, err := InstallHertzLogger(nil)
	require.Error(t, err)
	require.Nil(t, lease)

	lease, err = InstallHertzLogger(&HertzLoggerAdapter{})
	require.Error(t, err)
	require.Nil(t, lease)

	var typedNilLogger *bootstrap.LoggerWrap
	lease, err = InstallHertzLogger(NewHertzLoggerAdapter(
		typedNilLogger,
		appconfig.LogOrigin("Frame"),
	))
	if lease != nil {
		t.Cleanup(lease.Release)
	}
	require.Error(t, err)
	require.Nil(t, lease)
}

// TestHertzLoggerLease_NilReleaseIsSafe 验证零值 lease 释放不 panic
func TestHertzLoggerLease_NilReleaseIsSafe(t *testing.T) {
	var lease *HertzLoggerLease
	require.NotPanics(t, lease.Release)
}
