package fiberhouse

import (
	"testing"

	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
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
// 采用方案：手动构造 provider，直接向未导出字段 jcodec 写入一个 sentinel
// 值，再调用导出的 SetStatus(StateLoaded)，跳过完整 Initialize() 首次调用。
// 原因：SonicJCodecFiberProvider.Initialize() 首次调用会执行
// GetInstance[JsonWrapper](ctx.GetStarter().GetApplication().GetDefaultTrafficCodecKey())，
// 而 newJSONCodecProviderTestContext 构造的最小 IContext 未调用 RegisterStarterApp，
// ctx.GetStarter() 返回 nil，对 nil 调用 .GetApplication() 会 panic，
// 脱离完整 boot 流程无法安全触发首次初始化路径。
// 本测试文件与被测代码同包（package fiberhouse），可以直接访问未导出字段 jcodec。
// sentinel 使用 jsoncodec.StdJsonDefault()（*jsoncodec.StdJSON）构造——它实现了
// JsonWrapper 接口（Marshal/Unmarshal），是仓库内现成的、可直接构造的具体类型，
// 不代表真实的 Sonic 编解码器，只用于验证"缓存字段被原样返回"这一行为。
// 断言用 assert.Same 验证 Initialize() 在 StateLoaded 分支返回的正是这个
// 预先写入的 sentinel 实例，而不仅仅是"非 nil"，从而确保测试真正验证到
// "重复调用返回同一个之前缓存的实例"这个核心行为，不会因为换一种 nil 判断
// 方式而巧合通过。
func TestSonicJCodecFiberProvider_RepeatedInitializeReturnsSameCodec(t *testing.T) {
	p := NewSonicJCodecFiberProvider()
	ctx := newJSONCodecProviderTestContext(t)

	// 手动写入 sentinel 缓存值并将状态置为 StateLoaded，模拟"已经初始化过一次"
	// 的重入场景，不经过依赖 GetStarter() 的首次初始化路径。
	sentinel := jsoncodec.StdJsonDefault()
	p.jcodec = sentinel
	p.SetStatus(StateLoaded)

	second, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, second, "repeated Initialize() after StateLoaded must not return nil")
	assert.Same(t, sentinel, second, "repeated Initialize() must return the previously cached instance")
}

// TestSonicJCodecGinProvider_RepeatedInitializeReturnsSameCodec
//
// 采用方案：同 TestSonicJCodecFiberProvider_RepeatedInitializeReturnsSameCodec，
// 原因一致：SonicJCodecGinProvider.Initialize() 首次调用同样依赖
// ctx.GetStarter().GetApplication()，最小化测试 context 下 GetStarter() 为 nil 会 panic，
// 无法脱离完整 boot 流程安全触发首次初始化路径。
// 该 provider 的缓存字段类型是 ginJson.Core 接口（要求 Marshal/Unmarshal/
// MarshalIndent/NewEncoder/NewDecoder），仓库内没有可直接构造的具体实现，
// 所有实现（jsonApi/sonicApi/...）都是 gin 包内未导出类型，只能通过包级变量
// ginJson.API 取得——该变量在 gin/codec/json 包 init() 时已被赋值为一个具体
// 实现（默认构建标签下是 encoding/json 版本），因此可以安全地作为 sentinel：
// 它是一个已经存在、非 nil、实现了 ginJson.Core 接口的具体实例。
// 断言用 assert.Same 验证 Initialize() 在 StateLoaded 分支原样返回这个
// sentinel，而不只是判断非 nil，确保测试真正验证到"重复调用返回同一个
// 之前缓存的实例"这个核心行为。
func TestSonicJCodecGinProvider_RepeatedInitializeReturnsSameCodec(t *testing.T) {
	p := NewSonicJCodecGinProvider()
	ctx := newJSONCodecProviderTestContext(t)

	require.NotNil(t, ginJson.API, "ginJson.API must be initialized by the gin codec/json package's own init()")
	sentinel := ginJson.API
	p.jcodec = sentinel
	p.SetStatus(StateLoaded)

	second, err := p.Initialize(ctx)
	require.NoError(t, err)
	require.NotNil(t, second, "repeated Initialize() after StateLoaded must not return nil")
	assert.Same(t, sentinel, second, "repeated Initialize() must return the previously cached instance")
}
