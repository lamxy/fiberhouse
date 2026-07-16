package bufferpool

import (
	"bytes"
	"testing"
)

func TestBinaryCeil_ZeroUsesOneByteMinimum(t *testing.T) {
	if got := binaryCeil(0); got != 1 {
		t.Fatalf("binaryCeil(0) = %d, want 1", got)
	}
}

func TestBinaryCeil_Boundaries(t *testing.T) {
	for input, want := range map[uint32]uint32{
		1:       1,
		2:       2,
		3:       4,
		7:       8,
		8:       8,
		9:       16,
		1023:    1024,
		1024:    1024,
		1025:    2048,
		1 << 31: 1 << 31,
	} {
		if got := binaryCeil(input); got != want {
			t.Errorf("binaryCeil(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestNewBufferPool_ZeroBoundCreatesOneByteShard(t *testing.T) {
	pool := NewBufferPool(0, 0)
	if pool.begin != 1 || pool.end != 1 {
		t.Fatalf("zero-bound pool range = [%d,%d], want [1,1]", pool.begin, pool.end)
	}
	if _, ok := pool.shards[1]; !ok || len(pool.shards) != 1 {
		t.Fatalf("zero-bound shards = %#v", pool.shards)
	}
	buffer := pool.Get(0)
	if buffer.Cap() < 1 || buffer.Len() != 0 {
		t.Fatalf("Get(0) = len %d cap %d", buffer.Len(), buffer.Cap())
	}
	pool.Put(buffer)
}

func TestBufferPool_ShardCapacityOversizeAndReset(t *testing.T) {
	pool := NewBufferPool(16, 200)
	if pool.begin != 16 || pool.end != 256 || len(pool.shards) != 5 {
		t.Fatalf("pool = begin %d end %d shards %d", pool.begin, pool.end, len(pool.shards))
	}
	for requested, wantCapacity := range map[int]int{0: 16, 1: 16, 16: 16, 17: 32, 129: 256} {
		buffer := pool.Get(requested)
		if buffer.Cap() < wantCapacity || buffer.Len() != 0 {
			t.Errorf("Get(%d) = len %d cap %d, want cap >= %d", requested, buffer.Len(), buffer.Cap(), wantCapacity)
		}
		pool.Put(buffer)
	}

	buffer := pool.Get(20)
	buffer.WriteString("sensitive data")
	pool.Put(buffer)
	if buffer.Len() != 0 || buffer.String() != "" {
		t.Fatalf("Put() retained data in returned buffer: %q", buffer.String())
	}
	reused := pool.Get(20)
	if reused.Len() != 0 || reused.String() != "" {
		t.Fatalf("reused buffer retained data: %q", reused.String())
	}
	pool.Put(reused)

	oversize := pool.Get(300)
	if oversize.Cap() < 300 {
		t.Fatalf("oversize buffer capacity = %d, want >= 300", oversize.Cap())
	}
	pool.Put(oversize)
	pool.Put(nil)
	if got := pool.Get(-1); got.Cap() < 16 {
		t.Fatalf("Get(-1) capacity = %d, want >= 16", got.Cap())
	}
}

func TestGenericPool_ResetsValues(t *testing.T) {
	type pooledValue struct {
		items []string
	}
	created := 0
	resets := 0
	pool := NewPool(func() *pooledValue {
		created++
		return &pooledValue{}
	}, func(value *pooledValue) {
		resets++
		value.items = value.items[:0]
	})
	value := pool.Get()
	value.items = append(value.items, "item")
	pool.Put(value)
	if len(value.items) != 0 || resets != 1 || created != 1 {
		t.Fatalf("put value = %#v, resets = %d, created = %d", value, resets, created)
	}
	_ = pool.Get()

	withoutReset := NewPool(func() *bytes.Buffer { return &bytes.Buffer{} }, nil)
	plain := withoutReset.Get()
	plain.WriteString("retained")
	withoutReset.Put(plain)
	if got := plain.String(); got != "retained" {
		t.Fatalf("pool without reset changed value to %q", got)
	}
	_ = withoutReset.Get()

	builderPool := createStringBuilderPool()
	builder := builderPool.Get()
	builder.WriteString("text")
	builderPool.Put(builder)
	if got := builder.String(); got != "" {
		t.Fatalf("string builder was not reset: %q", got)
	}
	_ = builderPool.Get()
	if Max(2, 1) != 2 || Max(1, 2) != 2 {
		t.Fatal("Max() returned an unexpected value")
	}
}
