package fiberhouse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readinessChecker struct {
	healthy bool
	calls   atomic.Int32
}

func (c *readinessChecker) IsHealthy() bool {
	c.calls.Add(1)
	return c.healthy
}

func registerReadinessEntry(t *testing.T, gm *globalmanager.GlobalManager, key string, value interface{}) {
	t.Helper()
	require.True(t, gm.Register(key, func() (interface{}, error) { return value, nil }))
	_, err := gm.Get(key)
	require.NoError(t, err)
}

func decodeReadiness(t *testing.T, rec *httptest.ResponseRecorder) ReadinessStatus {
	t.Helper()
	var status ReadinessStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	return status
}

func TestCheckReadiness_ReportsUnhealthyKeysAndTreatsNonCheckersAsHealthy(t *testing.T) {
	gm := globalmanager.NewGlobalManager()
	registerReadinessEntry(t, gm, "healthy", &readinessChecker{healthy: true})
	registerReadinessEntry(t, gm, "plain-object", "no health contract")
	// 未 Get 的 lazy entry 不应被触发初始化，也不应影响就绪判定。
	require.True(t, gm.Register("lazy", func() (interface{}, error) {
		t.Fatal("readiness must not initialize lazy entries")
		return nil, nil
	}))

	status := CheckReadiness(gm)

	assert.True(t, status.Ready)
	assert.Empty(t, status.Unhealthy)

	registerReadinessEntry(t, gm, "broken", &readinessChecker{healthy: false})
	status = CheckReadiness(gm)

	assert.False(t, status.Ready)
	assert.Equal(t, []string{"broken"}, status.Unhealthy)
}

func TestCheckReadiness_NilManagerIsReady(t *testing.T) {
	assert.True(t, CheckReadiness(nil).Ready)
}

func TestNewReadinessHandler_StatusCodesReflectDependencyHealth(t *testing.T) {
	t.Run("ready returns 200", func(t *testing.T) {
		gm := globalmanager.NewGlobalManager()
		registerReadinessEntry(t, gm, "healthy", &readinessChecker{healthy: true})
		rec := httptest.NewRecorder()

		NewReadinessHandler(gm, WithReadinessCacheTTL(0))(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, decodeReadiness(t, rec).Ready)
	})

	t.Run("unhealthy dependency returns 503", func(t *testing.T) {
		gm := globalmanager.NewGlobalManager()
		registerReadinessEntry(t, gm, "broken", &readinessChecker{healthy: false})
		rec := httptest.NewRecorder()

		NewReadinessHandler(gm, WithReadinessCacheTTL(0))(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		status := decodeReadiness(t, rec)
		assert.False(t, status.Ready)
		assert.Equal(t, []string{"broken"}, status.Unhealthy)
	})
}

func TestNewReadinessHandler_CachesResultWithinTTL(t *testing.T) {
	gm := globalmanager.NewGlobalManager()
	checker := &readinessChecker{healthy: true}
	registerReadinessEntry(t, gm, "healthy", checker)
	handler := NewReadinessHandler(gm, WithReadinessCacheTTL(time.Minute))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// 缓存生效时，多次探针请求只应真实探测一次，避免高频探测压垮依赖。
	assert.Equal(t, int32(1), checker.calls.Load())
}

func TestNewReadinessHandler_DisabledCacheProbesEveryRequest(t *testing.T) {
	gm := globalmanager.NewGlobalManager()
	checker := &readinessChecker{healthy: true}
	registerReadinessEntry(t, gm, "healthy", checker)
	handler := NewReadinessHandler(gm, WithReadinessCacheTTL(0))

	for i := 0; i < 3; i++ {
		handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/readyz", nil))
	}

	assert.Equal(t, int32(3), checker.calls.Load())
}
