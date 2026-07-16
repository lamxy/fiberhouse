package fiberhouse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const task5CodecKey = "task5-json-codec"

type task5Starter struct {
	ApplicationStarter
	application IApplication
}

func (s *task5Starter) GetApplication() IApplication { return s.application }

type task5Application struct {
	IApplication
}

func (a *task5Application) GetFastTrafficCodecKey() string { return task5CodecKey }

func newTask5AppContext(t *testing.T, debugMode, binarySupport bool) IApplicationContext {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.recover.debugMode":        debugMode,
		"application.recover.enablePrintStack": false,
		"application.recover.enableDebugFlag":  false,
		"application.recover.debugFlag":        "X-Debug",
		"application.recover.debugFlagValue":   "enabled",
		"application.trace.requestID":          "X-Trace-ID",
	}).Initialize()
	logger := zerolog.Nop()
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&BootConfig{EnableBinaryProtocolSupport: binarySupport})
	ctx.RegisterStarterApp(&task5Starter{application: &task5Application{}})
	ctx.GetContainer().Unregister(task5CodecKey)
	require.True(t, ctx.GetContainer().Register(task5CodecKey, func() (interface{}, error) {
		return jsoncodec.StdJsonDefault(), nil
	}))
	t.Cleanup(func() { ctx.GetContainer().Unregister(task5CodecKey) })
	return ctx
}

func installTask5ResponseManager(t *testing.T, ctx IApplicationContext) *RespInfoPManager {
	t.Helper()
	previous := respInfoPManagerInstance
	base := NewProviderManager(ctx).
		SetName("task5-response-manager").
		SetType(ProviderTypeDefault().GroupResponseInfoChoose)
	manager := &RespInfoPManager{IProviderManager: base}
	base.MountToParent(manager)
	require.NoError(t, manager.Register(NewRespInfoProtobufProvider()))
	require.NoError(t, manager.Register(NewRespInfoMsgpackProvider()))

	respInfoPManagerInstance = manager
	respInfoPManagerOnce = sync.Once{}
	respInfoPManagerOnce.Do(func() {})
	t.Cleanup(func() {
		respInfoPManagerInstance = previous
		respInfoPManagerOnce = sync.Once{}
		if previous != nil {
			respInfoPManagerOnce.Do(func() {})
		}
	})
	return manager
}

func TestRecoverConfig_DefaultOverrideAndConcurrentIsolation(t *testing.T) {
	defaultConfig := configDefault()
	assert.False(t, defaultConfig.EnableStackTrace)
	assert.False(t, defaultConfig.DebugMode)
	assert.True(t, defaultConfig.Stdout)
	assert.NotNil(t, defaultConfig.StackTraceHandler)

	customHandler := func(adaptorctx.ICoreContext, interface{}) {}
	custom := configDefault(RecoverConfig{
		EnableStackTrace:  true,
		StackTraceHandler: customHandler,
		DebugMode:         true,
		Stdout:            false,
	})
	assert.True(t, custom.EnableStackTrace)
	assert.True(t, custom.DebugMode)
	assert.NotNil(t, custom.StackTraceHandler)

	injected := configDefault(RecoverConfig{EnableStackTrace: true})
	assert.NotNil(t, injected.StackTraceHandler)

	const workers = 64
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got := configDefault(RecoverConfig{DebugMode: i%2 == 0, Logger: i})
			if got.Logger != i || got.DebugMode != (i%2 == 0) {
				results <- fmt.Errorf("worker %d received another caller's config: %#v", i, got)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		t.Error(err)
	}
}

func TestRecoverConfig_StackHandlerRunsOnceForFiberAndGin(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	installTask5ResponseManager(t, ctx)

	for _, core := range []string{"fiber", "gin"} {
		t.Run(core, func(t *testing.T) {
			var stackCalls atomic.Int32
			cfg := RecoverConfig{
				EnableStackTrace: true,
				StackTraceHandler: func(adaptorctx.ICoreContext, interface{}) {
					stackCalls.Add(1)
				},
				JsonCodec: json.Marshal,
			}

			switch core {
			case "fiber":
				app := fiber.New()
				app.Use(NewFiberRecovery(ctx).RecoverPanic(cfg).(fiber.Handler))
				app.Get("/panic", func(*fiber.Ctx) error { panic("boom") })
				response, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
				require.NoError(t, err)
				response.Body.Close()
				assert.Equal(t, fiber.StatusInternalServerError, response.StatusCode)
			case "gin":
				preserveTask4GinMode(t)
				gin.SetMode(gin.TestMode)
				engine := gin.New()
				handler := NewGinRecovery(ctx).RecoverPanic(cfg).(func(*gin.Context))
				engine.Use(gin.HandlerFunc(handler))
				engine.GET("/panic", func(*gin.Context) { panic("boom") })
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, httptest.NewRequest("GET", "/panic", nil))
				assert.Equal(t, fiber.StatusInternalServerError, recorder.Code)
			}
			assert.Equal(t, int32(1), stackCalls.Load())
		})
	}
}

func TestRecoverHelpers_RequestMetadataSanitizationAndWrongContext(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	logger := ctx.GetLogger()
	fiberRecovery := NewFiberRecovery(ctx)
	ginRecovery := NewGinRecovery(ctx)
	failingEncoder := func(interface{}) ([]byte, error) { return nil, errors.New("encode failed") }

	fiberApp := fiber.New()
	fiberApp.Get("/items/:id", func(c *fiber.Ctx) error {
		wrapped := adaptorctx.WithFiberContext(c)
		assert.JSONEq(t, `{"id":"42"}`, string(fiberRecovery.GetParamsJson(wrapped, logger, json.Marshal, "trace")))
		assert.JSONEq(t, `{"q":"one"}`, string(fiberRecovery.GetQueriesJson(wrapped, logger, json.Marshal, "trace")))
		headers := fiberRecovery.GetHeadersJson(wrapped, logger, json.Marshal, "trace")
		assert.Contains(t, string(headers), `"Authorization":["Bear...***"]`)
		assert.Contains(t, string(headers), `"X-Visible":["plain"]`)
		assert.Nil(t, fiberRecovery.GetHeadersJson(wrapped, logger, failingEncoder, "trace"))
		assert.Equal(t, "Bearer long-sensitive-value", fiberRecovery.GetHeader(wrapped, "Authorization"))

		ginRecorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(ginRecorder)
		ginCtx.Request = httptest.NewRequest("GET", "/", nil)
		wrong := adaptorctx.WithGinContext(ginCtx)
		assert.Nil(t, fiberRecovery.GetParamsJson(wrong, logger, json.Marshal, "trace"))
		assert.Nil(t, fiberRecovery.GetQueriesJson(wrong, logger, json.Marshal, "trace"))
		assert.Nil(t, fiberRecovery.GetHeadersJson(wrong, logger, json.Marshal, "trace"))
		assert.Empty(t, fiberRecovery.GetHeader(wrong, "Authorization"))
		wrong.(*adaptorctx.GinContext).Release()
		wrapped.(*adaptorctx.FiberContext).Release()
		return c.SendStatus(fiber.StatusNoContent)
	})
	fiberReq := httptest.NewRequest("GET", "/items/42?q=one", nil)
	fiberReq.Header.Set("Authorization", "Bearer long-sensitive-value")
	fiberReq.Header.Set("X-Visible", "plain")
	fiberResponse, err := fiberApp.Test(fiberReq)
	require.NoError(t, err)
	fiberResponse.Body.Close()

	preserveTask4GinMode(t)
	gin.SetMode(gin.TestMode)
	ginEngine := gin.New()
	ginEngine.GET("/items/:id", func(c *gin.Context) {
		wrapped := adaptorctx.WithGinContext(c)
		assert.JSONEq(t, `{"id":"42"}`, string(ginRecovery.GetParamsJson(wrapped, logger, json.Marshal, "trace")))
		assert.JSONEq(t, `{"q":["one","two"]}`, string(ginRecovery.GetQueriesJson(wrapped, logger, json.Marshal, "trace")))
		headers := ginRecovery.GetHeadersJson(wrapped, logger, json.Marshal, "trace")
		assert.Contains(t, string(headers), `"Authorization":["Bear...***"]`)
		assert.Contains(t, string(headers), `"X-Visible":["plain"]`)
		assert.Nil(t, ginRecovery.GetQueriesJson(wrapped, logger, failingEncoder, "trace"))
		assert.Equal(t, "plain", ginRecovery.GetHeader(wrapped, "X-Visible"))
		assert.Nil(t, ginRecovery.GetParamsJson(&task5WrongCoreContext{}, logger, json.Marshal, "trace"))
		assert.Nil(t, ginRecovery.GetQueriesJson(&task5WrongCoreContext{}, logger, json.Marshal, "trace"))
		assert.Nil(t, ginRecovery.GetHeadersJson(&task5WrongCoreContext{}, logger, json.Marshal, "trace"))
		assert.Empty(t, ginRecovery.GetHeader(&task5WrongCoreContext{}, "X-Visible"))
		wrapped.(*adaptorctx.GinContext).Release()
		c.Status(fiber.StatusNoContent)
	})
	ginReq := httptest.NewRequest("GET", "/items/42?q=one&q=two", nil)
	ginReq.Header.Set("Authorization", "Bearer long-sensitive-value")
	ginReq.Header.Set("X-Visible", "plain")
	ginRecorder := httptest.NewRecorder()
	ginEngine.ServeHTTP(ginRecorder, ginReq)
	assert.Equal(t, fiber.StatusNoContent, ginRecorder.Code)
}

type task5WrongCoreContext struct{}

func (*task5WrongCoreContext) GetCtx() interface{}         { return struct{}{} }
func (*task5WrongCoreContext) GetHeader(string) string     { return "" }
func (*task5WrongCoreContext) SetHeader(string, string)    {}
func (*task5WrongCoreContext) JSON(int, interface{}) error { return nil }
func (*task5WrongCoreContext) Send(int, []byte) error      { return nil }

func TestRecoverHelpers_HeaderMaskingRules(t *testing.T) {
	assert.Equal(t, "", maskValue(""))
	assert.Equal(t, "***", maskValue("short"))
	assert.Equal(t, "1234...***", maskValue("123456789"))
	assert.True(t, isSensitiveHeader("authorization"))
	assert.True(t, isSensitiveHeader("x-session-token"))
	assert.True(t, isSensitiveHeader("client-secret"))
	assert.False(t, isSensitiveHeader("x-visible"))

	original := map[string][]string{
		"Cookie":    {"session-value"},
		"X-Visible": {"plain"},
	}
	masked := sanitizeHeaders(original)
	assert.Equal(t, []string{"sess...***"}, masked["Cookie"])
	assert.Equal(t, []string{"plain"}, masked["X-Visible"])
	assert.Equal(t, []string{"session-value"}, original["Cookie"])
}
