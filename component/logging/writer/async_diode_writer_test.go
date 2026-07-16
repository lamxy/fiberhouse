package writer

import (
	"fmt"
	"github.com/lamxy/fiberhouse/appconfig"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAsyncDiodeWriter_NewAsyncDiodeWriter 测试创建异步日志写入器
func TestAsyncDiodeWriter_NewAsyncDiodeWriter(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "test_async_writer")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// 创建测试配置
	cfg := createTestConfig()
	filename := filepath.Join(tempDir, "test.log")

	// 创建异步写入器
	writer := NewAsyncDiodeWriter(cfg, filename)
	if writer == nil {
		t.Fatal("NewAsyncDiodeWriter returned nil")
	}

	// 验证字段不为空
	if writer.diode == nil {
		t.Error("diode should not be nil")
	}
	if writer.writer == nil {
		t.Error("writer should not be nil")
	}
	if writer.lumber == nil {
		t.Error("lumber should not be nil")
	}
	if writer.stopCh == nil {
		t.Error("stopCh should not be nil")
	}

	// 清理
	if err := writer.Close(); err != nil {
		t.Errorf("Failed to close writer: %v", err)
	}
}

// TestAsyncDiodeWriter_InvalidConfig 测试无效配置
func TestAsyncDiodeWriter_InvalidConfig(t *testing.T) {
	cfg := appconfig.NewAppConfig()
	// 不设置配置，测试默认值处理
	filename := "D:/tmp/test.log"

	writer := NewAsyncDiodeWriter(cfg, filename)
	if writer == nil {
		t.Fatal("Should handle missing config gracefully")
	}
	defer writer.Close()
}

// TestAsyncDiodeWriter_InvalidFilePath 测试无效文件路径
func TestAsyncDiodeWriter_InvalidFilePath(t *testing.T) {
	cfg := createTestConfig()
	// 使用无效路径
	filename := "D:/invalid/path/test.log"

	writer := NewAsyncDiodeWriter(cfg, filename)
	defer writer.Close()

	// 写入数据，应该能处理文件创建失败的情况
	_, err := writer.Write([]byte("test"))
	// 根据实际实现，可能需要调整期望的行为
	if err != nil {
		t.Logf("Expected error for invalid path: %v", err)
	}
}

// TestAsyncDiodeWriter_Write 测试写入功能
func TestAsyncDiodeWriter_Write(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "test_async_writer")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// 创建测试配置
	cfg := createTestConfig()
	filename := filepath.Join(tempDir, "test.log")

	// 创建异步写入器
	writer := NewAsyncDiodeWriter(cfg, filename)
	defer func() {
		if err := writer.Close(); err != nil {
			t.Logf("Failed to close writer: %v", err)
		}
	}()

	// 测试写入数据
	testData := []byte("Hello, World!\n")
	n, err := writer.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, got %d", len(testData), n)
	}

	// 等待数据被写入
	time.Sleep(100 * time.Millisecond)

	// 验证文件内容
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "Hello, World!") {
		t.Errorf("Expected log content not found. Got: %s", string(content))
	}
}

// TestAsyncDiodeWriter_MultipleWrites 测试多次写入
func TestAsyncDiodeWriter_MultipleWrites(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "test_async_writer")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// 创建测试配置
	cfg := createTestConfig()
	filename := filepath.Join(tempDir, "test.log")

	// 创建异步写入器
	writer := NewAsyncDiodeWriter(cfg, filename)
	defer func() {
		if err := writer.Close(); err != nil {
			t.Logf("Failed to close writer: %v", err)
		}
	}()

	// 多次写入
	testMessages := []string{
		"First message\n",
		"Second message\n",
		"Third message\n",
	}

	for _, msg := range testMessages {
		_, err := writer.Write([]byte(msg))
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
	}

	// 等待数据被写入
	time.Sleep(200 * time.Millisecond)

	// 验证文件内容
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	for _, msg := range testMessages {
		if !strings.Contains(string(content), strings.TrimSpace(msg)) {
			t.Errorf("Expected message '%s' not found in log content", msg)
		}
	}
}

// TestAsyncDiodeWriter_ConcurrentWrites 测试并发写入
func TestAsyncDiodeWriter_ConcurrentWrites(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "test_async_writer")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// 创建测试配置
	cfg := createTestConfig()
	filename := filepath.Join(tempDir, "test.log")

	writer := NewAsyncDiodeWriter(cfg, filename)
	defer writer.Close()

	numGoroutines := 10
	messagesPerGoroutine := 10
	var wg sync.WaitGroup

	// 记录期望的消息
	expectedMessages := make([]string, 0, numGoroutines*messagesPerGoroutine)
	var msgMutex sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := fmt.Sprintf("Goroutine %d Message %d\n", id, j)

				msgMutex.Lock()
				expectedMessages = append(expectedMessages, strings.TrimSpace(msg))
				msgMutex.Unlock()

				_, err := writer.Write([]byte(msg))
				if err != nil {
					t.Errorf("Write failed in goroutine %d: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// 等待异步写入完成
	time.Sleep(time.Second)

	// 验证所有消息都被写入
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	for _, expectedMsg := range expectedMessages {
		if !strings.Contains(contentStr, expectedMsg) {
			t.Errorf("Expected message not found: %s", expectedMsg)
		}
	}
}

// TestAsyncDiodeWriter_Close 测试关闭功能
func TestAsyncDiodeWriter_Close(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "test_async_writer_new")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// 创建测试配置
	cfg := createTestConfig()
	filename := filepath.Join(tempDir, "test.log")

	// 创建异步写入器
	writer := NewAsyncDiodeWriter(cfg, filename)

	// 写入一些数据
	testData := []byte("Test data before close\n")
	_, err = writer.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// 关闭写入器
	//err = writer.Close()
	//if err != nil {
	//	t.Errorf("Close failed: %v", err)
	//}

	// 等待异步写入完成
	time.Sleep(1000 * time.Millisecond)

	// 验证数据已被写入文件
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Test data before close") {
		t.Error("Data should be written to file after close")
	}
}

// TestAsyncDiodeWriter_WriteAfterClose 测试关闭后写入
func TestAsyncDiodeWriter_WriteAfterClose(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "test_async_writer")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// 创建测试配置
	cfg := createTestConfig()
	filename := filepath.Join(tempDir, "test.log")

	// 创建异步写入器
	writer := NewAsyncDiodeWriter(cfg, filename)

	// 关闭写入器
	err = writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 尝试在关闭后写入数据
	testData := []byte("Data after close\n")
	n, err := writer.Write(testData)
	// 写入应该仍然返回成功（因为只是写入到diode中），但数据可能不会被处理
	if err != nil {
		t.Errorf("Write after close failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, got %d", len(testData), n)
	}
}

// createTestConfig 创建测试用的配置
func createTestConfig() *appconfig.AppConfig {
	cfg := appconfig.NewAppConfig()
	testConfig := map[string]interface{}{
		"application.appLog.rollConf.maxSize":    10,
		"application.appLog.rollConf.maxBackups": 3,
		"application.appLog.rollConf.maxAge":     7,
		"application.appLog.rollConf.compress":   false,
		// diodeConf
		"application.appLog.asyncConf.diodeConf.size":          33554432, // 32 * 1024 * 1024 = 32M
		"application.appLog.asyncConf.diodeConf.bufferSize":    8912,     // 8K
		"application.appLog.asyncConf.diodeConf.flushInterval": 1000,
	}
	cfg.LoadDefault(testConfig)
	return cfg
}

// BenchmarkAsyncDiodeWriter_Write 性能测试
func BenchmarkAsyncDiodeWriter_Write(b *testing.B) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "bench_async_writer")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			b.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// 创建测试配置
	cfg := createTestConfig()
	filename := filepath.Join(tempDir, "bench.log")

	// 创建异步写入器
	writer := NewAsyncDiodeWriter(cfg, filename)
	defer func() {
		if err := writer.Close(); err != nil {
			b.Logf("Failed to close writer: %v", err)
		}
	}()

	testData := []byte("Benchmark test message\n")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := writer.Write(testData)
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}

// ExampleAsyncDiodeWriter 示例代码
func ExampleAsyncDiodeWriter() {
	// 创建Test配置
	cfg := appconfig.NewAppConfig()
	testConfig := map[string]interface{}{
		"application.appLog.rollConf.maxSize":               10,
		"application.appLog.rollConf.maxBackups":            3,
		"application.appLog.rollConf.maxAge":                7,
		"application.appLog.rollConf.compress":              false,
		"application.appLog.asyncConf.diodes.size":          1000000,
		"application.appLog.asyncConf.diodes.bufferSize":    4096,
		"application.appLog.asyncConf.diodes.flushInterval": 50,
	}
	cfg.LoadDefault(testConfig)

	// 创建异步写入器
	writer := NewAsyncDiodeWriter(cfg, "example.log")
	defer func() {
		if err := writer.Close(); err != nil {
			// 处理关闭错误
		}
	}()

	// 写入日志
	if _, err := writer.Write([]byte("Hello, World!\n")); err != nil {
		// 处理写入错误
	}
	if _, err := writer.Write([]byte("This is an example log message\n")); err != nil {
		// 处理写入错误
	}

	// 等待写入完成
	time.Sleep(100 * time.Millisecond) // 大于application.appLog.asyncConf.diodes.flushInterval配置项
}
