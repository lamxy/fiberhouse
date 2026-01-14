// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"fmt"
	"sync"
)

// IProviderType 提供者类型接口
type IProviderType interface {
	// GetTypeID 获取类型序号
	GetTypeID() uint8
	// GetTypeName 获取类型名称
	GetTypeName() string
	// IsDefaultType 是否为默认类型
	IsDefaultType() bool
}

// PType 类型实现
type PType struct {
	id   uint8
	name string
}

func (t *PType) GetTypeID() uint8 {
	return t.id
}

func (t *PType) GetTypeName() string {
	return t.name
}

func (t *PType) IsDefaultType() bool {
	return t.id <= DefaultTypeEnd
}

const (
	/* 默认类型序号范围: 0-63 */

	DefaultTypeStart uint8 = 0
	DefaultTypeEnd   uint8 = 63

	/* 自定义类型序号范围: 64-255 */

	CustomTypeStart uint8 = 64
	CustomTypeEnd   uint8 = 255
)

// DefaultPType 预定义的默认类型对象集合
//
// 提供者类型分组的默认逻辑，同一类型的提供者仅允许注册进同一类型的管理器中并加载处理
// 1. GroupXXXChoose Choose结尾，表示选择其中一个提供者执行（仅符合Target()单个提供者执行，即匹配到提供者则中断后续提供者初始化）（比如切换核心引擎、切换编解码器等只取管理器注册的提供者列表中的一个提供者）
// 2. GroupYYYType Type结尾，表示受Target、Name、Version等约束条件限制，符合条件的多个提供者都可以执行（比如多个中间件注册、多个路由组注册的提供者都应用执行）
// 3. GroupZZZAutoRun AutoRun结尾，表示自动运行，不受条件约束，所有注册的提供者均执行一次（比如全局对象注册、默认启动对象初始化的提供者）
// 4. GroupWWWUnique Unique结尾，表示有且只有一个提供者存在和执行（比如框架启动器选项初始化提供者，唯一绑定管理器，管理器将无法注册更多的提供者）
// 5. 其他自定义，由开发者自行约定和实现
type DefaultPType struct {
	ZeroType                        IProviderType // 默认零值类型
	GroupDefaultManagerType         IProviderType // 默认管理器类型组，该类型提供者都注册进默认管理器进行处理
	GroupTrafficCodecChoose         IProviderType // 传输编解码器选择组，该类型提供者中仅选择一个进行流量编解码处理
	GroupCoreEngineChoose           IProviderType // 核心引擎选择组，该类型提供者中仅选择一个进行核心引擎处理
	GroupMiddlewareRegisterType     IProviderType // 中间件注册类型组，该类型提供者都注册进中间件链进行处理
	GroupRouteRegisterType          IProviderType // 路由注册类型组，该类型提供者都注册进路由表进行处理
	GroupCoreHookChoose             IProviderType // 核心钩子选择组，该类型提供者中仅选择一个进行核心钩子处理
	GroupFrameStarterChoose         IProviderType // 框架启动器选择组，该类型提供者中仅选择一个进行框架启动处理
	GroupCoreStarterChoose          IProviderType // 核心启动器选择组，该类型提供者中仅选择一个进行核心启动处理
	GroupProviderAutoRun            IProviderType // 提供者自动运行组，该类型提供者都自动运行一次进行处理
	GroupCoreContextChoose          IProviderType // 核心上下文选择组，该类型提供者中仅选择一个进行核心上下文处理
	GroupFrameStarterOptsInitUnique IProviderType // 框架启动器选项初始化唯一组，该类型提供者中仅唯一绑定一个管理器，并由该唯一的提供者进行处理
	GroupCoreStarterOptsInitUnique  IProviderType // 核心启动器选项初始化唯一组，该类型提供者中仅唯一绑定一个管理器，并由该唯一的提供者进行处理
	GroupRecoverMiddlewareChoose    IProviderType // 恢复中间件选择组，该类型提供者中仅选择一个进行恢复中间件处理（根据核心类型选择）
	GroupResponseInfoChoose         IProviderType // 响应信息选择组，该类型提供者中仅选择一个进行响应信息处理（根据name存储的http内容类型来选择）
}

var (
	providerTypeInstance *DefaultPType
	providerTypeOnce     sync.Once
)

// ProviderTypeDefault 获取预定义的默认类型对象集合（单例）
func ProviderTypeDefault() *DefaultPType {
	providerTypeOnce.Do(func() {
		registry := ProviderTypeGen()
		providerTypeInstance = &DefaultPType{
			ZeroType:                        registry.MustDefault("__ZERO__"),
			GroupDefaultManagerType:         registry.MustDefault("DefaultManagerType"),
			GroupTrafficCodecChoose:         registry.MustDefault("TrafficCodecChoose"),
			GroupCoreEngineChoose:           registry.MustDefault("CoreEngineChoose"),
			GroupMiddlewareRegisterType:     registry.MustDefault("MiddlewareRegisterType"),
			GroupRouteRegisterType:          registry.MustDefault("RouteRegisterType"),
			GroupCoreHookChoose:             registry.MustDefault("CoreHookChoose"),
			GroupFrameStarterChoose:         registry.MustDefault("FrameStarterChoose"),
			GroupCoreStarterChoose:          registry.MustDefault("CoreStarterChoose"),
			GroupProviderAutoRun:            registry.MustDefault("ProviderAutoRun"),
			GroupCoreContextChoose:          registry.MustDefault("CoreContextChoose"),
			GroupFrameStarterOptsInitUnique: registry.MustDefault("FrameStarterOptsInitUnique"),
			GroupCoreStarterOptsInitUnique:  registry.MustDefault("CoreStarterOptsInitUnique"),
			GroupRecoverMiddlewareChoose:    registry.MustDefault("RecoverMiddlewareChoose"),
			GroupResponseInfoChoose:         registry.MustDefault("ResponseInfoChoose"),
		}
	})
	return providerTypeInstance
}

// ProviderTypeRegistry 类型注册结构体
type ProviderTypeRegistry struct {
	mu            sync.RWMutex
	defaultTypes  map[string]IProviderType // 默认类型: 名称 -> 类型实例
	customTypes   map[string]IProviderType // 自定义类型: 名称 -> 类型实例
	nextDefaultID uint8                    // 下一个可用的默认类型ID
	nextCustomID  uint8                    // 下一个可用的自定义类型ID
}

var (
	registryInstance *ProviderTypeRegistry
	registryOnce     sync.Once
)

// ProviderTypeGen 获取类型注册结构体单例
func ProviderTypeGen() *ProviderTypeRegistry {
	registryOnce.Do(func() {
		registryInstance = &ProviderTypeRegistry{
			defaultTypes:  make(map[string]IProviderType),
			customTypes:   make(map[string]IProviderType),
			nextDefaultID: DefaultTypeStart,
			nextCustomID:  CustomTypeStart,
		}
	})
	return registryInstance
}

// Default 注册并获取默认类型对象（ID范围: 0-63）
func (r *ProviderTypeRegistry) Default(name string) (IProviderType, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查名称是否已存在于默认类型中
	if _, exists := r.defaultTypes[name]; exists {
		return nil, fmt.Errorf("default type name '%s' already registered", name)
	}

	// 检查名称是否已在自定义类型中使用
	if _, exists := r.customTypes[name]; exists {
		return nil, fmt.Errorf("type name '%s' already registered as custom type", name)
	}

	// 检查ID是否超出范围
	if r.nextDefaultID > DefaultTypeEnd {
		return nil, errors.New("default type ID exhausted (max 63)")
	}

	// 创建默认类型实例
	t := &PType{
		id:   r.nextDefaultID,
		name: name,
	}
	r.nextDefaultID++

	// 注册到默认类型集合
	r.defaultTypes[name] = t
	return t, nil
}

// MustDefault 注册并获取默认类型对象，失败时panic
func (r *ProviderTypeRegistry) MustDefault(name string) IProviderType {
	t, err := r.Default(name)
	if err != nil {
		panic(fmt.Sprintf("failed to register default type '%s': %v", name, err))
	}
	return t
}

// Custom 注册并获取自定义类型对象（ID范围: 64-255）
func (r *ProviderTypeRegistry) Custom(name string) (IProviderType, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查名称是否已存在于自定义类型中
	if _, exists := r.customTypes[name]; exists {
		return nil, fmt.Errorf("custom type name '%s' already registered", name)
	}

	// 检查名称是否已在默认类型中使用
	if _, exists := r.defaultTypes[name]; exists {
		return nil, fmt.Errorf("type name '%s' already registered as default type", name)
	}

	// 检查ID是否超出范围
	if r.nextCustomID > CustomTypeEnd {
		return nil, errors.New("custom type ID exhausted (max 255)")
	}

	// 创建自定义类型实例
	t := &PType{
		id:   r.nextCustomID,
		name: name,
	}
	r.nextCustomID++

	// 注册到自定义类型集合
	r.customTypes[name] = t
	return t, nil
}

// MustCustom 注册并获取自定义类型对象，失败时panic
func (r *ProviderTypeRegistry) MustCustom(name string) IProviderType {
	t, err := r.Custom(name)
	if err != nil {
		panic(fmt.Sprintf("failed to register custom type '%s': %v", name, err))
	}
	return t
}

// Type 根据名称获取类型对象（查找默认类型和自定义类型）
func (r *ProviderTypeRegistry) Type(name string) (IProviderType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 先查找自定义类型
	if t, exists := r.customTypes[name]; exists {
		return t, nil
	}

	// 再查找默认类型
	if t, exists := r.defaultTypes[name]; exists {
		return t, nil
	}

	return nil, fmt.Errorf("type '%s' not found", name)
}

// MustType 根据名称获取类型对象，不存在时panic
func (r *ProviderTypeRegistry) MustType(name string) IProviderType {
	t, err := r.Type(name)
	if err != nil {
		panic(fmt.Sprintf("failed to get type '%s': %v", name, err))
	}
	return t
}
