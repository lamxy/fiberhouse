package adaptor

import (
	ctxpkg "context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
)

const (
	// hertzLogComponent 引擎日志的组件标识，与 Gin 适配器的 Component 字段用法一致
	hertzLogComponent = "Hertz"

	// hertzSystemLogPrefix hertz 系统日志前缀（hlog 的 systemLogPrefix）。
	// 该前缀表达的信息已由结构化字段 Component 承载，转发前剥离以免污染 message。
	hertzSystemLogPrefix = "HERTZ: "
)

// TraceIDContextKey 请求追踪 ID 在标准 context 中的键。
//
// 使用具名类型而非裸字符串，避免与其他包的 context 键冲突。
type traceIDContextKey struct{}

// TraceIDContextKey 供中间件写入、日志适配器读取请求追踪 ID
var TraceIDContextKey = traceIDContextKey{}

// HertzLoggerAdapter 将 hertz 引擎的日志与调试输出映射到框架统一日志器。
//
// hertz 通过 hlog.SetLogger 接管全部引擎日志（相比 Gin 分散的四个全局钩子更集中），
// 因此本适配器实现 hlog.FullLogger 即可覆盖 Logger / FormatLogger / CtxLogger / Control 四组能力。
type HertzLoggerAdapter struct {
	logger bootstrap.LoggerWrapper
	origin appconfig.LogOrigin
}

// 编译期确保满足 hertz 的完整日志器契约
var _ hlog.FullLogger = (*HertzLoggerAdapter)(nil)

// NewHertzLoggerAdapter 创建 hertz 的结构化日志适配器
func NewHertzLoggerAdapter(
	logger bootstrap.LoggerWrapper,
	origin appconfig.LogOrigin,
) *HertzLoggerAdapter {
	return &HertzLoggerAdapter{
		logger: logger,
		origin: origin,
	}
}

// levelOf 将 hertz 日志级别映射到框架（zerolog）级别。
//
// hertz 的 Trace 与 Notice 在框架中没有对应级别，分别归并到 Debug 与 Info，
// 保持语义相近且不丢失信息。
func levelOf(lv hlog.Level) zerolog.Level {
	switch lv {
	case hlog.LevelTrace, hlog.LevelDebug:
		return zerolog.DebugLevel
	case hlog.LevelInfo, hlog.LevelNotice:
		return zerolog.InfoLevel
	case hlog.LevelWarn:
		return zerolog.WarnLevel
	case hlog.LevelError:
		return zerolog.ErrorLevel
	case hlog.LevelFatal:
		return zerolog.FatalLevel
	default:
		return zerolog.InfoLevel
	}
}

// event 按级别取得框架日志事件，并附加组件标识
func (a *HertzLoggerAdapter) event(lv hlog.Level) *zerolog.Event {
	var event *zerolog.Event
	switch levelOf(lv) {
	case zerolog.DebugLevel:
		event = a.logger.DebugWith(a.origin)
	case zerolog.WarnLevel:
		event = a.logger.WarnWith(a.origin)
	case zerolog.ErrorLevel:
		event = a.logger.ErrorWith(a.origin)
	case zerolog.FatalLevel:
		event = a.logger.FatalWith(a.origin)
	default:
		event = a.logger.InfoWith(a.origin)
	}
	return event.Str("Component", hertzLogComponent)
}

// normalize 剥离 hertz 系统前缀与行尾换行
func normalize(message string) string {
	message = strings.TrimRight(message, "\r\n")
	message = strings.TrimPrefix(message, hertzSystemLogPrefix)
	return strings.TrimRight(message, "\r\n")
}

// emit 输出一条日志；空消息直接丢弃
func (a *HertzLoggerAdapter) emit(lv hlog.Level, message string) {
	if message = normalize(message); message == "" {
		return
	}
	a.event(lv).Msg(message)
}

// emitCtx 输出一条带请求追踪信息的日志
func (a *HertzLoggerAdapter) emitCtx(
	ctx ctxpkg.Context,
	lv hlog.Level,
	message string,
) {
	if message = normalize(message); message == "" {
		return
	}

	event := a.event(lv)
	if traceID := traceIDFrom(ctx); traceID != "" {
		event = event.Str("traceId", traceID)
	}
	event.Msg(message)
}

// traceIDFrom 从标准 context 提取请求追踪 ID；缺失时返回空字符串
func traceIDFrom(ctx ctxpkg.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(TraceIDContextKey).(string); ok {
		return v
	}
	return ""
}

/* ---------- hlog.Logger ---------- */

func (a *HertzLoggerAdapter) Trace(v ...interface{}) {
	a.emit(hlog.LevelTrace, fmt.Sprint(v...))
}

func (a *HertzLoggerAdapter) Debug(v ...interface{}) {
	a.emit(hlog.LevelDebug, fmt.Sprint(v...))
}

func (a *HertzLoggerAdapter) Info(v ...interface{}) {
	a.emit(hlog.LevelInfo, fmt.Sprint(v...))
}

func (a *HertzLoggerAdapter) Notice(v ...interface{}) {
	a.emit(hlog.LevelNotice, fmt.Sprint(v...))
}

func (a *HertzLoggerAdapter) Warn(v ...interface{}) {
	a.emit(hlog.LevelWarn, fmt.Sprint(v...))
}

func (a *HertzLoggerAdapter) Error(v ...interface{}) {
	a.emit(hlog.LevelError, fmt.Sprint(v...))
}

func (a *HertzLoggerAdapter) Fatal(v ...interface{}) {
	a.emit(hlog.LevelFatal, fmt.Sprint(v...))
}

/* ---------- hlog.FormatLogger ---------- */

func (a *HertzLoggerAdapter) Tracef(format string, v ...interface{}) {
	a.emit(hlog.LevelTrace, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) Debugf(format string, v ...interface{}) {
	a.emit(hlog.LevelDebug, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) Infof(format string, v ...interface{}) {
	a.emit(hlog.LevelInfo, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) Noticef(format string, v ...interface{}) {
	a.emit(hlog.LevelNotice, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) Warnf(format string, v ...interface{}) {
	a.emit(hlog.LevelWarn, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) Errorf(format string, v ...interface{}) {
	a.emit(hlog.LevelError, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) Fatalf(format string, v ...interface{}) {
	a.emit(hlog.LevelFatal, fmt.Sprintf(format, v...))
}

/* ---------- hlog.CtxLogger ---------- */

func (a *HertzLoggerAdapter) CtxTracef(ctx ctxpkg.Context, format string, v ...interface{}) {
	a.emitCtx(ctx, hlog.LevelTrace, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) CtxDebugf(ctx ctxpkg.Context, format string, v ...interface{}) {
	a.emitCtx(ctx, hlog.LevelDebug, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) CtxInfof(ctx ctxpkg.Context, format string, v ...interface{}) {
	a.emitCtx(ctx, hlog.LevelInfo, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) CtxNoticef(ctx ctxpkg.Context, format string, v ...interface{}) {
	a.emitCtx(ctx, hlog.LevelNotice, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) CtxWarnf(ctx ctxpkg.Context, format string, v ...interface{}) {
	a.emitCtx(ctx, hlog.LevelWarn, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) CtxErrorf(ctx ctxpkg.Context, format string, v ...interface{}) {
	a.emitCtx(ctx, hlog.LevelError, fmt.Sprintf(format, v...))
}

func (a *HertzLoggerAdapter) CtxFatalf(ctx ctxpkg.Context, format string, v ...interface{}) {
	a.emitCtx(ctx, hlog.LevelFatal, fmt.Sprintf(format, v...))
}

/* ---------- hlog.Control ---------- */

// SetLevel 为满足 hlog.Control 契约而存在，实现为空操作。
//
// 日志级别由框架配置（application.appLog.level）统一掌管，
// 不允许引擎侧反向覆盖，否则应用的日志策略会被静默篡改。
func (a *HertzLoggerAdapter) SetLevel(hlog.Level) {}

// SetOutput 同 SetLevel，输出目标由框架日志器（含轮转、异步 writer）统一掌管。
func (a *HertzLoggerAdapter) SetOutput(io.Writer) {}
