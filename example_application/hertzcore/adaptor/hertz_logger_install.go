package adaptor

import (
	ctxpkg "context"
	"errors"
	"io"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// ErrHertzLoggerAlreadyInstalled 已存在活跃的日志所有者
var ErrHertzLoggerAlreadyInstalled = errors.New(
	"hertz framework logger is already installed",
)

// hertzLoggerInstallation 进程级安装状态。
//
// hlog.SetLogger 明确声明「非并发安全，且不得在 hlog 全局函数被使用后调用」，
// 因此这里只在首次安装时调用一次 SetLogger，装入一个稳定的转发器；
// 之后的所有权变更只通过 atomic 指针切换转发目标，不再触碰 hlog 全局变量。
// 这与 Gin 适配器的 lease 语义一致，但更必要——Gin 的全局钩子本身可重复赋值。
var hertzLoggerInstallation struct {
	once     sync.Once
	active   atomic.Pointer[HertzLoggerLease]
	fallback hlog.FullLogger
}

// HertzLoggerLease 持有当前活跃的转发目标，直到 Release
type HertzLoggerLease struct {
	adapter *HertzLoggerAdapter
}

// hertzLoggerForwarder 装入 hlog 的稳定转发器。
//
// 有活跃 lease 时转发给框架日志器；否则回落到 hertz 原始日志器，
// 保证应用关闭日志器后引擎仍能输出，而不是写入已关闭的 writer。
type hertzLoggerForwarder struct{}

var _ hlog.FullLogger = (*hertzLoggerForwarder)(nil)

// target 返回当前应接收日志的实现
func (f *hertzLoggerForwarder) target() hlog.FullLogger {
	if lease := hertzLoggerInstallation.active.Load(); lease != nil {
		return lease.adapter
	}
	return hertzLoggerInstallation.fallback
}

// InstallHertzLogger 将 hertz 引擎的进程级日志接管到 adapter。
//
// 返回的 lease 表示所有权；调用 Release 后引擎日志回落到 hertz 原始日志器。
// 同一时刻只允许一个活跃 lease，重复安装返回 ErrHertzLoggerAlreadyInstalled。
func InstallHertzLogger(adapter *HertzLoggerAdapter) (*HertzLoggerLease, error) {
	if adapter == nil {
		return nil, errors.New("hertz logger adapter is nil")
	}
	if isNilHertzFrameworkLogger(adapter.logger) {
		return nil, errors.New("hertz logger adapter has no framework logger")
	}

	lease := &HertzLoggerLease{adapter: adapter}

	// 安装发生在引擎产生日志之前；此后 hlog 全局量保持稳定，
	// 并发的引擎日志读取不会与所有权变更竞争。
	hertzLoggerInstallation.once.Do(func() {
		hertzLoggerInstallation.fallback = hlog.DefaultLogger()
		hlog.SetLogger(&hertzLoggerForwarder{})
	})

	if !hertzLoggerInstallation.active.CompareAndSwap(nil, lease) {
		return nil, ErrHertzLoggerAlreadyInstalled
	}

	return lease, nil
}

// Release 停用当前所有者，不修改 hlog 全局变量。
//
// 可重复调用（幂等），以支持 AppCoreRun 与 Shutdown 两条路径分别释放。
func (l *HertzLoggerLease) Release() {
	if l == nil {
		return
	}
	hertzLoggerInstallation.active.CompareAndSwap(l, nil)
}

// isNilHertzFrameworkLogger 识别 nil 接口与持有 nil 指针的非 nil 接口
func isNilHertzFrameworkLogger(logger any) bool {
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

/* ---------- hlog.Logger ---------- */

func (f *hertzLoggerForwarder) Trace(v ...interface{})  { f.target().Trace(v...) }
func (f *hertzLoggerForwarder) Debug(v ...interface{})  { f.target().Debug(v...) }
func (f *hertzLoggerForwarder) Info(v ...interface{})   { f.target().Info(v...) }
func (f *hertzLoggerForwarder) Notice(v ...interface{}) { f.target().Notice(v...) }
func (f *hertzLoggerForwarder) Warn(v ...interface{})   { f.target().Warn(v...) }
func (f *hertzLoggerForwarder) Error(v ...interface{})  { f.target().Error(v...) }
func (f *hertzLoggerForwarder) Fatal(v ...interface{})  { f.target().Fatal(v...) }

/* ---------- hlog.FormatLogger ---------- */

func (f *hertzLoggerForwarder) Tracef(format string, v ...interface{}) {
	f.target().Tracef(format, v...)
}

func (f *hertzLoggerForwarder) Debugf(format string, v ...interface{}) {
	f.target().Debugf(format, v...)
}

func (f *hertzLoggerForwarder) Infof(format string, v ...interface{}) {
	f.target().Infof(format, v...)
}

func (f *hertzLoggerForwarder) Noticef(format string, v ...interface{}) {
	f.target().Noticef(format, v...)
}

func (f *hertzLoggerForwarder) Warnf(format string, v ...interface{}) {
	f.target().Warnf(format, v...)
}

func (f *hertzLoggerForwarder) Errorf(format string, v ...interface{}) {
	f.target().Errorf(format, v...)
}

func (f *hertzLoggerForwarder) Fatalf(format string, v ...interface{}) {
	f.target().Fatalf(format, v...)
}

/* ---------- hlog.CtxLogger ---------- */

func (f *hertzLoggerForwarder) CtxTracef(ctx ctxpkg.Context, format string, v ...interface{}) {
	f.target().CtxTracef(ctx, format, v...)
}

func (f *hertzLoggerForwarder) CtxDebugf(ctx ctxpkg.Context, format string, v ...interface{}) {
	f.target().CtxDebugf(ctx, format, v...)
}

func (f *hertzLoggerForwarder) CtxInfof(ctx ctxpkg.Context, format string, v ...interface{}) {
	f.target().CtxInfof(ctx, format, v...)
}

func (f *hertzLoggerForwarder) CtxNoticef(ctx ctxpkg.Context, format string, v ...interface{}) {
	f.target().CtxNoticef(ctx, format, v...)
}

func (f *hertzLoggerForwarder) CtxWarnf(ctx ctxpkg.Context, format string, v ...interface{}) {
	f.target().CtxWarnf(ctx, format, v...)
}

func (f *hertzLoggerForwarder) CtxErrorf(ctx ctxpkg.Context, format string, v ...interface{}) {
	f.target().CtxErrorf(ctx, format, v...)
}

func (f *hertzLoggerForwarder) CtxFatalf(ctx ctxpkg.Context, format string, v ...interface{}) {
	f.target().CtxFatalf(ctx, format, v...)
}

/* ---------- hlog.Control ---------- */

// SetLevel 转发给当前目标；适配器实现为空操作，回落时保持 hertz 原语义
func (f *hertzLoggerForwarder) SetLevel(lv hlog.Level) { f.target().SetLevel(lv) }

// SetOutput 同 SetLevel
func (f *hertzLoggerForwarder) SetOutput(w io.Writer) { f.target().SetOutput(w) }
