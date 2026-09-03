package fiberhouse

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/constant"
)

// newPprofTestContext 构造用于 pprof 测试的应用上下文
func newPprofTestContext(t *testing.T) IApplicationContext {
	t.Helper()
	logger := zerolog.Nop()
	ctx := NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&BootConfig{CoreType: constant.CoreTypeWithFiber})
	return ctx
}

// newPprofTestAppConfig 构造仅含指定键值的应用配置
func newPprofTestAppConfig(t *testing.T, values map[string]interface{}) appconfig.IAppConfig {
	t.Helper()
	cfg := appconfig.NewAppConfig().Initialize()
	if len(values) > 0 {
		cfg.LoadDefault(values)
	}
	return cfg
}

// TestLoadPprofConfigDefaults 未配置任何键时应回落到安全默认值
func TestLoadPprofConfigDefaults(t *testing.T) {
	cfg := LoadPprofConfig(newPprofTestAppConfig(t, nil))

	assert.False(t, cfg.Enable, "默认必须关闭")
	assert.Equal(t, defaultPprofHost, cfg.Host, "默认必须绑定回环地址")
	assert.Equal(t, defaultPprofPort, cfg.Port)
	assert.Equal(t, defaultPprofPrefix, cfg.Prefix)
	assert.Zero(t, cfg.BlockRate, "block 采样默认必须关闭")
	assert.Zero(t, cfg.MutexFraction, "mutex 采样默认必须关闭")
	assert.Equal(t, defaultPprofReadTimeout, cfg.ReadTimeout)
	assert.Equal(t, defaultPprofWriteTimeout, cfg.WriteTimeout)
	assert.Equal(t, defaultPprofShutdownTimeout, cfg.ShutdownTimeout)
	assert.False(t, cfg.AuthEnable)
}

// TestLoadPprofConfigFromValues 配置值应被正确读取
func TestLoadPprofConfigFromValues(t *testing.T) {
	cfg := LoadPprofConfig(newPprofTestAppConfig(t, map[string]interface{}{
		"application.pprof.enable":          true,
		"application.pprof.host":            "0.0.0.0",
		"application.pprof.port":            7070,
		"application.pprof.prefix":          "/internal/diag/",
		"application.pprof.blockRate":       10000,
		"application.pprof.mutexFraction":   100,
		"application.pprof.readTimeout":     15,
		"application.pprof.writeTimeout":    300,
		"application.pprof.shutdownTimeout": 8,
		"application.pprof.auth.enable":     true,
		"application.pprof.auth.username":   "admin",
		"application.pprof.auth.password":   "s3cret",
	}))

	assert.True(t, cfg.Enable)
	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 7070, cfg.Port)
	assert.Equal(t, "/internal/diag", cfg.Prefix, "尾部斜杠应被规范化")
	assert.Equal(t, 10000, cfg.BlockRate)
	assert.Equal(t, 100, cfg.MutexFraction)
	assert.Equal(t, 15*time.Second, cfg.ReadTimeout, "yaml 中的秒数应转为 Duration")
	assert.Equal(t, 300*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 8*time.Second, cfg.ShutdownTimeout)
	assert.True(t, cfg.AuthEnable)
	assert.Equal(t, "admin", cfg.AuthUsername)
	assert.Equal(t, "s3cret", cfg.AuthPassword)
}

// TestPprofProvidersHaveDistinctTypes 启动与关闭提供者必须使用不同类型。
//
// 框架按类型 ID 把提供者分派给管理器且首个匹配即 break，
// 若二者类型相同会被归入同一管理器，导致关闭提供者在启动位点被执行。
func TestPprofProvidersHaveDistinctTypes(t *testing.T) {
	start := NewPprofStartProvider()
	shutdown := NewPprofShutdownProvider()

	require.NotEqual(t, start.Type().GetTypeID(), shutdown.Type().GetTypeID(),
		"启动与关闭提供者的类型 ID 必须不同")
}

// TestPprofManagersBindToExpectedLocations 管理器应绑定到预期位点
func TestPprofManagersBindToExpectedLocations(t *testing.T) {
	ctx := newPprofTestContext(t)

	startMgr := NewPprofStartPManager(ctx)
	shutdownMgr := NewPprofShutdownPManager(ctx)

	assert.Equal(t,
		ProviderLocationDefault().LocationServerRunBefore.GetLocationID(),
		startMgr.Location().GetLocationID(),
		"启动管理器应绑定到 ServerRunBefore 位点")

	assert.Equal(t,
		ProviderLocationDefault().LocationServerShutdownAfter.GetLocationID(),
		shutdownMgr.Location().GetLocationID(),
		"关闭管理器应绑定到 ServerShutdownAfter 位点")
}

// TestPprofManagerMatchesItsProvider 各管理器的类型应与对应提供者匹配
func TestPprofManagerMatchesItsProvider(t *testing.T) {
	ctx := newPprofTestContext(t)

	assert.Equal(t,
		NewPprofStartProvider().Type().GetTypeID(),
		NewPprofStartPManager(ctx).Type().GetTypeID(),
		"启动提供者应能匹配到启动管理器")

	assert.Equal(t,
		NewPprofShutdownProvider().Type().GetTypeID(),
		NewPprofShutdownPManager(ctx).Type().GetTypeID(),
		"关闭提供者应能匹配到关闭管理器")
}
