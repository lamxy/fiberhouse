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

// Manager New一个基础的提供者管理器
type ProviderManager struct {
	name      string
	ctx       IContext
	providers map[string]IProvider
	lock      sync.RWMutex
	pType     IProviderType
	pTypeOnce sync.Once
}

// NewManager 创建一个基础的提供者管理器
func NewProviderManager(ctx IContext) *ProviderManager {
	return &ProviderManager{
		ctx:       ctx,
		pType:     ProviderTypeDefault().ZeroType, // 默认零值类型
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

// GetContext 获取应用上下文
func (m *ProviderManager) GetContext() IContext {
	return m.ctx
}

// Register 注册一个 provider
func (m *ProviderManager) Register(name string, provider IProvider) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.providers[name]; exists {
		return ErrProviderAlreadyExists
	}

	m.providers[name] = provider
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
	// 如果提供了自定义加载函数，则使用该函数加载 providers
	if len(loadFunc) > 0 {
		return loadFunc[0](m)
	}
	return nil, errors.New("no provider load function provided")
}

type DefaultPManager struct {
	IProviderManager
}

func NewDefaultManager(ctx IContext) *DefaultPManager {
	return &DefaultPManager{
		IProviderManager: NewProviderManager(ctx).SetType(ProviderTypeDefault().GroupDefaultPManager),
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

	var (
		runServerProvider IProvider
		errs              []error
	)

	for _, provider := range m.List() {
		if provider.Type().GetTypeID() == ProviderTypeDefault().GroupProviderAutoRun.GetTypeID() { // 自动运行类型的提供者，不依赖Target约束可以直接初始化
			_, err := provider.Initialize(m.GetContext())
			errs = append(errs, err)
		} else if provider.Type().GetTypeID() == ProviderTypeDefault().GroupWebRunServer.GetTypeID() { // 提供者类型匹配WebRunServer类型的提供者
			runServerProvider = provider
		} else if provider.Target() == bootCfg.CoreType { // 目标类型匹配启动配置的核心类型
			_, err := provider.Initialize(m.GetContext())
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to load providers: %v", errs)
	}

	if runServerProvider != nil {
		serverStarter, err := runServerProvider.Initialize(m.GetContext())
		if err != nil {
			return nil, err
		}
		s, ok := serverStarter.(ApplicationStarter)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for ApplicationStarter from provider '%s'", runServerProvider.Name())
		}
		RunApplicationStarter(s)
	}

	return nil, nil
}
