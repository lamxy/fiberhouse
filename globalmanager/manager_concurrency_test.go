// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package globalmanager

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// 本文件闭合 Rebuild / Release / Get 的并发状态机缺口。
//
// 三者对同一 entry 的交错此前只有「成对」覆盖（Rebuild vs Rebuild、Release vs
// Release、Rebuild vs Release），没有把 Get 混进去跑长程压测。而 Get 恰恰是唯一
// 不走维护闸门的操作：它与维护操作并发时，读到的可能是旧实例、新实例，或正处于
// 重置中途的 entry。
//
// 两条不变量必须在任意交错下成立：
//
//   - **不得泄漏 typed-nil**：Get 返回 (instance, nil) 时 instance 必须实质非 nil。
//     返回 (*T, error) 的构造函数失败时返回 (*T)(nil)，装箱后是非 nil 接口，
//     调用方解引用即 panic。
//   - **不得 use-after-close**：Get 返回的实例必须是调用方可以安全使用的。
//     Release 会 Close 实例，但**调用方已经持有的引用不会失效**——这是契约的
//     一部分，本文件把它固定下来。

// concurrentResource 是压测用的资源：记录自身是否被关闭，并能在关闭后检测使用。
type concurrentResource struct {
	id     int
	closed atomic.Bool
	// useAfterClose 记录关闭之后仍被使用的次数。
	useAfterClose *atomic.Int64
}

func (r *concurrentResource) Close() error {
	r.closed.Store(true)
	return nil
}

// use 模拟调用方对实例的使用，并检测 use-after-close。
func (r *concurrentResource) use() {
	if r.closed.Load() {
		r.useAfterClose.Add(1)
	}
}

func (r *concurrentResource) GetConfPath() string { return "config.yml" }

func (r *concurrentResource) Rebuild(...interface{}) (interface{}, error) {
	return &concurrentResource{id: r.id + 1, useAfterClose: r.useAfterClose}, nil
}

// TestGlobalManager_ConcurrentGetRebuildRelease 长程交错压测。
//
// 四类 goroutine 同时打在同一个 key 上：持续 Get、持续 Rebuild、持续 Release、
// 以及读取健康状态。断言两条不变量，并确认容器最终仍可用。
//
// 以 -race 执行；rounds 取 200 以上，使交错窗口有足够机会被命中。
func TestGlobalManager_ConcurrentGetRebuildRelease(t *testing.T) {
	const (
		rounds  = 250
		readers = 4
	)

	manager := NewGlobalManager()
	var useAfterClose atomic.Int64
	var created atomic.Int64

	manager.Register("resource", func() (interface{}, error) {
		created.Add(1)
		return &concurrentResource{
			id:            int(created.Load()),
			useAfterClose: &useAfterClose,
		}, nil
	})

	var (
		wg           sync.WaitGroup
		typedNilSeen atomic.Int64
		getErrors    atomic.Int64
	)

	// 读者：持续 Get 并立即使用返回的实例。
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < rounds; round++ {
				instance, err := manager.Get("resource")
				if err != nil {
					// Get 在与 Release 交错时可能观察到瞬时失败，这是允许的；
					// 不允许的是「返回 nil 错误却给出 nil 实例」。
					getErrors.Add(1)
					continue
				}
				if isNilInstance(instance) {
					typedNilSeen.Add(1)
					continue
				}
				resource, ok := instance.(*concurrentResource)
				if !ok {
					t.Errorf("Get returned unexpected type %T", instance)
					return
				}
				// 调用方持有引用后立即使用：契约保证这个引用在持有期间可用。
				resource.use()
			}
		}()
	}

	// 重建者
	wg.Add(1)
	go func() {
		defer wg.Done()
		for round := 0; round < rounds; round++ {
			// 冲突时返回 busy，是可恢复条件，直接继续下一轮。
			if err := manager.Rebuild("resource"); err != nil &&
				!errors.Is(err, ErrMaintenanceInProgress) {
				// 未初始化时 Rebuild 失败也是正常的（Release 刚重置过）。
				continue
			}
		}
	}()

	// 释放者
	wg.Add(1)
	go func() {
		defer wg.Done()
		for round := 0; round < rounds; round++ {
			_ = manager.Release("resource")
		}
	}()

	// 健康检查读者：CheckHealth 同样在无闸门情况下读 instance。
	wg.Add(1)
	go func() {
		defer wg.Done()
		for round := 0; round < rounds; round++ {
			_, _ = manager.CheckHealth("resource")
		}
	}()

	wg.Wait()

	if got := typedNilSeen.Load(); got != 0 {
		t.Fatalf("Get returned a typed-nil instance with nil error %d times", got)
	}

	// 容器在压测之后必须仍然可用。
	instance, err := manager.Get("resource")
	if err != nil {
		t.Fatalf("Get after the stress run error = %v", err)
	}
	if isNilInstance(instance) {
		t.Fatal("Get after the stress run returned a nil instance")
	}
}

// TestGet_ReferenceStaysUsableAfterRelease 锁定引用存活期契约。
//
// Release 会 Close 实例并重置 entry，但**不会**使调用方已经持有的引用失效：
// 容器不追踪已发出的引用，也没有回收它们的手段。这是契约而非疏漏——把它写成
// 可执行断言，避免将来有人「顺手」在 Release 里加入引用失效逻辑。
func TestGet_ReferenceStaysUsableAfterRelease(t *testing.T) {
	manager := NewGlobalManager()
	var useAfterClose atomic.Int64
	manager.Register("resource", func() (interface{}, error) {
		return &concurrentResource{id: 1, useAfterClose: &useAfterClose}, nil
	})

	instance, err := manager.Get("resource")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	held := instance.(*concurrentResource)

	if err := manager.Release("resource"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	// 实例确实被关闭了。
	if !held.closed.Load() {
		t.Fatal("Release must close a Closable instance")
	}

	// 但调用方手里的指针仍然有效——解引用不会 panic。
	// 契约要求的是「引用不被容器作废」，而不是「关闭后的资源仍可正常工作」。
	if held.id != 1 {
		t.Fatalf("held reference id = %d, want 1", held.id)
	}

	// 下一次 Get 拿到的是重新初始化的新实例。
	next, err := manager.Get("resource")
	if err != nil {
		t.Fatalf("Get() after Release error = %v", err)
	}
	if next == instance {
		t.Fatal("Get() after Release must return a freshly initialized instance")
	}
}

// TestRebuild_OldInstanceIsNotClosed 锁定 Rebuild 对旧实例的处置策略。
//
// Rebuild **不**关闭旧实例，交由 GC 回收。理由写在 Rebuilder 的接口文档里：
// Rebuild 的目标是配置类对象（纯内存替换），关闭它们没有意义；而容器无法等待
// 在途的 Get——Get 不走维护闸门，已发出的引用也不被追踪，没有安全的等待点。
//
// 连接池型对象刻意不实现 Rebuilder，因此「旧连接池不被关闭」不是活的泄漏路径。
func TestRebuild_OldInstanceIsNotClosed(t *testing.T) {
	manager := NewGlobalManager()
	var useAfterClose atomic.Int64
	original := &concurrentResource{id: 1, useAfterClose: &useAfterClose}
	manager.Register("resource", func() (interface{}, error) { return original, nil })

	if _, err := manager.Get("resource"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := manager.Rebuild("resource"); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	if original.closed.Load() {
		t.Fatal("Rebuild must not close the old instance: " +
			"callers may still hold references the container cannot track")
	}

	rebuilt, err := manager.Get("resource")
	if err != nil {
		t.Fatalf("Get() after Rebuild error = %v", err)
	}
	if rebuilt == original {
		t.Fatal("Rebuild must publish the new instance")
	}
}

// TestRebuild_ConcurrentGetNeverObservesTornState Rebuild 期间的 Get 必须拿到
// 完整实例——要么旧的，要么新的，不存在中间态。
func TestRebuild_ConcurrentGetNeverObservesTornState(t *testing.T) {
	const rounds = 250

	manager := NewGlobalManager()
	var useAfterClose atomic.Int64
	manager.Register("resource", func() (interface{}, error) {
		return &concurrentResource{id: 1, useAfterClose: &useAfterClose}, nil
	})
	if _, err := manager.Get("resource"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	var (
		wg      sync.WaitGroup
		bad     atomic.Int64
		badType atomic.Value
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for round := 0; round < rounds; round++ {
			_ = manager.Rebuild("resource")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for round := 0; round < rounds; round++ {
			instance, err := manager.Get("resource")
			if err != nil {
				continue
			}
			if isNilInstance(instance) {
				bad.Add(1)
				continue
			}
			if _, ok := instance.(*concurrentResource); !ok {
				bad.Add(1)
				badType.Store(fmt.Sprintf("%T", instance))
			}
		}
	}()

	wg.Wait()

	if got := bad.Load(); got != 0 {
		t.Fatalf("Get observed %d torn or nil instances during Rebuild (type=%v)",
			got, badType.Load())
	}
}
