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
	mu       sync.Mutex
	values   map[string]string
	getCount int
	setCount int
	getErr   error
	setErr   error
}

func newCoordinatorFakeCache() *coordinatorFakeCache {
	return &coordinatorFakeCache{values: make(map[string]string)}
}

func (c *coordinatorFakeCache) Get(_ context.Context, key string, _ *CacheOption) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getCount++
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

	for loads.Load() == 0 {
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
	coord, _ := NewCoordinator(backend)
	first, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	loader := func(context.Context) (coordinatorPayload, error) {
		close(started)
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
	cancel()
	close(release)
	if !errors.Is(<-firstDone, context.Canceled) {
		t.Fatal("first caller did not observe cancellation")
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
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
