// 驗證程式：用 FiberHouse 真正的 DefaultStackTraceHandler 產生事件，經 sentryzerolog 送到本機 GlitchTip。
//
// VARIANT 環境變數標識框架版本（current / variant-a），寫入事件的 environment，便於在 GlitchTip 區分。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/lamxy/fiberhouse"
	adaptorerrorhandler "github.com/lamxy/fiberhouse/adaptor/errorhandler"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
)

const codecKey = "verify-json-codec"

type starter struct {
	fiberhouse.ApplicationStarter
	app fiberhouse.IApplication
}

func (s *starter) GetApplication() fiberhouse.IApplication { return s.app }

type application struct{ fiberhouse.IApplication }

func (a *application) GetFastTrafficCodecKey() string { return codecKey }

// recoverManager 让 ErrorHandler 加载 Fiber 恢复实现
type recoverManager struct {
	fiberhouse.IProviderManager
	recovery fiberhouse.IRecover
}

func (m *recoverManager) LoadProvider(...fiberhouse.ProviderLoadFunc) (any, error) {
	return m.recovery, nil
}

func main() {
	variant := os.Getenv("VARIANT")
	dumpDir := os.Getenv("DUMP_DIR")

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         os.Getenv("SENTRY_DSN"),
		Environment: variant,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if os.Getenv("APPSIDE") == "1" {
				event = fiberhouseBeforeSend("main")(event, hint)
			}
			// SDK 端证据：送出前的事件原样落盘
			b, _ := json.MarshalIndent(event, "", "  ")
			name := fmt.Sprintf("%s-%s.json", variant, event.Tags["scenario"])
			_ = os.WriteFile(filepath.Join(dumpDir, name), b, 0o644)
			return event
		},
	}); err != nil {
		panic(err)
	}
	// 共用全局 hub：BeforeSend 与 scope 标签对 writer 送出的事件同样生效；Levels 用默认值
	sentryWriter, err := sentryzerolog.NewWithHub(sentry.CurrentHub(), sentryzerolog.Options{})
	if err != nil {
		panic(err)
	}

	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.appLog.enableConsole":     true,
		"application.appLog.consoleJSON":       true,
		"application.appLog.level":             "debug",
		"application.recover.debugMode":        false,
		"application.recover.enablePrintStack": true,
		"application.trace.requestID":          "traceId",
	}).Initialize()
	// 与框架 FiberHouse.New 相同的日志器装配路径：附加 writer 追加在 console 之后
	logger := bootstrap.NewLoggerOnceWithWriters(cfg, []io.Writer{sentryWriter})

	ctx := fiberhouse.NewAppContext(cfg, logger)
	ctx.RegisterBootConfig(&fiberhouse.BootConfig{})
	ctx.RegisterStarterApp(&starter{app: &application{}})
	fiberhouse.NewRespInfoPManager(ctx)
	ctx.GetContainer().Register(codecKey, func() (interface{}, error) { return jsoncodec.StdJsonDefault(), nil })
	ctx.GetContainer().Register(constant.RegisterKeyPrefix+"exceptions", func() (interface{}, error) {
		return exception.ExceptionMap{"UnknownError": {Code: constant.UnknownErrCode, Msg: constant.UnknownErrMsg}}, nil
	})

	eh := fiberhouse.NewErrorHandler(ctx)
	eh.SetRecoverManager(&recoverManager{IProviderManager: fiberhouse.NewProviderManager(ctx), recovery: fiberhouse.NewFiberRecovery(ctx)})

	app := fiber.New(fiber.Config{ErrorHandler: adaptorerrorhandler.FiberErrorHandler(eh.ErrorHandler)})
	app.Use(fiberhouse.NewFiberRecovery(ctx).RecoverPanic(fiberhouse.RecoverConfig{
		EnableStackTrace:  true,
		StackTraceHandler: eh.DefaultStackTraceHandler,
		JsonCodec:         json.Marshal,
	}).(fiber.Handler))
	// 与示例应用相同的 requestid 配置：ID 存进 Locals("traceId") 并写到响应头
	app.Use(requestid.New(requestid.Config{ContextKey: "traceId"}))
	// scenario 头部用于在 BeforeSend 里给事件命名
	app.Use(func(c *fiber.Ctx) error {
		sentry.ConfigureScope(func(s *sentry.Scope) { s.SetTag("scenario", c.Path()[1:]) })
		return c.Next()
	})
	app.Get("/panic", panicRoute)
	app.Get("/error", errorRoute)

	for _, path := range []string{"/panic", "/error"} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil), -1)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(os.Stderr, "[%s] %s -> HTTP %d\n", variant, path, resp.StatusCode)
	}
	if !sentry.Flush(5 * time.Second) {
		fmt.Fprintln(os.Stderr, "sentry flush timeout")
	}
}

// panicSite 是 panic 发生的位置，用于检查 GlitchTip 的堆栈里有没有这一帧
func panicSite() {
	var p *struct{ N int }
	_ = p.N
}

func panicRoute(*fiber.Ctx) error {
	panicSite()
	return nil
}

// errorSite 是 handler 返回 error 的位置
func errorSite() error {
	msg := os.Getenv("ERRMSG")
	if msg == "" {
		msg = "db query timeout"
	}
	return errors.New(msg)
}

func errorRoute(*fiber.Ctx) error {
	return errorSite()
}
