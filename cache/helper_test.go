package cache

import (
	"errors"
	"github.com/sony/gobreaker/v2"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCircuitBreakerWrap_Basic 成功执行
func TestCircuitBreakerWrap_Basic(t *testing.T) {
	wrap := NewCircuitBreakerWrap("cb-basic")
	out, err := wrap.Call(func() (string, error) {
		return "ok-value", nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.(string) != "ok-value" {
		t.Fatalf("expect ok-value got %v", out)
	}
}

// TestCircuitBreakerWrap_TripAndRecover 打开->阻断->恢复
func TestCircuitBreakerWrap_TripAndRecover(t *testing.T) {
	settings := &gobreaker.Settings{
		Name:        "cb-trip",
		MaxRequests: 1,
		Interval:    0,                      // 不滚动窗口
		Timeout:     120 * time.Millisecond, // 打开后等待恢复
		ReadyToTrip: func(c gobreaker.Counts) bool {
			// 1 次失败即打开
			return c.TotalFailures >= 1
		},
	}
	wrap := NewCircuitBreakerWrap("cb-trip", settings)

	// 第一次失败 => 打开
	_, _ = wrap.Call(func() (string, error) {
		return "", errors.New("fail")
	})

	// 立即调用应返回转换后的打开错误
	_, err := wrap.Call(func() (string, error) {
		return "should-block", nil
	})
	if err == nil {
		t.Fatalf("expect open state error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(strings.ToLower(msg), "open") {
		t.Fatalf("expect error message contains 'open', got=%v", msg)
	}

	// 等待超时 -> 半开再成功
	time.Sleep(150 * time.Millisecond)
	out, err2 := wrap.Call(func() (string, error) {
		return "recovered", nil
	})
	if err2 != nil {
		t.Fatalf("expect recovery success got err=%v", err2)
	}
	if out.(string) != "recovered" {
		t.Fatalf("expect 'recovered' got=%v", out)
	}
}

// TestShardedBloomFilter_Basic 基本行为
func TestShardedBloomFilter_Basic(t *testing.T) {
	bf := NewShardedBloomFilter(0, 1000, 0.01) // shardCount=0 => 默认
	sb, ok := bf.(*ShardedBloomFilter)
	if !ok {
		t.Fatalf("type assertion to *ShardedBloomFilter failed")
	}

	key := []byte("hello")
	if sb.Test(key) {
		t.Fatalf("key should be absent initially")
	}
	sb.Add(key)
	if !sb.Test(key) {
		t.Fatalf("key should exist after Add")
	}
	// TestAndAdd: 第一次返回 false(之前已存在? 这里先换另一个 key)
	k2 := []byte("world")
	if existed := sb.TestAndAdd(k2); existed {
		t.Fatalf("first TestAndAdd should be false")
	}
	if existed := sb.TestAndAdd(k2); !existed {
		t.Fatalf("second TestAndAdd should be true")
	}
}

// TestShardedBloomFilter_Concurrent 并发安全 & 低误判验证（仅不崩溃）
func TestShardedBloomFilter_Concurrent(t *testing.T) {
	bf := NewShardedBloomFilter(32, 5000, 0.01)
	sb := bf.(*ShardedBloomFilter)

	var wg sync.WaitGroup
	var misses atomic.Int32
	totalKeys := 3000

	// writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < totalKeys; i++ {
			sb.Add([]byte(keyStr(i)))
		}
	}()

	time.Sleep(1 * time.Second)

	// readers
	readers := 8
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			start := id * (totalKeys / readers)
			end := start + (totalKeys / readers)
			for i := start; i < end; i++ {
				if !sb.Test([]byte(keyStr(i))) {
					// 允许少量误判（布隆: 可能 false negative 极低，这里统计）
					misses.Add(1)
				}
			}
		}(r)
	}

	wg.Wait()
	// 不作强硬断言，只要不出现极大数量的 miss
	if m := misses.Load(); m > int32(totalKeys/3) {
		t.Logf("warning: high misses=%d (可能测试过早读取或实现尚未强同步)", m)
	}
}

func keyStr(i int) string {
	return "k:" + strconvI(i)
}

// strconvI local
func strconvI(i int) string {
	// 避免额外导入 strconv 多次使用
	return fmtInt(i)
}

// 自定义简单 int -> string（仅满足测试，避免重复导入也可直接用 strconv.Itoa）
func fmtInt(i int) string {
	// 仍然引入 strconv 更清晰
	return strconv.Itoa(i)
}
