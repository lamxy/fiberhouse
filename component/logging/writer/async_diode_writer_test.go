package writer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse/appconfig"
)

type asyncDiodeWriterFixture struct {
	writer   *AsyncDiodeWriter
	filename string
	closed   bool
}

func newAsyncDiodeWriterFixture(t *testing.T) *asyncDiodeWriterFixture {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"application.appLog.rollConf.maxSize":                  10,
		"application.appLog.rollConf.maxBackups":               3,
		"application.appLog.rollConf.maxAge":                   7,
		"application.appLog.rollConf.compress":                 false,
		"application.appLog.asyncConf.diodeConf.size":          128,
		"application.appLog.asyncConf.diodeConf.bufferSize":    256,
		"application.appLog.asyncConf.diodeConf.flushInterval": 60_000,
	})
	filename := filepath.Join(t.TempDir(), "test.log")
	fixture := &asyncDiodeWriterFixture{
		writer:   NewAsyncDiodeWriter(cfg, filename),
		filename: filename,
	}
	t.Cleanup(func() {
		if !fixture.closed {
			_ = fixture.writer.Close()
		}
	})
	return fixture
}

func (f *asyncDiodeWriterFixture) close(t *testing.T) {
	t.Helper()
	err := f.writer.Close()
	f.closed = true
	if err != nil {
		t.Fatalf("close writer: %v", err)
	}
}

func (f *asyncDiodeWriterFixture) read(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(f.filename)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(content)
}

func TestAsyncDiodeWriter_CloseDrainsBeforeReturning(t *testing.T) {
	fixture := newAsyncDiodeWriterFixture(t)
	want := "message pending in diode\n"

	n, err := fixture.writer.Write([]byte(want))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(want) {
		t.Fatalf("write count: got %d, want %d", n, len(want))
	}
	fixture.close(t)

	if got := fixture.read(t); got != want {
		t.Fatalf("drained content: got %q, want %q", got, want)
	}
}

func TestAsyncDiodeWriter_MultipleWritesAreDrained(t *testing.T) {
	fixture := newAsyncDiodeWriterFixture(t)
	messages := []string{"first message\n", "second message\n", "third message\n"}

	for _, message := range messages {
		n, err := fixture.writer.Write([]byte(message))
		if err != nil {
			t.Fatalf("write %q: %v", message, err)
		}
		if n != len(message) {
			t.Fatalf("write count for %q: got %d, want %d", message, n, len(message))
		}
	}
	fixture.close(t)

	if got, want := fixture.read(t), strings.Join(messages, ""); got != want {
		t.Fatalf("drained content: got %q, want %q", got, want)
	}
}

func TestAsyncDiodeWriter_CopiesInputSlice(t *testing.T) {
	fixture := newAsyncDiodeWriterFixture(t)
	input := []byte("original value\n")
	want := string(input)

	if _, err := fixture.writer.Write(input); err != nil {
		t.Fatalf("write: %v", err)
	}
	copy(input, []byte("mutated value!\n"))
	fixture.close(t)

	if got := fixture.read(t); got != want {
		t.Fatalf("writer retained caller slice: got %q, want %q", got, want)
	}
}

func TestAsyncDiodeWriter_ConcurrentWritesAreDrained(t *testing.T) {
	fixture := newAsyncDiodeWriterFixture(t)
	const (
		writers         = 8
		writesPerWriter = 8
	)

	errs := make(chan error, writers*writesPerWriter)
	var wg sync.WaitGroup
	for writerID := 0; writerID < writers; writerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for writeID := 0; writeID < writesPerWriter; writeID++ {
				message := fmt.Sprintf("<writer=%02d write=%02d>\n", id, writeID)
				n, err := fixture.writer.Write([]byte(message))
				if err != nil {
					errs <- fmt.Errorf("write %q: %w", message, err)
					continue
				}
				if n != len(message) {
					errs <- fmt.Errorf("write %q returned %d bytes, want %d", message, n, len(message))
				}
			}
		}(writerID)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		return
	}
	fixture.close(t)

	content := fixture.read(t)
	for writerID := 0; writerID < writers; writerID++ {
		for writeID := 0; writeID < writesPerWriter; writeID++ {
			message := fmt.Sprintf("<writer=%02d write=%02d>\n", writerID, writeID)
			if count := strings.Count(content, message); count != 1 {
				t.Errorf("message %q occurred %d times, want 1", message, count)
			}
		}
	}
}

func TestAsyncDiodeWriter_WriteAfterCloseRejected(t *testing.T) {
	fixture := newAsyncDiodeWriterFixture(t)
	fixture.close(t)

	n, err := fixture.writer.Write([]byte("after close\n"))
	if err == nil {
		t.Fatal("write after close returned nil error")
	}
	if n != 0 {
		t.Fatalf("write after close count: got %d, want 0", n)
	}
}

func TestAsyncDiodeWriter_CloseIsIdempotent(t *testing.T) {
	fixture := newAsyncDiodeWriterFixture(t)
	fixture.close(t)

	if err := fixture.writer.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestAsyncDiodeWriter_ConcurrentCloseIsIdempotent(t *testing.T) {
	fixture := newAsyncDiodeWriterFixture(t)
	const closers = 8

	start := make(chan struct{})
	ready := make(chan struct{}, closers)
	results := make(chan error, closers)
	for i := 0; i < closers; i++ {
		go func() {
			ready <- struct{}{}
			<-start
			results <- fixture.writer.Close()
		}()
	}

	// All goroutines now share the same release barrier. The timeout only guards
	// against a deadlocked Close implementation; completion comes from results.
	fixture.closed = true
	for i := 0; i < closers; i++ {
		<-ready
	}
	close(start)
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	var first error
	for i := 0; i < closers; i++ {
		select {
		case err := <-results:
			if i == 0 {
				first = err
			}
			if (err == nil) != (first == nil) || err != nil && err.Error() != first.Error() {
				t.Errorf("close result %d = %v, want result consistent with %v", i, err, first)
			}
		case <-timer.C:
			t.Fatalf("concurrent Close calls did not all return; received %d of %d results", i, closers)
		}
	}
}
