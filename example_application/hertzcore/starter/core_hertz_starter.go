// Package starter 提供以 cloudwego/hertz 作為核心引擎的 fiberhouse.CoreStarter 實作。
//
// 本套件完全位於 example_application 之下，僅依賴框架的匯出 API，
// 用以驗證 fiberhouse 「核心引擎可插拔」的擴展設計：
// 新增核心引擎不需要修改框架任何一行程式碼。
//
// 各生命週期方法的統一骨架（自定義核心的實作範式）：
//  1. 以 Phase() >= fiberhouse.PhaseServing 短路已運行的應用；
//  2. 呼叫 fiberhouse.LoadProviderManagersAtLocation 分派該位點的 provider，
//     若回傳 replaced 為 true 表示已被擴展替代組接管，直接返回；
//  3. 執行本核心的預設邏輯。
package starter

import (
	ctxpkg "context"
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	hertzadaptor "github.com/lamxy/fiberhouse/example_application/hertzcore/adaptor"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
)

// CoreWithHertz 基於 hertz 的核心應用啟動器
type CoreWithHertz struct {
	ctx                 fiberhouse.IApplicationContext
	Options             []config.Option
	coreApp             *server.Hertz
	json                fiberhouse.JsonWrapper
	hertzLoggerLease    *hertzadaptor.HertzLoggerLease
	initErr             error
	shutdownCoordinated atomic.Bool
	// globalsReleased 保證全域物件只釋放一次：Shutdown 的正常路徑與 defer 各呼叫
	// 一次，提前返回時只有 defer 生效。掛在實例上而非套件層級。
	globalsReleased atomic.Bool
}

// releaseHertzLogger 釋放引擎日誌所有權，使引擎日誌回落到 hertz 原始日誌器。
//
// 必須在框架日誌器關閉前釋放，否則引擎後續日誌會寫入已關閉的 writer。
func (ch *CoreWithHertz) releaseHertzLogger() {
	if ch.hertzLoggerLease != nil {
		ch.hertzLoggerLease.Release()
	}
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
	if ch.coreApp != nil || ch.initializationFailed() || ch.GetAppContext().Phase() >= fiberhouse.PhaseServing {
		return
	}

	_, replaced, err := fiberhouse.LoadProviderManagersAtLocation(
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

	// 接管引擎日誌：須在 server.New 之前安裝，
	// 否則路由註冊等啟動期輸出會直接落到 stderr 而繞過框架日誌器。
	lease, err := hertzadaptor.InstallHertzLogger(
		hertzadaptor.NewHertzLoggerAdapter(
			ch.GetAppContext().GetLogger(),
			cfg.LogOriginFrame(),
		),
	)
	if err != nil {
		ch.initErr = fmt.Errorf("install Hertz framework logger: %w", err)
		ch.logErr(ch.initErr, "InitCoreApp Hertz logger bridge failed")
		return
	}
	ch.hertzLoggerLease = lease

	initialized := false
	defer func() {
		if !initialized {
			ch.releaseHertzLogger()
		}
	}()

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
	initialized = true
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
	if ch.initializationFailed() || ch.GetAppContext().Phase() >= fiberhouse.PhaseServing {
		return
	}

	_, replaced, err := fiberhouse.LoadProviderManagersAtLocation(
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
	if ch.initializationFailed() || ch.GetAppContext().Phase() >= fiberhouse.PhaseServing {
		return
	}

	if _, _, err := fiberhouse.LoadProviderManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationModuleMiddlewareInit, ch); err != nil {
		ch.logErr(err, "RegisterModuleInitialize module middleware providers failed")
	}

	routeHandled, _, err := fiberhouse.LoadProviderManagersAtLocation(
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
	if ch.initializationFailed() || ch.GetAppContext().Phase() >= fiberhouse.PhaseServing {
		return
	}

	_, replaced, err := fiberhouse.LoadProviderManagersAtLocation(
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
	if ch.initializationFailed() || ch.GetAppContext().Phase() >= fiberhouse.PhaseServing {
		return
	}

	_, replaced, err := fiberhouse.LoadProviderManagersAtLocation(
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
		// 監聽已真正建立，此刻才是 PhaseServing 的置位時機；不能等阻塞的 Run 返回
		// 之後再置位——那時伺服器已經停止。與 Fiber 的 OnListen hook 同義。
		if err := appCtx.TransitionTo(fiberhouse.PhaseServing); err != nil {
			ch.logErr(err, "App phase transition to serving was rejected")
		}
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
	// Run 返回即代表監聽結束；若非由 Shutdown 協調，則在此歸還引擎日誌所有權
	defer func() {
		if !ch.shutdownCoordinated.Load() {
			ch.releaseHertzLogger()
		}
	}()

	if ch.initializationFailed() {
		return ch.initErr
	}
	if ch.GetAppContext().Phase() >= fiberhouse.PhaseServing {
		return nil
	}

	_, replaced, err := fiberhouse.LoadProviderManagersAtLocation(
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

	// 這裡不再置位「已啟動」：Run 返回意味著伺服器已經停止。
	// PhaseServing 由 RegisterAppHooks 安裝的 OnRun 鉤子在監聽建立時置位。
	return nil
}

// Shutdown 優雅關閉應用
func (ch *CoreWithHertz) Shutdown(managers ...fiberhouse.IProviderManager) error {
	defer ch.releaseHertzLogger()
	// 以 defer 釋放全域物件：本方法有五處提前返回（初始化失敗、已運行、三處位點載入
	// 失敗、引擎關閉失敗），寫在方法尾端的話這些路徑全部拿不到釋放。
	defer ch.releaseHertzGlobals()

	// 關閉結束時推進到 PhaseTerminated。
	defer func() { _ = ch.GetAppContext().TransitionTo(fiberhouse.PhaseTerminated) }()

	if ch.initializationFailed() {
		return ch.initErr
	}

	// 守衛讀的是「關閉是否已經開始」，而不再是舊的 GetAppState()——後者在使用方
	// 提前 pin 之後恆為 true，會讓 Shutdown 直接返回並跳過全部資源釋放。
	if ch.GetAppContext().Phase() >= fiberhouse.PhaseShuttingDown {
		return nil
	}
	_ = ch.GetAppContext().TransitionTo(fiberhouse.PhaseShuttingDown)

	_, replaced, err := fiberhouse.LoadProviderManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationServerShutdown, ch)
	if err != nil {
		return fmt.Errorf("failed to load server shutdown providers: %w", err)
	}
	if replaced {
		return nil
	}

	if _, _, err = fiberhouse.LoadProviderManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationServerShutdownBefore, ch); err != nil {
		return fmt.Errorf("failed to load pre-shutdown providers: %w", err)
	}

	ch.logInfo("Hertz app Shutting down...")
	shutdownCtx, cancel := ctxpkg.WithTimeout(ctxpkg.Background(), 30*time.Second)
	defer cancel()

	ch.shutdownCoordinated.Store(true)
	if err = ch.coreApp.Shutdown(shutdownCtx); err != nil {
		ch.logErr(err, "Hertz app Shutdown failed.")
		return err
	}

	if _, _, err = fiberhouse.LoadProviderManagersAtLocation(
		managers, fiberhouse.ProviderLocationDefault().LocationServerShutdownAfter, ch); err != nil {
		return fmt.Errorf("failed to load post-shutdown providers: %w", err)
	}

	ch.logInfo("Hertz server shutdown complete")

	// 必須在關閉框架日誌器之前歸還引擎日誌所有權，
	// 否則引擎後續日誌會寫入已關閉的 writer。
	ch.releaseHertzLogger()
	// 同理，全域物件必須在日誌器關閉前釋放：釋放失敗要靠框架日誌器上報，
	// 日誌器已關則錯誤不可見。defer 的那次呼叫因 once 守衛成為 no-op。
	ch.releaseHertzGlobals()
	return ch.GetAppContext().GetLogger().Close()
}

// releaseHertzGlobals 釋放全域物件持有的資源，並按 key 上報關閉失敗。
//
// 此前 hertzcore 完全沒有釋放路徑：OnShutdown 鉤子只寫一行日誌，Shutdown 也不釋放，
// 因此應用註冊的 MySQL 連線池、Mongo 與 Redis 客戶端從未 Close。
//
// 這裡直接呼叫容器上已匯出的 ReleaseAllWithObserver 與 ClearAll，而不是使用框架的
// clearApplicationGlobals —— 後者未匯出。本套件的定位是「僅依賴框架的匯出 API」，
// 用以證明新增核心引擎不需修改框架任何一行程式碼；為了它而新增匯出入口會破壞這個
// 前提，也違反專案「不新增公共運行入口」的約束。
//
// once 守衛使重複呼叫成為 no-op：正常路徑在關閉日誌器前顯式呼叫一次，defer 再呼叫
// 一次；提前返回時只有 defer 那次生效。守衛掛在實例上而非套件層級，同進程的第二個
// 應用實例才不會被前一個的釋放狀態擋掉。
func (ch *CoreWithHertz) releaseHertzGlobals() {
	if !ch.globalsReleased.CompareAndSwap(false, true) {
		return
	}

	appCtx := ch.GetAppContext()
	if appCtx == nil {
		return
	}
	container := appCtx.GetContainer()
	if container == nil {
		return
	}

	container.ReleaseAllWithObserver(func(name globalmanager.KeyName, err error) {
		ch.logErr(err, "failed to release global object '"+string(name)+"' during shutdown")
	}, true)
	container.ClearAll(true)
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
