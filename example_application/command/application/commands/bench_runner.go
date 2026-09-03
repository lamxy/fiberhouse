package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// benchOp 标识一类被压测的 API 操作。
type benchOp string

const (
	opCacheGet benchOp = "cache_get"
	opGet      benchOp = "get"
	opList     benchOp = "list"
	opCreate   benchOp = "create"
	opUpdate   benchOp = "update"
)

// benchOpOrder 固定统计输出的操作顺序，避免 map 迭代顺序导致报告抖动。
var benchOpOrder = []benchOp{opCacheGet, opGet, opList, opCreate, opUpdate}

// benchConfig 是一次压测运行的完整参数。
type benchConfig struct {
	BaseURL  string
	RPS      int
	Duration time.Duration
	Warmup   time.Duration
	Seed     int
	Workers  int
	Timeout  time.Duration
	KeepData bool

	// Weights 为各操作的相对权重，值越大被抽中的概率越高。
	Weights map[benchOp]int

	// RandSeed 固定随机序列，便于复现；0 表示按时间取种子。
	RandSeed int64
}

// defaultBenchWeights 是默认的读写配比：读 80% / 写 20%。
//
// cache_get 占比最高是刻意为之：命中本地缓存时 DB 与网络几乎不参与，
// CPU profile 上剩下的绝大部分是框架自身开销（编解码、校验、中间件、
// 响应对象池），这正是本压测要暴露的目标。
func defaultBenchWeights() map[benchOp]int {
	return map[benchOp]int{
		opCacheGet: 40,
		opGet:      25,
		opList:     15,
		opCreate:   10,
		opUpdate:   10,
	}
}

// validate 校验配置并填充可推导的默认值。
func (c *benchConfig) validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base-url must not be empty")
	}
	if c.RPS <= 0 {
		return fmt.Errorf("rps must be greater than zero")
	}
	if c.Duration <= 0 {
		return fmt.Errorf("duration must be greater than zero")
	}
	if c.Warmup < 0 {
		return fmt.Errorf("warmup must not be negative")
	}
	if c.Seed <= 0 {
		return fmt.Errorf("seed must be greater than zero")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if len(c.Weights) == 0 {
		c.Weights = defaultBenchWeights()
	}
	total := 0
	for _, w := range c.Weights {
		if w < 0 {
			return fmt.Errorf("operation weights must not be negative")
		}
		total += w
	}
	if total == 0 {
		return fmt.Errorf("at least one operation weight must be greater than zero")
	}
	if c.Workers <= 0 {
		// 经验值：worker 数需覆盖 rps × p99 延迟，留足余量避免客户端成为瓶颈
		c.Workers = min(max(c.RPS/10, 16), 512)
	}
	return nil
}

// benchSample 是一次请求的观测结果。
type benchSample struct {
	op      benchOp
	latency time.Duration
	status  int
	err     bool
}

// opStats 汇总单一操作的观测数据。
type opStats struct {
	Op        benchOp
	Count     int
	Errors    int
	Latencies []time.Duration
}

// benchResult 是一次压测运行的完整统计结果。
type benchResult struct {
	Config      benchConfig
	Elapsed     time.Duration
	Total       int
	Errors      int
	Dropped     int64
	StatusCodes map[int]int
	PerOp       map[benchOp]*opStats
	Latencies   []time.Duration
	SeedCreated int
	Cleaned     int
}

// ActualRPS 返回稳态阶段实际达成的请求速率。
func (r *benchResult) ActualRPS() float64 {
	if r.Elapsed <= 0 {
		return 0
	}
	return float64(r.Total) / r.Elapsed.Seconds()
}

// percentile 返回已排序样本的 p 分位数（p 取 0-100）。
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	// 最近秩法：向上取整到第 ceil(p/100 * N) 个样本
	rank := min(max(int(p/100*float64(len(sorted))+0.999999)-1, 0), len(sorted)-1)
	return sorted[rank]
}

// benchClient 封装压测所需的 HTTP 调用。
type benchClient struct {
	baseURL string
	http    *http.Client
}

// newBenchClient 构建压测用 HTTP 客户端。
//
// 必须显式放大 MaxIdleConnsPerHost：其默认值为 2，会让客户端连接复用
// 成为瓶颈，导致测到的是压测工具自身的限制而非服务端性能。
func newBenchClient(baseURL string, workers int, timeout time.Duration) *benchClient {
	maxConns := max(workers*2, 32)
	return &benchClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        maxConns,
				MaxIdleConnsPerHost: maxConns,
				MaxConnsPerHost:     maxConns,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// envelope 是框架统一响应信封 {code,msg,data} 的解析结构。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// createdExample 只提取压测所需的 id 字段。
type createdExample struct {
	ID string `json:"id"`
}

// do 执行一次请求并返回状态码与响应体。
func (c *benchClient) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 必须读完 body 才能复用连接，否则每次请求都会新建连接
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, payload, nil
}

// createExample 创建一条记录并返回其 id。
func (c *benchClient) createExample(ctx context.Context, name string) (string, int, error) {
	body, err := json.Marshal(map[string]any{
		"name":        name,
		"description": "fiberhouse bench seed record",
		"status":      "active",
		"tags":        []string{"bench"},
	})
	if err != nil {
		return "", 0, err
	}
	status, payload, err := c.do(ctx, http.MethodPost, "/examples", body)
	if err != nil {
		return "", status, err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return "", status, fmt.Errorf("create returned status %d: %s", status, truncate(payload, 200))
	}
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return "", status, fmt.Errorf("decode create envelope: %w", err)
	}
	var created createdExample
	if err := json.Unmarshal(env.Data, &created); err != nil {
		return "", status, fmt.Errorf("decode create data: %w", err)
	}
	if created.ID == "" {
		return "", status, fmt.Errorf("create response contained no id")
	}
	return created.ID, status, nil
}

// deleteExample 删除一条记录。
func (c *benchClient) deleteExample(ctx context.Context, id string) error {
	status, payload, err := c.do(ctx, http.MethodDelete, "/examples/"+id, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusOK && status != http.StatusNotFound {
		return fmt.Errorf("delete %s returned status %d: %s", id, status, truncate(payload, 200))
	}
	return nil
}

// truncate 截断响应体用于错误信息，避免超长输出。
func truncate(b []byte, limit int) string {
	if len(b) <= limit {
		return string(b)
	}
	return string(b[:limit]) + "..."
}

// benchRunner 驱动一次完整的压测流程：播种 → 压测 → 清理。
type benchRunner struct {
	cfg    benchConfig
	client *benchClient
	out    io.Writer

	// seedIDs 是播种阶段建立的记录 id，供读写操作复用
	seedIDs []string
	// createdIDs 记录压测期间新建的记录，用于清理
	createdIDs []string
	createdMu  sync.Mutex
}

// newBenchRunner 构建压测执行器。
func newBenchRunner(cfg benchConfig, out io.Writer) *benchRunner {
	return &benchRunner{
		cfg:    cfg,
		client: newBenchClient(cfg.BaseURL, cfg.Workers, cfg.Timeout),
		out:    out,
	}
}

// Run 执行完整压测流程。
func (r *benchRunner) Run(ctx context.Context) (*benchResult, error) {
	if err := r.preflight(ctx); err != nil {
		return nil, err
	}
	if err := r.seed(ctx); err != nil {
		return nil, err
	}

	if r.cfg.Warmup > 0 {
		r.logf("warmup %s (not measured)...", r.cfg.Warmup)
		r.drive(ctx, r.cfg.Warmup, nil)
	}

	r.logf("benchmarking %s at %d rps with %d workers...", r.cfg.Duration, r.cfg.RPS, r.cfg.Workers)
	samples := make(chan benchSample, r.cfg.RPS*2)
	collected := make(chan *benchResult, 1)
	go func() { collected <- r.collect(samples) }()

	start := time.Now()
	dropped := r.drive(ctx, r.cfg.Duration, samples)
	elapsed := time.Since(start)
	close(samples)

	result := <-collected
	result.Config = r.cfg
	result.Elapsed = elapsed
	result.Dropped = dropped
	result.SeedCreated = len(r.seedIDs)

	if !r.cfg.KeepData {
		result.Cleaned = r.cleanup()
	}
	return result, nil
}

// preflight 在播种前确认服务可达，避免空跑一整轮才发现服务没起来。
func (r *benchRunner) preflight(ctx context.Context) error {
	status, _, err := r.client.do(ctx, http.MethodGet, "/examples?page=1&page_size=1", nil)
	if err != nil {
		return fmt.Errorf("preflight failed, is the server running at %s? %w", r.cfg.BaseURL, err)
	}
	if status >= 500 {
		return fmt.Errorf("preflight got status %d from %s; check server and datastore health", status, r.cfg.BaseURL)
	}
	return nil
}

// seed 建立压测所需的种子数据。
func (r *benchRunner) seed(ctx context.Context) error {
	r.logf("seeding %d records...", r.cfg.Seed)
	r.seedIDs = make([]string, 0, r.cfg.Seed)
	for i := 0; i < r.cfg.Seed; i++ {
		id, _, err := r.client.createExample(ctx, fmt.Sprintf("bench-seed-%d-%d", time.Now().UnixNano(), i))
		if err != nil {
			return fmt.Errorf("seed record %d: %w", i, err)
		}
		r.seedIDs = append(r.seedIDs, id)
	}
	if len(r.seedIDs) == 0 {
		return fmt.Errorf("seeding produced no records")
	}
	return nil
}

// drive 以固定速率（开环）注入请求，持续 duration。
//
// 开环模型：按墙钟时间固定速率发送，不等待前一个请求返回。这样服务端变慢时
// 请求会堆积、延迟如实反映；而闭环（固定并发、等回应再发）会在服务端变慢时
// 自动降速，产生协调遗漏（coordinated omission），低估真实延迟。
//
// samples 为 nil 时表示预热阶段，只发请求不记录。
// 返回因 worker 跟不上而被丢弃的节拍数——该值非零意味着测量结果不可信。
func (r *benchRunner) drive(ctx context.Context, duration time.Duration, samples chan<- benchSample) int64 {
	var dropped int64

	jobs := make(chan struct{}, r.cfg.Workers*2)
	var wg sync.WaitGroup
	for i := 0; i < r.cfg.Workers; i++ {
		wg.Add(1)
		// 每个 worker 持有独立的随机源，避免共享 rand 带来的锁争用干扰被测系统
		rng := rand.New(rand.NewSource(r.randSeed() + int64(i)))
		go func() {
			defer wg.Done()
			for range jobs {
				sample := r.executeOne(ctx, rng)
				if samples != nil {
					samples <- sample
				}
			}
		}()
	}

	interval := time.Duration(float64(time.Second) / float64(r.cfg.RPS))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.After(duration)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-deadline:
			break loop
		case <-ticker.C:
			select {
			case jobs <- struct{}{}:
			default:
				// worker 全忙，无法维持目标速率；记录并继续，报告时提示
				atomic.AddInt64(&dropped, 1)
			}
		}
	}

	close(jobs)
	wg.Wait()
	return dropped
}

// randSeed 返回随机源种子。
func (r *benchRunner) randSeed() int64 {
	if r.cfg.RandSeed != 0 {
		return r.cfg.RandSeed
	}
	return time.Now().UnixNano()
}

// executeOne 按权重抽取一个操作并执行。
func (r *benchRunner) executeOne(ctx context.Context, rng *rand.Rand) benchSample {
	op := r.pickOp(rng)
	start := time.Now()
	status, err := r.perform(ctx, op, rng)
	return benchSample{
		op:      op,
		latency: time.Since(start),
		status:  status,
		err:     err != nil,
	}
}

// pickOp 按配置权重随机选取一个操作。
func (r *benchRunner) pickOp(rng *rand.Rand) benchOp {
	total := 0
	for _, op := range benchOpOrder {
		total += r.cfg.Weights[op]
	}
	if total <= 0 {
		return opGet
	}
	n := rng.Intn(total)
	for _, op := range benchOpOrder {
		w := r.cfg.Weights[op]
		if n < w {
			return op
		}
		n -= w
	}
	return opGet
}

// perform 执行指定操作对应的 HTTP 请求。
func (r *benchRunner) perform(ctx context.Context, op benchOp, rng *rand.Rand) (int, error) {
	switch op {
	case opCacheGet:
		status, _, err := r.client.do(ctx, http.MethodGet, "/examples/cache/"+r.randomSeedID(rng), nil)
		return status, errIfBadStatus(status, err)
	case opGet:
		status, _, err := r.client.do(ctx, http.MethodGet, "/examples/"+r.randomSeedID(rng), nil)
		return status, errIfBadStatus(status, err)
	case opList:
		page := rng.Intn(5) + 1
		path := fmt.Sprintf("/examples?page=%d&page_size=20", page)
		status, _, err := r.client.do(ctx, http.MethodGet, path, nil)
		return status, errIfBadStatus(status, err)
	case opCreate:
		id, status, err := r.client.createExample(ctx, fmt.Sprintf("bench-%d", rng.Int63()))
		if err == nil && id != "" {
			r.trackCreated(id)
		}
		return status, err
	case opUpdate:
		body, mErr := json.Marshal(map[string]any{
			"description": fmt.Sprintf("bench update %d", rng.Int63()),
		})
		if mErr != nil {
			return 0, mErr
		}
		status, _, err := r.client.do(ctx, http.MethodPut, "/examples/"+r.randomSeedID(rng), body)
		return status, errIfBadStatus(status, err)
	default:
		return 0, fmt.Errorf("unknown operation %q", op)
	}
}

// errIfBadStatus 把 4xx/5xx 视为错误，便于统计区分。
func errIfBadStatus(status int, err error) error {
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("status %d", status)
	}
	return nil
}

// randomSeedID 从种子池中随机取一个 id。
func (r *benchRunner) randomSeedID(rng *rand.Rand) string {
	if len(r.seedIDs) == 0 {
		return ""
	}
	return r.seedIDs[rng.Intn(len(r.seedIDs))]
}

// trackCreated 记录压测期间新建的 id，供清理阶段删除。
func (r *benchRunner) trackCreated(id string) {
	r.createdMu.Lock()
	r.createdIDs = append(r.createdIDs, id)
	r.createdMu.Unlock()
}

// collect 汇总样本流并生成统计结果。
func (r *benchRunner) collect(samples <-chan benchSample) *benchResult {
	result := &benchResult{
		StatusCodes: make(map[int]int),
		PerOp:       make(map[benchOp]*opStats),
	}
	for _, op := range benchOpOrder {
		result.PerOp[op] = &opStats{Op: op}
	}

	for s := range samples {
		result.Total++
		if s.err {
			result.Errors++
		}
		if s.status > 0 {
			result.StatusCodes[s.status]++
		}
		result.Latencies = append(result.Latencies, s.latency)

		stats := result.PerOp[s.op]
		if stats == nil {
			stats = &opStats{Op: s.op}
			result.PerOp[s.op] = stats
		}
		stats.Count++
		if s.err {
			stats.Errors++
		}
		stats.Latencies = append(stats.Latencies, s.latency)
	}

	sortDurations(result.Latencies)
	for _, stats := range result.PerOp {
		sortDurations(stats.Latencies)
	}
	return result
}

// sortDurations 就地升序排序。
func sortDurations(values []time.Duration) {
	slices.Sort(values)
}

// cleanup 删除播种与压测期间建立的全部记录，返回删除成功的条数。
//
// 刻意不接收调用方 context：压测 context 可能已因超时或用户中断而取消，
// 而清理仍应尽力完成以免数据残留，因此内部使用独立的超时 context。
func (r *benchRunner) cleanup() int {
	r.createdMu.Lock()
	ids := make([]string, 0, len(r.seedIDs)+len(r.createdIDs))
	ids = append(ids, r.seedIDs...)
	ids = append(ids, r.createdIDs...)
	r.createdMu.Unlock()

	r.logf("cleaning up %d records...", len(ids))

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cleaned := 0
	for _, id := range ids {
		if err := r.client.deleteExample(cleanupCtx, id); err == nil {
			cleaned++
		}
	}
	return cleaned
}

// logf 输出进度信息。
func (r *benchRunner) logf(format string, args ...any) {
	if r.out == nil {
		return
	}
	_, _ = fmt.Fprintf(r.out, format+"\n", args...)
}
