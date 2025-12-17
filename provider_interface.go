package fiberhouse

type ProviderInitFunc func(IProvider) (any, error)

type ProviderLoadFunc func(manager IProviderManager) (any, error)

// Provider 提供者接口
type IProvider interface {
	Name() string
	Version() string
	Initialize(IContext, ...ProviderInitFunc) (any, error)
	RegisterTo(manager IProviderManager) error
	Status() IState
	// Target returns the target framework of the provider, e.g., "gin", "fiber",...
	Target() string
	// Type returns the type of the provider, e.g., "middleware", "route_register", "sonic_json_codec", "std_json_codec",...
	Type() IProviderType
	SetName(string) IProvider
	SetVersion(string) IProvider
	SetTarget(string) IProvider
	SetStatus(IState) IProvider
	// SetType 设置提供者类型，仅允许设置一次
	SetType(IProviderType) IProvider
}

// Manager 提供者管理器接口
type IProviderManager interface {
	// Type 返回提供者类型
	Type() IProviderType
	// SetType 设置提供者类型，仅允许设置一次
	SetType(IProviderType) IProviderManager
	GetContext() IContext
	Register(name string, provider IProvider) error
	Unregister(name string) error
	GetProvider(name string) (IProvider, error)
	List() []IProvider
	LoadProvider(loadFunc ...ProviderLoadFunc) (any, error)
}

// Stater 提供者状态接口
type IState interface {
	Id() uint8
	Name() string
	Set(uint8, string) IState
	SetState(IState) IState
}
