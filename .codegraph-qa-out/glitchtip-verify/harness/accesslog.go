package main

// MODE=accesslog：走框架公開的 Gin 啟動路徑（NewCoreWithGin → InitCoreApp → RegisterAppMiddleware），
// 讓真正的存取日誌中間件經 sentryzerolog 送到本機 GlitchTip，對照 Levels 是否含 Warn 時建了哪些事件。
//
// LEVELS=warn 時 Levels 為 Warn/Error/Fatal/Panic，否則用 sentryzerolog 預設值（Error/Fatal/Panic）。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/gin-gonic/gin"
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/rs/zerolog"
)

const ginCodecKey = "verify-gin-json-codec"

// ginApplication 只提供 InitCoreApp 需要的預設 JSON 編解碼器鍵
type ginApplication struct{ fiberhouse.IApplication }

func (a *ginApplication) GetDefaultTrafficCodecKey() string { return ginCodecKey }

// ginFrame 的 application 為 nil 時，RegisterAppMiddleware 跳過應用層中間件註冊
type ginFrame struct {
	fiberhouse.FrameStarter
	app fiberhouse.IApplication
}

func (f *ginFrame) GetApplication() fiberhouse.IApplication { return f.app }

func runAccessLog() {
	variant := os.Getenv("VARIANT")
	dumpDir := os.Getenv("DUMP_DIR")

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         os.Getenv("SENTRY_DSN"),
		Environment: variant,
		BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			b, _ := json.MarshalIndent(event, "", "  ")
			name := fmt.Sprintf("%s-%s-%s.json", variant, event.Tags["scenario"], event.Level)
			_ = os.WriteFile(filepath.Join(dumpDir, name), b, 0o644)
			return event
		},
	}); err != nil {
		panic(err)
	}
	opts := sentryzerolog.Options{}
	if os.Getenv("LEVELS") == "warn" {
		opts.Levels = []zerolog.Level{zerolog.WarnLevel, zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel}
	}
	sentryWriter, err := sentryzerolog.NewWithHub(sentry.CurrentHub(), opts)
	if err != nil {
		panic(err)
	}

	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.appLog.enableConsole":     true,
		"application.appLog.consoleJSON":       true,
		"application.appLog.level":             "info",
		"application.middleware.coreHttp":      true,
		"application.recover.debugMode":        false,
		"application.recover.enablePrintStack": false,
	}).Initialize()
	logger := bootstrap.NewLoggerOnceWithWriters(cfg, []io.Writer{sentryWriter})

	ctx := fiberhouse.NewAppContext(cfg, logger)
	ctx.RegisterBootConfig(&fiberhouse.BootConfig{})
	ctx.RegisterStarterApp(&starter{app: &application{}})
	fiberhouse.NewRespInfoPManager(ctx)
	ctx.GetContainer().Register(ginCodecKey, func() (interface{}, error) { return ginJson.API, nil })
	// Recover 的 stack handler 用應用的 fast traffic codec 序列化請求欄位
	ctx.GetContainer().Register(codecKey, func() (interface{}, error) { return jsoncodec.StdJsonDefault(), nil })
	ctx.GetContainer().Register(constant.RegisterKeyPrefix+"exceptions", func() (interface{}, error) {
		return exception.ExceptionMap{"UnknownError": {Code: constant.UnknownErrCode, Msg: constant.UnknownErrMsg}}, nil
	})
	fiberhouse.NewErrorHandlerOnce(ctx).SetRecoverManager(&recoverManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx),
		recovery:         fiberhouse.NewGinRecovery(ctx),
	})

	core := fiberhouse.NewCoreWithGin(ctx)
	core.InitCoreApp(&ginFrame{app: &ginApplication{}})
	core.RegisterAppMiddleware(&ginFrame{})
	engine := core.GetCoreApp().(*gin.Engine)
	engine.GET("/status/:code", func(c *gin.Context) {
		code, _ := strconv.Atoi(c.Param("code"))
		c.Status(code)
	})
	// 對照組：panic 由 Recover 記 ERROR，任何 Levels 下都應建事件
	engine.GET("/panic", func(*gin.Context) { panicSite() })

	for _, path := range []string{"/status/200", "/status/404", "/status/500", "/panic"} {
		scenario := "access" + filepath.Base(path)
		sentry.ConfigureScope(func(s *sentry.Scope) { s.SetTag("scenario", scenario) })
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		fmt.Fprintf(os.Stderr, "[%s] %s -> HTTP %d\n", variant, path, rec.Code)
	}
	if !sentry.Flush(5 * time.Second) {
		fmt.Fprintln(os.Stderr, "sentry flush timeout")
	}
}
