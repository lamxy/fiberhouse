package cache

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: 采样断言
func sampleRange(t *testing.T, n int, f func() time.Duration, min, max time.Duration) []time.Duration {
	s := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		v := f()
		if v < min || v > max {
			t.Fatalf("TTL sample out of range: %v not in [%v,%v]", v, min, max)
		}
		s = append(s, v)
	}
	return s
}

func newCacheOptionTestContext() fiberhouse.IContext {
	logger := zerolog.Nop()
	return fiberhouse.NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
}

func TestCacheOption_ClonePreservesContextAndOwnsTTLConfigs(t *testing.T) {
	appCtx := newCacheOptionTestContext()
	requestCtx := context.WithValue(context.Background(), struct{}{}, "request")
	original := NewCacheOption(appCtx).
		SetContextCtx(requestCtx).
		Level2().
		SetSyncStrategyAsyncWriteBoth().
		SetCacheKey("key").
		SetLocalTTLWithRandom(time.Minute, time.Second).
		SetRemoteTTLWithRandom(2*time.Minute, 2*time.Second).
		EnableProtectionAll()

	clone := original.Clone()
	defer clone.Release()
	assert.Equal(t, requestCtx, clone.GetContextCtx())
	assert.Same(t, appCtx, clone.GetContext())
	assert.Equal(t, original.GetCacheLevel(), clone.GetCacheLevel())
	assert.Equal(t, original.GetSyncStrategy(), clone.GetSyncStrategy())
	assert.Equal(t, original.GetCacheKey(), clone.GetCacheKey())
	assert.True(t, clone.GetSingleFlightState())
	assert.True(t, clone.GetBloomFilterState())
	assert.True(t, clone.GetCircuitBreakerState())

	clone.SetLocalTTL(time.Second).SetRemoteTTL(2 * time.Second)
	assert.Equal(t, time.Minute, original.GetLocalBaseTTL())
	assert.Equal(t, 2*time.Minute, original.GetRemoteBaseTTL())
}

func TestCacheOption_CloneAllowsContextOverride(t *testing.T) {
	originalCtx := context.Background()
	override := context.WithValue(context.Background(), struct{}{}, "override")
	clone := NewCacheOption(newCacheOptionTestContext()).SetContextCtx(originalCtx).Clone(override)
	defer clone.Release()
	assert.Equal(t, override, clone.GetContextCtx())
}

func TestCacheOption_ResetAndPoolReuseClearRequestState(t *testing.T) {
	appCtx := newCacheOptionTestContext()
	co := OptionPoolGet(appCtx)
	co.SetContextCtx(context.Background()).Level2().SetCacheKey("stale").
		SetLocalTTL(time.Minute).SetRemoteTTL(time.Minute).EnableProtectionAll().DisableCache()
	OptionPoolPut(co)

	reused := OptionPoolGet(appCtx)
	defer OptionPoolPut(reused)
	assert.Same(t, appCtx, reused.GetContext())
	assert.Nil(t, reused.GetContextCtx())
	assert.Empty(t, reused.GetCacheKey())
	assert.Zero(t, reused.GetCacheLevel())
	assert.Equal(t, WriteRemoteOnly, reused.GetSyncStrategy())
	assert.Zero(t, reused.GetLocalBaseTTL())
	assert.Zero(t, reused.GetRemoteBaseTTL())
	assert.False(t, reused.GetSingleFlightState())
	assert.False(t, reused.GetBloomFilterState())
	assert.False(t, reused.GetCircuitBreakerState())
	assert.True(t, reused.IsCache())
}

func TestCacheOption_TTLBoundsAndInvalidRanges(t *testing.T) {
	co := NewCacheOption(nil)
	co.SetLocalTTLRandomPercent(10*time.Second, -1)
	assert.Equal(t, 10*time.Second, co.GetLocalTTL())
	co.SetRemoteTTLRandomPercent(10*time.Second, 2)
	for i := 0; i < 100; i++ {
		assert.Greater(t, co.GetRemoteTTL(), time.Duration(0))
		assert.LessOrEqual(t, co.GetRemoteTTL(), 20*time.Second)
	}
	co.SetLocalTTLWithRandom(5*time.Second, -time.Second)
	assert.Equal(t, 5*time.Second, co.GetLocalTTL())
}

func TestCacheOption_DisabledCacheInvokesLoaderWithConfiguredContext(t *testing.T) {
	appCtx := newCacheOptionTestContext()
	ctx := context.WithValue(context.Background(), struct{}{}, "loader-context")
	co := NewCacheOption(appCtx).Local().SetCacheKey("disabled").SetContextCtx(ctx).DisableCache()

	called := 0
	got, err := GetCached(co, func(gotCtx context.Context) (string, error) {
		called++
		assert.Same(t, ctx, gotCtx)
		return "loaded", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "loaded", got)
	assert.Equal(t, 1, called)
}

// TestCacheOption_LocalTTL_Fixed 固定 TTL
func TestCacheOption_LocalTTL_Fixed(t *testing.T) {
	ctx := context.Background()
	co := NewCacheOption(nil).
		SetContextCtx(ctx).
		Local().
		EnableCache().
		SetLocalTTL(150 * time.Millisecond)

	got := co.GetLocalTTL()
	if got != 150*time.Millisecond {
		t.Fatalf("expect fixed ttl=150ms got=%v", got)
	}
	// 再次获取仍应为同值（固定）
	if got2 := co.GetLocalTTL(); got2 != got {
		t.Fatalf("fixed ttl changed: %v -> %v", got, got2)
	}
}

// TestCacheOption_LocalTTL_RandomRange 绝对值随机范围
func TestCacheOption_LocalTTL_RandomRange(t *testing.T) {
	base := 2 * time.Second
	delta := 600 * time.Millisecond
	minTl := base - delta
	maxTl := base + delta

	co := NewCacheOption(nil).
		Local().
		EnableCache().
		SetLocalTTLWithRandom(base, delta)

	_ = sampleRange(t, 40, co.GetLocalTTL, minTl, maxTl)
}

// TestCacheOption_LocalTTL_RandomPercent 百分比随机
func TestCacheOption_LocalTTL_RandomPercent(t *testing.T) {
	base := 3 * time.Second
	pct := 0.3 // ±30%
	minTl := time.Duration(float64(base) * (1 - pct))
	maxTl := time.Duration(float64(base) * (1 + pct))

	co := NewCacheOption(nil).
		Local().
		EnableCache().
		SetLocalTTLRandomPercent(base, pct)

	samples := sampleRange(t, 60, co.GetLocalTTL, minTl, maxTl)

	// 粗略均值检验（允许 25% 偏差）
	var sum time.Duration
	for _, v := range samples {
		sum += v
	}
	avg := sum / time.Duration(len(samples))
	diff := math.Abs(float64(avg-base)) / float64(base)
	if diff > 0.25 {
		t.Fatalf("avg deviates too much: avg=%v base=%v ratio=%.2f", avg, base, diff)
	}
}

// TestCacheOption_Chaining 方法链应返回同一个实例
func TestCacheOption_Chaining(t *testing.T) {
	co := NewCacheOption(nil)
	after := co.SetContextCtx(context.Background()).
		Local().
		EnableCache().
		SetLocalTTL(10 * time.Millisecond)
	if co != after {
		t.Fatalf("method chaining should return same *CacheOption pointer")
	}
	if ttl := co.GetLocalTTL(); ttl <= 0 {
		t.Fatalf("ttl should be set >0 got=%v", ttl)
	}
}

// TestCacheOption_RemoteTTL_Fixed 固定远程 TTL
func TestCacheOption_RemoteTTL_Fixed(t *testing.T) {
	co := NewCacheOption(nil).
		Remote().
		EnableCache().
		SetRemoteTTL(500 * time.Millisecond)

	got := co.GetRemoteTTL()
	if got != 500*time.Millisecond {
		t.Fatalf("expect fixed remote ttl=500ms got=%v", got)
	}
	// 再次获取仍应为同值（固定）
	if got2 := co.GetRemoteTTL(); got2 != got {
		t.Fatalf("fixed remote ttl changed: %v -> %v", got, got2)
	}
}

// TestCacheOption_RemoteTTL_RandomRange 远程 TTL 绝对值随机范围
func TestCacheOption_RemoteTTL_RandomRange(t *testing.T) {
	base := 4 * time.Second
	delta := 800 * time.Millisecond
	minTl := base - delta
	maxTl := base + delta

	co := NewCacheOption(nil).
		Remote().
		EnableCache().
		SetRemoteTTLWithRandom(base, delta)

	_ = sampleRange(t, 40, co.GetRemoteTTL, minTl, maxTl)
}

// TestCacheOption_RemoteTTL_RandomPercent 远程 TTL 百分比随机
func TestCacheOption_RemoteTTL_RandomPercent(t *testing.T) {
	base := 5 * time.Second
	pct := 0.2 // ±20%
	minTl := time.Duration(float64(base) * (1 - pct))
	maxTl := time.Duration(float64(base) * (1 + pct))

	co := NewCacheOption(nil).
		Remote().
		EnableCache().
		SetRemoteTTLRandomPercent(base, pct)

	samples := sampleRange(t, 60, co.GetRemoteTTL, minTl, maxTl)

	// 粗略均值检验（允许 25% 偏差）
	var sum time.Duration
	for _, v := range samples {
		sum += v
	}
	avg := sum / time.Duration(len(samples))
	diff := math.Abs(float64(avg-base)) / float64(base)
	if diff > 0.25 {
		t.Fatalf("remote avg deviates too much: avg=%v base=%v ratio=%.2f", avg, base, diff)
	}
}
