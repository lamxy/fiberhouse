package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/urfave/cli/v2"
)

// benchCommand 为「bench」CLI 命令组实现 fiberhouse.CommandGetter。
//
// 该命令对运行中的 web 服务施加固定速率的 HTTP 负载，用于在压测窗口内
// 采集 pprof 性能剖析数据。它刻意只依赖标准库 net/http，不引入压测框架。
type benchCommand struct{}

// NewBenchCommand 构建「bench」CLI 命令。
//
// 压测通过 HTTP 打到运行中的服务，不复用 command-module 的 service 层：
// web API 走 MongoDB 栈，而 command-module 走 MySQL 栈，两者数据不互通。
func NewBenchCommand(fiberhouse.ICommandContext) fiberhouse.CommandGetter {
	return &benchCommand{}
}

// GetCommand 返回「bench」命令的 urfave/cli 定义。
func (c *benchCommand) GetCommand() interface{} {
	return &cli.Command{
		Name:  "bench",
		Usage: "load-test the example CRUD API while collecting pprof profiles",
		Description: strings.TrimSpace(`
Drives the example CRUD API at a fixed request rate for a preset duration so
that CPU, heap, goroutine, block and mutex profiles can be captured against a
steady load.

The load is open-loop: requests are emitted on a wall-clock schedule rather
than waiting for the previous response. This avoids coordinated omission,
where a slowing server causes a closed-loop client to back off and thereby
hide the very latency being measured.

Read operations dominate by default (80/20), and cache_get carries the largest
share on purpose: when it hits the local cache the datastore barely
participates, so framework overhead (codec, validation, middleware, response
pooling) is what remains visible in the CPU profile.

Example:
  cmdgo bench --rps=500 --duration=60s --seed=1000

Then, in another shell, capture a profile mid-run:
  go tool pprof -http=:8000 http://127.0.0.1:6060/debug/pprof/profile?seconds=30`),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "base-url",
				Value: "http://127.0.0.1:8080",
				Usage: "base URL of the running web service",
			},
			&cli.IntFlag{
				Name:  "rps",
				Value: 500,
				Usage: "target requests per second (open-loop injection rate)",
			},
			&cli.DurationFlag{
				Name:  "duration",
				Value: 60 * time.Second,
				Usage: "how long to sustain the load, excluding warmup",
			},
			&cli.DurationFlag{
				Name:  "warmup",
				Value: 5 * time.Second,
				Usage: "warmup period before measuring; fills pools and caches",
			},
			&cli.IntFlag{
				Name:  "seed",
				Value: 1000,
				Usage: "number of records to create before the run",
			},
			&cli.IntFlag{
				Name:  "workers",
				Value: 0,
				Usage: "concurrent senders; 0 derives it from --rps (rps/10, clamped to 16..512)",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 5 * time.Second,
				Usage: "per-request timeout",
			},
			&cli.BoolFlag{
				Name:  "keep-data",
				Usage: "keep seeded and created records instead of deleting them",
			},
			&cli.Int64Flag{
				Name:  "rand-seed",
				Value: 0,
				Usage: "fix the random sequence for reproducible runs; 0 uses the clock",
			},
			&cli.IntFlag{Name: "w-cache-get", Value: 40, Usage: "weight of GET /examples/cache/{id}"},
			&cli.IntFlag{Name: "w-get", Value: 25, Usage: "weight of GET /examples/{id}"},
			&cli.IntFlag{Name: "w-list", Value: 15, Usage: "weight of GET /examples"},
			&cli.IntFlag{Name: "w-create", Value: 10, Usage: "weight of POST /examples"},
			&cli.IntFlag{Name: "w-update", Value: 10, Usage: "weight of PUT /examples/{id}"},
		},
		Action: func(cliCtx *cli.Context) error {
			cfg := benchConfig{
				BaseURL:  strings.TrimRight(cliCtx.String("base-url"), "/"),
				RPS:      cliCtx.Int("rps"),
				Duration: cliCtx.Duration("duration"),
				Warmup:   cliCtx.Duration("warmup"),
				Seed:     cliCtx.Int("seed"),
				Workers:  cliCtx.Int("workers"),
				Timeout:  cliCtx.Duration("timeout"),
				KeepData: cliCtx.Bool("keep-data"),
				RandSeed: cliCtx.Int64("rand-seed"),
				Weights: map[benchOp]int{
					opCacheGet: cliCtx.Int("w-cache-get"),
					opGet:      cliCtx.Int("w-get"),
					opList:     cliCtx.Int("w-list"),
					opCreate:   cliCtx.Int("w-create"),
					opUpdate:   cliCtx.Int("w-update"),
				},
			}
			if err := cfg.validate(); err != nil {
				return err
			}

			writer := cliCtx.App.Writer
			runner := newBenchRunner(cfg, writer)

			result, err := runner.Run(cliCtx.Context)
			if err != nil {
				// 应用的 ExitErrHandler 只把错误写入日志器；压测通常在关闭控制台
				// 日志的配置下运行，若不在此显式打印，操作者只会看到 exit status 1。
				_, _ = fmt.Fprintf(writer, "\nbench failed: %v\n", err)
				return err
			}
			if err := writeBenchReport(writer, result); err != nil {
				return err
			}
			if result.Total == 0 {
				return fmt.Errorf("no requests were completed; check --rps and --duration")
			}
			return nil
		},
	}
}
