package writer

import (
	"bufio"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/appconfig"
)

type asyncChannelCloseResult struct {
	err        error
	panicValue any
}

type gatedWriter struct {
	writer      io.Writer
	entered     chan struct{}
	release     chan struct{}
	enteredOnce sync.Once
	releaseOnce sync.Once
}

func (w *gatedWriter) Write(p []byte) (int, error) {
	w.enteredOnce.Do(func() { close(w.entered) })
	<-w.release
	return w.writer.Write(p)
}

func (w *gatedWriter) Release() {
	w.releaseOnce.Do(func() { close(w.release) })
}

func newAsyncChannelWriterForTest(t *testing.T) *AsyncChannelWriter {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.appLog.rollConf.maxSize":              10,
		"application.appLog.rollConf.maxBackups":           3,
		"application.appLog.rollConf.maxAge":               7,
		"application.appLog.rollConf.compress":             false,
		"application.appLog.asyncConf.chanConf.bufferSize": 256,
		"application.appLog.asyncConf.chanConf.chanSize":   0,
	})
	w := NewAsyncChannelWriter(cfg, t.TempDir()+"/test.log")
	t.Cleanup(func() {
		_ = closeAsyncChannelWriter(w)
	})
	return w
}

func closeAsyncChannelWriter(w *AsyncChannelWriter) (result asyncChannelCloseResult) {
	defer func() {
		result.panicValue = recover()
	}()
	result.err = w.Close()
	return result
}

func writeAsyncChannelWriter(w *AsyncChannelWriter, p []byte) (n int, err error, panicValue any) {
	defer func() {
		panicValue = recover()
	}()
	n, err = w.Write(p)
	return n, err, panicValue
}

func waitForBlockedAsyncChannelWrite(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stack := make([]byte, 1<<20)
		n := runtime.Stack(stack, true)
		for _, goroutine := range strings.Split(string(stack[:n]), "\n\n") {
			if strings.Contains(goroutine, "[select]") &&
				strings.Contains(goroutine, "(*AsyncChannelWriter).Write") &&
				strings.Contains(goroutine, "writeAsyncChannelWriter") {
				return
			}
		}
		runtime.Gosched()
	}
	t.Fatal("writer did not block in AsyncChannelWriter.Write select")
}

func TestAsyncChannelWriter_SequentialCloseReturnsFirstResult(t *testing.T) {
	w := newAsyncChannelWriterForTest(t)

	first := closeAsyncChannelWriter(w)
	second := closeAsyncChannelWriter(w)

	if first.panicValue != nil {
		t.Fatalf("first Close panicked: %v", first.panicValue)
	}
	if second.panicValue != nil {
		t.Fatalf("second Close panicked: %v", second.panicValue)
	}
	if fmt.Sprint(second.err) != fmt.Sprint(first.err) {
		t.Fatalf("second Close returned %v, want first result %v", second.err, first.err)
	}
}

func TestAsyncChannelWriter_ConcurrentCloseReturnsFirstResult(t *testing.T) {
	w := newAsyncChannelWriterForTest(t)
	const closers = 8

	ready := make(chan struct{}, closers)
	start := make(chan struct{})
	results := make(chan asyncChannelCloseResult, closers)
	for i := 0; i < closers; i++ {
		go func() {
			ready <- struct{}{}
			<-start
			results <- closeAsyncChannelWriter(w)
		}()
	}
	for i := 0; i < closers; i++ {
		<-ready
	}
	close(start)

	var first error
	for i := 0; i < closers; i++ {
		select {
		case result := <-results:
			if result.panicValue != nil {
				t.Errorf("Close %d panicked: %v", i, result.panicValue)
			}
			if i == 0 {
				first = result.err
			} else if fmt.Sprint(result.err) != fmt.Sprint(first) {
				t.Errorf("Close %d returned %v, want first result %v", i, result.err, first)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("received %d of %d Close results", i, closers)
		}
	}
}

func TestAsyncChannelWriter_WriteAdmittedBeforeCloseCompletes(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })
	w := newAsyncChannelWriterForTest(t)

	gate := &gatedWriter{
		writer:  w.lumber,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	t.Cleanup(gate.Release)
	w.writer = bufio.NewWriterSize(gate, 1)

	firstWrite := make(chan error, 1)
	go func() {
		n, err, panicValue := writeAsyncChannelWriter(w, []byte("prime the consumer\n"))
		if panicValue != nil {
			firstWrite <- fmt.Errorf("prime Write panicked: %v", panicValue)
			return
		}
		if err != nil || n != len("prime the consumer\n") {
			firstWrite <- fmt.Errorf("prime Write = (%d, %v)", n, err)
			return
		}
		firstWrite <- nil
	}()
	select {
	case <-gate.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("consumer did not enter the gated real-file write")
	}

	message := []byte("admitted before close\n")
	writeResult := make(chan error, 1)
	go func() {
		n, err, panicValue := writeAsyncChannelWriter(w, message)
		if panicValue != nil {
			writeResult <- fmt.Errorf("admitted Write panicked: %v", panicValue)
			return
		}
		if err != nil || n != len(message) {
			writeResult <- fmt.Errorf("admitted Write = (%d, %v), want (%d, nil)", n, err, len(message))
			return
		}
		writeResult <- nil
	}()
	waitForBlockedAsyncChannelWrite(t)

	closeResult := make(chan asyncChannelCloseResult, 1)
	go func() { closeResult <- closeAsyncChannelWriter(w) }()
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&w.closed) == 0 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if atomic.LoadInt32(&w.closed) == 0 {
		t.Fatal("Close did not mark the writer closed")
	}
	gate.Release()

	joinTimer := time.NewTimer(2 * time.Second)
	defer joinTimer.Stop()
	for joined := 0; joined < 3; {
		select {
		case err := <-firstWrite:
			firstWrite = nil
			joined++
			if err != nil {
				t.Error(err)
			}
		case err := <-writeResult:
			writeResult = nil
			joined++
			if err != nil {
				t.Error(err)
			}
		case result := <-closeResult:
			closeResult = nil
			joined++
			if result.panicValue != nil {
				t.Errorf("Close panicked: %v", result.panicValue)
			}
			if result.err != nil {
				t.Errorf("Close returned %v", result.err)
			}
		case <-joinTimer.C:
			t.Fatalf("joined %d of 3 terminal results", joined)
		}
	}
}

func TestAsyncChannelWriter_ImmediateSendAllocatesOnlyCopiedInput(t *testing.T) {
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.appLog.rollConf.maxSize":              10,
		"application.appLog.rollConf.maxBackups":           3,
		"application.appLog.rollConf.maxAge":               7,
		"application.appLog.rollConf.compress":             false,
		"application.appLog.asyncConf.chanConf.bufferSize": 1 << 20,
		"application.appLog.asyncConf.chanConf.chanSize":   4096,
	})
	w := NewAsyncChannelWriter(cfg, t.TempDir()+"/allocations.log")
	t.Cleanup(func() {
		if result := closeAsyncChannelWriter(w); result.panicValue != nil || result.err != nil {
			t.Errorf("cleanup Close = (%v, panic %v)", result.err, result.panicValue)
		}
	})
	payload := make([]byte, 64)
	var writeErr error

	allocs := testing.AllocsPerRun(1000, func() {
		var n int
		n, writeErr = w.Write(payload)
		if writeErr == nil && n != len(payload) {
			writeErr = fmt.Errorf("Write returned %d bytes, want %d", n, len(payload))
		}
	})
	if writeErr != nil {
		t.Fatalf("immediate Write: %v", writeErr)
	}
	if allocs > 1.1 {
		t.Fatalf("immediate Write allocated %.2f objects/run, want approximately 1 copied input", allocs)
	}
}
