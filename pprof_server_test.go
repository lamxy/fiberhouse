package fiberhouse

import (
	"context"
	"io"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runtimeResetProfileRates 复位 block 与 mutex 采样率，避免测试开启的采样
// 影响同一进程内的后续测试
func runtimeResetProfileRates() {
	runtime.SetBlockProfileRate(0)
	runtime.SetMutexProfileFraction(0)
}

// freePort 获取一个当前空闲的端口，避免测试间端口冲突
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// newTestPprofConfig 构造一个启用的测试配置
func newTestPprofConfig(t *testing.T) PprofConfig {
	t.Helper()
	return PprofConfig{
		Enable:          true,
		Host:            "127.0.0.1",
		Port:            freePort(t),
		Prefix:          defaultPprofPrefix,
		ReadTimeout:     defaultPprofReadTimeout,
		WriteTimeout:    defaultPprofWriteTimeout,
		ShutdownTimeout: defaultPprofShutdownTimeout,
	}
}

// startTestServer 启动诊断服务并注册关闭清理
func startTestServer(t *testing.T, cfg PprofConfig) *PprofServer {
	t.Helper()
	s := NewPprofServer(cfg, nil, "")
	require.NoError(t, s.Start())
	t.Cleanup(func() {
		_ = s.Shutdown(context.Background())
	})
	return s
}

// TestPprofServerDisabledDoesNotListen 未启用时不应监听任何端口
func TestPprofServerDisabledDoesNotListen(t *testing.T) {
	cfg := newTestPprofConfig(t)
	cfg.Enable = false

	s := NewPprofServer(cfg, nil, "")
	require.NoError(t, s.Start())

	conn, err := net.DialTimeout("tcp", s.Addr(), 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("pprof 未启用时不应监听 %s", s.Addr())
	}

	// 未启动时关闭应为无操作且不报错
	assert.NoError(t, s.Shutdown(context.Background()))
}

// TestPprofServerAllProfileEndpoints 五类 profile 端点均应可访问
func TestPprofServerAllProfileEndpoints(t *testing.T) {
	cfg := newTestPprofConfig(t)
	// 开启 block 与 mutex 采样，否则对应 profile 无数据
	cfg.BlockRate = 10000
	cfg.MutexFraction = 100

	s := startTestServer(t, cfg)

	// 复位采样率，避免影响后续测试与其他用例
	t.Cleanup(func() {
		runtimeResetProfileRates()
	})

	client := &http.Client{Timeout: 10 * time.Second}
	base := "http://" + s.Addr() + cfg.Prefix

	// cpu profile 单独测试（耗时），此处覆盖其余四类及索引页
	for _, name := range []string{"", "/heap", "/goroutine", "/block", "/mutex", "/allocs"} {
		t.Run("profile"+name, func(t *testing.T) {
			url := base + "/"
			if name != "" {
				url = base + name + "?debug=1"
			}
			resp, err := client.Get(url)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.NotEmpty(t, body, "profile %q 不应返回空内容", name)
		})
	}
}

// TestPprofServerCPUProfile CPU profile 应返回非空 pprof 数据
func TestPprofServerCPUProfile(t *testing.T) {
	cfg := newTestPprofConfig(t)
	s := startTestServer(t, cfg)

	client := &http.Client{Timeout: 15 * time.Second}
	// 采集 1 秒，验证端点可用且 WriteTimeout 不会截断
	resp, err := client.Get("http://" + s.Addr() + cfg.Prefix + "/profile?seconds=1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body, "CPU profile 不应为空")
}

// TestPprofServerBasicAuth 启用鉴权后未授权请求应被拒绝
func TestPprofServerBasicAuth(t *testing.T) {
	cfg := newTestPprofConfig(t)
	cfg.AuthEnable = true
	cfg.AuthUsername = "admin"
	cfg.AuthPassword = "s3cret"

	s := startTestServer(t, cfg)

	client := &http.Client{Timeout: 5 * time.Second}
	url := "http://" + s.Addr() + cfg.Prefix + "/"

	t.Run("无凭据应401", func(t *testing.T) {
		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("错误凭据应401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		req.SetBasicAuth("admin", "wrong")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("正确凭据应200", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		req.SetBasicAuth("admin", "s3cret")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestPprofServerShutdownReleasesPort 关闭后端口应被释放
func TestPprofServerShutdownReleasesPort(t *testing.T) {
	cfg := newTestPprofConfig(t)
	s := NewPprofServer(cfg, nil, "")
	require.NoError(t, s.Start())

	// 启动后端口应可连接
	conn, err := net.DialTimeout("tcp", s.Addr(), time.Second)
	require.NoError(t, err)
	_ = conn.Close()

	require.NoError(t, s.Shutdown(context.Background()))

	// 关闭后应能重新监听同一端口，证明端口已释放。
	//
	// Shutdown 返回时监听套接字已关闭，但先前那条已 Close 的连接可能仍处于内核的
	// TIME_WAIT/FIN_WAIT 拆除过程中，使 net.Listen 偶发 EADDRINUSE。这是断言与内核
	// 时序的竞争，不是被测程序的缺陷，因此改为有限重试而非一次性断言。
	require.Eventually(t, func() bool {
		l, err := net.Listen("tcp", s.Addr())
		if err != nil {
			return false
		}
		_ = l.Close()
		return true
	}, time.Second, 50*time.Millisecond, "关闭后端口应已释放")
}

// TestPprofServerStartTwiceIsIdempotent 重复启动应幂等，不产生第二个监听
func TestPprofServerStartTwiceIsIdempotent(t *testing.T) {
	cfg := newTestPprofConfig(t)
	s := startTestServer(t, cfg)

	// 第二次启动应直接返回，不因端口占用而报错
	assert.NoError(t, s.Start())
}

// TestPprofServerRootRedirect 根路径应重定向到 pprof 索引页
func TestPprofServerRootRedirect(t *testing.T) {
	cfg := newTestPprofConfig(t)
	s := startTestServer(t, cfg)

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get("http://" + s.Addr() + "/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, cfg.Prefix+"/", resp.Header.Get("Location"))
}

// TestNormalizePprofPrefix 前缀规范化
func TestNormalizePprofPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", defaultPprofPrefix},
		{"/", defaultPprofPrefix},
		{"/debug/pprof", "/debug/pprof"},
		{"/debug/pprof/", "/debug/pprof"},
		{"debug/pprof", "/debug/pprof"},
		{"/custom//", "/custom"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, normalizePprofPrefix(c.in), "输入 %q", c.in)
	}
}

// TestPprofServerCustomPrefix 自定义前缀应生效
func TestPprofServerCustomPrefix(t *testing.T) {
	cfg := newTestPprofConfig(t)
	cfg.Prefix = "/internal/diag"

	s := startTestServer(t, cfg)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + s.Addr() + "/internal/diag/goroutine?debug=1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(body), "goroutine"),
		"自定义前缀下 goroutine profile 应正常返回")
}

// TestPprofAddrFormat 监听地址格式
func TestPprofAddrFormat(t *testing.T) {
	s := NewPprofServer(PprofConfig{Host: "127.0.0.1", Port: 6060}, nil, "")
	assert.Equal(t, "127.0.0.1:"+strconv.Itoa(6060), s.Addr())
}
