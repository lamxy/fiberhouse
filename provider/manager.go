package provider

import (
	"github.com/lamxy/fiberhouse"
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
func (s *State) Set(id uint8, name string) IStater {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.id = id
	s.name = name
	return s
}

// SetState 从另一个状态器设置状态
func (s *State) SetState(state IStater) IStater {
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
type Manager struct {
	ctx       fiberhouse.IContext
	providers map[string]IProvider
	lock      sync.RWMutex
}

// NewManager 创建一个基础的提供者管理器
func NewManager(ctx fiberhouse.IContext) *Manager {
	return &Manager{
		ctx:       ctx,
		providers: make(map[string]IProvider),
	}
}

// GetAppContext 获取应用上下文
func (m *Manager) GetAppContext() fiberhouse.IContext {
	return m.ctx
}

// Register 注册一个 provider
func (m *Manager) Register(name string, provider IProvider) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.providers[name]; exists {
		return ErrProviderAlreadyExists
	}

	m.providers[name] = provider
	return nil
}

// Unregister 注销一个 provider
func (m *Manager) Unregister(name string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.providers[name]; !exists {
		return ErrProviderNotFound
	}

	delete(m.providers, name)
	return nil
}

// GetProvider 获取指定名称的 provider
func (m *Manager) GetProvider(name string) (IProvider, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	provider, exists := m.providers[name]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return provider, nil
}

// List 返回所有已注册的 providers
func (m *Manager) List() []IProvider {
	m.lock.RLock()
	defer m.lock.RUnlock()

	list := make([]IProvider, 0, len(m.providers))
	for _, provider := range m.providers {
		list = append(list, provider)
	}

	return list
}

// LoadProvider 加载 providers
func (m *Manager) LoadProvider(loadFunc ...func(manager IManager) error) error {
	// 如果提供了自定义加载函数，则使用该函数加载 providers
	if len(loadFunc) > 0 {
		if err := loadFunc[0](m); err != nil {
			return err
		}
		return nil
	}

	// TODO 默认加载逻辑
	// TODO 模拟获取的启动配置，实际从全局上下文获取启动配置
	//  启动配置为sync.Map或map[string]interface{}，支持追加自定义配置项？
	bootCfg := map[string]interface{}{
		"core": "gin",
	}

	if len(m.List()) == 0 {
		return ErrProviderNotFound
	}

	//var errs []error

	for _, provider := range m.List() {
		if provider.Target() == bootCfg["core"] && provider.Type() == bootCfg["type"] {
			err := provider.Initialize(m.GetAppContext())
			m.GetAppContext().GetLogger().Error().Err(err).Msg("provider load failed")
			return err
		}
		return nil
	}

	//if len(errs) > 0 {
	//	return fmt.Errorf("failed to load providers: %v", errs)
	//}

	return nil
}
