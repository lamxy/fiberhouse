package jsonconvert

import (
	"testing"
)

// sink 防编译器优化
var sink string

// 禁止内联，避免编译器把调用直接内联到基准循环中
//
//go:noinline
func callGetStringOriginal(dw *DataWrap) string {
	return dw.GetString()
}

//go:noinline
func callGetStringOpt(dw *DataWrapOpt) string {
	return dw.GetString()
}

func Benchmark_GetString_String_Large_Original(b *testing.B) {
	// 使用较大的字符串
	long := make([]byte, 1024)
	for i := range long {
		long[i] = 'a' + byte(i%26)
	}
	s := string(long)

	dw := NewDataWrap(s)
	defer dw.Release()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = callGetStringOriginal(dw)
	}
}

func Benchmark_GetString_String_Large_Optimized(b *testing.B) {
	long := make([]byte, 1024)
	for i := range long {
		long[i] = 'a' + byte(i%26)
	}
	s := string(long)

	dw := NewDataWrapOpt(s)
	defer dw.Release()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = callGetStringOpt(dw)
	}
}

// GetString on []byte 大 payload，测 string([]byte) 的开销
func Benchmark_GetString_Bytes_Large_Original(b *testing.B) {
	buf := make([]byte, 1024)
	for i := range buf {
		buf[i] = byte(i % 256)
	}
	dw := NewDataWrap(buf) // original: store []byte
	defer dw.Release()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = callGetStringOriginal(dw)
	}
}

func Benchmark_GetString_Bytes_Large_Optimized(b *testing.B) {
	buf := make([]byte, 1024)
	for i := range buf {
		buf[i] = byte(i % 256)
	}
	dw := NewDataWrapOpt(buf)
	defer dw.Release()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = callGetStringOpt(dw)
	}
}

// 包含 NewDataWrap + GetString，每次迭代创建 DataWrap
func Benchmark_NewAndGet_String_Original(b *testing.B) {
	long := make([]byte, 512)
	for i := range long {
		long[i] = 'x'
	}
	s := string(long)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dw := NewDataWrap(s)
		sink = dw.GetString()
		dw.Release()
	}
}

func Benchmark_NewAndGet_String_Optimized(b *testing.B) {
	long := make([]byte, 512)
	for i := range long {
		long[i] = 'x'
	}
	s := string(long)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dw := NewDataWrapOpt(s)
		sink = dw.GetString()
		dw.Release()
	}
}

// 并行，多 goroutine 调用 GetString
func Benchmark_GetString_String_Original_Parallel(b *testing.B) {
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'p'
	}
	s := string(long)
	dw := NewDataWrap(s)
	defer dw.Release()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sink = callGetStringOriginal(dw)
		}
	})
}

func Benchmark_GetString_String_Optimized_Parallel(b *testing.B) {
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'p'
	}
	s := string(long)
	dw := NewDataWrapOpt(s)
	defer dw.Release()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sink = callGetStringOpt(dw)
		}
	})
}
