// Package starter 提供以 cloudwego/hertz 作為核心引擎的 fiberhouse.CoreStarter 實作。
//
// 本套件完全位於 example_application 之下，僅依賴框架的匯出 API，
// 用以驗證 fiberhouse 「核心引擎可插拔」的擴展設計：
// 新增核心引擎不需要修改框架任何一行程式碼。
package starter

import (
	ctxpkg "context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
)

// CoreWithHertz 基於 hertz 的核心應用啟動器
type CoreWithHertz struct {
	ctx     fiberhouse.IApplicationContext
	Options []config.Option
	coreApp *server.Hertz
	json    fiberhouse.JsonWrapper
	initErr error
}

// 編譯期確保實作了框架的核心啟動器介面
var _ fiberhouse.CoreStarter = (*CoreWithHertz)(nil)

// NewCoreWithHertz 建立一個基於 hertz 的核心啟動器物件
func NewCoreWithHertz(ctx fiberhouse.IApplicationContext, opts ...fiberhouse.CoreStarterOption) fiberhouse.CoreStarter {
	core := &CoreWithHertz{ctx: ctx}
	for _, opt := range opts {
		opt(core)
	}
	return core
}

// GetAppContext 獲取應用上下文
func (ch *CoreWithHertz) GetAppContext() fiberhouse.IApplicationContext {
	return ch.ctx
}

// GetCoreApp 獲取核心 hertz 實例
func (ch *CoreWithHertz) GetCoreApp() interface{} {
	return ch.coreApp
}

// GetJsonCodec 獲取已解析的 JSON 編解碼器，供中間件與路由註冊方復用
func (ch *CoreWithHertz) GetJsonCodec() fiberhouse.JsonWrapper {
	return ch.json
}

// initializationFailed 初始化是否已失敗，失敗後續流程一律短路
func (ch *CoreWithHertz) initializationFailed() bool {
	return ch.initErr != nil
}

// InitCoreApp 初始化核心應用（基於 server.Hertz）
func (ch *CoreWithHertz) InitCoreApp(fs fiberhouse.FrameStarter, managers ...fiberhouse.IProviderManager) {
	if ch.coreApp != nil || ch.initializationFailed() || ch.GetAppContext().GetAppState() {
		return
	}

	_, replaced, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationCoreEngineInit, ch)
	if err != nil {
		ch.logErr(err, "InitCoreApp providers failed")
		return
	}
	if replaced {
		return
	}

	cfg := ch.GetAppContext().GetConfig()
	ch.logInfo("InitCoreApp starting...")

	ch.json, err = ch.resolveJSONCodec(fs, managers...)
	if err != nil {
		ch.initErr = err
		ch.logErr(err, "InitCoreApp resolve json codec failed")
		return
	}

	opts := ch.Options
	if len(opts) == 0 {
		opts = ch.buildServerOptions(cfg)
	}
	ch.coreApp = server.New(opts...)
}

// buildServerOptions 依全域配置組裝 hertz 服務端選項。
//
// 配置命名空間沿用框架既有的 application.plugins.engine.servers.<coreType> 慣例。
func (ch *CoreWithHertz) buildServerOptions(cfg appconfig.IAppConfig) []config.Option {
	const prefix = "application.plugins.engine.servers.hertz."

	host := cfg.String(prefix+"host", "0.0.0.0")
	port := cfg.String(prefix+"port", "8080")

	opts := []config.Option{
		server.WithHostPorts(host + ":" + port),
		server.WithReadTimeout(cfg.Duration(prefix+"readTimeout", 30) * time.Second),
		server.WithWriteTimeout(cfg.Duration(prefix+"writeTimeout", 30) * time.Second),
		server.WithIdleTimeout(cfg.Duration(prefix+"idleTimeout", 60) * time.Second),
		server.WithMaxRequestBodySize(cfg.Int(prefix+"maxRequestBodySize", 4*1024*1024)),
		server.WithExitWaitTime(cfg.Duration(prefix+"exitWaitTime", 5) * time.Second),
		server.WithStreamBody(cfg.Bool(prefix + "streamRequestBody")),
	}

	if tlsCfg := ch.buildTLSConfig(cfg, prefix); tlsCfg != nil {
		opts = append(opts, server.WithTLS(tlsCfg))
	}
	return opts
}

// buildTLSConfig 依配置載入 TLS 憑證；未啟用或憑證缺失時回傳 nil 並降級為 HTTP
func (ch *CoreWithHertz) buildTLSConfig(cfg appconfig.IAppConfig, prefix string) *tls.Config {
	if !cfg.Bool(prefix + "tls.enable") {
		return nil
	}
	certFile := cfg.String(prefix+"tls.certFile", "")
	keyFile := cfg.String(prefix+"tls.keyFile", "")
	if certFile == "" || keyFile == "" {
		return nil
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		ch.logErr(err, "Failed to load TLS certificates, fallback to HTTP")
		return nil
	}
	ch.logInfo("TLS/HTTPS enabled")
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}

// resolveJSONCodec 解析 JSON 編解碼器實例。
//
// 行為對齊框架 Fiber 核心的同名邏輯：未提供管理器時取全域預設編解碼器，
// 提供時則由編解碼位點的管理器選出符合 TrafficCodec 與 CoreType 的提供者。
func (ch *CoreWithHertz) resolveJSONCodec(fs fiberhouse.FrameStarter, managers ...fiberhouse.IProviderManager) (fiberhouse.JsonWrapper, error) {
	if len(managers) == 0 {
		if fs == nil || fs.GetApplication() == nil {
			return nil, fmt.Errorf("hertz core: no json codec manager and no frame starter available")
		}
		ch.logInfo("No JSON codec manager provided, using default JSON codec.")
		return fiberhouse.GetMustInstance[fiberhouse.JsonWrapper](
			fs.GetApplication().GetDefaultTrafficCodecKey()), nil
	}

	var codecManager fiberhouse.IProviderManager
	for _, m := range managers {
		if m == nil || m.Location() == nil {
			continue
		}
		if m.Location().GetLocationID() == fiberhouse.ProviderLocationDefault().LocationCoreCodecInit.GetLocationID() &&
			m.Type().GetTypeID() == fiberhouse.ProviderTypeDefault().GroupTrafficCodecChoose.GetTypeID() {
			codecManager = m
			break
		}
	}
	if codecManager == nil {
		return nil, fmt.Errorf("hertz core: no JSON codec manager found in provided managers")
	}

	codec, err := codecManager.LoadProvider()
	if err != nil {
		return nil, fmt.Errorf("hertz core: load json codec provider failed: %w", err)
	}
	json, ok := codec.(fiberhouse.JsonWrapper)
	if !ok {
		return nil, fmt.Errorf("hertz core: loaded JSON codec does not implement JsonWrapper, got %T", codec)
	}
	return json, nil
}

// RegisterAppMiddleware 註冊應用級中間件
func (ch *CoreWithHertz) RegisterAppMiddleware(fs fiberhouse.FrameStarter, managers ...fiberhouse.IProviderManager) {
	if ch.initializationFailed() || ch.GetAppContext().GetAppState() {
		return
	}

	_, replaced, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationAppMiddlewareInit, ch)
	if err != nil {
		ch.logErr(err, "RegisterAppMiddleware providers failed")
		return
	}
	if replaced {
		return
	}

	ch.logInfo("RegisterAppMiddleware")

	// 由框架的恢復管理器依 CoreType 選出 hertz 的恢復中間件
	eh := fiberhouse.NewErrorHandlerOnce(ch.GetAppContext())
	recoverHandler := eh.RecoverMiddleware(fiberhouse.RecoverConfig{
		AppCtx:            ch.GetAppContext(),
		EnableStackTrace:  true,
		StackTraceHandler: eh.DefaultStackTraceHandler,
		Logger:            ch.GetAppContext().GetLogger(),
		Stdout:            false,
		JsonCodec:         ch.json.Marshal,
		DebugMode:         ch.GetAppContext().GetConfig().GetRecover().DebugMode,
	})
	ch.coreApp.Use(fiberhouse.MustRecoverMiddleware[app.HandlerFunc](recoverHandler))

	// HTTP 請求日誌中間件
	ch.coreApp.Use(ch.loggerMiddleware())

	if fs.GetApplication() != nil {
		fs.GetApplication().(fiberhouse.ApplicationRegister).RegisterAppMiddleware(ch)
	}
}

// loggerMiddleware HTTP 請求日誌中間件
func (ch *CoreWithHertz) loggerMiddleware() app.HandlerFunc {
	return func(c ctxpkg.Context, reqCtx *app.RequestContext) {
		if !ch.GetAppContext().GetConfig().GetMiddlewareSwitch("coreHttp") {
			reqCtx.Next(c)
			return
		}

		start := time.Now()
		reqCtx.Next(c)

		appCtx := ch.GetAppContext()
		event := appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginCoreHttp()).
			Str("Component", "Hertz").
			Str("method", string(reqCtx.Method())).
			Str("path", string(reqCtx.Path())).
			Int("status", reqCtx.Response.StatusCode()).
			Dur("latency", time.Since(start)).
			Str("ip", reqCtx.ClientIP()).
			Int("bodySize", len(reqCtx.Response.Body()))

		if query := string(reqCtx.QueryArgs().QueryString()); query != "" {
			event.Str("query", query)
		}
		event.Msg("HTTP Request")
	}
}

// RegisterModuleInitialize 註冊模組級中間件與路由處理器
func (ch *CoreWithHertz) RegisterModuleInitialize(fs fiberhouse.FrameStarter, managers ...fiberhouse.IProviderManager) {
	if ch.initializationFailed() || ch.GetAppContext().GetAppState() {
		return
	}

	if _, _, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationModuleMiddlewareInit, ch); err != nil {
		ch.logErr(err, "RegisterModuleInitialize module middleware providers failed")
	}

	routeHandled, _, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationRouteRegisterInit, ch)
	if err != nil {
		ch.logErr(err, "RegisterModuleInitialize route providers failed")
	}

	if !routeHandled && fs.GetModule() != nil {
		fs.GetModule().RegisterModuleRouteHandlers(ch)
	}
}

// RegisterModuleSwagger 註冊模組級 swagger
func (ch *CoreWithHertz) RegisterModuleSwagger(fs fiberhouse.FrameStarter, managers ...fiberhouse.IProviderManager) {
	if ch.initializationFailed() || ch.GetAppContext().GetAppState() {
		return
	}

	_, replaced, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationModuleSwaggerInit, ch)
	if err != nil {
		ch.logErr(err, "RegisterModuleSwagger providers failed")
		return
	}
	if replaced {
		return
	}

	if ch.GetAppContext().GetConfig().Bool("application.swagger.enable") && fs.GetModule() != nil {
		fs.GetModule().RegisterSwagger(ch)
	}
}

// RegisterAppHooks 註冊核心應用生命週期鉤子
func (ch *CoreWithHertz) RegisterAppHooks(fs fiberhouse.FrameStarter, managers ...fiberhouse.IProviderManager) {
	if ch.initializationFailed() || ch.GetAppContext().GetAppState() {
		return
	}

	_, replaced, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationCoreHookInit, ch)
	if err != nil {
		ch.logErr(err, "RegisterAppHooks providers failed")
		return
	}
	if replaced {
		return
	}

	if fs.GetApplication() != nil {
		fs.GetApplication().(fiberhouse.ApplicationRegister).RegisterCoreHook(ch)
	}

	// 框架層預設鉤子：啟動日誌與關閉時的全域資源清理
	appCtx := ch.GetAppContext()
	ch.coreApp.OnRun = append(ch.coreApp.OnRun, func(c ctxpkg.Context) error {
		ch.logInfo("Hertz app listening on " + ch.listenAddr())
		return nil
	})
	ch.coreApp.OnShutdown = append(ch.coreApp.OnShutdown, func(c ctxpkg.Context) {
		appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).
			Str("applicationStarter", "HertzApplication").
			Str("appShutdown", "ok").
			Msg("")
	})
}

// listenAddr 取得監聽位址，用於啟動日誌
func (ch *CoreWithHertz) listenAddr() string {
	const prefix = "application.plugins.engine.servers.hertz."
	cfg := ch.GetAppContext().GetConfig()
	return cfg.String(prefix+"host", "0.0.0.0") + ":" + cfg.String(prefix+"port", "8080")
}

// AppCoreRun 啟動 hertz 服務並監聽套接字
//
// 使用 Run() 而非 Spin()：框架的 FiberHouse.RunServer 已統一接管系統信號與優雅關閉，
// Spin() 會重複註冊信號監聽並自行呼叫 Shutdown，與框架的關閉流程衝突。
func (ch *CoreWithHertz) AppCoreRun(managers ...fiberhouse.IProviderManager) error {
	if ch.initializationFailed() {
		return ch.initErr
	}
	if ch.GetAppContext().GetAppState() {
		return nil
	}

	_, replaced, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationServerRun, ch)
	if err != nil {
		return fmt.Errorf("failed to load server run providers: %w", err)
	}
	if replaced {
		return nil
	}

	ch.logInfo("App listening...")
	if err = ch.coreApp.Run(); err != nil {
		ch.logErr(err, "Hertz app listen failed")
		return err
	}

	ch.GetAppContext().RegisterAppState(true)
	return nil
}

// Shutdown 優雅關閉應用
func (ch *CoreWithHertz) Shutdown(managers ...fiberhouse.IProviderManager) error {
	if ch.initializationFailed() {
		return ch.initErr
	}
	if ch.GetAppContext().GetAppState() {
		return nil
	}

	_, replaced, err := loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationServerShutdown, ch)
	if err != nil {
		return fmt.Errorf("failed to load server shutdown providers: %w", err)
	}
	if replaced {
		return nil
	}

	if _, _, err = loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationServerShutdownBefore, ch); err != nil {
		return fmt.Errorf("failed to load pre-shutdown providers: %w", err)
	}

	ch.logInfo("Hertz app Shutting down...")
	shutdownCtx, cancel := ctxpkg.WithTimeout(ctxpkg.Background(), 30*time.Second)
	defer cancel()

	if err = ch.coreApp.Shutdown(shutdownCtx); err != nil {
		ch.logErr(err, "Hertz app Shutdown failed.")
		return err
	}

	if _, _, err = loadManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationServerShutdownAfter, ch); err != nil {
		return fmt.Errorf("failed to load post-shutdown providers: %w", err)
	}

	ch.logInfo("Hertz server shutdown complete")
	return ch.GetAppContext().GetLogger().Close()
}

// logInfo 以框架日誌器輸出資訊級日誌
func (ch *CoreWithHertz) logInfo(msg string) {
	appCtx := ch.GetAppContext()
	appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).
		Str("applicationStarter", hertzconst.StarterName).
		Msg(msg)
}

// logErr 以框架日誌器輸出錯誤級日誌
func (ch *CoreWithHertz) logErr(err error, msg string) {
	appCtx := ch.GetAppContext()
	appCtx.GetLogger().ErrorWith(appCtx.GetConfig().LogOriginFrame()).
		Str("applicationStarter", hertzconst.StarterName).
		Err(err).
		Msg(msg)
}
