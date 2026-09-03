package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// writeBenchReport 以固定格式输出压测统计结果。
func writeBenchReport(w io.Writer, result *benchResult) error {
	var b strings.Builder

	b.WriteString("\n=== Bench Result ===\n")
	fmt.Fprintf(&b, "target        %s\n", result.Config.BaseURL)
	fmt.Fprintf(&b, "duration      %-12s target rps  %-8d actual rps  %.1f\n",
		roundDuration(result.Elapsed), result.Config.RPS, result.ActualRPS())
	fmt.Fprintf(&b, "requests      %-12d errors      %d (%s)\n",
		result.Total, result.Errors, percentOf(result.Errors, result.Total))
	fmt.Fprintf(&b, "seeded        %-12d cleaned     %d\n", result.SeedCreated, result.Cleaned)

	if result.Dropped > 0 {
		fmt.Fprintf(&b, "\n!! WARNING: %d ticks dropped — workers could not sustain %d rps.\n",
			result.Dropped, result.Config.RPS)
		b.WriteString("   Results understate real load. Raise --workers or lower --rps.\n")
	}

	b.WriteString("\nlatency       ")
	writeLatencyLine(&b, result.Latencies)

	b.WriteString("\nby operation:\n")
	for _, op := range benchOpOrder {
		stats := result.PerOp[op]
		if stats == nil || stats.Count == 0 {
			continue
		}
		fmt.Fprintf(&b, "  %-11s %-8d ", string(op), stats.Count)
		writeLatencyLine(&b, stats.Latencies)
		fmt.Fprintf(&b, "                          errors %d (%s)\n",
			stats.Errors, percentOf(stats.Errors, stats.Count))
	}

	b.WriteString("\nstatus codes: ")
	writeStatusCodes(&b, result.StatusCodes)
	b.WriteString("\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// writeLatencyLine 输出一行分位数统计。
func writeLatencyLine(b *strings.Builder, latencies []time.Duration) {
	if len(latencies) == 0 {
		b.WriteString("(no samples)\n")
		return
	}
	fmt.Fprintf(b, "p50 %-9s p90 %-9s p99 %-9s max %s\n",
		roundDuration(percentile(latencies, 50)),
		roundDuration(percentile(latencies, 90)),
		roundDuration(percentile(latencies, 99)),
		roundDuration(latencies[len(latencies)-1]),
	)
}

// writeStatusCodes 按状态码升序输出计数。
func writeStatusCodes(b *strings.Builder, codes map[int]int) {
	if len(codes) == 0 {
		b.WriteString("(none)")
		return
	}
	keys := make([]int, 0, len(codes))
	for code := range codes {
		keys = append(keys, code)
	}
	sort.Ints(keys)
	for i, code := range keys {
		if i > 0 {
			b.WriteString("  ")
		}
		fmt.Fprintf(b, "%d:%d", code, codes[code])
	}
}

// percentOf 返回 part 占 total 的百分比字符串。
func percentOf(part, total int) string {
	if total == 0 {
		return "0.00%"
	}
	return fmt.Sprintf("%.2f%%", float64(part)/float64(total)*100)
}

// roundDuration 按量级取合适精度，避免输出过长的纳秒数。
func roundDuration(d time.Duration) time.Duration {
	switch {
	case d >= time.Second:
		return d.Round(100 * time.Millisecond)
	case d >= time.Millisecond:
		return d.Round(100 * time.Microsecond)
	default:
		return d.Round(time.Microsecond)
	}
}
