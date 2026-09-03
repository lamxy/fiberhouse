package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeExampleServer 是一个最小的 example CRUD 假服务，用于在不依赖
// MongoDB 与真实 web 服务的前提下验证压测器行为。
type fakeExampleServer struct {
	mu      sync.Mutex
	records map[string]bool
	nextID  int64

	hits    map[string]int64
	deletes atomic.Int64

	// latency 为每个请求注入的人为延迟
	latency time.Duration
}

func newFakeExampleServer() *fakeExampleServer {
	return &fakeExampleServer{
		records: make(map[string]bool),
		hits:    make(map[string]int64),
	}
}

func (s *fakeExampleServer) record(key string) {
	s.mu.Lock()
	s.hits[key]++
	s.mu.Unlock()
}

func (s *fakeExampleServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/examples", func(w http.ResponseWriter, r *http.Request) {
		if s.latency > 0 {
			time.Sleep(s.latency)
		}
		switch r.Method {
		case http.MethodPost:
			s.record("create")
			s.mu.Lock()
			s.nextID++
			// 模拟 ObjectID：24 位十六进制，满足 alphanum/min=18/max=32 约束
			id := fmt.Sprintf("%024x", s.nextID)
			s.records[id] = true
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok",
				"data": map[string]any{"id": id, "name": "x"},
			})
		case http.MethodGet:
			s.record("list")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok",
				"data": map[string]any{"items": []any{}, "total": 0},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/examples/", func(w http.ResponseWriter, r *http.Request) {
		if s.latency > 0 {
			time.Sleep(s.latency)
		}
		path := strings.TrimPrefix(r.URL.Path, "/examples/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "cache/"):
			s.record("cache_get")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": map[string]any{"id": strings.TrimPrefix(path, "cache/")}})
		case r.Method == http.MethodGet:
			s.record("get")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": map[string]any{"id": path}})
		case r.Method == http.MethodPut:
			s.record("update")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": map[string]any{"id": path}})
		case r.Method == http.MethodDelete:
			s.deletes.Add(1)
			s.mu.Lock()
			delete(s.records, path)
			s.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

// newTestBenchConfig 返回一个跑得很快的压测配置。
func newTestBenchConfig(baseURL string) benchConfig {
	return benchConfig{
		BaseURL:  baseURL,
		RPS:      200,
		Duration: 300 * time.Millisecond,
		Warmup:   0,
		Seed:     5,
		Workers:  8,
		Timeout:  2 * time.Second,
		RandSeed: 42,
		Weights:  defaultBenchWeights(),
	}
}

func TestBenchRunnerSeedsBenchmarksAndCleansUp(t *testing.T) {
	fake := newFakeExampleServer()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	cfg := newTestBenchConfig(srv.URL)
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	var out bytes.Buffer
	runner := newBenchRunner(cfg, &out)
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if result.SeedCreated != cfg.Seed {
		t.Fatalf("seeded = %d, want %d", result.SeedCreated, cfg.Seed)
	}
	if result.Total == 0 {
		t.Fatal("no requests were recorded")
	}
	if result.Errors != 0 {
		t.Fatalf("unexpected errors: %d", result.Errors)
	}

	// 清理必须删除种子 + 压测期间新建的全部记录
	fake.mu.Lock()
	remaining := len(fake.records)
	fake.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d records left behind after cleanup", remaining)
	}
	if result.Cleaned < cfg.Seed {
		t.Fatalf("cleaned = %d, want at least %d", result.Cleaned, cfg.Seed)
	}
}

func TestBenchRunnerKeepDataSkipsCleanup(t *testing.T) {
	fake := newFakeExampleServer()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	cfg := newTestBenchConfig(srv.URL)
	cfg.KeepData = true
	cfg.Duration = 100 * time.Millisecond
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	result, err := newBenchRunner(cfg, io.Discard).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Cleaned != 0 {
		t.Fatalf("cleaned = %d, want 0 when --keep-data is set", result.Cleaned)
	}

	fake.mu.Lock()
	remaining := len(fake.records)
	fake.mu.Unlock()
	if remaining == 0 {
		t.Fatal("records were deleted despite --keep-data")
	}
}

func TestBenchRunnerRespectsOperationWeights(t *testing.T) {
	fake := newFakeExampleServer()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	cfg := newTestBenchConfig(srv.URL)
	cfg.Duration = 600 * time.Millisecond
	// 只允许 cache_get，其余权重归零
	cfg.Weights = map[benchOp]int{opCacheGet: 1}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	result, err := newBenchRunner(cfg, io.Discard).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := result.PerOp[opCacheGet].Count; got != result.Total {
		t.Fatalf("cache_get = %d, want all %d requests", got, result.Total)
	}
	for _, op := range []benchOp{opGet, opList, opCreate, opUpdate} {
		if c := result.PerOp[op].Count; c != 0 {
			t.Fatalf("%s = %d, want 0 when its weight is zero", op, c)
		}
	}
}

func TestBenchRunnerPreflightFailsFastWhenServerDown(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	addr := srv.URL
	srv.Close() // 立即关闭，模拟服务未启动

	cfg := newTestBenchConfig(addr)
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	_, err := newBenchRunner(cfg, io.Discard).Run(context.Background())
	if err == nil {
		t.Fatal("expected preflight to fail when the server is unreachable")
	}
	if !strings.Contains(err.Error(), "preflight failed") {
		t.Fatalf("error = %v, want a preflight failure", err)
	}
}

func TestBenchRunnerCountsErrorStatuses(t *testing.T) {
	// failing 打开后，所有请求一律返回 500；在此之前放行 preflight 与播种，
	// 以便隔离验证「稳态期间全部失败」的统计路径。
	var failing atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/examples", func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok",
				"data": map[string]any{"id": "0123456789abcdef01234567"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": map[string]any{}})
	})
	mux.HandleFunc("/examples/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := newTestBenchConfig(srv.URL)
	cfg.Seed = 1
	cfg.Duration = 200 * time.Millisecond
	cfg.KeepData = true
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	runner := newBenchRunner(cfg, io.Discard)
	if err := runner.preflight(context.Background()); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if err := runner.seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// 播种完成后再让服务端全面失败，隔离验证稳态期的错误统计
	failing.Store(true)

	samples := make(chan benchSample, 1024)
	done := make(chan *benchResult, 1)
	go func() { done <- runner.collect(samples) }()
	runner.drive(context.Background(), cfg.Duration, samples)
	close(samples)
	result := <-done

	if result.Total == 0 {
		t.Fatal("no requests recorded")
	}
	if result.Errors != result.Total {
		t.Fatalf("errors = %d, want all %d requests to fail", result.Errors, result.Total)
	}
	if result.StatusCodes[http.StatusInternalServerError] == 0 {
		t.Fatal("500 responses were not counted")
	}
}

func TestBenchConfigValidateDerivesWorkers(t *testing.T) {
	cases := []struct {
		rps  int
		want int
	}{
		{rps: 50, want: 16},    // 下限钳制
		{rps: 500, want: 50},   // rps/10
		{rps: 90000, want: 512}, // 上限钳制
	}
	for _, c := range cases {
		cfg := benchConfig{
			BaseURL: "http://x", RPS: c.rps, Duration: time.Second,
			Seed: 1, Timeout: time.Second,
		}
		if err := cfg.validate(); err != nil {
			t.Fatalf("rps=%d validate: %v", c.rps, err)
		}
		if cfg.Workers != c.want {
			t.Fatalf("rps=%d workers = %d, want %d", c.rps, cfg.Workers, c.want)
		}
	}
}

func TestBenchConfigValidateRejectsBadInput(t *testing.T) {
	base := func() benchConfig {
		return benchConfig{BaseURL: "http://x", RPS: 10, Duration: time.Second, Seed: 1, Timeout: time.Second}
	}
	cases := map[string]func(*benchConfig){
		"empty base url":  func(c *benchConfig) { c.BaseURL = "" },
		"zero rps":        func(c *benchConfig) { c.RPS = 0 },
		"zero duration":   func(c *benchConfig) { c.Duration = 0 },
		"zero seed":       func(c *benchConfig) { c.Seed = 0 },
		"zero timeout":    func(c *benchConfig) { c.Timeout = 0 },
		"negative warmup": func(c *benchConfig) { c.Warmup = -time.Second },
		"all zero weight": func(c *benchConfig) { c.Weights = map[benchOp]int{opGet: 0} },
		"negative weight": func(c *benchConfig) { c.Weights = map[benchOp]int{opGet: -1} },
	}
	for name, mutate := range cases {
		cfg := base()
		mutate(&cfg)
		if err := cfg.validate(); err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
	}
}

func TestPercentile(t *testing.T) {
	samples := make([]time.Duration, 100)
	for i := range samples {
		samples[i] = time.Duration(i+1) * time.Millisecond
	}
	cases := []struct {
		p    float64
		want time.Duration
	}{
		{50, 50 * time.Millisecond},
		{90, 90 * time.Millisecond},
		{99, 99 * time.Millisecond},
		{100, 100 * time.Millisecond},
		{0, 1 * time.Millisecond},
	}
	for _, c := range cases {
		if got := percentile(samples, c.p); got != c.want {
			t.Fatalf("p%.0f = %v, want %v", c.p, got, c.want)
		}
	}
	if got := percentile(nil, 50); got != 0 {
		t.Fatalf("percentile of empty = %v, want 0", got)
	}
}

func TestWriteBenchReportIncludesKeyFields(t *testing.T) {
	result := &benchResult{
		Config:      benchConfig{BaseURL: "http://x", RPS: 100},
		Elapsed:     2 * time.Second,
		Total:       200,
		Errors:      2,
		Dropped:     7,
		StatusCodes: map[int]int{200: 198, 500: 2},
		PerOp: map[benchOp]*opStats{
			opGet: {Op: opGet, Count: 200, Errors: 2, Latencies: []time.Duration{time.Millisecond}},
		},
		Latencies:   []time.Duration{time.Millisecond, 2 * time.Millisecond},
		SeedCreated: 10,
		Cleaned:     10,
	}
	var out bytes.Buffer
	if err := writeBenchReport(&out, result); err != nil {
		t.Fatalf("report: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Bench Result", "actual rps", "200", "1.00%",
		"WARNING", "ticks dropped", "by operation", "get", "status codes", "200:198", "500:2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q:\n%s", want, got)
		}
	}
}

func TestWriteBenchReportOmitsWarningWhenNoDrops(t *testing.T) {
	result := &benchResult{
		Config:      benchConfig{BaseURL: "http://x", RPS: 100},
		Elapsed:     time.Second,
		Total:       10,
		StatusCodes: map[int]int{200: 10},
		PerOp:       map[benchOp]*opStats{},
		Latencies:   []time.Duration{time.Millisecond},
	}
	var out bytes.Buffer
	if err := writeBenchReport(&out, result); err != nil {
		t.Fatalf("report: %v", err)
	}
	if strings.Contains(out.String(), "WARNING") {
		t.Fatalf("unexpected warning with zero drops:\n%s", out.String())
	}
}
