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
		log.Fatal(err)
	}

	// 类型断言使用
	database := db.(*Database)
*/
package globalmanager

import (
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
	instance    atomic.Value              // 类型安全的原子值，原子化读写实例
	initialized int32                     // 原子标志位：0 未初始化，1 初始化成功，-1 初始化失败，使用atomic原子操作
	mu          sync.Mutex                // 用于保护重置操作(如多条原子操作)
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
		if instance = entity.instance.Load(); instance != nil {
			return
		} else {
			err = fmt.Errorf("instance '%s' from GlobalManager is nil, but initialized flag is set to 1", name)
			return
		}
	}

	// 如果初始化失败，重置初始化状态
	if atomic.LoadInt32(&entity.initialized) == -1 {
		entity.mu.Lock()
		if atomic.LoadInt32(&entity.initialized) == -1 { // 双重检查
			entity.once.Store(&sync.Once{})           // 重置once以便下次可以重新初始化
			atomic.StoreInt32(&entity.initialized, 0) // 重置初始化状态
		}
		entity.mu.Unlock()
	}

	// 仅初始化一次
	currentOnce := entity.once.Load()
	currentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				// 捕获 panic 并设置初始化状态为-1
				atomic.StoreInt32(&entity.initialized, -1)
				err = fmt.Errorf("panic occurred while initializing global object '%s': %v", name, r)
			}
		}()
		instance, err = entity.initializer()
		if err != nil {
			// 初始化失败，设置初始化状态为-1
			atomic.StoreInt32(&entity.initialized, -1)
			err = fmt.Errorf("failed to initialize global object '%s': %v", name, err)
			return
		}
		entity.instance.Store(instance)
		// 初始化成功，设置初始化状态为1
		atomic.StoreInt32(&entity.initialized, 1)
	})

	instance = entity.instance.Load()
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
	instance := entity.instance.Load()
	if checker, ok := instance.(HealthChecker); ok {
		return checker.IsHealthy(), nil
	}

	// 如果未实现 HealthChecker，则默认为健康
	return true, nil
}

// Rebuild 重建全局对象
func (gm *GlobalManager) Rebuild(name KeyName) error {
	origin, ok := gm.container.Load(name)
	if !ok {
		return fmt.Errorf("global key '%s' not found in GlobalManager with rebuild method", name)
	}
	entity, ok := origin.(*entry)
	if !ok {
		return fmt.Errorf("global entry '%s' type assertion failed with rebuild method", name)
	}

	currentInstance := entity.instance.Load()
	if currentInstance == nil {
		return fmt.Errorf("global object '%s' not initialized with rebuild method", name)
	}
	if rebuilder, ok := currentInstance.(Rebuilder); ok {
		newInstance, err := rebuilder.Rebuild(rebuilder.GetConfPath())
		if err != nil {
			return fmt.Errorf("failed to rebuild global object '%s': %v", name, err)
		}
		entity.instance.Store(newInstance)
		return nil
	}

	return fmt.Errorf("global object '%s' does not support rebuilding", name)
}

// Release 扩展 GlobalManager 支持资源释放, 不删除key，要重建sync.Once，以便可以Get时重新初始化
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

	// 检查是否实现了 Closable 接口
	if closable, ok := entity.instance.Load().(Closable); ok {
		err := closable.Close()
		if err != nil {
			return fmt.Errorf("unable to close object resource by key '%s' : %v", name, err)
		}
		// close成功，重置sync.Once，以便Get时重新初始化实例
		entity.mu.Lock()
		// 清空实例
		entity.instance.Store(nil)
		// 重置初始化状态
		atomic.StoreInt32(&entity.initialized, 0)
		// 重置once以便下次可以重新初始化
		entity.once.Store(&sync.Once{})
		entity.mu.Unlock()
	}
	return nil
}

// ReleaseAll 释放所有已注册对象的资源，仅仅释放，支持get时重建对象
func (gm *GlobalManager) ReleaseAll(conform ...bool) {
	if len(conform) > 0 && conform[0] {
		gm.container.Range(func(key, value interface{}) bool {
			name := key.(string)
			err := gm.Release(name)
			if err != nil {
				fmt.Printf("failed to release global object '%s': %v.\n", name, err)
			}
			return true
		})
	}
}

// Clear 直接永久删除key，后续无法Get()，只能重新注册
func (gm *GlobalManager) Clear(name KeyName) {
	gm.Unregister(name)
}

// ClearAll 不受阻碍的清空全局管理器管理的全部对象和资源
func (gm *GlobalManager) ClearAll(conform ...bool) {
	if len(conform) > 0 && conform[0] {
		gm.container = sync.Map{}
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
