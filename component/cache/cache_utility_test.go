// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jsoncodec "github.com/lamxy/fiberhouse/component/codec/json"
)

type coordinatorPayload struct {
	ID int
}

// coordinatorFakeCache is a deterministic in-memory Cache used to prove the
// coordinator's observable read-through behavior without a network dependency.
type coordinatorFakeCache struct {
	mu        sync.Mutex
	values    map[string]string
	getCount  int
	setCount  int
	getErr    error
	setErr    error
	setSignal chan struct{}
	getSignal chan struct{}
}

func newCoordinatorFakeCache() *coordinatorFakeCache {
	return &coordinatorFakeCache{values: make(map[string]string)}
}

func (c *coordinatorFakeCache) Get(_ context.Context, key string, _ *CacheOption) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getCount++
	if c.getSignal != nil {
		select {
		case c.getSignal <- struct{}{}:
		default:
		}
	}
	if c.getErr != nil {
		return "", c.getErr
	}
	value, ok := c.values[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return value, nil
}

func (c *coordinatorFakeCache) Set(_ context.Context, key string, value interface{}, _ *CacheOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setCount++
	if c.setErr != nil {
		return c.setErr
	}
	stringValue, ok := value.(string)
	if !ok {
		return ErrSerializationFailed
	}
	c.values[key] = stringValue
	if c.setSignal != nil {
		select {
		case c.setSignal <- struct{}{}:
		default:
		}
	}
	return nil
}

func (c *coordinatorFakeCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		delete(c.values, key)
	}
	return nil
}

func (*coordinatorFakeCache) Close() error    { return nil }
func (*coordinatorFakeCache) Wait() error     { return nil }
func (*coordinatorFakeCache) GetLevel() Level { return Local }

func coordinatorRequest(ctx context.Context, key string) *CacheOption {
	return NewCacheOption(newCacheOptionTestContext()).
		Local().
		SetCacheKey(key).
		SetContextCtx(ctx).
		SetJsonWrapper(jsoncodec.StdJsonDefault())
}

func TestCoordinator_CoalescesCompleteLoad(t *testing.T) {
	// Removing the coordinator flight must make this test observe more than one
	// origin load while all callers request the same absent cache entry.
	backend := newCoordinatorFakeCache()
	coord, err := NewCoordinator(backend)
	if err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	var loads atomic.Int32
	loader := func(context.Context) (coordinatorPayload, error) {
		loads.Add(1)
		<-release
		return coordinatorPayload{ID: 7}, nil
	}

	const callers = 32
	results := make(chan error, callers)
	for range callers {
		go func() {
			got, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "coalesced"), loader)
			if err != nil {
				results <- err
				return
			}
			if got.ID != 7 {
				results <- ErrDeserializationFailed
				return
			}
			results <- nil
		}()
	}

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for loads.Load() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("loader did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(release)
	for range callers {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if got := loads.Load(); got != 1 {
		t.Fatalf("loader count = %d, want 1", got)
	}
}

func TestCoordinator_ReturnCacheErrorSkipsLoader(t *testing.T) {
	// Replacing the policy check with "load on every error" must invoke loader.
	backend := newCoordinatorFakeCache()
	backend.getErr = errors.New("network unavailable")
	coord, _ := NewCoordinator(backend)
	called := false
	_, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "network"), func(context.Context) (coordinatorPayload, error) {
		called = true
		return coordinatorPayload{}, nil
	})
	if err == nil || called {
		t.Fatalf("err = %v, loader called = %v; want cache error without loader", err, called)
	}
}

func TestCoordinator_LoadOnCacheErrorCoalescesLoader(t *testing.T) {
	backend := newCoordinatorFakeCache()
	backend.getErr = errors.New("network unavailable")
	coord, _ := NewCoordinator(backend)
	var loads atomic.Int32
	release := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "network").SetCacheErrorPolicy(LoadOnCacheError), func(context.Context) (coordinatorPayload, error) {
				loads.Add(1)
				<-release
				return coordinatorPayload{ID: 9}, nil
			})
			done <- err
		}()
	}
	deadline := time.After(time.Second)
	for loads.Load() != 1 {
		select {
		case <-deadline:
			t.Fatalf("loader count = %d, want 1", loads.Load())
		default:
		}
	}
	close(release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestCoordinator_CanceledWaiterDoesNotCancelSharedLoader(t *testing.T) {
	backend := newCoordinatorFakeCache()
	coord, _ := NewCoordinator(backend)
	started := make(chan struct{})
	release := make(chan struct{})
	loader := func(context.Context) (coordinatorPayload, error) {
		close(started)
		<-release
		return coordinatorPayload{ID: 3}, nil
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "cancel"), loader)
		firstDone <- err
	}()
	<-started
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := GetOrLoad(coord, coordinatorRequest(canceled, "cancel"), loader)
	if !errors.Is(err, context.Canceled) || time.Since(start) > 100*time.Millisecond {
		t.Fatalf("waiter err = %v after %s, want prompt cancellation", err, time.Since(start))
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestCoordinator_FirstCallerCancellationDoesNotPoisonSecondWaiter(t *testing.T) {
	backend := newCoordinatorFakeCache()
	backend.getSignal = make(chan struct{}, 3)
	coord, _ := NewCoordinator(backend)
	first, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	var loads atomic.Int32
	loader := func(context.Context) (coordinatorPayload, error) {
		if loads.Add(1) == 1 {
			close(started)
		}
		<-release
		return coordinatorPayload{ID: 4}, nil
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := GetOrLoad(coord, coordinatorRequest(first, "first-cancel"), loader)
		firstDone <- err
	}()
	<-started
	secondDone := make(chan error, 1)
	go func() {
		got, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "first-cancel"), loader)
		if err == nil && got.ID != 4 {
			err = ErrDeserializationFailed
		}
		secondDone <- err
	}()
	for range 3 {
		select {
		case <-backend.getSignal:
		case <-time.After(time.Second):
			t.Fatal("second waiter did not reach cache read")
		}
	}
	cancel()
	close(release)
	if !errors.Is(<-firstDone, context.Canceled) {
		t.Fatal("first caller did not observe cancellation")
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 1 {
		t.Fatalf("loader count = %d, want 1", loads.Load())
	}
}

func TestCoordinator_DistinctTypesDoNotShareFlight(t *testing.T) {
	backend := newCoordinatorFakeCache()
	coord, _ := NewCoordinator(backend)
	var loads atomic.Int32
	_, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "types"), func(context.Context) (coordinatorPayload, error) { loads.Add(1); return coordinatorPayload{ID: 1}, nil })
	if err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	delete(backend.values, "types")
	backend.mu.Unlock()
	_, err = GetOrLoad(coord, coordinatorRequest(context.Background(), "types"), func(context.Context) (string, error) { loads.Add(1); return "different", nil })
	if err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 2 {
		t.Fatalf("loader count = %d, want 2", loads.Load())
	}
}

func TestCoordinator_CorruptCachedJSONReturnsCodecError(t *testing.T) {
	backend := newCoordinatorFakeCache()
	backend.values["corrupt"] = "{"
	coord, _ := NewCoordinator(backend)
	called := false
	_, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "corrupt"), func(context.Context) (coordinatorPayload, error) { called = true; return coordinatorPayload{}, nil })
	if err == nil || called {
		t.Fatalf("err = %v, loader called = %v; want codec error without loader", err, called)
	}
}

func TestCoordinator_LeaderOptionReleaseDoesNotMutateSharedFlight(t *testing.T) {
	// Resetting the leader's pooled option after cancellation must not change the
	// asynchronous flight's key or codec used after the origin loader returns.
	backend := newCoordinatorFakeCache()
	backend.setSignal = make(chan struct{}, 1)
	coord, _ := NewCoordinator(backend)
	ctx, cancel := context.WithCancel(context.Background())
	request := OptionPoolGet(newCacheOptionTestContext()).Local().SetCacheKey("stable-key").SetContextCtx(ctx).SetJsonWrapper(jsoncodec.StdJsonDefault())
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := GetOrLoad(coord, request, func(context.Context) (coordinatorPayload, error) {
			close(started)
			<-release
			return coordinatorPayload{ID: 12}, nil
		})
		done <- err
	}()
	<-started
	cancel()
	if !errors.Is(<-done, context.Canceled) {
		t.Fatal("leader did not return context cancellation")
	}
	request.Release()
	reused := OptionPoolGet(newCacheOptionTestContext())
	reused.Local().SetCacheKey("reused-key").SetContextCtx(context.Background()).SetJsonWrapper(jsoncodec.StdJsonDefault())
	defer reused.Release()
	close(release)
	select {
	case <-backend.setSignal:
	case <-time.After(time.Second):
		t.Fatal("shared flight did not fill cache")
	}
	backend.mu.Lock()
	_, stable := backend.values["stable-key"]
	_, reusedValue := backend.values["reused-key"]
	backend.mu.Unlock()
	if !stable || reusedValue {
		t.Fatalf("stable=%v reused=%v, want stable key only", stable, reusedValue)
	}
}

func TestCoordinator_AnyNilLoaderValueIsReturned(t *testing.T) {
	// Returning untyped nil through an `any` result must remain a successful load.
	coord, _ := NewCoordinator(newCoordinatorFakeCache())
	got, err := GetOrLoad[any](coord, coordinatorRequest(context.Background(), "nil-any"), func(context.Context) (any, error) { return nil, nil })
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v, want nil nil", got, err)
	}
}

func TestCoordinator_BloomColdKeyReachesLoader(t *testing.T) {
	backend := newCoordinatorFakeCache()
	filter, err := NewMemoryBloomFilter(100, 0.01, 1)
	if err != nil {
		t.Fatal(err)
	}
	coord, err := NewCoordinator(backend, WithExistenceFilter(filter))
	if err != nil {
		t.Fatal(err)
	}
	loads := 0
	got, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "cold").EnableBloomFilter(), func(context.Context) (coordinatorPayload, error) {
		loads++
		return coordinatorPayload{ID: 1}, nil
	})
	if err != nil || got.ID != 1 || loads != 1 {
		t.Fatalf("got=%+v err=%v loads=%d; want loaded value and one loader call", got, err, loads)
	}
}

func TestCoordinator_BloomPreloadedKeyReachesCacheAndLoader(t *testing.T) {
	backend := newCoordinatorFakeCache()
	filter, err := NewMemoryBloomFilter(100, 0.01, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := filter.Preload(context.Background(), []string{"known"}); err != nil {
		t.Fatal(err)
	}
	coord, err := NewCoordinator(backend, WithExistenceFilter(filter))
	if err != nil {
		t.Fatal(err)
	}
	loads := 0
	got, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "known").EnableBloomFilter(), func(context.Context) (coordinatorPayload, error) {
		loads++
		return coordinatorPayload{ID: 2}, nil
	})
	if err != nil || got.ID != 2 || loads != 1 {
		t.Fatalf("got=%+v err=%v loads=%d; want preloaded key to reach loader", got, err, loads)
	}
	backend.mu.Lock()
	gets := backend.getCount
	backend.mu.Unlock()
	if gets == 0 {
		t.Fatal("preloaded key bypassed cache lookup")
	}
}

func TestCoordinator_BloomRejectsAbsentKeyOnlyAfterPreload(t *testing.T) {
	backend := newCoordinatorFakeCache()
	filter, err := NewMemoryBloomFilter(100, 0.01, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := filter.Preload(context.Background(), []string{"known"}); err != nil {
		t.Fatal(err)
	}
	coord, err := NewCoordinator(backend, WithExistenceFilter(filter))
	if err != nil {
		t.Fatal(err)
	}
	called := false
	_, err = GetOrLoad(coord, coordinatorRequest(context.Background(), "absent").EnableBloomFilter(), func(context.Context) (coordinatorPayload, error) {
		called = true
		return coordinatorPayload{}, nil
	})
	var rejected ErrRejectedByBloomFilter
	if !errors.As(err, &rejected) || called {
		t.Fatalf("err=%v loader called=%v; want Bloom rejection before loader", err, called)
	}
	backend.mu.Lock()
	gets := backend.getCount
	backend.mu.Unlock()
	if gets != 0 {
		t.Fatalf("cache Get count = %d, want 0 for rejected key", gets)
	}
}

type failingExistenceFilter struct{ err error }

func (f failingExistenceFilter) Ready() bool { return true }
func (f failingExistenceFilter) MayContain(context.Context, string) (bool, error) {
	return false, f.err
}
func (f failingExistenceFilter) Add(context.Context, string) error { return nil }

func TestCoordinator_BloomFilterErrorUsesConfiguredFailureMode(t *testing.T) {
	filterErr := errors.New("filter unavailable")
	for _, tc := range []struct {
		name       string
		mode       FilterFailureMode
		wantLoad   bool
		wantReject bool
	}{
		{name: "fail open", mode: FilterFailOpen, wantLoad: true},
		{name: "fail closed", mode: FilterFailClosed, wantReject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := newCoordinatorFakeCache()
			coord, err := NewCoordinator(backend, WithExistenceFilter(failingExistenceFilter{err: filterErr}), WithFilterFailureMode(tc.mode))
			if err != nil {
				t.Fatal(err)
			}
			loads := 0
			_, err = GetOrLoad(coord, coordinatorRequest(context.Background(), "error").EnableBloomFilter(), func(context.Context) (coordinatorPayload, error) {
				loads++
				return coordinatorPayload{ID: 3}, nil
			})
			var rejected ErrRejectedByBloomFilter
			if tc.wantLoad && (err != nil || loads != 1) {
				t.Fatalf("err=%v loads=%d; want filter error to fail open", err, loads)
			}
			if tc.wantReject && (!errors.As(err, &rejected) || loads != 0) {
				t.Fatalf("err=%v loads=%d; want filter error to fail closed", err, loads)
			}
		})
	}
}

type addFailingExistenceFilter struct{ err error }

func (f addFailingExistenceFilter) Ready() bool { return false }
func (f addFailingExistenceFilter) MayContain(context.Context, string) (bool, error) {
	return true, nil
}
func (f addFailingExistenceFilter) Add(context.Context, string) error { return f.err }

func TestCoordinator_BloomAddFailureOnlyObservesSuccessfulLoad(t *testing.T) {
	backend := newCoordinatorFakeCache()
	addErr := errors.New("add unavailable")
	events := make([]Event, 0, 1)
	coord, err := NewCoordinator(backend,
		WithExistenceFilter(addFailingExistenceFilter{err: addErr}),
		WithObserver(func(event Event) { events = append(events, event) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err := GetOrLoad(coord, coordinatorRequest(context.Background(), "add-error").EnableBloomFilter(), func(context.Context) (coordinatorPayload, error) {
		return coordinatorPayload{ID: 4}, nil
	})
	if err != nil || got.ID != 4 {
		t.Fatalf("got=%+v err=%v; want loaded value despite Add error", got, err)
	}
	if len(events) != 2 || events[0] != EventCacheMiss || events[1] != EventFilterError {
		t.Fatalf("events=%v; want cache miss followed by EventFilterError", events)
	}
}

type nonComparableValueCache struct{ values map[string]string }

func (c nonComparableValueCache) Get(_ context.Context, key string, _ *CacheOption) (string, error) {
	value, ok := c.values[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return value, nil
}
func (c nonComparableValueCache) Set(_ context.Context, key string, value interface{}, _ *CacheOption) error {
	c.values[key] = value.(string)
	return nil
}
func (nonComparableValueCache) Delete(context.Context, ...string) error { return nil }
func (nonComparableValueCache) Close() error                            { return nil }
func (nonComparableValueCache) Wait() error                             { return nil }
func (nonComparableValueCache) GetLevel() Level                         { return Local }

func TestCoordinatorFor_NonComparableValueBackendsDoNotShareCoordinator(t *testing.T) {
	// Collapsing these values by type and level would make the second caller use
	// the first backend's storage.
	first := nonComparableValueCache{values: make(map[string]string)}
	second := nonComparableValueCache{values: make(map[string]string)}
	firstCoordinator, err := coordinatorFor(first)
	if err != nil {
		t.Fatal(err)
	}
	secondCoordinator, err := coordinatorFor(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstCoordinator == secondCoordinator {
		t.Fatal("distinct non-comparable backends shared a coordinator")
	}
}
