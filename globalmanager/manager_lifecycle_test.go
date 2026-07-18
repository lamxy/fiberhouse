package globalmanager

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const lifecycleTestTimeout = 2 * time.Second

func lifecycleAwait(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(lifecycleTestTimeout):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func lifecycleReceive[T any](t *testing.T, result <-chan T, label string) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(lifecycleTestTimeout):
		var zero T
		t.Fatalf("timed out waiting for %s", label)
		return zero
	}
}

type lifecycleGetResult struct {
	value interface{}
	err   error
}

type lifecycleClosable struct {
	closed *int
	err    error
}

func (c *lifecycleClosable) Close() error {
	*c.closed++
	return c.err
}

func TestRelease_ClosableClosesAndCanReinitialize(t *testing.T) {
	manager := NewGlobalManager()
	initializations := 0
	closed := 0
	manager.Register("resource", func() (interface{}, error) {
		initializations++
		return &lifecycleClosable{closed: &closed}, nil
	})

	first, err := manager.Get("resource")
	if err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	if err := manager.Release("resource"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if closed != 1 {
		t.Fatalf("Close() calls = %d, want 1", closed)
	}

	second, err := manager.Get("resource")
	if err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if first == second {
		t.Fatal("Get() after Release() returned the released instance")
	}
	if initializations != 2 {
		t.Fatalf("initializer calls = %d, want 2", initializations)
	}
}

func TestGet_TransientFailureThenSuccessClearsCachedError(t *testing.T) {
	manager := NewGlobalManager()
	attempts := 0
	manager.Register("transient", func() (interface{}, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("transient failure")
		}
		return "ready", nil
	})

	if _, err := manager.Get("transient"); err == nil {
		t.Fatal("first Get() error = nil, want transient failure")
	}
	got, err := manager.Get("transient")
	if err != nil {
		t.Fatalf("second Get() error = %v, want nil", err)
	}
	if got != "ready" {
		t.Fatalf("second Get() = %v, want ready", got)
	}
}

func TestGet_NilInstanceIsRetryableInitializationFailure(t *testing.T) {
	manager := NewGlobalManager()
	calls := 0
	manager.Register("nil-resource", func() (interface{}, error) {
		calls++
		return nil, nil
	})

	var firstMessage string
	for attempt := 1; attempt <= 2; attempt++ {
		instance, err := manager.Get("nil-resource")
		if err == nil {
			t.Fatalf("Get() attempt %d error = nil, instance = %#v", attempt, instance)
		}
		if instance != nil {
			t.Fatalf("Get() attempt %d instance = %#v, want nil", attempt, instance)
		}
		if attempt == 1 {
			firstMessage = err.Error()
		} else if err.Error() != firstMessage {
			t.Fatalf("Get() errors differ: first %q, second %q", firstMessage, err.Error())
		}
	}
	if calls != 2 {
		t.Fatalf("initializer calls = %d, want 2", calls)
	}
}

func TestGet_FailurePublishesErrorBeforeConcurrentRetry(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previousProcs)

	for _, panicOnFirstAttempt := range []bool{false, true} {
		name := "error"
		if panicOnFirstAttempt {
			name = "panic"
		}
		t.Run(name, func(t *testing.T) {
			for attempt := 0; attempt < 1000; attempt++ {
				manager := NewGlobalManager()
				initializerEntered := make(chan struct{})
				releaseFailure := make(chan struct{})
				var initializerCalls int32
				manager.Register("transient", func() (interface{}, error) {
					if atomic.AddInt32(&initializerCalls, 1) == 1 {
						close(initializerEntered)
						<-releaseFailure
						if panicOnFirstAttempt {
							panic("transient panic")
						}
						return nil, errors.New("transient failure")
					}
					return "ready", nil
				})

				origin, _ := manager.container.Load("transient")
				entity := origin.(*entry)
				observerReady := make(chan struct{})
				retryDone := make(chan struct {
					published bool
					value     interface{}
					err       error
				}, 1)
				go func() {
					close(observerReady)
					for atomic.LoadInt32(&entity.initialized) != -1 {
						runtime.Gosched()
					}
					published := entity.initErr.Load() != nil
					value, err := manager.Get("transient")
					retryDone <- struct {
						published bool
						value     interface{}
						err       error
					}{published: published, value: value, err: err}
				}()

				<-observerReady
				firstDone := make(chan struct{}, 1)
				go func() {
					_, _ = manager.Get("transient")
					firstDone <- struct{}{}
				}()
				<-initializerEntered
				close(releaseFailure)

				result := <-retryDone
				<-firstDone
				if !result.published {
					t.Fatalf("attempt %d observed failed state before initErr was published", attempt)
				}
				if result.err != nil || result.value != "ready" {
					t.Fatalf("attempt %d retry = (%v, %v), want ready, nil", attempt, result.value, result.err)
				}
				if entity.initErr.Load() != nil {
					t.Fatalf("attempt %d successful retry retained initErr", attempt)
				}
			}
		})
	}
}

type lifecycleHealth struct{ healthy bool }

func (h *lifecycleHealth) IsHealthy() bool { return h.healthy }

type lifecycleRebuilder struct {
	path string
	next interface{}
	err  error
}

func (r *lifecycleRebuilder) Rebuild(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 || arguments[0] != r.path {
		return nil, errors.New("unexpected rebuild path")
	}
	return r.next, r.err
}

func (r *lifecycleRebuilder) GetConfPath() string { return r.path }

func TestGet_CachesSuccessfulInitialization(t *testing.T) {
	manager := NewGlobalManager()
	calls := 0
	manager.Register("cached", func() (interface{}, error) {
		calls++
		return &struct{ value int }{value: calls}, nil
	})

	first, err := manager.Get("cached")
	if err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	second, err := manager.Get("cached")
	if err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if first != second || calls != 1 {
		t.Fatalf("cached values = (%p, %p), initializer calls = %d", first, second, calls)
	}
}

func TestGet_PanicCanRetryWithoutAffectingAnotherEntry(t *testing.T) {
	manager := NewGlobalManager()
	attempts := 0
	manager.Register("panic", func() (interface{}, error) {
		attempts++
		if attempts == 1 {
			panic("first attempt")
		}
		return "recovered", nil
	})
	manager.Register("healthy", func() (interface{}, error) { return "healthy", nil })

	if _, err := manager.Get("panic"); err == nil || !strings.Contains(err.Error(), "first attempt") {
		t.Fatalf("first panic Get() error = %v", err)
	}
	if got, err := manager.Get("healthy"); err != nil || got != "healthy" {
		t.Fatalf("unrelated Get() = (%v, %v)", got, err)
	}
	if got, err := manager.Get("panic"); err != nil || got != "recovered" {
		t.Fatalf("retry Get() = (%v, %v)", got, err)
	}
}

func TestRelease_CloseErrorRetainsInitializedResource(t *testing.T) {
	manager := NewGlobalManager()
	closed := 0
	wantErr := errors.New("close failed")
	resource := &lifecycleClosable{closed: &closed, err: wantErr}
	manager.Register("resource", func() (interface{}, error) { return resource, nil })
	if _, err := manager.Get("resource"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if err := manager.Release("resource"); err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("Release() error = %v", err)
	}
	got, err := manager.Get("resource")
	if err != nil || got != resource {
		t.Fatalf("Get() after failed Release() = (%v, %v)", got, err)
	}
	if closed != 1 {
		t.Fatalf("Close() calls = %d, want 1", closed)
	}
}

func TestCheckHealth_InitializedCheckerAndDefaults(t *testing.T) {
	manager := NewGlobalManager()
	manager.Register("checker", func() (interface{}, error) { return &lifecycleHealth{healthy: false}, nil })
	manager.Register("plain", func() (interface{}, error) { return "value", nil })
	if _, err := manager.Get("checker"); err != nil {
		t.Fatalf("Get(checker) error = %v", err)
	}
	if healthy, err := manager.CheckHealth("checker"); err != nil || healthy {
		t.Fatalf("CheckHealth(checker) = (%v, %v), want false, nil", healthy, err)
	}
	if _, err := manager.Get("plain"); err != nil {
		t.Fatalf("Get(plain) error = %v", err)
	}
	if healthy, err := manager.CheckHealth("plain"); err != nil || !healthy {
		t.Fatalf("CheckHealth(plain) = (%v, %v), want true, nil", healthy, err)
	}
	if healthy, err := manager.CheckHealth("missing"); err == nil || !healthy {
		t.Fatalf("CheckHealth(missing) = (%v, %v), want true and error", healthy, err)
	}
}

func TestRebuild_SuccessErrorAndTypeChange(t *testing.T) {
	manager := NewGlobalManager()
	rebuilder := &lifecycleRebuilder{path: "config.yml", next: "new type"}
	manager.Register("resource", func() (interface{}, error) { return rebuilder, nil })
	if _, err := manager.Get("resource"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := manager.Rebuild("resource"); err != nil {
		t.Fatalf("Rebuild() type change error = %v", err)
	}
	if got, err := manager.Get("resource"); err != nil || got != "new type" {
		t.Fatalf("Get() rebuilt value = (%v, %v)", got, err)
	}

	errorManager := NewGlobalManager()
	wantErr := errors.New("rebuild failed")
	failing := &lifecycleRebuilder{path: "config.yml", err: wantErr}
	errorManager.Register("resource", func() (interface{}, error) { return failing, nil })
	if _, err := errorManager.Get("resource"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := errorManager.Rebuild("resource"); err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("Rebuild() error = %v", err)
	}
	if got, _ := errorManager.Get("resource"); got != failing {
		t.Fatal("failed rebuild replaced the current resource")
	}
}

func TestRebuild_NilResultReturnsErrorAndRetainsCurrentInstance(t *testing.T) {
	manager := NewGlobalManager()
	current := &lifecycleRebuilder{path: "config.yml", next: nil}
	manager.Register("resource", func() (interface{}, error) { return current, nil })
	if _, err := manager.Get("resource"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if err := manager.Rebuild("resource"); err == nil {
		t.Fatal("Rebuild() nil result error = nil")
	}
	got, err := manager.Get("resource")
	if err != nil {
		t.Fatalf("Get() after rejected rebuild error = %v", err)
	}
	if got != current {
		t.Fatalf("Get() after rejected rebuild = %#v, want original %#v", got, current)
	}
}

func TestLifecycle_ErrorBranchesAndRemoval(t *testing.T) {
	manager := NewGlobalManager()
	if err := manager.Release("missing"); err == nil {
		t.Fatal("Release(missing) error = nil")
	}
	if err := manager.Rebuild("missing"); err == nil {
		t.Fatal("Rebuild(missing) error = nil")
	}
	manager.Register("uninitialized", func() (interface{}, error) { return "value", nil })
	if err := manager.Release("uninitialized"); err != nil {
		t.Fatalf("Release(uninitialized) error = %v", err)
	}
	if err := manager.Rebuild("uninitialized"); err == nil {
		t.Fatal("Rebuild(uninitialized) error = nil")
	}
	manager.container.Store("broken", "not an entry")
	if _, err := manager.Get("broken"); err == nil {
		t.Fatal("Get(broken) error = nil")
	}
	if err := manager.Release("broken"); err == nil {
		t.Fatal("Release(broken) error = nil")
	}
	if err := manager.Rebuild("broken"); err == nil {
		t.Fatal("Rebuild(broken) error = nil")
	}
	if _, err := manager.CheckHealth("broken"); err == nil {
		t.Fatal("CheckHealth(broken) error = nil")
	}

	manager.Register("clear", func() (interface{}, error) { return 1, nil })
	manager.Clear("clear")
	if manager.IsRegistered("clear") {
		t.Fatal("Clear() retained key")
	}
	manager.Register("unregister", func() (interface{}, error) { return 1, nil })
	manager.Unregister("unregister")
	if manager.IsRegistered("unregister") {
		t.Fatal("Unregister() retained key")
	}
}

func TestReleaseAllAndClearAll_RequireConfirmation(t *testing.T) {
	manager := NewGlobalManager()
	closed := 0
	for _, key := range []string{"one", "two"} {
		manager.Register(key, func() (interface{}, error) {
			return &lifecycleClosable{closed: &closed}, nil
		})
		if _, err := manager.Get(key); err != nil {
			t.Fatalf("Get(%s) error = %v", key, err)
		}
	}
	manager.ReleaseAll()
	if closed != 0 {
		t.Fatalf("ReleaseAll() without confirmation closed %d resources", closed)
	}
	manager.ReleaseAll(true)
	if closed != 2 {
		t.Fatalf("ReleaseAll(true) closed %d resources, want 2", closed)
	}

	visited := 0
	manager.Range(func(_, _ interface{}) bool {
		visited++
		return true
	})
	if visited != 2 {
		t.Fatalf("Range() visited %d entries, want 2", visited)
	}
	manager.ClearAll()
	if !manager.IsRegistered("one") {
		t.Fatal("ClearAll() without confirmation removed entries")
	}
	manager.ClearAll(true)
	if manager.IsRegistered("one") || manager.IsRegistered("two") {
		t.Fatal("ClearAll(true) retained entries")
	}
}

func TestClearAll_DoesNotCloseResources(t *testing.T) {
	manager := NewGlobalManager()
	closed := 0
	manager.Register("resource", func() (interface{}, error) {
		return &lifecycleClosable{closed: &closed}, nil
	})
	if _, err := manager.Get("resource"); err != nil {
		t.Fatal(err)
	}

	manager.ClearAll(true)

	if closed != 0 {
		t.Fatalf("ClearAll(true) closed %d resources, want 0", closed)
	}
	if manager.IsRegistered("resource") {
		t.Fatal("ClearAll(true) retained resource")
	}
}

func TestClearAll_ConcurrentMapOperations(t *testing.T) {
	manager := NewGlobalManager()
	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			manager.Register("resource", func() (interface{}, error) { return i, nil })
			_, _ = manager.Get("resource")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			manager.Range(func(_, _ interface{}) bool { return true })
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			manager.ClearAll(true)
		}
	}()

	wg.Wait()
}

func TestGetInitialization_RemovalOnlyAffectsFutureLookups(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*GlobalManager)
	}{
		{name: "unregister", remove: func(manager *GlobalManager) { manager.Unregister("resource") }},
		{name: "clear-all", remove: func(manager *GlobalManager) { manager.ClearAll(true) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := NewGlobalManager()
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)

			if !manager.Register("resource", func() (interface{}, error) {
				close(entered)
				<-release
				return "detached", nil
			}) {
				t.Fatal("Register(resource) = false")
			}

			result := make(chan lifecycleGetResult, 1)
			go func() {
				value, err := manager.Get("resource")
				result <- lifecycleGetResult{value: value, err: err}
			}()

			lifecycleAwait(t, entered, "initializer entry")
			test.remove(manager)
			if manager.IsRegistered("resource") {
				t.Fatal("removal retained resource")
			}
			if _, err := manager.Get("resource"); err == nil {
				t.Fatal("Get after removal error = nil")
			}

			unblock()
			got := lifecycleReceive(t, result, "in-flight Get")
			if got.err != nil || got.value != "detached" {
				t.Fatalf("in-flight Get = (%v, %v), want detached, nil", got.value, got.err)
			}
			if _, err := manager.Get("resource"); err == nil {
				t.Fatal("later Get after removal error = nil")
			}
		})
	}
}
