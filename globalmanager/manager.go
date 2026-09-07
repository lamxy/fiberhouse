// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

/*
Package globalmanager 提供全局对象管理功能，用于管理应用程序中的单例对象。

该包提供了一个线程安全的全局对象管理器，支持延迟初始化、健康检查、重建和资源释放等功能。
适用于读多写少的场景。

基本用法:

	// 创建全局管理器
	gm := globalmanager.NewGlobalManagerOnce() 或 gm := globalmanager.NewGlobalManager()

	// 注册对象初始化器
	gm.Register("database", func() (interface{}, error) {
		return &Database{}, nil
	})

	// 获取对象实例
	db, err := gm.Get("database")
	if err != nil {
		log.Error(err)
	}

	// 类型断言使用
	database := db.(*Database)
*/
package globalmanager

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// GlobalManager 全局对象管理器，简称全局对象管理器，或名全局对象管理容器、全局对象容器、全部对象单例容器等
//
// 注意：该全局管理容器适用于读多写少场景
type GlobalManager struct {
	container sync.Map // 存储所有全局对象实例
}

type entry struct {
	initializer InitializerFunc           // 对象初始化器函数，用于延迟实例化
	once        atomic.Pointer[sync.Once] // 实现对象单例化，原子指针，确保读安全性，重置once的写时使用锁(多条原子操作)
	instance    atomic.Pointer[storedValue]
	initErr     atomic.Pointer[storedError]
	maintenance atomic.Bool
	initialized int32      // 原子标志位：0 未初始化，1 初始化成功，-1 初始化失败，使用atomic原子操作
	mu          sync.Mutex // 用于保护重置操作(如多条原子操作)
}

type storedValue struct {
	value interface{}
}

type storedError struct {
	err error
}

// ErrMaintenanceInProgress 表示同一 entry 上已有维护操作（Rebuild 或 Release）在执行。
//
// Rebuild 与 Release 在同一 entry 上以 fail-fast 方式互斥：冲突的调用不会排队等待，
// 而是立即返回包装了本哨兵的错误。这是一个**可恢复**条件——被拒绝的调用没有产生任何
// 副作用，调用方可以退避后重试：
//
//	if err := gm.Release(key); errors.Is(err, globalmanager.ErrMaintenanceInProgress) {
//		// 另一个维护操作正在进行，稍后重试
//	}
//
// 返回的错误始终用 %w 包装本哨兵并附加 key 名，因此 errors.Is 可靠，错误文本也可直接
// 用于日志定位。
var ErrMaintenanceInProgress = errors.New("global object maintenance already in progress")

func (e *entry) beginMaintenance(name KeyName) error {
	if !e.maintenance.CompareAndSwap(false, true) {
		return fmt.Errorf("%w: %s", ErrMaintenanceInProgress, name)
	}
	return nil
}

func (e *entry) endMaintenance() {
	e.maintenance.Store(false)
}

// 全局管理器实例
var (
	globalManager *GlobalManager
	once          sync.Once
)

// NewGlobalManager 获取全新的GlobalManager
func NewGlobalManager() *GlobalManager {
	return &GlobalManager{
		container: sync.Map{},
	}
}

// NewGlobalManagerOnce 获取GlobalManager单例
func NewGlobalManagerOnce() *GlobalManager {
	once.Do(func() {
		globalManager = NewGlobalManager()
	})
	return globalManager
}

// Register 注册一个全局对象的初始化器
func (gm *GlobalManager) Register(name KeyName, initializer InitializerFunc) bool {
	if initializer == nil {
		return false
	}
	// 先尝试 Load，已存在直接返回 false，避免 newEntry 新建
	if _, ok := gm.container.Load(name); ok {
		return false
	}
	newEntry := &entry{
		initializer: initializer,
		initialized: 0, // 显式初始化为未初始化状态
	}
	newEntry.once.Store(&sync.Once{}) // 初始化原子指针
	_, loaded := gm.container.LoadOrStore(name, newEntry)
	// 返回 true 表示新注册， false 表示已loaded，已注册/存在
	return !loaded
}

// Registers 批量注册全局对象的初始化器
func (gm *GlobalManager) Registers(initializers InitializerMap) {
	for name, initializer := range initializers {
		_ = gm.Register(name, initializer)
	}
}

// Get 获取一个全局对象
func (gm *GlobalManager) Get(name KeyName) (instance interface{}, err error) {
	origin, ok := gm.container.Load(name)
	if !ok {
		err = fmt.Errorf("entry '%s' not found for loading", name)
		return
	}
	entity, ok := origin.(*entry)
	if !ok {
		err = fmt.Errorf("assertion failure for '%s' entry", name)
		return
	}

	// 如果已经初始化过，直接返回已有实例
	if atomic.LoadInt32(&entity.initialized) == 1 {
		// 先尝试获取已有实例
		if stored := entity.instance.Load(); stored != nil && stored.value != nil {
			instance = stored.value
			return
		} else {
			err = fmt.Errorf("instance '%s' from GlobalManager is nil, but initialized flag is set to 1", name)
			return
		}
	}

	// 如果初始化失败，重置初始化状态
	if atomic.LoadInt32(&entity.initialized) == -1 {
		entity.mu.Lock()
		// todo 直接重新初始化新的 entity
		if atomic.LoadInt32(&entity.initialized) == -1 { // 双重检查
			entity.once.Store(&sync.Once{})           // 重置once以便下次可以重新初始化
			atomic.StoreInt32(&entity.initialized, 0) // 重置初始化状态
		}
		entity.mu.Unlock()
	}

	// 仅初始化一次
	entity.once.Load().Do(func() {
		defer func() {
			if r := recover(); r != nil {
				e := fmt.Errorf("panic occurred while initializing global object '%s': %v", name, r)
				entity.initErr.Store(&storedError{err: e})
				// 最后发布失败状态，确保重试方一定能先观察到错误。
				atomic.StoreInt32(&entity.initialized, -1)
			}
		}()
		instance, err = entity.initializer()
		if err == nil && instance == nil {
			err = fmt.Errorf("initializer returned nil instance")
		}
		if err != nil {
			e := fmt.Errorf("failed to initialize global object '%s': %v", name, err)
			entity.initErr.Store(&storedError{err: e})
			// 最后发布失败状态，确保重试方一定能先观察到错误。
			atomic.StoreInt32(&entity.initialized, -1)
			return
		}
		entity.instance.Store(&storedValue{value: instance})
		entity.initErr.Store(nil)
		// 初始化成功，设置初始化状态为1
		atomic.StoreInt32(&entity.initialized, 1)
	})

	if stored := entity.initErr.Load(); stored != nil && stored.err != nil {
		err = stored.err
		return
	}
	if stored := entity.instance.Load(); stored != nil {
		instance = stored.value
	}
	return
}

// Range 遍历全局管理器中的所有对象
func (gm *GlobalManager) Range(f func(key, value interface{}) bool) {
	gm.container.Range(f)
}

// CheckHealth 检查全局对象是否健康
func (gm *GlobalManager) CheckHealth(name KeyName) (bool, error) {
	origin, ok := gm.container.Load(name)
	if !ok {
		return true, fmt.Errorf("global instance '%s' not found in GlobalManager with CheckHealth method", name)
	}
	entity, ok := origin.(*entry)
	if !ok {
		return false, fmt.Errorf("global entry '%s' type assertion failure with CheckHealth method", name)
	}
	// 检查是否实现了 HealthChecker 接口
	var instance interface{}
	if stored := entity.instance.Load(); stored != nil {
		instance = stored.value
	}
	if checker, ok := instance.(HealthChecker); ok {
		return checker.IsHealthy(), nil
	}

	// 如果未实现 HealthChecker，则默认为健康
	return true, nil
}

// Rebuild 重建全局对象，用于配置热更新等需要替换底层客户端的场景。
//
// 仅供显式手动调用：框架的周期性健康检查不会自动触发重建，
// 连接池型对象的失效恢复由驱动层负责。详见 Rebuilder 接口说明。
func (gm *GlobalManager) Rebuild(name KeyName) error {
	origin, ok := gm.container.Load(name)
	if !ok {
		return fmt.Errorf("global key '%s' not found in GlobalManager with rebuild method", name)
	}
	entity, ok := origin.(*entry)
	if !ok {
		return fmt.Errorf("global entry '%s' type assertion failed with rebuild method", name)
	}
	if err := entity.beginMaintenance(name); err != nil {
		return err
	}
	defer entity.endMaintenance()

	var currentInstance interface{}
	if stored := entity.instance.Load(); stored != nil {
		currentInstance = stored.value
	}
	if currentInstance == nil {
		return fmt.Errorf("global object '%s' not initialized with rebuild method", name)
	}
	if rebuilder, ok := currentInstance.(Rebuilder); ok {
		newInstance, err := rebuilder.Rebuild(rebuilder.GetConfPath())
		if err != nil {
			return fmt.Errorf("failed to rebuild global object '%s': %v", name, err)
		}
		if newInstance == nil {
			return fmt.Errorf("failed to rebuild global object '%s': rebuilder returned nil instance", name)
		}
		entity.instance.Store(&storedValue{value: newInstance})
		return nil
	}

	return fmt.Errorf("global object '%s' does not support rebuilding", name)
}

// Release 释放已注册对象持有的资源，并把 entry 重置回未初始化状态。
//
// 不删除 key：重置 sync.Once 后，下一次 Get 会重新执行 initializer。要永久移除
// key 请使用 Unregister 或 Clear。
//
// 三种情况，返回值语义各不相同：
//
//   - 对象实现 Closable：先 Close 再重置。Close 失败时**保留**实例与初始化状态
//     并返回错误——释放没有发生，调用方拿到的仍是原来那个可用对象。
//   - 对象未实现 Closable：没有需要关闭的资源，直接重置。返回 nil。
//   - entry 尚未初始化：无操作，返回 nil（幂等）。
//
// 关键点是重置对**所有**已初始化对象都会发生，而不只是对 Closable 对象。
// 早期实现把重置放在 Closable 类型分支内部，导致非 Closable 对象在 Release
// 返回 nil 的同时保持初始化状态不变，调用方随后 Get 会拿回同一个实例——
// 一次报告成功但实际未生效的释放。
//
// 并发：同一 entry 上 Release 与 Rebuild 以 fail-fast 方式互斥，冲突时返回包装
// 了 ErrMaintenanceInProgress 的错误。Release 不会等待或取消已经开始的 Get
// 初始化，也不会使调用方已经持有的引用失效。
func (gm *GlobalManager) Release(name KeyName) error {
	// 获取实例
	origin, ok := gm.container.Load(name)
	if !ok {
		return fmt.Errorf("global object '%s' key not found", name)
	}
	entity, ok := origin.(*entry)
	if !ok {
		return fmt.Errorf("assertion of global object '%s' type failed with Release method", name)
	}
	if err := entity.beginMaintenance(name); err != nil {
		return err
	}
	defer entity.endMaintenance()

	var instance interface{}
	if stored := entity.instance.Load(); stored != nil {
		instance = stored.value
	}
	// 未初始化的 entry 没有资源需要释放，也没有状态需要重置。
	if instance == nil {
		return nil
	}

	// 实现了 Closable 的对象先关闭；关闭失败则保留实例，不重置状态。
	if closable, ok := instance.(Closable); ok {
		if err := closable.Close(); err != nil {
			return fmt.Errorf("unable to close object resource by key '%s' : %v", name, err)
		}
	}

	// 重置为未初始化状态，使下一次 Get 重新执行 initializer。
	entity.resetToUninitialized()
	return nil
}

// resetToUninitialized 把 entry 重置回未初始化状态，使下一次 Get 重新执行
// initializer。调用方必须已持有维护闸门。
func (e *entry) resetToUninitialized() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.instance.Store(nil)
	e.initErr.Store(nil)
	atomic.StoreInt32(&e.initialized, 0)
	e.once.Store(&sync.Once{})
}

// ReleaseAll 释放所有已注册对象的资源，仅仅释放，支持get时重建对象。
//
// 需要显式传入 true 才会执行，避免误调用清空整个容器。
//
// 关闭顺序：先释放实现了 CompositeClosable 且 OwnsChildren() 为 true 的组合对象，
// 再释放其余对象。组合对象需要先排空在途异步操作再关闭子资源，若容器先关掉子资源，
// 组合对象排空期间就会打在已关闭的资源上。遍历顺序本身不确定（sync.Map.Range 未定义
// 迭代顺序），所以这个约束必须显式表达而不能依赖运气。详见 CompositeClosable。
//
// 别名与共享子资源天然会被关闭多次（同一个 Redis 客户端既在 KEY_REDIS 下，又在
// KEY_REMOTE_CACHE 下，还被 Level2Cache 持有），因此实现方必须保证 Close 幂等。
//
// 释放错误通过 ReleaseAllWithObserver 上报；本方法丢弃错误，仅用于不关心失败的场景。
func (gm *GlobalManager) ReleaseAll(conform ...bool) {
	gm.ReleaseAllWithObserver(nil, conform...)
}

// ReleaseAllWithObserver 与 ReleaseAll 相同，但把每个释放失败交给 observer 上报。
//
// globalmanager 不依赖任何日志实现，因此错误上报由调用方注入：框架侧传入一个写入
// 框架日志器的回调即可。observer 为 nil 时错误被丢弃。
//
// observer 在释放循环内同步调用，实现应保持轻量且不得再次调用本容器的维护方法。
func (gm *GlobalManager) ReleaseAllWithObserver(observer func(name KeyName, err error), conform ...bool) {
	if len(conform) == 0 || !conform[0] {
		return
	}

	report := func(name KeyName, err error) {
		if err != nil && observer != nil {
			observer(name, err)
		}
	}

	// 第一轮：组合对象。它们负责排空在途操作并关闭自己持有的子资源。
	var deferred []KeyName
	gm.container.Range(func(key, _ interface{}) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		if gm.ownsChildren(name) {
			report(name, gm.Release(name))
		} else {
			deferred = append(deferred, name)
		}
		return true
	})

	// 第二轮：其余对象。此时组合对象已排空，子资源的重复关闭由幂等 Close 吸收。
	for _, name := range deferred {
		report(name, gm.Release(name))
	}
}

// ownsChildren 判断某个 key 当前的实例是否为负责关闭子资源的组合对象。
//
// 未初始化的 entry 返回 false：没有实例就没有子资源，放进第二轮即可。
func (gm *GlobalManager) ownsChildren(name KeyName) bool {
	origin, ok := gm.container.Load(name)
	if !ok {
		return false
	}
	entity, ok := origin.(*entry)
	if !ok {
		return false
	}
	stored := entity.instance.Load()
	if stored == nil {
		return false
	}
	composite, ok := stored.value.(CompositeClosable)
	return ok && composite.OwnsChildren()
}

// Clear 直接永久删除key，后续无法Get()，只能重新注册
func (gm *GlobalManager) Clear(name KeyName) {
	gm.Unregister(name)
}

// ClearAll 不受阻碍的清空全局管理器管理的全部对象和资源
func (gm *GlobalManager) ClearAll(conform ...bool) {
	if len(conform) > 0 && conform[0] {
		gm.container.Clear()
	}
}

// Unregister 直接永久删除key，后续无法Get()，只能重新注册
func (gm *GlobalManager) Unregister(name KeyName) {
	gm.container.Delete(name)
}

// IsRegistered 方法：检查 key 是否存在
func (gm *GlobalManager) IsRegistered(name KeyName) bool {
	_, ok := gm.container.Load(name)
	return ok
}
