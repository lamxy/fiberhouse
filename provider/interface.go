package provider

import "github.com/lamxy/fiberhouse"

type InitFunc func(origin any) error

// Provider 提供者接口
type IProvider interface {
	Name() string
	Version() string
	Initialize(fiberhouse.IContext, ...InitFunc) error
	RegisterToManager(manager IManager) error // 提供者管理器可由全局管理容器管理
	Status() IStater
	// Target returns the target framework of the provider, e.g., "gin", "fiber", "gearbox",...
	Target() string
	// Type returns the type of the provider, e.g., "middleware", "route_register", "sonic_json_codec", "std_json_codec",...
	Type() string
	SetName(string) IProvider
	SetVersion(string) IProvider
	SetTarget(string) IProvider
	SetStatus(IStater) IProvider
	SetType(string) IProvider
}

// Manager 提供者管理器接口
type IManager interface {
	GetAppContext() fiberhouse.IContext
	Register(name string, provider IProvider) error
	Unregister(name string) error
	GetProvider(name string) (IProvider, error)
	List() []IProvider
	LoadProvider(loadFunc ...func(manager IManager) error) error
}

// Stater 提供者状态接口
type IStater interface {
	Id() uint8
	Name() string
	Set(uint8, string) IStater
	SetState(IStater) IStater
}
