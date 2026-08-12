package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMemoryBloomFilter_ReadinessAndPreload(t *testing.T) {
	filter, err := NewMemoryBloomFilter(100, 0.01, 3)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"known", "absent"} {
		mayContain, err := filter.MayContain(context.Background(), key)
		if err != nil || !mayContain {
			t.Fatalf("unready filter MayContain(%q) = %v, %v; want true, nil", key, mayContain, err)
		}
	}
	if filter.Ready() {
		t.Fatal("new filter is ready; want unready")
	}

	if err := filter.Preload(context.Background(), []string{"known"}); err != nil {
		t.Fatal(err)
	}
	if !filter.Ready() {
		t.Fatal("successful preload did not make filter ready")
	}
	for _, tc := range []struct {
		key  string
		want bool
	}{
		{key: "known", want: true},
		{key: "absent", want: false},
	} {
		mayContain, err := filter.MayContain(context.Background(), tc.key)
		if err != nil || mayContain != tc.want {
			t.Fatalf("ready filter MayContain(%q) = %v, %v; want %v, nil", tc.key, mayContain, err, tc.want)
		}
	}

	filter.Reset()
	if filter.Ready() {
		t.Fatal("Reset left filter ready")
	}
	mayContain, err := filter.MayContain(context.Background(), "absent")
	if err != nil || !mayContain {
		t.Fatalf("reset filter MayContain = %v, %v; want true, nil", mayContain, err)
	}
}

func TestMemoryBloomFilter_AddDoesNotMakeFilterReady(t *testing.T) {
	filter, err := NewMemoryBloomFilter(100, 0.01, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := filter.Add(context.Background(), "loaded"); err != nil {
		t.Fatal(err)
	}
	if filter.Ready() {
		t.Fatal("Add made filter ready")
	}
	mayContain, err := filter.MayContain(context.Background(), "other")
	if err != nil || !mayContain {
		t.Fatalf("unready filter MayContain = %v, %v; want true, nil", mayContain, err)
	}
}

func TestMemoryBloomFilter_InvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name      string
		capacity  uint
		errorRate float64
	}{
		{name: "zero capacity", capacity: 0, errorRate: 0.01},
		{name: "zero error rate", capacity: 1, errorRate: 0},
		{name: "one error rate", capacity: 1, errorRate: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewMemoryBloomFilter(tc.capacity, tc.errorRate, 1); err == nil {
				t.Fatal("NewMemoryBloomFilter unexpectedly accepted invalid configuration")
			}
		})
	}
}

func TestMemoryBloomFilter_RejectsExtremeCapacityAndShardCount(t *testing.T) {
	maxUint := ^uint(0)
	if _, err := NewMemoryBloomFilter(maxUint-1, 0.01, 3); err == nil {
		t.Fatal("NewMemoryBloomFilter accepted an extreme capacity that cannot be safely allocated")
	}
	if _, err := NewMemoryBloomFilter(1, 0.01, int(^uint(0)>>1)); err == nil {
		t.Fatal("NewMemoryBloomFilter accepted an extreme shard count")
	}
}

func TestMemoryBloomFilter_NormalizesShardCountToNextPowerOfTwo(t *testing.T) {
	for _, tc := range []struct {
		requested int
		want      int
	}{
		{requested: 3, want: 4},
		{requested: 0, want: 1},
		{requested: 17, want: 32},
	} {
		t.Run("shards", func(t *testing.T) {
			filter, err := NewMemoryBloomFilter(100, 0.01, tc.requested)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(filter.shards); got != tc.want {
				t.Fatalf("shards = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMemoryBloomFilter_PreloadAndMayContainAreConcurrentSafe(t *testing.T) {
	filter, err := NewMemoryBloomFilter(100, 0.01, 3)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if _, err := filter.MayContain(context.Background(), "key"); err != nil {
					t.Errorf("MayContain: %v", err)
				}
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 25 {
				if err := filter.Preload(context.Background(), []string{"key"}); err != nil {
					t.Errorf("Preload: %v", err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestMemoryBloomFilter_PreloadPreservesSuccessfulConcurrentAdd(t *testing.T) {
	filter, err := NewMemoryBloomFilter(100, 0.01, 1)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	preloadDone := make(chan error, 1)
	go func() {
		preloadDone <- filter.preload(context.Background(), []string{"preloaded"}, func() {
			close(started)
			<-release
		})
	}()
	<-started
	addDone := make(chan error, 1)
	go func() { addDone <- filter.Add(context.Background(), "added") }()
	select {
	case err := <-addDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Add blocked behind off-side preload build")
	}
	close(release)
	if err := <-preloadDone; err != nil {
		t.Fatal(err)
	}
	if !filter.Ready() {
		t.Fatal("preload did not make filter ready")
	}
	mayContain, err := filter.MayContain(context.Background(), "added")
	if err != nil || !mayContain {
		t.Fatalf("successful Add was lost by Preload: MayContain=%v err=%v", mayContain, err)
	}
}

func TestMemoryBloomFilter_PreloadKeepsReadyLookupsResponsive(t *testing.T) {
	filter, err := NewMemoryBloomFilter(100, 0.01, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := filter.Preload(context.Background(), []string{"known"}); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- filter.preload(context.Background(), []string{"replacement"}, func() {
			close(started)
			<-release
		})
	}()
	<-started
	lookup := make(chan struct {
		mayContain bool
		err        error
	}, 1)
	go func() {
		mayContain, err := filter.MayContain(context.Background(), "known")
		lookup <- struct {
			mayContain bool
			err        error
		}{mayContain, err}
	}()
	select {
	case result := <-lookup:
		if result.err != nil || !result.mayContain {
			t.Fatalf("MayContain during preload = %v, %v; want true, nil", result.mayContain, result.err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("MayContain blocked behind off-side preload build")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMemoryBloomFilter_CanceledPreloadPreservesPreviousReadySet(t *testing.T) {
	filter, err := NewMemoryBloomFilter(100, 0.01, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := filter.Preload(context.Background(), []string{"known"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- filter.preload(ctx, []string{"replacement"}, func() {
			close(started)
			<-release
		})
	}()
	<-started
	cancel()
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Preload error = %v, want context cancellation", err)
	}
	for _, tc := range []struct {
		key  string
		want bool
	}{
		{key: "known", want: true},
		{key: "replacement", want: false},
	} {
		mayContain, err := filter.MayContain(context.Background(), tc.key)
		if err != nil || mayContain != tc.want {
			t.Fatalf("MayContain(%q) = %v, %v; want %v, nil", tc.key, mayContain, err, tc.want)
		}
	}
}
