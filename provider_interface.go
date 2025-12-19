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
	Check() // 检查提供者是否设置类型值
}

// Manager 提供者管理器接口
type IProviderManager interface {
	Name() string
	SetName(string) IProviderManager
	// Type 返回提供者类型
	Type() IProviderType
	// SetType 设置提供者类型，仅允许设置一次
	SetType(IProviderType) IProviderManager
	// 获取管理器的执行位置点
	Location() IProviderLocation
	// 设置管理器的执行位置点，仅允许设置一次
	SetOrBindToLocation(IProviderLocation, ...bool) IProviderManager
	GetContext() IContext
	Register(name string, provider IProvider) error
	Unregister(name string) error
	GetProvider(name string) (IProvider, error)
	List() []IProvider
	LoadProvider(loadFunc ...ProviderLoadFunc) (any, error)
	Check() // 检查提供者管理器是否设置类型值
}

// IState 提供者状态接口
type IState interface {
	Id() uint8
	Name() string
	Set(uint8, string) IState
	SetState(IState) IState
}
