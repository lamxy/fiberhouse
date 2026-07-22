// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// PLocation 位点实现
type PLocation struct {
	id       uint8
	name     string
	mu       sync.RWMutex       // 保护 managers 的并发访问
	managers []IProviderManager // 绑定到该位点的管理器列表
}

func (l *PLocation) GetLocationID() uint8 {
	return l.id
}

func (l *PLocation) GetLocationName() string {
	return l.name
}

func (l *PLocation) IsDefaultLocation() bool {
	return l.id <= DefaultLocationEnd
}

// Bind 绑定管理器到该位点的管理器列表中
func (l *PLocation) Bind(manager IProviderManager) error {
	if isNilProviderManager(manager) {
		return errors.New("manager cannot be nil")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查同一管理器实例是否已绑定（同一位点允许按顺序绑定多个不同管理器）
	for _, m := range l.managers {
		if sameProviderManagerInstance(m, manager) {
			return fmt.Errorf("manager '%s' already bound to location '%s'", manager.Name(), l.name)
		}
	}

	l.managers = append(l.managers, manager)
	return nil
}

func isNilProviderManager(manager IProviderManager) bool {
	if manager == nil {
		return true
	}
	value := reflect.ValueOf(manager)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func sameProviderManagerInstance(left, right IProviderManager) bool {
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	if !leftValue.IsValid() || !rightValue.IsValid() || leftValue.Type() != rightValue.Type() {
		return false
	}
	switch leftValue.Kind() {
	case reflect.Chan, reflect.Map, reflect.Pointer:
		return leftValue.Pointer() == rightValue.Pointer()
	default:
		return false
	}
}

// GetManagers 获取已绑定到该位点的管理器列表（返回副本，确保外部修改不影响内部数据）
func (l *PLocation) GetManagers() []IProviderManager {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.managers) == 0 {
		return nil
	}

	// 返回副本，避免外部修改影响内部数据
	result := make([]IProviderManager, len(l.managers))
	copy(result, l.managers)
	return result
}

const (
	/* 默认位点序号范围: 0-63 */

	DefaultLocationStart uint8 = 0
	DefaultLocationEnd   uint8 = 63

	/* 自定义位点序号范围: 64-255 */

	CustomLocationStart uint8 = 64
	CustomLocationEnd   uint8 = 255
)

// DefaultPLocation 预定义的默认位点对象集合
//
// 位点用于标识提供者的执行位置，相同位点的管理器会被收集并按顺序执行
// 1. LocationXXXBefore 在某个阶段之前执行
// 2. LocationXXXAfter 在某个阶段之后执行
// 3. LocationXXXInit 在某个初始化阶段执行
// 4. LocationXXXRun 在XXX运行阶段执行
// 5. LocationXXXCreate 在XXX创建阶段执行
// 6. 其他，由开发者自定义
// 注：这里默认给出的执行位置点，框架默认实现仅使用了部分，若使所有位置点生效，需要自定义实现核心启动器接口
type DefaultPLocation struct {
	ZeroLocation                   IProviderLocation // 初始化默认位点/零位点/保留为初始化状态
	LocationAdaptCoreCtxChoose     IProviderLocation // 适配核心上下文选择位点（用于统一输出响应时屏蔽不同核心引擎上下文差异）
	LocationBootStrapConfig        IProviderLocation // 引导配置阶段位点
	LocationFrameStarterOptionInit IProviderLocation // 框架启动器选项初始化位点
	LocationCoreStarterOptionInit  IProviderLocation // 核心启动器选项初始化位点
	LocationFrameStarterCreate     IProviderLocation // 创建框架启动器位点
	LocationCoreStarterCreate      IProviderLocation // 创建核心引擎启动器位点
	LocationGlobalInit             IProviderLocation // 全局初始化位点
	LocationGlobalKeepaliveInit    IProviderLocation // 全局对象保活初始化位点
	LocationCoreEngineInit         IProviderLocation // 核心引擎初始化位点
	LocationCoreHookInit           IProviderLocation // 核心引擎生命周期钩子（如有）初始化位点
	LocationAppMiddlewareInit      IProviderLocation // 注册应用中间件初始化位点
	LocationModuleMiddlewareInit   IProviderLocation // 注册模块中间件初始化位点
	LocationRouteRegisterInit      IProviderLocation // 注册路由初始化位点
	LocationTaskServerInit         IProviderLocation // 任务服务器初始化位点（如需要）
	LocationModuleSwaggerInit      IProviderLocation // 注册Swagger初始化位点（如需要）
	LocationServerRunBefore        IProviderLocation // 服务运行前位点
	LocationServerRun              IProviderLocation // 服务运行位点
	LocationServerRunAfter         IProviderLocation // 服务运行后位点（框架默认实现未使用该位点）
	LocationServerShutdownBefore   IProviderLocation // 服务关闭前位点
	LocationServerShutdown         IProviderLocation // 服务关闭位点（框架默认实现未使用该位点）
	LocationServerShutdownAfter    IProviderLocation // 服务关闭后位点
	LocationResponseInfoInit       IProviderLocation // 响应信息初始化位点
}

var (
	providerLocationInstance *DefaultPLocation
	providerLocationOnce     sync.Once
)

// ProviderLocationDefault 获取预定义的默认位点对象集合（单例）
func ProviderLocationDefault() *DefaultPLocation {
	providerLocationOnce.Do(func() {
		registry := ProviderLocationGen()
		providerLocationInstance = &DefaultPLocation{
			ZeroLocation:                   registry.MustDefault("__ZERO__"),
			LocationAdaptCoreCtxChoose:     registry.MustDefault("AdaptCoreCtxChoose"),     // 适配核心上下文选择位点
			LocationBootStrapConfig:        registry.MustDefault("BootStrapConfig"),        // 引导配置阶段位点
			LocationFrameStarterOptionInit: registry.MustDefault("FrameStarterOptionInit"), // 框架启动器选项初始化位点
			LocationCoreStarterOptionInit:  registry.MustDefault("CoreStarterOptionInit"),  // 核心启动器选项初始化位点
			LocationFrameStarterCreate:     registry.MustDefault("FrameStarterCreate"),     // 创建框架启动器位点
			LocationCoreStarterCreate:      registry.MustDefault("CoreStarterCreate"),      // 创建核心引擎启动器位点
			LocationGlobalInit:             registry.MustDefault("GlobalInit"),             // 全局初始化位点
			LocationGlobalKeepaliveInit:    registry.MustDefault("GlobalKeepaliveInit"),    // 全局对象保活初始化位点
			LocationCoreEngineInit:         registry.MustDefault("CoreEngineInit"),         // 核心引擎初始化位点
			LocationCoreHookInit:           registry.MustDefault("CoreHookInit"),           // 核心引擎钩子（如有）初始化位点
			LocationAppMiddlewareInit:      registry.MustDefault("AppMiddlewareInit"),      // 注册核心应用中间件初始化位点
			LocationModuleMiddlewareInit:   registry.MustDefault("ModuleMiddlewareInit"),   // 注册模块中间件初始化位点
			LocationRouteRegisterInit:      registry.MustDefault("RouteRegisterInit"),      // 注册路由初始化位点
			LocationTaskServerInit:         registry.MustDefault("TaskServerInit"),         // 任务服务器初始化位点
			LocationModuleSwaggerInit:      registry.MustDefault("ModuleSwaggerInit"),      // 注册Swagger初始化位点
			LocationServerRunBefore:        registry.MustDefault("ServerRunBefore"),        // 服务运行前位点
			LocationServerRun:              registry.MustDefault("ServerRun"),              // 服务运行位点
			LocationServerRunAfter:         registry.MustDefault("ServerRunAfter"),         // 服务运行后位点（框架默认实现未使用该位点）
			LocationServerShutdownBefore:   registry.MustDefault("ServerShutdownBefore"),   // 服务关闭前位点
			LocationServerShutdown:         registry.MustDefault("ServerShutdown"),         // 服务关闭位点（框架默认实现未使用该位点）
			LocationServerShutdownAfter:    registry.MustDefault("ServerShutdownAfter"),    // 服务关闭后位点
			LocationResponseInfoInit:       registry.MustDefault("ResponseInfoInit"),       // 响应信息初始化位点
		}
	})
	return providerLocationInstance
}

// ProviderLocationRegistry 位点注册结构体
type ProviderLocationRegistry struct {
	mu               sync.RWMutex
	defaultLocations map[string]IProviderLocation // 默认位点: 名称 -> 位点实例
	customLocations  map[string]IProviderLocation // 自定义位点: 名称 -> 位点实例
	nextDefaultID    uint16                       // 下一个可用的默认位点ID（需要表达耗尽哨兵值）
	nextCustomID     uint16                       // 下一个可用的自定义位点ID（需要表达耗尽哨兵值）
}

var (
	locationRegistryInstance *ProviderLocationRegistry
	locationRegistryOnce     sync.Once
)

// ProviderLocationGen 获取位点注册结构体单例
func ProviderLocationGen() *ProviderLocationRegistry {
	locationRegistryOnce.Do(func() {
		locationRegistryInstance = &ProviderLocationRegistry{
			defaultLocations: make(map[string]IProviderLocation),
			customLocations:  make(map[string]IProviderLocation),
			nextDefaultID:    uint16(DefaultLocationStart),
			nextCustomID:     uint16(CustomLocationStart),
		}
	})
	return locationRegistryInstance
}

// Default 注册并获取默认位点对象（ID范围: 0-63）
func (r *ProviderLocationRegistry) Default(name string) (IProviderLocation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查名称是否已存在于默认位点中
	if _, exists := r.defaultLocations[name]; exists {
		return nil, fmt.Errorf("default location name '%s' already registered", name)
	}

	// 检查名称是否已在自定义位点中使用
	if _, exists := r.customLocations[name]; exists {
		return nil, fmt.Errorf("location name '%s' already registered as custom location", name)
	}

	// 检查ID是否超出范围
	if r.nextDefaultID > uint16(DefaultLocationEnd) {
		return nil, errors.New("default location ID exhausted (max 63)")
	}

	// 创建默认位点实例
	l := &PLocation{
		id:   uint8(r.nextDefaultID),
		name: name,
	}
	r.nextDefaultID++

	// 注册到默认位点集合
	r.defaultLocations[name] = l
	return l, nil
}

// MustDefault 注册并获取默认位点对象，失败时panic
func (r *ProviderLocationRegistry) MustDefault(name string) IProviderLocation {
	l, err := r.Default(name)
	if err != nil {
		panic(fmt.Sprintf("failed to register default location '%s': %v", name, err))
	}
	return l
}

// Custom 注册并获取自定义位点对象（ID范围: 64-255）
func (r *ProviderLocationRegistry) Custom(name string) (IProviderLocation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查名称是否已存在于自定义位点中
	if _, exists := r.customLocations[name]; exists {
		return nil, fmt.Errorf("custom location name '%s' already registered", name)
	}

	// 检查名称是否已在默认位点中使用
	if _, exists := r.defaultLocations[name]; exists {
		return nil, fmt.Errorf("location name '%s' already registered as default location", name)
	}

	// 检查ID是否超出范围
	if r.nextCustomID > uint16(CustomLocationEnd) {
		return nil, errors.New("custom location ID exhausted (max 255)")
	}

	// 创建自定义位点实例
	l := &PLocation{
		id:   uint8(r.nextCustomID),
		name: name,
	}
	r.nextCustomID++

	// 注册到自定义位点集合
	r.customLocations[name] = l
	return l, nil
}

// MustCustom 注册并获取自定义位点对象，失败时panic
func (r *ProviderLocationRegistry) MustCustom(name string) IProviderLocation {
	l, err := r.Custom(name)
	if err != nil {
		panic(fmt.Sprintf("failed to register custom location '%s': %v", name, err))
	}
	return l
}

// Location 根据名称获取位点对象（查找默认位点和自定义位点）
func (r *ProviderLocationRegistry) Location(name string) (IProviderLocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 先查找自定义位点
	if l, exists := r.customLocations[name]; exists {
		return l, nil
	}

	// 再查找默认位点
	if l, exists := r.defaultLocations[name]; exists {
		return l, nil
	}

	return nil, fmt.Errorf("location '%s' not found", name)
}

// MustLocation 根据名称获取位点对象，不存在时panic
func (r *ProviderLocationRegistry) MustLocation(name string) IProviderLocation {
	l, err := r.Location(name)
	if err != nil {
		panic(fmt.Sprintf("failed to get location '%s': %v", name, err))
	}
	return l
}
