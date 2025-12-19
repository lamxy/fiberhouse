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
// 分组类默认逻辑
// 1. GroupXXXChoose 选择其中唯一一个提供者执行（仅符合Target单个提供者执行，即匹配到提供者则中断后续提供者执行）（比如切换核心引擎、切换编解码器）
// 2. GroupYYYType 类型，受Target条件和Name等约束，符合条件的多个提供者都可以执行（比如多个中间件注册、多个路由组注册）
// 3. GroupZZZAutoRun 自动运行，不受条件约束，所有注册的提供者均执行一次（比如全局对象注册、默认启动对象初始化）
// 其他自定义，又开发者自行约定和实现（比如自定义扩展逻辑）
type DefaultPType struct {
	ZeroType                    IProviderType
	GroupDefaultManagerType     IProviderType
	GroupJsonCodecChoose        IProviderType
	GroupCoreEngineChoose       IProviderType
	GroupMiddlewareRegisterType IProviderType
	GroupRouteRegisterType      IProviderType
	GroupFrameStarterChoose     IProviderType
	GroupCoreStarterChoose      IProviderType
	GroupProviderAutoRun        IProviderType
	GroupCoreContextChoose      IProviderType
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
			ZeroType:                    registry.MustDefault("__ZERO__"),
			GroupDefaultManagerType:     registry.MustDefault("DefaultManagerType"),
			GroupJsonCodecChoose:        registry.MustDefault("JsonCodecChoose"),
			GroupCoreEngineChoose:       registry.MustDefault("CoreEngineChoose"),
			GroupMiddlewareRegisterType: registry.MustDefault("MiddlewareRegisterType"),
			GroupRouteRegisterType:      registry.MustDefault("RouteRegisterType"),
			GroupFrameStarterChoose:     registry.MustDefault("FrameStarterChoose"),
			GroupCoreStarterChoose:      registry.MustDefault("CoreStarterChoose"),
			GroupProviderAutoRun:        registry.MustDefault("ProviderAutoRun"),
			GroupCoreContextChoose:      registry.MustDefault("CoreContextChoose"),
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
