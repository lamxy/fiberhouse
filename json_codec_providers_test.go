package fiberhouse

import (
	"testing"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newJSONCodecProviderTestContext 构造最小化的 IContext，
// 参照 provider_manager_impl_test.go:newTask2AppContext 的等价写法。
// 该 context 未注册 StarterApp，因此 ctx.GetStarter() 返回 nil，
// 适用于 Std Json 系列 provider（不依赖 GetStarter），
// 不适用于需要 ctx.GetStarter().GetApplication() 的 Sonic 系列 provider。
func newJSONCodecProviderTestContext(t *testing.T) IContext {
	t.Helper()
	cfg := appconfig.NewAppConfig()
	logger := zerolog.Nop()
	ctx := NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	return ctx
}

func TestJsonJCodecFiberProvider_RepeatedInitializeReturnsSameCodec(t *testing.T) {
	p := NewJsonJCodecFiberProvider()
	ctx := newJSONCodecProviderTestContext(t)

	first, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, first, "first Initialize() must return a non-nil codec")

	second, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, second, "repeated Initialize() after StateLoaded must not return nil")
	assert.Same(t, first, second, "repeated Initialize() must return the same cached codec instance")
}

func TestJsonJCodecGinProvider_RepeatedInitializeReturnsSameCodec(t *testing.T) {
	p := NewJsonJCodecGinProvider()
	ctx := newJSONCodecProviderTestContext(t)

	first, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, first, "first Initialize() must return a non-nil codec")

	second, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, second, "repeated Initialize() after StateLoaded must not return nil")
	assert.Same(t, first, second, "repeated Initialize() must return the same cached codec instance")
}

// TestSonicJCodecFiberProvider_RepeatedInitializeReturnsSameCodec
//
// 采用方案：手动构造 provider 并直接调用导出的 SetStatus(StateLoaded)，跳过完整
// Initialize() 首次调用。原因：SonicJCodecFiberProvider.Initialize() 首次调用会执行
// GetInstance[JsonWrapper](ctx.GetStarter().GetApplication().GetDefaultTrafficCodecKey())，
// 而 newJSONCodecProviderTestContext 构造的最小 IContext 未调用 RegisterStarterApp，
// ctx.GetStarter() 返回 nil，对 nil 调用 .GetApplication() 会 panic，
// 脱离完整 boot 流程无法安全触发首次初始化路径。
// 因此本测试直接验证规格文件描述的核心问题本身：
// 一旦 Status() 变为 StateLoaded，重复调用 Initialize() 直接进入该分支，
// 现有生产代码在该分支 return nil, nil，与 provider 是否曾经"真正"初始化过无关。
func TestSonicJCodecFiberProvider_RepeatedInitializeReturnsSameCodec(t *testing.T) {
	p := NewSonicJCodecFiberProvider()
	ctx := newJSONCodecProviderTestContext(t)

	// 手动将状态置为 StateLoaded，模拟"已经初始化过一次"的重入场景，
	// 不经过依赖 GetStarter() 的首次初始化路径。
	p.SetStatus(StateLoaded)

	second, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, second, "repeated Initialize() after StateLoaded must not return nil")
}

// TestSonicJCodecGinProvider_RepeatedInitializeReturnsSameCodec
//
// 采用方案：同 TestSonicJCodecFiberProvider_RepeatedInitializeReturnsSameCodec，
// 原因一致：SonicJCodecGinProvider.Initialize() 首次调用同样依赖
// ctx.GetStarter().GetApplication()，最小化测试 context 下 GetStarter() 为 nil 会 panic。
// 直接置 StateLoaded 后验证重复调用分支的行为。
func TestSonicJCodecGinProvider_RepeatedInitializeReturnsSameCodec(t *testing.T) {
	p := NewSonicJCodecGinProvider()
	ctx := newJSONCodecProviderTestContext(t)

	p.SetStatus(StateLoaded)

	second, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, second, "repeated Initialize() after StateLoaded must not return nil")
}
