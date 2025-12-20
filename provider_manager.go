package fiberhouse

import (
	"errors"
	"fmt"
	"sync"
)

// 定义状态变量
var (
	StateUnload = new(State).Set(0, "unload")
	StateLoaded = new(State).Set(1, "loaded")
)

// State 提供者的状态结构体，实现状态器接口
type State struct {
	id   uint8
	name string
	lock sync.RWMutex
}

// Id 状态Id
func (s *State) Id() uint8 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.id
}

// Name 状态名称
func (s *State) Name() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.name
}

// Set 设置状态Id和名称
func (s *State) Set(id uint8, name string) IState {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.id = id
	s.name = name
	return s
}

// SetState 从另一个状态器设置状态
func (s *State) SetState(state IState) IState {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.id = state.Id()
	s.name = state.Name()
	return s
}

// 定义提供者错误类型
var (
	ErrProviderAlreadyExists = &ProviderError{msg: "provider already exists"}
	ErrProviderNotFound      = &ProviderError{msg: "provider not found"}
)

type ProviderError struct {
	msg string
}

func (e *ProviderError) Error() string {
	return e.msg
}

// ProviderManager New一个基础的提供者管理器，用于组合/继承和扩展
type ProviderManager struct {
	name       string
	sonManager IProviderManager
	ctx        IContext
	providers  map[string]IProvider
	lock       sync.RWMutex
	pType      IProviderType
	pTypeOnce  sync.Once
	location   IProviderLocation
	isUnique   bool // 标识管理器是否处于唯一提供者模式
}

// NewProviderManager 创建一个基础的提供者管理器，用于组合/继承和扩展
func NewProviderManager(ctx IContext) *ProviderManager {
	return &ProviderManager{
		ctx:       ctx,
		pType:     ProviderTypeDefault().ZeroType,         // 默认零值类型
		location:  ProviderLocationDefault().ZeroLocation, // 默认零值位置
		providers: make(map[string]IProvider),
	}
}

// Check 检查提供者类型是否设置，未设置则抛出异常，强制Initialize方法内优先进行检查
func (m *ProviderManager) Check() {
	if m.pType.GetTypeID() == ProviderTypeDefault().ZeroType.GetTypeID() {
		panic(fmt.Errorf("manager '%s' type is not set", m.name))
	}
}

// Name 返回提供者管理器名称
func (m *ProviderManager) Name() string {
	return m.name
}

// SetName 设置提供者管理器名称
func (m *ProviderManager) SetName(name string) IProviderManager {
	m.name = name
	return m
}

// Type 返回提供者类型
func (m *ProviderManager) Type() IProviderType {
	return m.pType
}

// SetType 设置提供者类型，仅允许设置一次
func (m *ProviderManager) SetType(typ IProviderType) IProviderManager {
	m.pTypeOnce.Do(func() {
		m.pType = typ
	})
	return m
}

// Location 返回管理器执行位置点标识
func (m *ProviderManager) Location() IProviderLocation {
	return m.location
}

// SetOrBindToLocation 设置管理器执行位置点标识
func (m *ProviderManager) SetOrBindToLocation(l IProviderLocation, bind ...bool) IProviderManager {
	m.location = l
	// 绑定管理器到位点对象
	if len(bind) > 0 && bind[0] {
		_ = l.Bind(m)
	}
	return m
}

// GetContext 获取应用上下文
func (m *ProviderManager) GetContext() IContext {
	return m.ctx
}

// Register 注册一个 provider
func (m *ProviderManager) Register(provider IProvider) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.providers[provider.Name()]; exists {
		return ErrProviderAlreadyExists
	}

	m.providers[provider.Name()] = provider
	return nil
}

// Unregister 注销一个 provider
func (m *ProviderManager) Unregister(name string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.providers[name]; !exists {
		return ErrProviderNotFound
	}

	delete(m.providers, name)
	return nil
}

// GetProvider 获取指定名称的 provider
func (m *ProviderManager) GetProvider(name string) (IProvider, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	provider, exists := m.providers[name]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return provider, nil
}

// List 返回所有已注册的 providers
func (m *ProviderManager) List() []IProvider {
	m.lock.RLock()
	defer m.lock.RUnlock()

	list := make([]IProvider, 0, len(m.providers))
	for _, provider := range m.providers {
		list = append(list, provider)
	}

	return list
}

// LoadProvider 加载 providers
func (m *ProviderManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	m.Check()
	if m.sonManager == nil {
		return nil, errors.New("sonManager is not set, need to call the MountToParent method of the subclass instance to attach the subclass instance to the parent class's sonManager field")
	}
	return m.sonManager.LoadProvider(loadFunc...)
}

// IsUnique 返回管理器是否处于唯一提供者模式
func (m *ProviderManager) IsUnique() bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.isUnique
}

// BindToUniqueProvider 绑定唯一的提供者到管理器
// 确保管理器有且仅有一个提供者注册进来
// 如果已存在相同的提供者记录，视为注册成功并设置唯一属性为 true
// 如果已存在多个提供者，则 panic 错误
// 返回管理器自身以支持链式调用
func (m *ProviderManager) BindToUniqueProvider(provider IProvider) IProviderManager {
	m.lock.Lock()
	defer m.lock.Unlock()

	providerCount := len(m.providers)

	// 检查是否已存在多个提供者
	if providerCount > 1 {
		panic(fmt.Errorf("manager '%s' already has multiple providers (%d), cannot bind unique provider", m.name, providerCount))
	}

	// 检查是否已存在一个提供者
	if providerCount == 1 {
		// 检查是否是相同的提供者
		if existingProvider, exists := m.providers[provider.Name()]; exists {
			// 相同提供者，视为注册成功
			if existingProvider == provider {
				m.isUnique = true
				return m
			}
		}
		// 已存在不同的提供者，无法绑定
		panic(fmt.Errorf("manager '%s' already has a different provider, cannot bind unique provider '%s'", m.name, provider.Name()))
	}

	// 没有提供者，直接注册
	m.providers[provider.Name()] = provider
	m.isUnique = true
	return m
}

// MountToParent 将子类管理器实例挂载到父类管理器的 sonManager 字段上
func (m *ProviderManager) MountToParent(son ...IProviderManager) IProviderManager {
	if len(son) == 0 {
		panic(errors.New(m.name + "MountToParent() must provide at least one IProviderManager"))
	}
	if m == son[0] {
		panic(errors.New(m.name + "MountToParent() sonManager parameter cannot be the same as the parent manager instance"))
	}
	m.sonManager = son[0]
	return m
}

// DefaultPManager 默认提供者管理器，实现默认的提供者加载逻辑
type DefaultPManager struct {
	IProviderManager
}

// NewDefaultPManager 创建一个默认提供者管理器实例，实现默认的提供者加载逻辑
func NewDefaultPManager(ctx IContext) *DefaultPManager {
	return &DefaultPManager{
		IProviderManager: NewProviderManager(ctx).SetName("DefaultPManager").SetType(ProviderTypeDefault().GroupDefaultManagerType).SetOrBindToLocation(ProviderLocationDefault().ZeroLocation),
	}
}

func (m *DefaultPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	if len(loadFunc) > 0 {
		return m.IProviderManager.LoadProvider(loadFunc...)
	}

	// 默认加载逻辑，根据引导配置加载相应的提供者
	bootCfg := m.GetContext().(IApplicationContext).GetBootConfig()

	if len(m.List()) == 0 {
		return nil, ErrProviderNotFound
	}

	var errs []error

	// TODO 记录最终未被加载的提供者列表日志
	for _, provider := range m.List() {
		if provider.Type().GetTypeID() == ProviderTypeDefault().GroupProviderAutoRun.GetTypeID() {
			// 自动运行类型的提供者，不依赖Target约束可以直接初始化
			_, err := provider.Initialize(m.GetContext())
			errs = append(errs, err)
		} else if provider.Target() == bootCfg.CoreType {
			// 目标类型匹配启动配置的核心类型的提供者，进行初始化
			_, err := provider.Initialize(m.GetContext())
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to load providers: %v", errs)
	}

	return nil, nil
}
