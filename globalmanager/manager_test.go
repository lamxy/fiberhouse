package globalmanager

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// helper: 获取 *GlobalManager 全新实例
func newMgr() *GlobalManager {
	return NewGlobalManager()
}

func TestNewGlobalManager(t *testing.T) {
	m1 := NewGlobalManager()
	if m1 == nil {
		t.Fatal("NewGlobalManager 返回 nil")
	}
	m2 := NewGlobalManager()
	if m1 == m2 {
		t.Error("NewGlobalManager 应返回全新实例，而不是同一实例")
	}
}

func TestNewGlobalManagerOnce(t *testing.T) {
	s1 := NewGlobalManagerOnce()
	s2 := NewGlobalManagerOnce()
	if s1 == nil || s2 == nil {
		t.Fatal("NewGlobalManagerOnce 返回 nil")
	}
	if s1 != s2 {
		t.Error("NewGlobalManagerOnce 未实现单例语义")
	}
}

func TestRegisterBasic(t *testing.T) {
	m := newMgr()

	ok := m.Register("db", func() (interface{}, error) { return "INSTANCE", nil })
	if !ok {
		t.Fatalf("首次注册应返回 true")
	}

	// 重复注册应失败
	ok = m.Register("db", func() (interface{}, error) { return "OTHER", nil })
	if ok {
		t.Fatalf("重复注册应返回 false")
	}

	// nil initializer
	ok = m.Register("nil_case", nil)
	if ok {
		t.Fatalf("nil 初始化器不应注册成功")
	}

	if !m.IsRegistered("db") {
		t.Errorf("IsRegistered 期望为 true")
	}
	if m.IsRegistered("not_exist") {
		t.Errorf("不存在的 key 不应返回 true")
	}
}

func TestRegistersBatch(t *testing.T) {
	m := newMgr()
	batch := InitializerMap{
		"s1": func() (interface{}, error) { return 1, nil },
		"s2": func() (interface{}, error) { return 2, nil },
		"s3": func() (interface{}, error) { return 3, nil },
	}
	m.Registers(batch)
	for k := range batch {
		if !m.IsRegistered(k) {
			t.Fatalf("批量注册失败: %s", k)
		}
	}
}

func TestGetSuccessLazyInitOnce(t *testing.T) {
	m := newMgr()
	var initCount int32
	type Obj struct {
		ID int
	}
	m.Register("once_obj", func() (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&initCount, 1)
		return &Obj{ID: 7}, nil
	})

	// 并发获取
	const n = 40
	var wg sync.WaitGroup
	wg.Add(n)
	results := make([]interface{}, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			inst, err := m.Get("once_obj")
			results[i] = inst
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("第 %d 次 Get 返回错误: %v", i, e)
		}
	}
	if c := atomic.LoadInt32(&initCount); c != 1 {
		t.Fatalf("初始化函数应只执行一次，实际: %d", c)
	}
	first := results[0]
	for i, r := range results {
		if r != first {
			t.Fatalf("所有获取返回的应是同一实例，index=%d 不一致", i)
		}
	}
	obj := first.(*Obj)
	if obj.ID != 7 {
		t.Errorf("实例内容不符合预期")
	}
}

func TestGetNotFound(t *testing.T) {
	m := newMgr()
	_, err := m.Get("absent")
	if err == nil || !contains(err.Error(), "not found") {
		t.Fatalf("未找到对象应返回 not found 错误, got: %v", err)
	}
}

func TestInitializerReturnError(t *testing.T) {
	m := newMgr()
	var calls int32
	m.Register("err_svc", func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New("boom-error")
	})
	_, err1 := m.Get("err_svc")
	if err1 == nil || !contains(err1.Error(), "boom-error") {
		t.Fatalf("首次 Get 应返回初始化错误, got: %v", err1)
	}
	_, err2 := m.Get("err_svc")
	if err2 == nil || !contains(err2.Error(), "boom-error") {
		t.Fatalf("二次 Get 应复用错误, got: %v", err2)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("初始化出错后允许再次调用 initializer, 调用次数=%d", calls)
	}
}

func TestInitializerPanic(t *testing.T) {
	m := newMgr()
	var calls int32
	m.Register("panic_svc", func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		panic("panic-xyz")
	})
	_, err := m.Get("panic_svc")
	if err == nil || !contains(err.Error(), "panic-xyz") {
		t.Fatalf("panic 应包装为错误返回, got: %v", err)
	}
	// 初始化异常，再次获取允许重新执行 initializer
	_, err2 := m.Get("panic_svc")
	if err2 == nil {
		t.Fatalf("第二次获取仍应返回错误")
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("panic 场景 initializer 允许重复执行, 次数=%d", calls)
	}
}

func TestUnregisterIfExists(t *testing.T) {
	m := newMgr()
	// 通过接口探测 Unregister
	type unreg interface {
		Unregister(name KeyName)
	}
	u, ok := any(m).(unreg)
	if !ok {
		t.Skip("Unregister 方法不存在，跳过该测试")
	}
	m.Register("to_remove", func() (interface{}, error) { return 1, nil })
	if !m.IsRegistered("to_remove") {
		t.Fatal("预注册失败")
	}
	u.Unregister("to_remove")
	if m.IsRegistered("to_remove") {
		t.Fatal("Unregister 未生效")
	}
	// 再次注册应成功
	if ok := m.Register("to_remove", func() (interface{}, error) { return 2, nil }); !ok {
		t.Fatal("Unregister 后再注册应成功")
	}
}

func TestRebuildIfSupported(t *testing.T) {
	m := newMgr()
	type rebuildable interface {
		Rebuild(name KeyName) (interface{}, error)
	}
	rb, ok := any(m).(rebuildable)
	if !ok {
		t.Skip("Rebuild 方法不存在，跳过")
	}

	var counter int32
	m.Register("re_svc", func() (interface{}, error) {
		return fmt.Sprintf("V%d", atomic.AddInt32(&counter, 1)), nil
	})
	inst1, err := m.Get("re_svc")
	if err != nil {
		t.Fatalf("初次获取失败: %v", err)
	}
	if counter != 1 {
		t.Fatalf("初始化次数应=1, got=%d", counter)
	}
	inst2, err := rb.Rebuild("re_svc")
	if err != nil {
		t.Fatalf("Rebuild 失败: %v", err)
	}
	if counter != 2 {
		t.Fatalf("Rebuild 后计数应=2, got=%d", counter)
	}
	if inst1 == inst2 {
		t.Fatalf("Rebuild 应返回全新实例")
	}
	// 再次 Get 不应再创建新实例
	inst3, err := m.Get("re_svc")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if inst3 != inst2 {
		t.Fatalf("Rebuild 后的 Get 应复用新实例")
	}
}

func TestConcurrentGetDifferentKeys(t *testing.T) {
	m := newMgr()
	var totalInit int32
	keyCount := 30
	for i := 0; i < keyCount; i++ {
		k := KeyName(fmt.Sprintf("k_%d", i))
		m.Register(k, func() (interface{}, error) {
			atomic.AddInt32(&totalInit, 1)
			return &struct{}{}, nil
		})
	}
	var wg sync.WaitGroup
	const perKey = 20
	for i := 0; i < keyCount; i++ {
		k := KeyName(fmt.Sprintf("k_%d", i))
		for j := 0; j < perKey; j++ {
			wg.Add(1)
			go func(k KeyName) {
				defer wg.Done()
				_, err := m.Get(k)
				if err != nil {
					t.Errorf("并发获取失败 key=%s err=%v", k, err)
				}
			}(k)
		}
	}
	wg.Wait()
	// 每个 key 只应初始化一次
	if totalInit != int32(keyCount) {
		t.Fatalf("每个 key 应只初始化一次，期望=%d 实际=%d", keyCount, totalInit)
	}
}

func TestErrorAndPanicIsolationBetweenKeys(t *testing.T) {
	m := newMgr()
	var goodInit int32
	m.Register("good", func() (interface{}, error) {
		atomic.AddInt32(&goodInit, 1)
		return "OK", nil
	})
	m.Register("bad_err", func() (interface{}, error) {
		return nil, errors.New("E1")
	})
	m.Register("bad_panic", func() (interface{}, error) {
		panic("P1")
	})

	// 错误 key
	_, e1 := m.Get("bad_err")
	if e1 == nil {
		t.Fatal("应返回错误")
	}
	// panic key
	_, e2 := m.Get("bad_panic")
	if e2 == nil {
		t.Fatal("应返回 panic 包装错误")
	}
	// 正常 key 不受影响
	v, e3 := m.Get("good")
	if e3 != nil || v.(string) != "OK" {
		t.Fatalf("其它 key 不应受影响: v=%v err=%v", v, e3)
	}
	if goodInit != 1 {
		t.Fatalf("正常 key 初始化次数错误=%d", goodInit)
	}
}

func TestIsRegistered(t *testing.T) {
	m := newMgr()
	if m.IsRegistered("x") {
		t.Fatal("未注册不应存在")
	}
	m.Register("x", func() (interface{}, error) { return 1, nil })
	if !m.IsRegistered("x") {
		t.Fatal("已注册应返回 true")
	}
}

// contains 简易包含判断
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		return reflect.ValueOf(s).String() != "" && (indexOf(s, sub) >= 0)
	})()
}

// indexOf（避免 strings.Index 也可直接用 strings）
func indexOf(s, sub string) int {
	ls, lsub := len(s), len(sub)
	if lsub == 0 {
		return 0
	}
	if lsub > ls {
		return -1
	}
outer:
	for i := 0; i <= ls-lsub; i++ {
		for j := 0; j < lsub; j++ {
			if s[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}
