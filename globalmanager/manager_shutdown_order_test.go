// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package globalmanager

import (
	"errors"
	"sync"
	"testing"
)

// orderRecorder 记录关闭顺序，供断言使用。
type orderRecorder struct {
	mu    sync.Mutex
	order []string
}

func (r *orderRecorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append(r.order, name)
}

func (r *orderRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

func (r *orderRecorder) indexOf(name string) int {
	for i, entry := range r.snapshot() {
		if entry == name {
			return i
		}
	}
	return -1
}

// orderedChild 是一个普通的可关闭子资源，关闭幂等。
type orderedChild struct {
	name     string
	recorder *orderRecorder
	closed   bool
	mu       sync.Mutex
}

func (c *orderedChild) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.recorder.record(c.name)
	return nil
}

func (c *orderedChild) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// orderedComposite 模拟 Level2Cache：持有子资源，关闭时先排空再关子资源。
// 排空期间会检查子资源是否已被提前关闭——那正是要防止的 use-after-close。
type orderedComposite struct {
	name        string
	recorder    *orderRecorder
	children    []*orderedChild
	owns        bool
	drainFailed bool
	mu          sync.Mutex
}

func (c *orderedComposite) OwnsChildren() bool { return c.owns }

func (c *orderedComposite) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recorder.record(c.name)

	// 排空阶段：此时子资源必须仍然可用。若已被提前关闭，说明容器的关闭顺序错误。
	for _, child := range c.children {
		if child.isClosed() {
			c.drainFailed = true
			return errors.New("use-after-close: child " + child.name + " was closed before the composite drained")
		}
	}

	if !c.owns {
		return nil
	}
	for _, child := range c.children {
		if err := child.Close(); err != nil {
			return err
		}
	}
	return nil
}

// TestReleaseAll_ClosesCompositesBeforePlainObjects 锁定关闭顺序契约。
//
// 容器允许同一底层资源以别名注册在多个 key 下，也允许组合对象持有子资源。组合对象
// 需要先排空在途异步操作再关闭子资源；若容器先关掉子资源，排空期间的操作就会打在
// 已关闭的资源上。sync.Map.Range 的迭代顺序未定义，所以这个约束必须由 ReleaseAll
// 显式保证，不能依赖运气。
//
// 本用例重复多轮：顺序缺陷是概率性的，单轮通过不能证明契约成立。
func TestReleaseAll_ClosesCompositesBeforePlainObjects(t *testing.T) {
	const rounds = 200

	for round := 0; round < rounds; round++ {
		recorder := &orderRecorder{}
		manager := NewGlobalManager()

		local := &orderedChild{name: "local", recorder: recorder}
		remote := &orderedChild{name: "remote", recorder: recorder}
		composite := &orderedComposite{
			name:     "level2",
			recorder: recorder,
			children: []*orderedChild{local, remote},
			owns:     true,
		}

		// 与示例应用同构：子资源各自注册，remote 还以别名再注册一次，
		// 组合对象持有两者。
		register(t, manager, "local", local)
		register(t, manager, "remote", remote)
		register(t, manager, "remote-alias", remote)
		register(t, manager, "level2", composite)

		manager.ReleaseAll(true)

		if composite.drainFailed {
			t.Fatalf("round %d: composite drained against an already-closed child; order = %v",
				round, recorder.snapshot())
		}

		compositeAt := recorder.indexOf("level2")
		if compositeAt == -1 {
			t.Fatalf("round %d: composite was never closed; order = %v", round, recorder.snapshot())
		}
		for _, child := range []string{"local", "remote"} {
			childAt := recorder.indexOf(child)
			if childAt == -1 {
				t.Fatalf("round %d: child %q was never closed; order = %v", round, child, recorder.snapshot())
			}
			if childAt < compositeAt {
				t.Fatalf("round %d: child %q closed at %d, before composite at %d; order = %v",
					round, child, childAt, compositeAt, recorder.snapshot())
			}
		}
	}
}

// nestedComposite 是一个可以持有其他组合对象的组合对象，用于验证嵌套排序。
type nestedComposite struct {
	name     string
	recorder *orderRecorder
	children []closableProbe
	owns     bool
	closed   bool
	violated bool
	mu       sync.Mutex
}

// closableProbe 是 nestedComposite 子资源的最小契约：能关闭、能被查询是否已关闭。
type closableProbe interface {
	Close() error
	isClosed() bool
}

func (c *nestedComposite) OwnsChildren() bool { return c.owns }

func (c *nestedComposite) OwnsInstance(candidate interface{}) bool {
	if !c.owns || candidate == nil {
		return false
	}
	for _, child := range c.children {
		if candidate == child {
			return true
		}
	}
	return false
}

func (c *nestedComposite) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *nestedComposite) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.recorder.record(c.name)

	// 排空阶段：子资源必须仍然可用。
	for _, child := range c.children {
		if child.isClosed() {
			c.mu.Lock()
			c.violated = true
			c.mu.Unlock()
		}
	}

	if !c.owns {
		return nil
	}
	for _, child := range c.children {
		if err := child.Close(); err != nil {
			return err
		}
	}
	return nil
}

// TestReleaseAll_NestedCompositesReleaseOutermostFirst 锁定嵌套组合对象的关闭顺序。
//
// 两层排序（组合对象 vs 普通对象）不足以处理「组合对象持有另一个组合对象」的情况：
// 两者都在第一轮，轮内顺序仍取决于 sync.Map.Range，外层可能在内层已被关闭之后才
// 开始排空。容器通过 CompositeOwnershipInspector 识别嵌套关系并按深度排序。
//
// 本用例重复多轮：顺序缺陷是概率性的，单轮通过不能证明契约成立。
func TestReleaseAll_NestedCompositesReleaseOutermostFirst(t *testing.T) {
	const rounds = 300

	for round := 0; round < rounds; round++ {
		recorder := &orderRecorder{}
		manager := NewGlobalManager()

		leaf := &orderedChild{name: "leaf", recorder: recorder}
		inner := &nestedComposite{
			name:     "inner",
			recorder: recorder,
			children: []closableProbe{leaf},
			owns:     true,
		}
		outer := &nestedComposite{
			name:     "outer",
			recorder: recorder,
			children: []closableProbe{inner},
			owns:     true,
		}

		register(t, manager, "leaf", leaf)
		register(t, manager, "inner", inner)
		register(t, manager, "outer", outer)

		manager.ReleaseAll(true)

		if outer.violated {
			t.Fatalf("round %d: outer drained against an already-closed inner; order = %v",
				round, recorder.snapshot())
		}
		if inner.violated {
			t.Fatalf("round %d: inner drained against an already-closed leaf; order = %v",
				round, recorder.snapshot())
		}

		outerAt, innerAt, leafAt := recorder.indexOf("outer"), recorder.indexOf("inner"), recorder.indexOf("leaf")
		if outerAt == -1 || innerAt == -1 || leafAt == -1 {
			t.Fatalf("round %d: not everything was closed; order = %v", round, recorder.snapshot())
		}
		if !(outerAt < innerAt && innerAt < leafAt) {
			t.Fatalf("round %d: want outer < inner < leaf, got %v", round, recorder.snapshot())
		}
	}
}

// TestReleaseAll_AliasedResourceIsClosedOnce 确认别名注册不会导致重复关闭被观测到。
//
// 同一个 *orderedChild 注册在两个 key 下，并且还被组合对象持有：一轮释放会对它发起
// 三次 Close。幂等实现必须让实际关闭只发生一次。
func TestReleaseAll_AliasedResourceIsClosedOnce(t *testing.T) {
	recorder := &orderRecorder{}
	manager := NewGlobalManager()

	shared := &orderedChild{name: "shared", recorder: recorder}
	composite := &orderedComposite{
		name:     "composite",
		recorder: recorder,
		children: []*orderedChild{shared},
		owns:     true,
	}

	register(t, manager, "shared", shared)
	register(t, manager, "shared-alias", shared)
	register(t, manager, "composite", composite)

	manager.ReleaseAll(true)

	closes := 0
	for _, entry := range recorder.snapshot() {
		if entry == "shared" {
			closes++
		}
	}
	if closes != 1 {
		t.Fatalf("aliased resource closed %d times, want 1; order = %v", closes, recorder.snapshot())
	}
}

// TestReleaseAll_NonOwningCompositeIsNotPrioritized 确认 OwnsChildren() 为 false 的
// 组合对象按普通对象处理：它借用子资源而不负责关闭，因此没有顺序约束。
func TestReleaseAll_NonOwningCompositeIsNotPrioritized(t *testing.T) {
	recorder := &orderRecorder{}
	manager := NewGlobalManager()

	child := &orderedChild{name: "child", recorder: recorder}
	borrower := &orderedComposite{
		name:     "borrower",
		recorder: recorder,
		children: []*orderedChild{child},
		owns:     false,
	}

	register(t, manager, "child", child)
	register(t, manager, "borrower", borrower)

	manager.ReleaseAll(true)

	// 借用者不关闭子资源，容器仍会各自释放两者。
	if borrower.drainFailed {
		// 借用者不拥有子资源，即使子资源先关也不算违约；这里只确认没有误报。
		t.Log("borrower observed a closed child; this is allowed for a non-owning composite")
	}
	if !child.isClosed() {
		t.Fatal("child was not closed")
	}
	if recorder.indexOf("borrower") == -1 {
		t.Fatal("borrower was not closed")
	}
}

// TestReleaseAllWithObserver_ReportsFailuresPerKey 确认释放失败按 key 上报，
// 且一个失败不会中断其余对象的回收。
func TestReleaseAllWithObserver_ReportsFailuresPerKey(t *testing.T) {
	manager := NewGlobalManager()
	closed := 0
	failing := &lifecycleClosable{closed: &closed, err: errors.New("close failed")}
	healthy := &lifecycleClosable{closed: &closed}

	register(t, manager, "failing", failing)
	register(t, manager, "healthy", healthy)

	reported := make(map[string]error)
	manager.ReleaseAllWithObserver(func(name KeyName, err error) {
		reported[name] = err
	}, true)

	if len(reported) != 1 {
		t.Fatalf("observer received %d reports, want 1: %v", len(reported), reported)
	}
	if err, ok := reported["failing"]; !ok || err == nil {
		t.Fatalf("observer did not report the failing key: %v", reported)
	}
	// 失败不中断回收：两个对象的 Close 都被调用过。
	if closed != 2 {
		t.Fatalf("Close() calls = %d, want 2 (a failure must not abort the sweep)", closed)
	}
}

// TestReleaseAllWithObserver_NilObserverIsSafe 确认 observer 为 nil 时错误被丢弃而不 panic。
func TestReleaseAllWithObserver_NilObserverIsSafe(t *testing.T) {
	manager := NewGlobalManager()
	closed := 0
	register(t, manager, "failing", &lifecycleClosable{closed: &closed, err: errors.New("close failed")})

	manager.ReleaseAllWithObserver(nil, true)

	if closed != 1 {
		t.Fatalf("Close() calls = %d, want 1", closed)
	}
}

// TestReleaseAll_RequiresConfirmation 确认未确认时不释放任何资源。
func TestReleaseAll_RequiresConfirmation(t *testing.T) {
	manager := NewGlobalManager()
	closed := 0
	register(t, manager, "resource", &lifecycleClosable{closed: &closed})

	manager.ReleaseAll()
	manager.ReleaseAllWithObserver(func(KeyName, error) {})
	manager.ReleaseAllWithObserver(func(KeyName, error) {}, false)

	if closed != 0 {
		t.Fatalf("Close() calls = %d without confirmation, want 0", closed)
	}
}

func register(t *testing.T, manager *GlobalManager, key string, instance interface{}) {
	t.Helper()
	if !manager.Register(key, func() (interface{}, error) { return instance, nil }) {
		t.Fatalf("Register(%q) returned false", key)
	}
	if _, err := manager.Get(key); err != nil {
		t.Fatalf("Get(%q) error = %v", key, err)
	}
}
