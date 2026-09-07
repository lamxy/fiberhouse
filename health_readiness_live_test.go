//go:build liveintegration

// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache/cacheremote"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// tcpProxy 在测试与真实 Redis 容器之间转发 TCP 流量，可以随时切断。
//
// 这是让就绪探针观察到真实依赖故障的关键：直接停容器会影响并行运行的其他 live
// 测试，而代理只切断本用例自己的连接，故障注入范围可控且可恢复。
type tcpProxy struct {
	listener net.Listener
	upstream string

	mu      sync.Mutex
	blocked bool
	conns   []net.Conn
	wg      sync.WaitGroup
}

func newTCPProxy(t *testing.T, upstream string) *tcpProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxy := &tcpProxy{listener: listener, upstream: upstream}
	proxy.wg.Add(1)
	go proxy.serve()
	t.Cleanup(proxy.close)
	return proxy
}

func (p *tcpProxy) addr() string {
	return p.listener.Addr().String()
}

func (p *tcpProxy) serve() {
	defer p.wg.Done()
	for {
		client, err := p.listener.Accept()
		if err != nil {
			return
		}

		p.mu.Lock()
		blocked := p.blocked
		if !blocked {
			p.conns = append(p.conns, client)
		}
		p.mu.Unlock()

		// 故障注入期间：立即拒绝新连接，模拟依赖不可达。
		if blocked {
			_ = client.Close()
			continue
		}

		upstream, err := net.DialTimeout("tcp", p.upstream, 2*time.Second)
		if err != nil {
			_ = client.Close()
			continue
		}

		p.mu.Lock()
		p.conns = append(p.conns, upstream)
		p.mu.Unlock()

		go func() { _, _ = copyConn(upstream, client) }()
		go func() { _, _ = copyConn(client, upstream) }()
	}
}

// block 切断当前所有连接并拒绝后续连接，模拟依赖宕机。
func (p *tcpProxy) block() {
	p.mu.Lock()
	p.blocked = true
	conns := p.conns
	p.conns = nil
	p.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

// unblock 恢复转发，模拟依赖恢复。
func (p *tcpProxy) unblock() {
	p.mu.Lock()
	p.blocked = false
	p.mu.Unlock()
}

func (p *tcpProxy) close() {
	_ = p.listener.Close()
	p.block()
	p.wg.Wait()
}

func copyConn(dst, src net.Conn) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			written, werr := dst.Write(buf[:n])
			total += int64(written)
			if werr != nil {
				return total, werr
			}
		}
		if err != nil {
			return total, err
		}
	}
}

// probe 发起一次就绪探测，返回状态码与解析后的响应体。
func probe(t *testing.T, handler http.HandlerFunc) (int, fiberhouse.ReadinessStatus) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var status fiberhouse.ReadinessStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	return rec.Code, status
}

// TestLive_Readiness_ReflectsRealDependencyOutage 端到端验证就绪探针对真实依赖
// 故障的反应：依赖正常返回 200，依赖中断返回 503，依赖恢复后重新返回 200。
//
// 为什么需要这个用例：保活改为健康检查后，「依赖异常时由编排系统摘除流量」是这套
// 机制的核心价值主张，而既有测试全部使用假的 HealthChecker——它们证明的是
// CheckReadiness 会转发 IsHealthy 的返回值，不是真实依赖断开时 IsHealthy 真的会
// 变 false。这两者之间隔着 go-redis 的连接池与重试行为，只能用真实连接验证。
//
// 故障注入通过本地 TCP 代理完成，而不是停掉容器：代理只切断本用例自己的连接，
// 不影响并行运行的其他 live 测试，且可以精确控制恢复时机。
func TestLive_Readiness_ReflectsRealDependencyOutage(t *testing.T) {
	proxy := newTCPProxy(t, "127.0.0.1:6379")

	host, port, err := net.SplitHostPort(proxy.addr())
	require.NoError(t, err)

	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"live.redis.host":     host,
		"live.redis.port":     port,
		"live.redis.db":       13, // 独立于其他 live 测试使用的 db
		"live.redis.poolSize": 4,
	})
	logger := zerolog.Nop()
	appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))

	origin, err := cacheremote.NewRedisDb(appCtx, "live.redis")
	require.NoError(t, err)
	redis := origin.(*cacheremote.RedisDb)
	t.Cleanup(func() { _ = redis.Close() })

	gm := globalmanager.NewGlobalManager()
	const key = "readiness-live-redis"
	require.True(t, gm.Register(key, func() (interface{}, error) { return redis, nil }))
	_, err = gm.Get(key)
	require.NoError(t, err)

	// 关闭缓存：本用例要观察状态变化，缓存会掩盖它。缓存行为本身由
	// TestNewReadinessHandler_CachesResultWithinTTL 覆盖。
	handler := fiberhouse.NewReadinessHandler(gm, fiberhouse.WithReadinessCacheTTL(0))

	// ① 依赖正常：探针必须返回 200。
	code, status := probe(t, handler)
	require.Equal(t, http.StatusOK, code, "probe must report ready while Redis is reachable")
	require.True(t, status.Ready)
	require.Empty(t, status.Unhealthy)

	// ② 依赖中断：探针必须转为 503，并点名不健康的 key。
	proxy.block()
	code, status = probe(t, handler)
	require.Equal(t, http.StatusServiceUnavailable, code,
		"probe must report not-ready once Redis becomes unreachable")
	require.False(t, status.Ready)
	require.Equal(t, []string{key}, status.Unhealthy,
		"probe must name the unhealthy dependency so operators can locate it")

	// ③ 依赖恢复：探针必须重新返回 200。
	//
	// 这一步才真正验证了移除 Rebuilder 的论证：框架不做任何重建，恢复完全依赖
	// go-redis 连接池丢弃坏连接并重新拨号。若该论证不成立，探针会一直停在 503。
	proxy.unblock()
	require.Eventually(t, func() bool {
		code, status := probe(t, handler)
		return code == http.StatusOK && status.Ready
	}, 30*time.Second, 250*time.Millisecond,
		"probe must recover to ready after the dependency comes back, without any framework-side rebuild")
}

// TestLive_Readiness_UnhealthyDependencyDoesNotMaskHealthyOnes 确认单个依赖故障
// 只会点名该依赖，不影响其余依赖的判定。
func TestLive_Readiness_UnhealthyDependencyDoesNotMaskHealthyOnes(t *testing.T) {
	brokenProxy := newTCPProxy(t, "127.0.0.1:6379")
	healthyProxy := newTCPProxy(t, "127.0.0.1:6379")

	newRedis := func(t *testing.T, addr string, db int) *cacheremote.RedisDb {
		t.Helper()
		host, port, err := net.SplitHostPort(addr)
		require.NoError(t, err)
		cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
			"live.redis.host":     host,
			"live.redis.port":     port,
			"live.redis.db":       db,
			"live.redis.poolSize": 2,
		})
		logger := zerolog.Nop()
		appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
		origin, err := cacheremote.NewRedisDb(appCtx, "live.redis")
		require.NoError(t, err)
		instance := origin.(*cacheremote.RedisDb)
		t.Cleanup(func() { _ = instance.Close() })
		return instance
	}

	gm := globalmanager.NewGlobalManager()
	entries := map[string]*cacheremote.RedisDb{
		"redis-broken": newRedis(t, brokenProxy.addr(), 12),
		"redis-ok":     newRedis(t, healthyProxy.addr(), 11),
	}
	for key, instance := range entries {
		value := instance
		require.True(t, gm.Register(key, func() (interface{}, error) { return value, nil }))
		_, err := gm.Get(key)
		require.NoError(t, err)
	}

	handler := fiberhouse.NewReadinessHandler(gm, fiberhouse.WithReadinessCacheTTL(0))

	code, status := probe(t, handler)
	require.Equal(t, http.StatusOK, code, "both dependencies start reachable")
	require.True(t, status.Ready)

	brokenProxy.block()
	code, status = probe(t, handler)
	require.Equal(t, http.StatusServiceUnavailable, code)
	require.False(t, status.Ready)
	require.Equal(t, []string{"redis-broken"}, status.Unhealthy,
		fmt.Sprintf("only the broken dependency may be listed, got %v", status.Unhealthy))
}
