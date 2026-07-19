package writer

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lamxy/fiberhouse/appconfig"
)

func newAsyncChannelWriterForBenchmark(b *testing.B) *AsyncChannelWriter {
	b.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.appLog.rollConf.maxSize":              100,
		"application.appLog.rollConf.maxBackups":           1,
		"application.appLog.rollConf.maxAge":               1,
		"application.appLog.rollConf.compress":             false,
		"application.appLog.asyncConf.chanConf.bufferSize": 1 << 20,
		"application.appLog.asyncConf.chanConf.chanSize":   1 << 16,
	})
	return NewAsyncChannelWriter(cfg, filepath.Join(b.TempDir(), "benchmark.log"))
}

func BenchmarkAsyncChannelWriterSerial64B(b *testing.B) {
	w := newAsyncChannelWriterForBenchmark(b)
	payload := make([]byte, 64)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if n, err := w.Write(payload); err != nil || n != len(payload) {
			b.Fatalf("Write = (%d, %v), want (%d, nil)", n, err, len(payload))
		}
	}
	b.StopTimer()
	if err := w.Close(); err != nil {
		b.Fatalf("Close: %v", err)
	}
}

func BenchmarkAsyncChannelWriterParallel64B(b *testing.B) {
	w := newAsyncChannelWriterForBenchmark(b)
	payload := make([]byte, 64)
	errs := make(chan error, 1)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if n, err := w.Write(payload); err != nil || n != len(payload) {
				select {
				case errs <- fmt.Errorf("Write = (%d, %v), want (%d, nil)", n, err, len(payload)):
				default:
				}
			}
		}
	})
	b.StopTimer()
	select {
	case err := <-errs:
		b.Fatal(err)
	default:
	}
	if err := w.Close(); err != nil {
		b.Fatalf("Close: %v", err)
	}
}
