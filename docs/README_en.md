# FiberHouse Framework

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![Fiber Version](https://img.shields.io/badge/fiber-v2.x-green.svg)](https://github.com/gofiber/fiber)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
<img src="https://img.shields.io/github/issues/lamxy/fiberhouse.svg" alt="GitHub Issues"></img>

📖 English | [中文](../README.md)

## 🏠 About FiberHouse

FiberHouse is a high-performance, pluggable, and modular Go Web & CMD framework built on top of the Fiber core by default. It includes a global manager, configurator, unified logger, validation wrapper, database/cache/middleware components, unified exception handling, and a complete command-line sub-framework, ready to use out of the box.

- Provides a powerful global management container that lets you register custom components once and reuse them everywhere, making replacement and extension easy.
- Defines interfaces and default implementations for the application starter, global context, and business layering at the framework level, supporting custom implementations and modular development.
- Allows you to assemble a flexible, complete, and switchable Go Web and CMD application just like furnishing a “house.”

### 🏆 Development Focus

Deliver a high-performance, extensible, customizable, and ready-to-use Go Web framework.

## ✨ Features

- **High performance**: Built on Fiber, offering blazing-fast HTTP performance with object pools, goroutine pools, caching, async optimizations, and more.
- **Modular design**: Clear layered architecture with standard interface contracts and implementations for teamwork, extension, and modular development.
- **Global manager**: Lock-free global object container with instant registration, lazy initialization, singleton traits, and a dependency solution that can replace DI tools plus unified lifecycle management.
- **Global configuration**: Unified loading, parsing, and management of configs; supports multiple formats and environment overrides for different scenarios.
- **Unified logging**: High-performance logging with structured logs, sync/async writers, and source tagging.
- **Unified exception handling**: Standardized error definitions/handling, modular error codes, integrated parameter validation, error tracing, and a friendly debugging experience.
- **Parameter validation**: Integrated validation wrapper with custom language validators, tag rules, and multilingual translators.
- **Database support**: Built-in MySQL/MongoDB drivers and model base classes.
- **Caching**: High-performance local/remote/two-level cache combo with model base support.
- **Task queue**: Redis-based high-performance C/S async task queue with scheduling, delay, and retries.
- **API docs**: Integrated swag for automatic API documentation.
- **Command-line apps**: Full CLI framework support with unified modular design for collaboration and extension.
- **Sample templates**: Complete Web and CMD templates covering common scenarios and best practices; tweak and go.
- **More**: Continuous optimization and updates...

## 🏗️ Architecture Overview & Notes
### Core Architectural Layers

```
fiberhouse/  # FiberHouse framework core
├── Core Interface Definitions
│   ├── `application_interface.go`         # Application starter interface; defines lifecycle contracts
│   ├── `command_interface.go`             # CLI app interface; defines CLI command registration/execution contracts
│   ├── `context_interface.go`             # Global context interface; unified access to app context
│   ├── `locator_interface.go`             # Service locator interface; service lookup and dependency resolution
│   ├── `model_interface.go`               # Data model interface; unified data access contracts
│   ├── `provider_interface.go`            # Provider interface; component registration/initialization contracts
│   └── `recover_interface.go`             # Recovery handler interface; panic capture/recovery contracts
├── Core Implementations
│   ├── `application_impl.go`              # Default application starter implementation; standard start-up flow
│   ├── `context_impl.go`                  # Default global context; manages config, logging, container, etc.
│   ├── `provider_impl.go`                 # Provider base implementation; base capabilities for registration
│   ├── `provider_manager_impl.go`         # Provider manager; unified lifecycle for all providers
│   └── `service_impl.go`                  # Service locator implementation; service lookup and DI capabilities
├── Provider Management
│   ├── `provider_type.go`                 # Provider type groups; classification and identifiers
│   ├── `provider_location.go`             # Provider execution locations; ordering in the start-up flow
│   └── `providers/`                       # Built-in providers; framework preset core component providers
│       ├── `core_starter_fiber_provider.go`     # Fiber core starter provider
│       ├── `core_starter_gin_provider.go`       # Gin core starter provider
│       ├── `json_sonic_fiber_provider.go`       # Sonic JSON codec provider
│       └── `response_providers_manager_impl.go` # Response provider manager
├── Application Boot
│   ├── `boot.go`                          # Unified boot; one-click start and options
│   ├── `frame_starter_impl.go`            # Framework starter; orchestrates framework-level start-up
│   ├── `frame_starter_manager.go`         # Framework starter manager; coordinates multiple starters
│   ├── `core_fiber_starter_impl.go`       # Fiber core starter; HTTP service via Fiber
│   ├── `core_gin_starter_impl.go`         # Gin core starter; HTTP service via Gin
│   └── `commandstarter/`                  # CLI start-up; CLI app start and command management
│       ├── `cmdline_starter.go`                 # CLI starter; manages CLI start-up flow
│       └── `core_cmd_application.go`            # Core CLI app; CLI framework core
├── Configuration Management
│   ├── `bootstrap/`
│   │   └── `bootstrap.go`                 # Config and logger init; infrastructure before start-up
│   └── `appconfig/`
│       └── `config.go`                    # Multi-format config loading; YAML/JSON/env, etc.
├── Global Management
│   ├── `globalmanager/`
│   │   ├── `interface.go`                 # Manager interface; unified global object management
│   │   ├── `manager.go`                   # Manager implementation; lock-free, lazy-init container
│   │   └── `types.go`                     # Type definitions; related types/constants
│   └── `global_utility.go`                # Global utilities; registration, lookup, namespaces, etc.
├── Data Access
│   └── `database/`
│       ├── `dbmysql/`
│       │   ├── `interface.go`                   # MySQL interface
│       │   ├── `mysql.go`                       # MySQL connection management/config
│       │   └── `mysql_model.go`                 # MySQL model base; GORM basics
│       └── `dbmongo/`
│           ├── `interface.go`                   # MongoDB interface
│           ├── `mongo.go`                       # MongoDB connection management/config
│           └── `mongo_model.go`                 # MongoDB model base; document operations
├── Cache System
│   └── `cache/`
│       ├── `cache_interface.go`           # Cache interface; unified cache operations
│       ├── `cache_option.go`              # Cache options; flexible strategies
│       ├── `cache_utility.go`             # Cache helpers; convenient cache operations
│       ├── `helper.go`                    # Cache helpers; key generation, etc.
│       ├── `cache2/`
│       │   └── `level2_cache.go`                # Two-level cache; local + remote strategy
│       ├── `cachelocal/`
│       │   ├── `local_cache.go`                 # Local cache; high-performance Ristretto
│       │   └── `type.go`                        # Local cache types
│       └── `cacheremote/`
│           ├── `cache_model.go`                 # Remote cache model; serialization helpers
│           └── `redis_cache.go`                 # Redis cache; distributed cache
├── Core Components
│   └── `component/`
│       ├── `dig_container.go`             # DI container; Uber Dig
│       ├── `jsoncodec/`
│       │   └── `sonicjson.go`                   # Sonic JSON codec; high-performance JSON
│       ├── `validate/`
│       │   ├── `type_interface.go`              # Validator interface
│       │   ├── `validate_wrapper.go`            # Validation wrapper; unified parameter validation
│       │   ├── `en.go`                          # English validation translations
│       │   ├── `zh_cn.go`                       # Simplified Chinese validation translations
│       │   └── `zh_tw.go`                       # Traditional Chinese validation translations
│       ├── `writer/`
│       │   ├── `async_channel_writer.go`        # Async logger writer via channel
│       │   ├── `async_diode_writer.go`          # Async logger writer via diode
│       │   └── `sync_lumberjack_writer.go`      # Sync log rotation via Lumberjack
│       └── `tasklog/`
│           └── `logger_adapter.go`              # Task log adapter; Asynq integration
├── Middleware
│   └── `middleware/`
│       ├── `recover_config.go`            # Recovery middleware config; panic recovery strategies
│       ├── `recover_error_handler_impl.go` # Unified panic handler
│       └── `recover_interface.go`         # Recovery middleware interface
├── Response Handling
│   └── `response/`
│       ├── `response_interface.go`        # Response interface; unified response contract
│       ├── `response_info_impl.go`        # Standard JSON response
│       ├── `response_proto_impl.go`       # Protobuf response
│       ├── `response_msgpack_impl.go`     # MessagePack response
│       └── `response.go`                  # Response utilities; quick helpers
├── Exception Handling
│   └── `exception/`
│       ├── `types.go`                     # Exception types; business error codes
│       └── `exception_error.go`           # Exception implementation; unified handling/propagation
├── Utilities
│   ├── `utils/`
│   │   └── `common.go`                    # Common helpers; strings, time, etc.
│   └── `constant/`
│       ├── `constant.go`                  # Framework constants
│       └── `exception.go`                 # Exception constants; predefined codes/messages
└── Business Layer Interfaces
    [...]
```

### Architectural Principles

- **Interface-driven**: Core functionality defined by interface contracts for flexible extension.
- **Provider mechanism**: Register and manage components via the Provider pattern.
- **Clear layering**: Strict layered architecture with clear responsibilities.
- **Pluggable design**: Freely switch core frameworks (Fiber/Gin) and components.

## 🚀 Quick Start

### Requirements

- Go 1.24 or higher (recommend 1.25+)
- MySQL 5.7+ or MongoDB 4.0+
- Redis 5.0+

### Start DB and cache with docker for framework debugging

- Docker Compose file: [docker-compose.yml](docs/docker_compose_db_redis_yaml/docker-compose.yml)
- Start command: `docker compose up -d`

```bash
cd  docs/docker_compose_db_redis_yaml/
docker compose up -d
```

### Installation

FiberHouse requires **Go 1.24 or higher**. To install or upgrade Go, visit the [official Go downloads](https://go.dev/dl/).
Create a project directory and initialize Go Modules:

```bash
go mod init github.com/your/repo
```
Install FiberHouse:

```bash
go get github.com/lamxy/fiberhouse
```

### Main file example

See: [example_main/main.go](./example_main/main.go)

```go
package main

import (
  "github.com/lamxy/fiberhouse"
  "github.com/lamxy/fiberhouse/constant"
  "github.com/lamxy/fiberhouse/example_application/providers/middleware"
  "github.com/lamxy/fiberhouse/example_application/providers/module"
  "github.com/lamxy/fiberhouse/example_application/providers/optioninit"
  _ "github.com/lamxy/fiberhouse/example_main/docs" // swagger docs
)

// Version information, injected via ldflags at build time
// Usage: go build -ldflags "-X main.Version=v1.0.0"
var (
  Version string // version
)

func main() {
  // Create FiberHouse application instance
  fh := fiberhouse.New(&fiberhouse.BootConfig{
    AppName:                     "Default FiberHouse Application",          // application name
    Version:                     Version,                                   // application version
    FrameType:                   constant.FrameTypeWithDefaultFrameStarter, // default frame starter identifier: DefaultFrameStarter
    CoreType:                    constant.CoreTypeWithFiber,                // fiber | gin | ...
    TrafficCodec:                constant.TrafficCodecWithSonic,            // codec for traffic: sonic_json_codec|std_json_codec|go_json_codec|pb...
    EnableBinaryProtocolSupport: true,                                      // whether to enable binary protocol support, such as Protobuf
    ConfigPath:                  "./example_config",                        // global application config path
    LogPath:                     "./example_main/logs",                     // log file path
  })

  // Collect providers and managers
  providers := fiberhouse.DefaultProviders().AndMore(
    // Option initialization providers for frame starter and core starter.
    // Note: Since the option init manager is uniquely bound to the corresponding provider when New is called,
    // these providers need not be created/collected here.
    // See NewFrameOptionInitPManager() function
    //optioninit.NewFrameOptionInitProvider(),
    //optioninit.NewCoreOptionInitProvider(),

    // Middleware providers for Fiber-based app
    middleware.NewFiberAppMiddlewareProvider(),
    middleware.NewFiberModuleMiddlewareProvider(),
    // Middleware providers for Gin-based app
    middleware.NewGinAppMiddlewareProvider(),
    // Other framework-related middleware providers (switchable)
    // ...

    // Fiber module route and swagger registration provider
    module.NewFiberRouteRegisterProvider(),
    // Gin module route and swagger registration provider
    module.NewGinRouteRegisterProvider(),
    // More module route registration providers for other core frameworks
    // ...
  )
  managers := fiberhouse.DefaultPManagers(fh.AppCtx).AndMore(
    // Frame option init manager, obtain the list of option init functions from the frame starter
    optioninit.NewFrameOptionInitPManager(fh.AppCtx),
    // Core option init manager, obtain the list of option init functions from the core starter
    optioninit.NewCoreOptionInitPManager(fh.AppCtx).MountToParent(),
    // App middleware manager, registers application-level middleware to the core app instance
    middleware.NewAppMiddlewarePManager(fh.AppCtx),
    // Module route register manager, registers module routes to the core app instance
    module.NewRouteRegisterPManager(fh.AppCtx),
  )

  // Initialize providers and managers and run the server
  fh.WithProviders(providers...).WithPManagers(managers...).RunServer()
}
```

### Quick Try

- Web app quick try

```bash
# Clone
git clone https://github.com/lamxy/fiberhouse.git

# Enter repo
cd fiberhouse

# Install deps
go mod tidy

# Enter example_main/
cd example_main/

# View README
cat README_go_build.md

# Build (Windows example; see cross-compiling for others)
# Current working dir: fiberhouse/; output to example_main/target/
cd ..
go build "-ldflags=-X 'main.Version=v0.0.1'" -o ./example_main/target/examplewebserver.exe ./example_main/main.go

# Run
./example_main/target/examplewebserver.exe
# or Linux/MacOS
./example_main/target/examplewebserver
```

Access hello world: http://127.0.0.1:8080/example/hello/world

Response: {"code":0,"msg":"ok","data":"Hello World!"}

```bash
curl -sL  "http://127.0.0.1:8080/example/hello/world"

# Response:
{
    "code": 0,
    "msg": "ok",
    "data": "Hello World!"
}
```

- CMD app quick try

```bash
# Prepare MySQL
mysqlsh root:root@localhost:3306

# Create test DB
CREATE DATABASE IF NOT EXISTS test CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

# Clone
git clone https://github.com/lamxy/fiberhouse.git

# Enter repo
cd fiberhouse

# Install deps
go mod tidy

# Enter example_application/command/
cd example_application/command/

# View README
cat README_go_build.md

# Build (Windows keeps .exe; Linux/MacOS omit)
go build -o ./target/cmdstarter.exe ./main.go

# Set env; Windows reads application_dev.yml
set APP_ENV_application_env=dev

# Linux or MacOS
# export APP_ENV_application_env=dev

# Run help
./target/cmdstarter.exe -h
# or
./target/cmdstarter -h

# Run subcommand, view log
./target/cmdstarter.exe test-orm -m ok
# or
./target/cmdstarter test-orm -m ok

# Console output: ok
# result:  ExampleMysqlService.TestOK: OK --from: ok
```

## ⚙️ Core Interfaces & Key Design

### Design Philosophy

FiberHouse adopts **interface-driven** and **provider-based** design to achieve high extensibility and customization through clear contracts and flexible providers.

### Core Interface System

#### 1. Application Start Interfaces

##### FrameStarter

**Location**: `application_interface.go` [jump](./application_interface.go)

**Role**: Defines common framework initialization

- Global object init and management
- Task server start
- Access to app context
- Register custom init logic

**Default implementation**: `frame_starter_impl.go`

```go
type FrameStarter interface {
    IStarter
    // GetContext gets the application context
    // Returns the global application context, providing access to config, logger, global container and other infrastructure
    GetContext() IApplicationContext
    
    // RegisterApplication registers an application registrar
    // Injects the application registrar instance into the starter for subsequent global object initialization and configuration
    RegisterApplication(application ApplicationRegister)
    
    // RegisterModule registers a module registrar
    // Injects the module registrar instance into the starter for module-level middleware, route and Swagger registration
    RegisterModule(module ModuleRegister)
    
    // GetModule retrieves the module registrar
    // Returns the registered module registrar instance
    GetModule() ModuleRegister
    
    // RegisterTask registers a task registrar
    // Injects the task registrar instance into the starter for async task server initialization and start
    RegisterTask(task TaskRegister)
    
    // GetTask retrieves the task registrar
    // Returns the registered task registrar instance
    GetTask() TaskRegister
    
    // RegisterToCtx registers the starter into the context
    // Registers the starter instance into the application context for access by other components
    RegisterToCtx(starter ApplicationStarter)
    
    // RegisterApplicationGlobals registers initialization of application global objects and necessary instances
    // Registers global object initializers, initializes required global instances, config validators, etc.
    // Includes initialization of database, cache, Redis, validators, custom tags, etc.
    RegisterApplicationGlobals(...IProviderManager)
    
    // RegisterLoggerWithOriginToContainer registers loggers with origin identifiers to the container
    // Initializes and registers sub-loggers defined in config with origin tags for convenient retrieval
    RegisterLoggerWithOriginToContainer()
    
    // RegisterGlobalsKeepalive registers global object keepalive mechanism
    // Starts background health checks to periodically inspect global objects and automatically rebuild unhealthy instances
    RegisterGlobalsKeepalive(...IProviderManager)
    
    // RegisterTaskServer registers the asynchronous task server
    // Starts the async task server according to configuration, registers task handlers, runs worker services and begins listening on task queues
    RegisterTaskServer(...IProviderManager)
    
    // GetFrameApp retrieves the frame starter instance
    GetFrameApp() FrameStarter
}
```

**Extend**: Implement `FrameStarter` for custom framework init.

##### CoreStarter

**Location**: `application_interface.go` [jump](./application_interface.go)

**Role**: Defines underlying core framework start logic

- Core app creation (Fiber/Gin/...)
- Middleware registration
- Route registration
- Service listen/start

**Built-ins**:

- Fiber core starter: `core_fiber_starter_impl.go`
- Gin core starter: `core_gin_starter_impl.go`

```go
// CoreStarter - Core application starter interface
type CoreStarter interface {
  // GetAppContext - get application context
  // Returns the global application context which provides access to configuration, logger, global container, etc.
  GetAppContext() IApplicationContext
  
  // InitCoreApp - initialize the core application
  // Create and configure the underlying HTTP service instance (e.g., Fiber app)
  InitCoreApp(fs FrameStarter, managers ...IProviderManager)
  
  // RegisterAppMiddleware - register application-level middlewares
  // Register global middlewares such as recovery, request logging, CORS, etc.
  RegisterAppMiddleware(fs FrameStarter, managers ...IProviderManager)
  
  // RegisterModuleSwagger - register module Swagger documentation
  // Decide whether to register Swagger API documentation routes based on configuration
  RegisterModuleSwagger(fs FrameStarter, managers ...IProviderManager)
  
  // RegisterAppHooks - register application hooks
  // Register lifecycle hook callbacks such as start and shutdown handlers
  RegisterAppHooks(fs FrameStarter, managers ...IProviderManager)
  
  // RegisterModuleInitialize - register module initialization
  // Perform module-level initialization, including registering module middlewares and route handlers
  RegisterModuleInitialize(fs FrameStarter, managers ...IProviderManager)
  
  // AppCoreRun - run the core application
  // Start HTTP server listener and handle graceful shutdown signals
  AppCoreRun(...IProviderManager)
  
  // GetCoreApp - get the core instance
  GetCoreApp() interface{}
}
```

**Extend**: Implement `CoreStarter` to integrate other web frameworks.

##### Register Interfaces

**Location**: `application_interface.go` [jump](./application_interface.go)

**List**:

- `ApplicationRegister`: app-level init logic
- `ModuleRegister`: module-level init logic
- `TaskRegister`: task-level init logic

```go
// ApplicationRegister - Application registrar
//
// Called by the starter during application boot, used for:
// 1. Registering the application's custom configuration, dependencies and initialization logic;
// 2. Binding the registrar instance to the ApplicationStarter's application field for use during the startup flow.
type ApplicationRegister interface {
  IRegister
  IApplication
  // GetContext - return the global context
  GetContext() IApplicationContext
  
  // ConfigGlobalInitializers - configure and return the mapping/list of global object initializers
  ConfigGlobalInitializers() globalmanager.InitializerMap
  // ConfigRequiredGlobalKeys - configure and return the slice of global object key names that need initialization
  ConfigRequiredGlobalKeys() []globalmanager.KeyName
  // ConfigCustomValidateInitializers - configure and return a slice of custom language validator initializers
  // see framework component: validate.Wrap
  ConfigCustomValidateInitializers() []validate.ValidateInitializer
  // ConfigValidatorCustomTags - configure and return a slice of validator custom tag functions and translations
  // (used to provide translations when a validator tag lacks the required language translation)
  // see framework component: validate.RegisterValidatorTagFunc
  ConfigValidatorCustomTags() []validate.RegisterValidatorTagFunc
  
  // RegisterAppMiddleware - register application-level middleware
  RegisterAppMiddleware(cs CoreStarter)
  
  // RegisterCoreHook - register lifecycle hooks for the core application (coreApp)
  RegisterCoreHook(cs CoreStarter)
}
```

```go
// ModuleRegister module registrar
//
// Used to register application modules/subsystems, including middleware, routes, swagger, etc.
// The starter will call the module registrar to complete module initialization.
type ModuleRegister interface {
  IRegister
  // GetContext returns the global context
  GetContext() IApplicationContext
  
  // RegisterModuleMiddleware register module-level/subsystem middleware
  // RegisterModuleMiddleware(cs CoreStarter)
  
  // RegisterModuleRouteHandlers register module-level/subsystem route handlers
  RegisterModuleRouteHandlers(cs CoreStarter)
  // RegisterSwagger register swagger
  RegisterSwagger(cs CoreStarter)
}
```

```go
// TaskRegister task register (based on asynq)
//
// Users should implement this interface and register it to the ApplicationStarter during application startup.
// The registered TaskRegister instance will be bound to the ApplicationStarter's task field, and the starter will call its methods to complete task component initialization.
//
// When the global configuration enables the asynchronous task component, the TaskRegister is responsible for:
// 1. Centrally declaring and registering task types (asynq task names) and their handler functions into a mapping container.
// 2. Registering initializers for the task dispatcher and task worker into the global container.
// 3. Providing access methods to obtain the task dispatcher and worker instances.
type TaskRegister interface {
    IRegister
    // GetContext returns the global context
    GetContext() IApplicationContext
    
    // GetTaskHandlerMap returns the mapping of task handlers
    //
    // Example:
    // func myTaskHandler(ctx context.Context, t *asynq.Task) error {
    //     // handle task logic
    //     return nil // or return an error
    // }
    //
    // taskHandlerMap := map[string]func(context.Context, *asynq.Task) error{
    //     "task_type_1": myTaskHandler,
    //     // more task types and their handlers
    // }
    GetTaskHandlerMap() map[string]func(context.Context, *asynq.Task) error
    
    // AddTaskHandlerToMap adds a new task handler to the handler mapping
    //
    // Example:
    // func myTaskHandler2(ctx context.Context, t *asynq.Task) error {
    //     // handle task logic
    //     return nil // or return an error
    // }
    //
    // taskRegister.AddTaskHandlerToMap("task_type_2", myTaskHandler2)
    AddTaskHandlerToMap(pattern string, handler func(context.Context, *asynq.Task) error)
    
    // RegisterTaskServerToContainer registers the async task server initializer into the container
    RegisterTaskServerToContainer()
    
    // RegisterTaskDispatcherToContainer registers the async task client/dispatcher initializer into the container
    RegisterTaskDispatcherToContainer()
    
    // GetTaskDispatcher returns the task client/dispatcher instance
    GetTaskDispatcher() (*TaskDispatcher, error)
    
    // GetTaskWorker returns the task server/worker instance
    GetTaskWorker(key string) (*TaskWorker, error)
}
```

**Purpose**: Layered management of init logic for app, modules/subsystems, and tasks.

#### 2. Provider Mechanism

##### IProvider

**Location**: `provider_interface.go` [jump](./provider_interface.go)

**Role**: Contract for extensible components

- Provider name/type
- Registration logic
- Dependency declaration

**Base**: `provider_impl.go` [jump](./provider_impl.go)

```go
// IProvider provider interface
type IProvider interface {
    // Name returns the provider name
    Name() string
    // Version returns the provider version
    Version() string
    // Initialize performs provider initialization
    Initialize(IContext, ...ProviderInitFunc) (any, error)
    // RegisterTo registers the provider to a provider manager
    RegisterTo(manager IProviderManager) error
    // Status returns the provider's current status
    Status() IState
    // Target returns the provider's target framework engine type, e.g., "gin", "fiber", ...
    // This field distinguishes provider implementations for different framework engines and can also be used to differentiate other dimensions
    Target() string
    // Type returns the provider type, e.g., "middleware", "route_register", "sonic_json_codec", "std_json_codec", ...
    Type() IProviderType
    // SetName sets the provider name
    SetName(string) IProvider
    // SetVersion sets the provider version
    SetVersion(string) IProvider
    // SetTarget sets the provider target framework
    SetTarget(string) IProvider
    // SetStatus sets the provider status
    SetStatus(IState) IProvider
    // SetType sets the provider type; allowed to set only once
    SetType(IProviderType) IProvider
    // Check verifies whether the provider type has been set
    Check()
    // BindToUniqueManagerIfSingleton binds the provider to a unique manager
    // Note: the provided manager should be a singleton implementation to ensure global uniqueness
    // This method internally calls the manager's BindToUniqueProvider to create a mutual unique binding
    // Returns the provider itself to support chaining
    // Effective conditions: 1. the passed manager is a singleton; 2. the subclass provider overrides this method and the subclass instance calls it; 3. the subclass instance needs to be mounted back to a parent field
    BindToUniqueManagerIfSingleton(IProviderManager) IProvider
    // MountToParent mounts the current provider to a parent provider
    MountToParent(son ...IProvider) IProvider
}
```

**Use cases**:

- Custom middleware registration
- Custom JSON codec
- Custom core starter
- Any extension

**Note**: Base implementation is provided; compose/extend without re-implementing.

##### IProviderManager

**Location**: `provider_interface.go` [jump](./provider_interface.go)

**Role**: Central provider management and location binding

- Collect providers
- Batch registration
- Bind to execution locations
- Lifecycle management

**Base**: `provider_manager_impl.go` [jump](./provider_manager_impl.go)

```go
// IProviderManager provider manager interface
type IProviderManager interface {
    // Name returns the provider manager's name
    Name() string
    // SetName sets the provider manager's name
    SetName(string) IProviderManager
    // Type returns the provider type
    Type() IProviderType
    // SetType sets the provider type; allowed to set only once
    SetType(IProviderType) IProviderManager
    // Location returns the execution location of the manager
    Location() IProviderLocation
    // SetOrBindToLocation sets (or binds) the manager's execution location; allowed to set only once
    SetOrBindToLocation(IProviderLocation, ...bool) IProviderManager
    // GetContext returns the context associated with the manager
    GetContext() IContext
    // Register registers a provider into the manager
    Register(provider IProvider) error
    // Unregister removes a provider from the manager by name
    Unregister(name string) error
    // GetProvider retrieves a provider instance by name
    GetProvider(name string) (IProvider, error)
    // List lists all providers registered in the manager
    List() []IProvider
    // Map returns a map of provider name to provider instance for all registered providers
    Map() map[string]IProvider
    // LoadProvider loads providers using provided load functions
    LoadProvider(loadFunc ...ProviderLoadFunc) (any, error)
    // Check verifies whether the provider manager has its type set
    Check()
    // BindToUniqueProvider binds a single unique provider to the manager
    // Ensures the manager has at most one provider registered
    // If the same provider record already exists, treat as success
    // If multiple providers exist, panic
    // Returns the manager to allow chaining
    BindToUniqueProvider(IProvider) IProviderManager
    // IsUnique returns whether the manager is in unique-provider mode
    IsUnique() bool
    // MountToParent mounts the current manager to a parent manager
    MountToParent(son ...IProviderManager) IProviderManager
}
```

**Note**: Base manager provided; compose/extend directly.

##### Provider Type Groups

**Location**: `provider_type.go` [jump](./provider_type.go)

**Built-ins**:

```go
// DefaultPType is the predefined collection of default provider types.
//
// Default grouping logic for provider types: providers of the same type are only allowed
// to register into managers of the same type and be processed accordingly.
// 1. GroupXXXChoose: types ending with Choose indicate selecting one provider to execute
//    (only a single provider matching Target() is executed; subsequent providers are skipped).
//    Example: switching core engine or traffic codec — select one provider from the manager's list.
// 2. GroupYYYType: types ending with Type indicate multiple providers that meet constraints
//    like Target/Name/Version can execute (e.g., multiple middleware or route-register providers).
// 3. GroupZZZAutoRun: types ending with AutoRun indicate automatic execution; all registered
//    providers run once (e.g., global object registration, default starter initializers).
// 4. GroupWWWUnique: types ending with Unique indicate exactly one provider exists and executes
//    (e.g., frame starter option init provider bound uniquely; manager cannot register more).
// 5. Others: custom-defined by developers.
type DefaultPType struct {
    ZeroType                        IProviderType // Default zero-value type
    GroupDefaultManagerType         IProviderType // Default manager type group; providers of this type register into the default manager
    GroupTrafficCodecChoose         IProviderType // Traffic codec choose group; select a single provider for traffic encoding/decoding
    GroupCoreEngineChoose           IProviderType // Core engine choose group; select a single provider for core engine handling
    GroupMiddlewareRegisterType     IProviderType // Middleware register group; providers of this type register into the middleware chain
    GroupRouteRegisterType          IProviderType // Route register group; providers of this type register into the route table
    GroupCoreHookChoose             IProviderType // Core hook choose group; select a single provider for core hook handling
    GroupFrameStarterChoose         IProviderType // Frame starter choose group; select a single provider for frame starter handling
    GroupCoreStarterChoose          IProviderType // Core starter choose group; select a single provider for core starter handling
    GroupProviderAutoRun            IProviderType // Provider auto-run group; providers of this type run automatically once
    GroupCoreContextChoose          IProviderType // Core context choose group; select a single provider for core context handling
    GroupFrameStarterOptsInitUnique IProviderType // Frame starter options init unique group; only one manager/provider is bound and used
    GroupCoreStarterOptsInitUnique  IProviderType // Core starter options init unique group; only one manager/provider is bound and used
    GroupRecoverMiddlewareChoose    IProviderType // Recover middleware choose group; select a single provider for recovery middleware (based on core type)
    GroupResponseInfoChoose         IProviderType // Response info choose group; select a single provider for response info handling (chosen by name/content-type)
}
```

**Extend**: `ProviderTypeDefault().MustCustom("xxx")`.

##### Execution Locations

**Location**: `provider_location.go` [jump](./provider_location.go)

**Built-ins**:

```go
// DefaultPLocation predefined default location object collection
//
// Locations are used to mark provider execution points; managers with the same location are collected and executed in order
// 1. LocationXXXBefore: executed before a certain stage
// 2. LocationXXXAfter: executed after a certain stage
// 3. LocationXXXInit: executed during an initialization stage
// 4. LocationXXXRun: executed during a run stage
// 5. LocationXXXCreate: executed during a creation stage
// 6. Others: customizable by developers
type DefaultPLocation struct {
    ZeroLocation                   IProviderLocation // initial/default/zero location (reserved for initialization state)
    LocationAdaptCoreCtxChoose     IProviderLocation // adapt core context selection location (used to normalize response output across different core engine contexts)
    LocationBootStrapConfig        IProviderLocation // bootstrap configuration stage location
    LocationFrameStarterOptionInit IProviderLocation // frame starter option initialization location
    LocationCoreStarterOptionInit  IProviderLocation // core starter option initialization location
    LocationFrameStarterCreate     IProviderLocation // frame starter creation location
    LocationCoreStarterCreate      IProviderLocation // core engine starter creation location
    LocationGlobalInit             IProviderLocation // global initialization location
    LocationGlobalKeepaliveInit    IProviderLocation // global object keepalive initialization location
    LocationCoreEngineInit         IProviderLocation // core engine initialization location
    LocationCoreHookInit           IProviderLocation // core engine hook (if any) initialization location
    LocationAppMiddlewareInit      IProviderLocation // application middleware registration initialization location
    LocationModuleMiddlewareInit   IProviderLocation // module middleware registration initialization location
    LocationRouteRegisterInit      IProviderLocation // route registration initialization location
    LocationTaskServerInit         IProviderLocation // task server initialization location
    LocationModuleSwaggerInit      IProviderLocation // Swagger registration initialization location
    LocationServerRunBefore        IProviderLocation // server run before location
    LocationServerRun              IProviderLocation // server run location
    LocationServerRunAfter         IProviderLocation // server run after location
    LocationServerShutdownBefore   IProviderLocation // server shutdown before location
    LocationServerShutdown         IProviderLocation // server shutdown location
    LocationServerShutdownAfter    IProviderLocation // server shutdown after location
    LocationResponseInfoInit       IProviderLocation // response info initialization location
}
```

**How it works**:

1. Manager calls `SetOrBindToLocation(LocationServerRun)` to bind.
2. Framework triggers locations during lifecycle (e.g., server run).
3. Loads and executes bound managers automatically.

**Benefit**: Precise control of load timing; fine-grained lifecycle management.

#### 3. Global Context Interface

##### IAppContext

**Location**: `context_interface.go` [jump](./context_interface.go)

**Role**: Access global singletons at runtime

- Start options
- Configurator
- Logger
- Global manager
- Validator
- Starter instance

**Default**: `context_impl.go` [jump](./context_impl.go)

```go
// IContext global context interface
type IContext interface {
    // GetConfig defines method to obtain global configuration
    GetConfig() appconfig.IAppConfig
    // GetLogger defines method to obtain the global logger
    GetLogger() bootstrap.LoggerWrapper
    // GetContainer defines method to obtain the global manager
    GetContainer() *globalmanager.GlobalManager
    // GetStarter defines method to obtain the starter instance (used to access IApplication methods)
    GetStarter() IStarter
    // GetLoggerWithOrigin defines method to obtain a singleton child logger with origin (retrieved from the global manager)
    GetLoggerWithOrigin(originFormCfg appconfig.LogOrigin) (*zerolog.Logger, error)
    // GetMustLoggerWithOrigin defines method to obtain a child logger with origin and panic on failure (retrieved from the global manager)
    GetMustLoggerWithOrigin(originFormCfg appconfig.LogOrigin) *zerolog.Logger
    // GetValidateWrap defines method to obtain the global validator wrapper
    GetValidateWrap() validate.ValidateWrapper
}
```

```go
// IApplicationContext framework web application context interface
type IApplicationContext interface {
    IContext
    // RegisterStarterApp mount framework starter app
    RegisterStarterApp(sApp ApplicationStarter)
    // GetStarterApp get framework application starter instance (e.g., WebApplication)
    GetStarterApp() ApplicationStarter
    // RegisterAppState register application start state
    RegisterAppState(bool)
    // GetAppState get application start state
    GetAppState() bool
    // GetBootConfig get boot configuration
    GetBootConfig() *BootConfig
    // RegisterBootConfig register boot configuration
    RegisterBootConfig(bc *BootConfig)
}
```

**Note**: Default global app context implementation provided; compose as needed.

#### 4. Business Layer Interfaces

##### Locator Interfaces

**Location**: `locator_interface.go` [jump](./locator_interface.go)

**List**:

- `ApiLocator`
- `ServiceLocator`
- `RepositoryLocator`
- `TaskLocator`

**Capabilities**:

- Access app context
- Access config/logger
- Access global manager
- Unified logging

**Example**:

```go
type ExampleService struct {
    fiberhouse.ServiceLocator
    Repo *repository.ExampleRepository
}

func (s *ExampleService) DoSomething() {
    // 直接使用定位器能力
    logger := s.GetLogger()
    config := s.GetConfig()
    instance := s.GetInstance("key")
}
```

**Note**: Base locator implementations provided; compose/extend directly.

#### 5. Exception Handling Interfaces

##### IErrorHandler

**Location**: `recover_interface.go` [jump](./recover_interface.go)

```go
// IErrorHandler error handling interface, used to uniformly define stack trace logging and error handling methods
type IErrorHandler interface {
    DefaultStackTraceHandler(providerctx.ICoreContext, interface{})
    ErrorHandler(providerctx.ICoreContext, error) error
    GetContext() IApplicationContext
    RecoverMiddleware(...RecoverConfig) any
}
```

**Role**: Unified error handling

- Panic capture
- Error logging
- Response formatting
- Multi-framework adaptation
  - Fiber error handler adapter: `fiber_error_handler.go` [jump to file](`./provider/adaptor/fiber_error_handler.go`)
  - Gin error handler adapter: `gin_error_handler.go` [jump to file](`./provider/adaptor/gin_error_handler.go`)

**Built-in**:

- Unified handler: `recover_error_handler_impl.go` [jump](./recover_error_handler_impl.go)

**Note**: Custom implementations supported.

##### IRecover

**Location**: `recover_interface.go` [jump](./recover_interface.go)

```go
// IRecover is the panic recovery interface used to extract route params, query params,
// retrieve the traceID from different framework request contexts, and to define recovery middleware methods.
type IRecover interface {
    // GetParamsJson returns the JSON-encoded bytes of route parameters.
    GetParamsJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
    // GetQueriesJson returns the JSON-encoded bytes of query parameters.
    GetQueriesJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
    // GetHeadersJson returns the JSON-encoded bytes of request headers (with sensitive information masked).
    GetHeadersJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte
    // RecoverPanic returns the recovery middleware function. It returns the framework-specific middleware (e.g., fiber, gin).
    // The recovery middleware manager automatically selects and returns the appropriate provider based on boot configuration.
    RecoverPanic(...RecoverConfig) any
    TraceID(ctx providerctx.ICoreContext, flag ...string) string
    GetHeader(ctx providerctx.ICoreContext, key string) string
}
```

**Role**: Panic recovery

- Panic capture
- Stack trace
- Error response

**Built-ins**:
- FiberRecovery: `recover_recoveries_impl.go`
- GinRecovery: `recover_recoveries_impl.go`

#### 6. Response Handling Interfaces

##### IResponse

**Location**: `response/response_interface.go`  [jump](./response/response_interface.go)

**Role**: Unified response format

- Code/message/data
- Multiple serialization protocols
- Object pool optimizations

**Built-ins**:

- `RespInfo`: JSON response (pool) [jump](./response/response_impl.go)
- `Exception`: exception response (pool) [jump](./response/response_impl.go)
- `ValidateException`: validation exception (pool) [jump](./response/response_impl.go)
- `RespInfoProto`: Protobuf response (pool) [jump](./response/response_proto_impl.go)
- `RespInfoMagPack`: MsgPack response (pool) [jump](./response/response_msgpack_impl.go)
- `RespInfoProtobufProvider`: Protobuf response provider [jump](./response_providers_manager_impl.go)
- `RespInfoMsgpackProvider`: MsgPack response provider [jump](./response_providers_manager_impl.go)
- `RespInfoPManager`: response provider manager [jump](./response_providers_manager_impl.go)

```go
type IResponse interface {
    GetCode() int
    GetMsg() string
    GetData() interface{}
    SendWithCtx(c providerctx.ICoreContext, status ...int) error
    JsonWithCtx(c providerctx.ICoreContext, status ...int) error
    Reset(code int, msg string, data interface{}) IResponse
    Release()
    From(resp IResponse, needToRelease bool) IResponse
    SuccessWithData(data ...interface{}) IResponse
    ErrorCustom(code int, msg string) IResponse
}
```

### Key Design Patterns

#### 1. Provider Pattern

**Idea**: Register functionality as providers.

**Benefits**:

- Decoupling
- Flexibility: load/replace on demand
- Extension: non-intrusive

**Flow**:

```go
// 1. Implement provider
// RespInfoProtobufProvider - Protobuf response information provider
type RespInfoProtobufProvider struct {
	IProvider // embed base provider implementation
}

func NewRespInfoProtobufProvider() *RespInfoProtobufProvider {
  son := &RespInfoProtobufProvider{
        IProvider: NewProvider().SetName("application/x-protobuf").SetType(ProviderTypeDefault().GroupResponseInfoChoose),
  }
  son.MountToParent(son)
  return son
}

// Initialize
func (p *RespInfoProtobufProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
    return response.GetRespInfoPB(), nil
}

// 2. Collect providers
providers := fiberhouse.DefaultProviders().AndMore(
    NewRespInfoProtobufProvider(),
)

// 3. Create provider manager
// RespInfoPManager response info provider manager
type RespInfoPManager struct {
    IProviderManager  // compose base provider manager implementation
}

func NewRespInfoPManager(ctx IContext) *RespInfoPManager {
    son := &RespInfoPManager{
        IProviderManager: NewProviderManager(ctx).
        SetName("RespInfoPManager").
        SetType(ProviderTypeDefault().GroupResponseInfoChoose),
    }
    // Mount the child instance to the parent field, and set and bind the child instance (current instance) to the execution location
    son.MountToParent(son).SetOrBindToLocation(ProviderLocationDefault().LocationResponseInfoInit, true)
    return son
}

// LoadProvider
func (m *RespInfoPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
    if len(loadFunc) == 0 {
        return nil, fmt.Errorf("manager '%s': no load function provided", m.Name())
    }
    anything, err := loadFunc[0](m)
    if err != nil {
        return nil, err
    }
    contentType, ok := anything.(string)
    if !ok {
        return nil, errors.New("loadFunc manager '" + m.Name() + "': expected string of http Content-Type")
    }
    return m.GetProvider(contentType)
}

// 4. Auto-load: RunServer() auto-registers providers of same type group
fiberhouse.New().WithProviders(providers).WithPManagers(managers).RunServer()
```

#### 2. Service Locator Pattern

**Idea**: Access dependencies via locator interfaces.

**Benefits**:

- No explicit DI wiring
- Lazy dependency retrieval
- Simpler code

**Example**:

```go
type MyService struct {
    fiberhouse.ServiceLocator
    repoInstanceRegisterKey string
}

func (s *MyService) Method() {
    // 通过定位器获取依赖
    dep := s.GetInstance(s.repoInstanceRegisterKey)
}
```

#### 3. Object Pool Pattern

**Use**: Response objects, cache options.

**Benefits**:

- Reduce GC pressure
- Improve performance
- Memory reuse

**Example**:

```go
// Get from pool
resp := response.GetRespInfo()
defer resp.Release()

// Cache option pool
co := cache.OptionPoolGet(ctx)
defer cache.OptionPoolPut(co)
```

### Extension Notes

#### Add a new core framework

1. Implement `CoreStarter`
2. Create a provider
3. Add to provider set
4. Register with the framework

#### Add a new response protocol

1. Implement `IResponse`
2. Add object pool support
3. Add to manager set
3. Register with the framework

FiberHouse, with clear interfaces and flexible providers, achieves:

- ✅ High extensibility
- ✅ Low coupling
- ✅ Easy testing
- ✅ Team collaboration
- ✅ Smooth evolution

## 📖 Business Application Guide

- Example template structure
- DI tools usage
- Resolve dependencies without DI tools via global manager
- CRUD API sample
- How to add new modules and APIs
- Task async example
- Cache usage example
- CMD CLI usage example

### Example template structure

- Architecture overview

```
example_application/                    # Sample app root
├── Application Config Layer
│   ├── application_impl.go            # Application register implementation
│   ├── constant.go                    # App-level constants
│   └── customizer_interface.go        # App customizer interface
│
├── API Layer
│   └── apivo/                         # API value objects
│       ├── commonvo/                  # Common VOs
│       │   └── vo.go                  # Common VO
│       └── example/                   # Example module VO
│           ├── api_interface.go       # API interface
│           ├── requestvo/             # Request VOs
│           │   └── example_reqvo.go
│           └── responsevo/            # Response VOs
│               └── example_respvo.go
│
├── Command-Line Layer
│   └── command/                       # CLI program
│       ├── main.go                    # CLI entry
│       ├── README_go_build.md         # Build notes
│       ├── application/               # CLI app config
│       │   ├── application.go         # CLI logic
│       │   ├── constants.go           # CLI constants
│       │   ├── functions.go           # Utils
│       │   └── commands/              # Command scripts
│       │       ├── test_orm_command.go
│       │       └── test_other_command.go
│       ├── component/                 # CLI components
│       │   └── cron.go                # Cron jobs
│       └── target/                    # Build outputs
│
├── Exception Layer
│   ├── get_exceptions.go              # Exception getter
│   └── example-module/                # Module exceptions
│       └── exceptions.go
│
├── Provider Layer
│   └── providers/                     # Providers
│       ├── middleware/                # Middleware providers
│       │   ├── fiber_app_middleware_provider.go
│       │   ├── fiber_module_middleware_provider.go
│       │   └── gin_app_middleware_provider.go
│       ├── module/                    # Module providers
│       │   ├── fiber_route_register_provider.go
│       │   └── gin_route_register_provider.go
│       └── optioninit/                # Option init providers
│           ├── frame_option_init_provider.go
│           └── core_option_init_provider.go
│
├── Business Modules
│   └── module/                        # Business modules
│       ├── module.go                  # Module register
│       ├── route_register.go          # Route register
│       ├── swagger.go                 # Swagger config
│       ├── task.go                    # Task register
│       │
│       ├── command-module/            # Command business module
│       │   ├── entity/                # Entities
│       │   ├── model/                 # Models
│       │   └── service/               # Services
│       │
│       ├── common-module/             # Common module
│       │   ├── attrs/                 # Attributes
│       │   ├── fields/                # Common fields
│       │   ├── model/                 # Models
│       │   ├── repository/            # Repos
│       │   ├── service/               # Services
│       │   └── vars/                  # Vars
│       │
│       ├── constant/                  # Constants
│       │   └── constants.go
│       │
│       └── example-module/            # Core example module
│           ├── api/                # API controllers
│           │   ├── api_provider_wire_gen.go  # Wire-generated
│           │   ├── api_provider.go    # API provider
│           │   ├── common_api.go      # Common API
│           │   ├── example_api.go     # Example API
│           │   ├── health_api.go      # Health check
│           │   └── register_api_router.go    # Route registration
│           │
│           ├── dto/                # DTOs
│           │
│           ├── entity/             # Entities
│           │   └── types.go
│           │
│           ├── model/              # Models
│           │   ├── example_model.go
│           │   ├── example_mysql_model.go
│           │   └── model_wireset.go
│           │
│           ├── repository/         # Repositories
│           │   ├── example_repository.go
│           │   ├── health_repository.go
│           │   └── repository_wireset.go
│           │
│           ├── service/            # Services
│           │   ├── example_service.go
│           │   ├── health_service.go
│           │   ├── service_wireset.go
│           │   └── test_service.go
│           │
│           └── task/               # Tasks
│               ├── names.go           # Task names
│               ├── task.go            # Task register
│               └── handler/           # Handlers
│                   ├── handle.go
│                   └── mount.go
│
├── Utilities
│   └── utils/                         # App utilities
│       └── common.go
│
└── Custom Validators
    [...]
```

### Directory Notes

#### Core Layers
- **Application Config**: App-level config/constants.
- **API Layer**: Unified API value objects.
- **CLI Layer**: Independent CLI sub-framework.
- **Exception Layer**: Modular exception definitions.
- **Provider Layer**: Provider implementations for extension points.
- **Business Modules**: Module-organized business logic.

#### Inside a module (example-module)
- **api/**: API controllers for HTTP.
- **dto/**: Data transfer objects.
- **entity/**: Entities mapped to DB tables.
- **model/**: Data models wrapping DB ops.
- **repository/**: Persistence layer.
- **service/**: Business logic.
- **task/**: Async tasks.

### DI Tools Usage

- DI tools and libs
  - google wire: Dependency injection code generation tool; official repo: [https://github.com/google/wire](https://github.com/google/wire)
  - uber dig: Dependency injection container; recommended to use only during application startup; official repo: [https://github.com/uber-go/dig](https://github.com/uber-go/dig)
- Google Wire usage and examples:
  - [example_application/module/example-module/api/api_provider.go](./example_application/module/example-module/api/api_provider.go)
  - [example_application/module/example-module/api/README_wire_gen.md](./example_application/module/example-module/api/README_wire_gen.md)
- Uber Dig usage and examples:
  - [component/dig_container.go](component/dig_container.go)

### Resolving deps via global manager (no DI tool)

- Route example: [example_application/module/example-module/api/register_api_router.go](./example_application/module/example-module/api/register_api_router.go)

```go
func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app fiber.Router) {
    // 获取exampleApi处理器
    exampleApi, _ := InjectExampleApi(ctx) // 由wire编译依赖注入生成注入函数获取ExampleApi
    
    // 获取CommonApi处理器，直接NewCommonHandler
    
    // 直接New，无需依赖注入(Wire注入)，内部依赖走全局管理器延迟获取依赖组件，
    // 见 common_api.go: api.CommonHandler
    commonApi := NewCommonHandler(ctx)
    
    // 获取注册更多api处理器并注册相应路由...
    
    // 注册Example模块的路由
    exampleGroup := app.Group("/example")
    // hello world
    exampleGroup.Get("/hello/world", exampleApi.HelloWorld).Name("ex_get_example_test")
}
```

- CommonHandler using global manager without compile-time DI: [example_application/module/example-module/api/common_api.go](./example_application/module/example-module/api/common_api.go)

```go
// CommonHandler example common handler, embeds fiberhouse.ApiLocator, provides access to context, config, logger, and instance registration capabilities
type CommonHandler struct {
    fiberhouse.ApiLocator
    KeyTestService string // Defines the key for the dependent component in the global manager. Use this key with h.GetInstance(key) to retrieve the instance, or use fiberhouse.GetMustInstance[T](key) generic function.
    // No need for Wire or other DI tools
}

// NewCommonHandler creates directly without DI (Wire) for TestService; dependencies are obtained via the global manager internally
func NewCommonHandler(ctx fiberhouse.IApplicationContext) *CommonHandler {
    return &CommonHandler{
        ApiLocator:     fiberhouse.NewApi(ctx).SetName(GetKeyCommonHandler()),
        
        // Registers the initializer for the dependent TestService instance and returns the registered instance key; use h.GetInstance(key) to obtain the TestService instance
        KeyTestService: service.RegisterKeyTestService(ctx),
    }
}

// TestGetInstance tests retrieving a registered instance via h.GetInstance(key); no compile-time Wire DI is required
func (h *CommonHandler) TestGetInstance(c *fiber.Ctx) error {
    t := c.Query("t", "test")
    
    // Retrieve the registered instance via h.GetInstance(h.KeyTestService)
    testService, err := h.GetInstance(h.KeyTestService)
    if err != nil {
        return err
    }
    
    if ts, ok := testService.(*service.TestService); ok {
        return response.RespSuccess(t + ":" + ts.HelloWorld()).JsonWithCtx(c)
    }

    return fmt.Errorf("type assertion failed")
}
```

### Sample CRUD API

- Entity: [example_application/module/example-module/entity/types.go](./example_application/module/example-module/entity/types.go)

```go
// Example
type Example struct {
 ID                bson.ObjectID             `json:"id" bson:"_id,omitempty"`
 Name              string                    `json:"name" bson:"name"`
 Age               int                       `json:"age" bson:"age,minsize"`
 Courses           []string                  `json:"courses" bson:"courses,omitempty"`
 Profile           map[string]interface{}    `json:"profile" bson:"profile,omitempty"`
 fields.Timestamps `json:"-" bson:",inline"`
}
```

- Route registration: [example_application/module/example-module/api/register_api_router.go](./example_application/module/example-module/api/register_api_router.go)

```go
func RegisterRouteHandlers(ctx fiberhouse.IApplicationContext, app fiber.Router) {
    // Get exampleApi handler
    exampleApi, _ := InjectExampleApi(ctx) // Wire DI

    // Register Example routes
    exampleGroup := app.Group("/example")

    // hello world route
    exampleGroup.Get("/hello/world", exampleApi.HelloWorld).Name("ex_get_example_test")
    
    // CURD route
    exampleGroup.Get("/get/:id", exampleApi.GetExample).Name("ex_get_example")
    exampleGroup.Get("/on-async-task/get/:id", exampleApi.GetExampleWithTaskDispatcher).Name("ex_get_example_on_task")
    exampleGroup.Post("/create", exampleApi.CreateExample).Name("ex_create_example")
    exampleGroup.Get("/list", exampleApi.GetExamples).Name("ex_get_examples")
}
```

- API handler: [example_application/module/example-module/api/example_api.go](./example_application/module/example-module/api/example_api.go)

```go
// ExampleHandler is an example handler that embeds fiberhouse.ApiLocator,
// providing access to context, configuration, logger, and instance registration.
type ExampleHandler struct {
    fiberhouse.ApiLocator
    Service        *service.ExampleService
    KeyTestService string
}

func NewExampleHandler(ctx fiberhouse.IApplicationContext, es *service.ExampleService) *ExampleHandler {
    return &ExampleHandler{
        ApiLocator:     fiberhouse.NewApi(ctx).SetName(GetKeyExampleHandler()),
        Service:        es,
        KeyTestService: service.RegisterKeyTestService(ctx),
    }
}

// GetKeyExampleHandler defines and returns the instance key used to register
// ExampleHandler into the global manager.
func GetKeyExampleHandler(ns ...string) string {
    return fiberhouse.RegisterKeyName("ExampleHandler", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// GetExample retrieves sample data.
func (h *ExampleHandler) GetExample(c *fiber.Ctx) error {
    // Get language
var lang = c.Get(constant.XLanguageFlag, "en")

id := c.Params("id")

    // Construct the struct that needs validation
var objId = &requestvo.ObjId{
    ID: id, 
}
    // Get the validation wrapper object
vw := h.GetContext().GetValidateWrap()

    // Get the validator for the specified language and validate the struct
if errVw := vw.GetValidate(lang).Struct(objId); errVw != nil {
    var errs validator.ValidationErrors
    if errors.As(errVw, &errs) {
        return vw.Errors(errs, lang, true)
    }
}

    // Fetch data from the service layer
resp, err := h.Service.GetExample(id)
    if err != nil {
    return err
}

    // Return successful response
fiberhouse.Response().SuccessWithData(resp).JsonWithCtx(providerctx.WithFiberContext(c))
}
```

- Service: [example_application/module/example-module/service/example_service.go](./example_application/module/example-module/service/example_service.go)

```go
// ExampleService sample service, embeds fiberhouse.ServiceLocator to provide access to context, config, logger and instance registration
type ExampleService struct {
    fiberhouse.ServiceLocator                               // embeds service locator interface
    Repo                 *repository.ExampleRepository // dependent component: example repository, injected via wire
}

func NewExampleService(ctx fiberhouse.IApplicationContext, repo *repository.ExampleRepository) *ExampleService {
    name := GetKeyExampleService()
    return &ExampleService{
        ServiceLocator: fiberhouse.NewService(ctx).SetName(name),
        Repo:           repo,
    }
}

// GetKeyExampleService returns the registration key name for ExampleService
func GetKeyExampleService(ns ...string) string {
    return fiberhouse.RegisterKeyName("ExampleService", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// GetExample retrieves sample data by ID
func (s *ExampleService) GetExample(id string) (*responsevo.ExampleRespVo, error) {
    resp := responsevo.ExampleRespVo{}
    // call repository layer to get data
    example, err := s.Repo.GetExampleById(id)
    if err != nil {
        return nil, err
    }
    // map data
    resp.ExamName = example.Name
    resp.ExamAge = example.Age
    resp.Courses = example.Courses
    resp.Profile = example.Profile
    resp.CreatedAt = example.CreatedAt
    resp.UpdatedAt = example.UpdatedAt
    // return result
    return &resp, nil
}
```

- Repository: [example_application/module/example-module/repository/example_repository.go](./example_application/module/example-module/repository/example_repository.go)

```go
// ExampleRepository is the Example repository responsible for persisting Example business data.
// It embeds fiberhouse.RepositoryLocator repository locator interface and provides access to context,
// configuration, logger, instance registration and other capabilities.
type ExampleRepository struct {
    fiberhouse.RepositoryLocator
    Model *model.ExampleModel
}

func NewExampleRepository(ctx fiberhouse.IApplicationContext, m *model.ExampleModel) *ExampleRepository {
    return &ExampleRepository{
        RepositoryLocator: fiberhouse.NewRepository(ctx).SetName(GetKeyExampleRepository()),
        Model:             m,
    }
}

// GetKeyExampleRepository returns the registration key name for ExampleRepository
func GetKeyExampleRepository(ns ...string) string {
    return fiberhouse.RegisterKeyName("ExampleRepository", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleRepository registers the ExampleRepository to the container (lazy initialization)
// and returns the registration key
func RegisterKeyExampleRepository(ctx fiberhouse.IApplicationContext, ns ...string) string {
    return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleRepository(ns...), func() (interface{}, error) {
        m := model.NewExampleModel(ctx)
        return NewExampleRepository(ctx, m), nil
    })
}

// GetExampleById retrieves Example data by ID
func (r *ExampleRepository) GetExampleById(id string) (*entity.Example, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    result, err := r.Model.GetExampleByID(ctx, id)
    if err != nil {
        if errors.Is(err, mongo.ErrNoDocuments) {
            return nil, exception.GetNotFoundDocument() // return error
        }
        exception.GetInternalError().RespData(err.Error()).Panic() // directly panic
    }
    return result, nil
}
```

- Model: [example_application/module/example-module/model/example_model.go](./example_application/module/example-module/model/example_model.go)

```go
// ExampleModel is the example model. It embeds dbmongo.MongoLocator locator interface
// and provides access to context, config, logger, registered instances and basic MongoDB operations.
type ExampleModel struct {
    dbmongo.MongoLocator
    ctx context.Context // optional field
}

func NewExampleModel(ctx fiberhouse.IApplicationContext) *ExampleModel {
    return &ExampleModel{
        MongoLocator: dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).SetDbName(constant.DbNameMongo).SetTable(constant.CollExample).
          SetName(GetKeyExampleModel()).
		  (dbmongo.MongoLocator), // set the model's config name (mongodb) and database name (test)
          ctx: context.Background(),
    }
}

// GetKeyExampleModel gets the model registration key
func GetKeyExampleModel(ns ...string) string {
    return fiberhouse.RegisterKeyName("ExampleModel", fiberhouse.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleModel registers the model to the container (lazy init) and returns the registration key
func RegisterKeyExampleModel(ctx fiberhouse.IApplicationContext, ns ...string) string {
    return fiberhouse.RegisterKeyInitializerFunc(GetKeyExampleModel(ns...), func() (interface{}, error) {
        return NewExampleModel(ctx), nil
    })
}

// GetExampleByID gets the example document by ID
func (m *ExampleModel) GetExampleByID(ctx context.Context, oid string) (*entity.Example, error) {
    _id, err := bson.ObjectIDFromHex(oid)
    if err != nil {
        exception.GetInputError().RespData(err.Error()).Panic() // panic on invalid input
    }
    filter := bson.D{{"_id", _id}}
    opts := options.FindOne().SetProjection(bson.M{
        "_id":     0,
        "profile": 0,
    })
    var example entity.Example
    err = m.GetCollection(m.GetColl()).FindOne(ctx, filter, opts).Decode(&example)
    if err != nil {
        return nil, err // return error
    }
    return &example, nil
}
```

- Call chain example: GET /example/get/:id
  - Route: RegisterRouteHandlers -> exampleGroup.Get("/get/:id", exampleApi.GetExample)
  - API: ExampleHandler.GetExample -> h.Service.GetExample
  - Service: ExampleService.GetExample -> s.Repo.GetExampleById
  - Repo: ExampleRepository.GetExampleById -> r.Model.GetExampleByID
  - Model: ExampleModel.GetExampleByID -> m.GetCollection(...).FindOne(...)
  - Entity: entity.Example
  - Response: response.RespSuccess(resp).JsonWithCtx(c) -> response.RespInfo

### How to add a new module and API
- Refer to [example_application/module/example-module](./example_application/module/example-module)

- Copy template:

```bash
cp -r example_application/module/example-module example_application/module/mymodule
```

- Update files:
  - **Constants**: edit `constant/constants.go`
  - **Entities**: edit `entity/types.go`
  - **Model**: update `model/` files (names, tables)
  - **Repository**: update `repository/` files
  - **Service**: update `service/` files
  - **API**: update `api/` controllers

- Register new module routes in `module/route_register.go`:

```go
// In RegisterApiRouters
mymodule.RegisterRouteHandlers(ctx, app)
```

- Regenerate Wire:

```bash
cd example_application/module/mymodule/api
wire gen -output_file_prefix api_provider_
```

### Task async example

- Task name: [example_application/module/example-module/task/names.go](./example_application/module/example-module/task/names.go)

```go
// Task types
const (
 // TypeExampleCreate: async create example data
 TypeExampleCreate = "ex:example:create:create-an-example"
)
```

- Create task: [example_application/module/example-module/task/task.go](./example_application/module/example-module/task/task.go)

```go
/*
Task payload list
*/

// PayloadExampleCreate data payload for creating an example
type PayloadExampleCreate struct {
    fiberhouse.PayloadBase // Embeds base payload struct, automatically provides JSON codec methods
    /**
    Payload data
    */
    Age int8
}

// NewExampleCreateTask generates an ExampleCreate task, takes parameters from the caller and returns the task
func NewExampleCreateTask(ctx fiberhouse.IContext, age int8) (*asynq.Task, error) {
    vo := PayloadExampleCreate{
    Age: age,
}
    // Get JSON codec and marshal the payload into JSON bytes
payload, err := vo.GetMustJsonHandler(ctx).Marshal(&vo)
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(TypeExampleCreate, payload, asynq.Retention(24*time.Hour), asynq.MaxRetry(3), asynq.ProcessIn(1*time.Minute)), nil
}
```


- Defined task handler: [example_application/module/example-module/task/handler/handle.go](./example_application/module/example-module/task/handler/handle.go)

```go
// HandleExampleCreateTask is the handler for example create tasks
func HandleExampleCreateTask(ctx context.Context, t *asynq.Task) error {
	// Retrieve appCtx from context to access config, logger, registered instances, etc.
	appCtx, _ := ctx.Value(fiberhouse.ContextKeyAppCtx).(fiberhouse.IApplicationContext)

	// Declare task payload object
	var p task.PayloadExampleCreate

	// Parse task payload
	if err := p.GetMustJsonHandler(appCtx).Unmarshal(t.Payload(), &p); err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginWeb()).Str("From", "HandleExampleCreateTask").Err(err).Msg("[Asynq]: Unmarshal error")
		return err
	}

	// Get the instance that handles the task. Note service.TestService must be registered to the global manager during the task mounting phase.
	// See `task/handler/mount.go`: service.RegisterKeyTestService(ctx)
	instance, err := fiberhouse.GetInstance[*service.TestService](service.GetKeyTestService())
	if err != nil {
		return err
	}

	// Pass payload parameters into the instance's handler method
	result, err := instance.DoAgeDoubleCreateForTaskHandle(p.Age)
	if err != nil {
		return err
	}

	// Log the result
	appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginTask()).Msgf("HandleExampleCreateTask succeeded, result Age double: %d", result)
	return nil
}

```

- Mount handlers: [example_application/module/example-module/task/handler/mount.go](./example_application/module/example-module/task/handler/mount.go)

```go
package handler

import (
  "github.com/lamxy/fiberhouse/example_application/module/example-module/service"
  "github.com/lamxy/fiberhouse/example_application/module/example-module/task"
  "github.com/lamxy/fiberhouse"
)

// RegisterTaskHandlers registers task handler functions and dependency component initializer functions centrally
func RegisterTaskHandlers(tk fiberhouse.TaskRegister) {
    // append task handler to global taskHandlerMap
    // Register initializer functions for task handler instances via RegisterKeyXXX and obtain the registered instance key name
  
    // Register global manager instance initializers centrally. These instances can be obtained in task handlers
    // via tk.GetContext().GetContainer().GetXXXService() to perform specific task processing logic
    service.RegisterKeyTestService(tk.GetContext())
  
    // Append task handler functions to the TaskRegister's task name-to-handler mapping
    tk.AddTaskHandlerToMap(task.TypeExampleCreate, HandleExampleCreateTask)
}
```

- Enqueue tasks: [example_application/module/example-module/api/example_api.go](./example_application/module/example-module/api/example_api.go) calls [example_application/module/example-module/service/example_service.go](./example_application/module/example-module/service/example_service.go) `GetExampleWithTaskDispatcher`

```go
// GetExampleWithTaskDispatcher Example method demonstrating how to use the task dispatcher to execute tasks asynchronously within a service method
func (s *ExampleService) GetExampleWithTaskDispatcher(id string) (*responsevo.ExampleRespVo, error) {
    resp := responsevo.ExampleRespVo{}
    example, err := s.Repo.GetExampleById(id)
    if err != nil {
        return nil, err
    }
    
    // Get a logger tagged for tasks; obtain a logger with the task log origin from the global manager
    log := s.GetContext().GetMustLoggerWithOrigin(s.GetContext().GetConfig().LogOriginTask())
    
    // After successfully obtaining example data, push a delayed async task
    dispatcher, err := s.GetContext().(fiberhouse.IApplicationContext).GetStarterApp().GetTask().GetTaskDispatcher()
    if err != nil {
        log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher GetTaskDispatcher failed")
    }
    // Create task object
    task1, err := task.NewExampleCreateTask(s.GetContext(), int8(example.Age))
    if err != nil {
        log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher NewExampleCountTask failed")
    }
    // Enqueue the task object
    tInfo, err := dispatcher.Enqueue(task1, asynq.MaxRetry(constant.TaskMaxRetryDefault), asynq.ProcessIn(1*time.Minute)) // task enqueued; it will be executed in 1 minute
    
    if err != nil {
        log.Warn().Err(err).Msg("GetExampleWithTaskDispatcher Enqueue failed")
    } else if tInfo != nil {
        log.Warn().Msgf("GetExampleWithTaskDispatcher Enqueue task info: %v", tInfo)
    }
    
    // Normal business logic
    resp.ExamName = example.Name
    resp.ExamAge = example.Age
    resp.Courses = example.Courses
    resp.Profile = example.Profile
    resp.CreatedAt = example.CreatedAt
    resp.UpdatedAt = example.UpdatedAt
    return &resp, nil
}
```

### Cache usage example

- See GetExamples: [example_application/module/example-module/api/example_api.go](./example_application/module/example-module/api/example_api.go) calling `GetExamplesWithCache` in [example_application/module/example-module/service/example_service.go](./example_application/module/example-module/service/example_service.go)

```go

func (s *ExampleService) GetExamples(page, size int) ([]responsevo.ExampleRespVo, error) {
    // Get cache option object from the cache option pool
    co := cache.OptionPoolGet(s.GetContext())
    // Return the cache option object to the pool when done
    defer cache.OptionPoolPut(co)
    
    // Configure cache options: enable level-2 cache, enable local cache, set cache key,
    // set local TTL with randomization (10s ±10%), set remote TTL with randomization (3min ±1min),
    // use write-remote-only sync strategy, set context, enable all protection measures
    co.Level2().EnableCache().SetCacheKey("key:example:list:page:"+strconv.Itoa(page)+":size:"+strconv.Itoa(size)).SetLocalTTLRandomPercent(10*time.Second, 0.1).
    SetRemoteTTLWithRandom(3*time.Minute, 1*time.Minute).SetSyncStrategyWriteRemoteOnly().SetContextCtx(context.TODO()).EnableProtectionAll()
    
    // Retrieve cached data by calling cache.GetCached with the cache option object and a data-fetch callback
    return cache.GetCached[[]responsevo.ExampleRespVo](co, func(ctx context.Context) ([]responsevo.ExampleRespVo, error) {
        list, err := s.Repo.GetExamples(page, size)
        
        if err != nil {
            return nil, err
        }
        examples := make([]responsevo.ExampleRespVo, 0, len(list))
        for i := range list {
            example := responsevo.ExampleRespVo{
                ID:       list[i].ID.Hex(),
                ExamName: list[i].Name,
                ExamAge:  list[i].Age,
                Courses:  list[i].Courses,
                Profile:  list[i].Profile,
                Timestamps: commonvo.Timestamps{
                    CreatedAt: list[i].CreatedAt,
                    UpdatedAt: list[i].UpdatedAt,
                },
            }
            examples = append(examples, example)
        }
        return examples, nil
    })
}
```

### CMD CLI usage example

- CLI main: [example_application/command/main.go](./example_application/command/main.go)

```go
package main

import (
  "github.com/lamxy/fiberhouse/example_application/command/application"
  "github.com/lamxy/fiberhouse"
  "github.com/lamxy/fiberhouse/bootstrap"
  "github.com/lamxy/fiberhouse/commandstarter"
)

func main() {
    // bootstrap initialize start config (global config, global logger), config path is "./../../example_config" relative to current working directory
    cfg := bootstrap.NewConfigOnce("./../../example_config")
  
    // global logger, define log directory as "./logs" under current working directory
    logger := bootstrap.NewLoggerOnce(cfg, "./logs")
  
    // initialize global command context
    ctx := fiberhouse.NewCmdContextOnce(cfg, logger)
  
    // initialize application registrar object, inject application starter
    appRegister := application.NewApplication(ctx) // must implement framework's fiberhouse.ApplicationCmdRegister interface
  
    // instantiate command-line application starter
    cmdlineStarter := &commandstarter.CMDLineApplication{
      // instantiate framework command starter object
      FrameCmdStarter: commandstarter.NewFrameCmdApplication(ctx, option.WithCmdRegister(appRegister)),
      // instantiate core command starter object
      CoreCmdStarter: commandstarter.NewCoreCmdCli(ctx),
    }
    // run command-line starter
    commandstarter.RunCommandStarter(cmdlineStarter)
}
```

- Write a command: [example_application/command/application/commands/test_orm_command.go](./example_application/command/application/commands/test_orm_command.go)

```go
// TestOrmCMD command to test go-orm CRUD operations. Implements the fiberhouse.CommandGetter interface and returns the CLI command object via GetCommand.
type TestOrmCMD struct {
	Ctx fiberhouse.IApplicationContext
}

func NewTestOrmCMD(ctx fiberhouse.IApplicationContext) fiberhouse.CommandGetter {
	return &TestOrmCMD{
		Ctx: ctx,
	}
}

// GetCommand returns the CLI command object, implementing the fiberhouse.CommandGetter interface.
func (m *TestOrmCMD) GetCommand() interface{} {
	return &cli.Command{
		Name:    "test-orm",
		Aliases: []string{"orm"},
		Usage:   "Test go-orm CRUD operations",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "method",
				Aliases:  []string{"m"},
				Usage:    "Test type (ok/orm)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "operation",
				Aliases:  []string{"o"},
				Usage:    "CRUD (c create | u update | r read | d delete)",
				Required: false,
			},
			&cli.UintFlag{
				Name:     "id",
				Aliases:  []string{"i"},
				Usage:    "Primary key ID",
				Required: true,
			},
		},
		Action: func(cCtx *cli.Context) error {
			var (
				ems  *service.ExampleMysqlService
                wrap = component.NewWrap[*service.ExampleMysqlService]()
			)

			// Use dig to inject required dependencies via chained Provide calls
			dc := m.Ctx.GetDigContainer().
				Provide(func() fiberhouse.IApplicationContext { return m.Ctx }).
				Provide(model.NewExampleMysqlModel).
				Provide(service.NewExampleMysqlService)

			// Error handling
			if dc.GetErrorCount() > 0 {
				return fmt.Errorf("dig container init error: %v", dc.GetProvideErrs())
			}

			/*
			// Use Invoke to obtain dependencies and operate on them in the callback
			err := dc.Invoke(func(ems *service.ExampleMysqlService) error {
				err := ems.AutoMigrate()
				if err != nil {
					return err
				}
				// other operations...
				return nil
			})
			*/

			// Another way: use generic Invoke to get dependencies via component.Wrap helper
			err := component.Invoke[*service.ExampleMysqlService](wrap)
			if err != nil {
				return err
			}

			// Retrieve dependency
			ems = wrap.Get()

			// Auto-migrate (create table once)
			err = ems.AutoMigrate()
			if err != nil {
				return err
			}

			// Get CLI parameters
			method := cCtx.String("method")

			// Execute test
			if method == "ok" {
				testOk := ems.TestOk()

				fmt.Println("result: ", testOk, "--from:", method)
			} else if method == "orm" {
				// Get more CLI parameters
				op := cCtx.String("operation")
				id := cCtx.Uint("id")

				// Run ORM test
				err := ems.TestOrm(m.Ctx, op, id)
				if err != nil {
					return err
				}

				fmt.Println("result: testOrm OK", "--from:", method)
			} else {
				return fmt.Errorf("unknown method: %s", method)
			}

			return nil
		},
	}
}
```

- CLI build: [example_application/command/README_go_build.md](./example_application/command/README_go_build.md)

```bash
# Build
cd command/
go build -o ./target/cmdstarter.exe ./main.go

# Help
cd command/
./target/cmdstarter.exe -h
```

- CLI usage
  - Build: `go build -o ./target/cmdstarter.exe ./main.go`
  - Help: `./target/cmdstarter.exe -h`
  - Test go-orm CRUD: `./target/cmdstarter.exe test-orm --method ok` or `./target/cmdstarter.exe test-orm -m ok`
  - Test go-orm CRUD (create): `./target/cmdstarter.exe test-orm --method orm --operation c --id 1` or `./target/cmdstarter.exe test-orm -m orm -o c -i 1`
  - Subcommand help: `./target/cmdstarter.exe test-orm -h`

## 🔧 Configuration

### Global application config
FiberHouse supports environment-based multiple config files under `example_config/`. The global config is in the context and accessible via `ctx.GetConfig()`.

- Config README: [example_config/README.md](./example_config/README.md)

- Naming rules

```
Format: application_[env].yml
Environments: dev | test | prod

Examples:
- application_dev.yml
- application_test.yml
- application_prod.yml
```

- Environment variables

```
# Bootstrap env (APP_ENV_):
APP_ENV_application_env=prod       # dev/test/prod

# Overrides (APP_CONF_):
APP_CONF_application_appName=MyApp
APP_CONF_application_server_port=9090
APP_CONF_application_appLog_level=error
APP_CONF_application_appLog_asyncConf_type=chan
```

#### Core config items

- Application:

```yaml
application:
  appName: "FiberHouse"           # Application name
  env: "dev"                      # Runtime environment: dev/test/prod

  server:
  host: "127.0.0.1"              # Server host
  port: 8080                     # Server port
```

- Logging:

```yaml
application:
  appLog:
    level: "info"                # Log level: debug/info/warn/error
    enableConsole: true          # Enable console output
    consoleJSON: false           # Console JSON format
    enableFile: true             # Enable file output
    filename: "app.log"          # Log filename

    # Asynchronous logging configuration
    asyncConf:
      enable: true              # Enable asynchronous logging
      type: "diode"             # Async type: chan/diode

    # Log rotation configuration  
    rotateConf:
      maxSize: 5                             # megabytes
      maxBackups: 5                          # maximum number of backup files
      maxAge: 7                              # days
      compress: false                        # disabled by default
```

- Database:
```yaml
# MySQL configuration
mysql:
  dsn: "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
  gorm:
    maxIdleConns: 10                       # max idle connections
    maxOpenConns: 100                      # max open connections
    connMaxLifetime: 3600                  # max connection lifetime in seconds
    connMaxIdleTime: 300                   # max connection idle time in seconds
    logger:
      level: info                        # log level: silent, error, warn, info
      slowThreshold: 200 * time.Millisecond # slow SQL threshold, recommended 200 * time.Millisecond, adjust per workload
      colorful: false                    # enable colored output
      enable: true                       # enable logging
      skipDefaultFields: true            # skip default fields
  pingTry: false

# Redis:
redis:
  host: "127.0.0.1"
  port: 6379
  password: ""
  database: 0
  poolSize: 100                # connection pool size

  # Cluster configuration (optional)
  cluster:
    addrs: ["127.0.0.1:6379"]
    poolSize: 100
```
- Cache:

```yaml
cache:
  # Local cache
  local:                                     # Local cache configuration
    numCounters: 1000000                     # 1,000,000 counters
    maxCost: 134217728                       # Maximum cache: 128M
    bufferItems: 64                          # Buffer size per cache shard
    metrics: true                            # Enable cache metrics
    IgnoreInternalCost: false                # Whether to ignore internal overhead

  # Remote cache  
  redis:                                     # Remote cache (Redis) configuration
    host: 127.0.0.1                          # Redis server address
    port: 6379                               # Redis server port
    password: ""                             # Redis server password

  # Async pool configuration
  asyncPool:                                 # Async goroutine pool configuration used when second-level cache is enabled; handles cache updates and sync strategies
    ants:                                    # Ants goroutine pool configuration
      local:
        size: 248                            # Local cache async goroutine pool size
        expiryDuration: 5                    # Idle goroutine timeout in seconds
        preAlloc: false                      # Do not preallocate
        maxBlockingTasks: 512                # Maximum number of blocking tasks
        nonblocking: false                   # Allow blocking (nonblocking=false)
```
- More as needed

- Full examples:
  - Test env: [example_config/application_test.yml](./example_config/application_test.yml)
  - CLI test env: [application_test.yml](./example_config/application_test.yml)

## 🤝 Contributing

### Quick Start
- Fork & Clone
- Branch: git checkout -b feature/your-feature
- Format & lint: go fmt ./... && golangci-lint run
- Test: go test ./... -race -cover
- Commit: feat(module): description
- Push & PR

### Branching
- main: stable releases
- develop: integration
- feature/*: features
- fix/*: fixes
- Others as needed

### PR Requirements
- Title: same as commit
- Content: background / solution / impact / tests / related Issue
- CI must pass

### Security
Report vulnerabilities privately: pytho5170@hotmail.com

## 📄 License

This project is open-sourced under the MIT License - see [LICENSE](LICENSE) for details.

## 🙋‍♂️ Support & Feedback

- If you like it or want to support ongoing development, please star the project on GitHub: [GitHub Star](https://github.com/lamxy/fiberhouse/stargazers)
- Issues: [Issues](https://github.com/lamxy/fiberhouse/issues)
- Email: pytho5170@hotmail.com

## 🌟 Acknowledgements

Thanks to these projects:

- [gofiber/fiber](https://github.com/gofiber/fiber) - High-performance HTTP core
- [rs/zerolog](https://github.com/rs/zerolog) - High-performance structured logging
- [knadh/koanf](https://github.com/knadh/koanf) - Flexible multi-source config
- [bytedance/sonic](https://github.com/bytedance/sonic) - High-performance JSON codec
- [dgraph-io/ristretto](https://github.com/dgraph-io/ristretto) - High-performance local cache
- [hibiken/asynq](https://github.com/hibiken/asynq) - Redis-based distributed task queue
- [redis/go-redis](https://github.com/redis/go-redis) - Redis client
- [go.mongodb.org/mongo-driver](https://github.com/mongodb/mongo-go-driver) - Official MongoDB driver
- [gorm.io/gorm](https://gorm.io) - ORM abstraction and MySQL support
- [panjf2000/ants](https://github.com/panjf2000/ants) - High-performance goroutine pool

Also thanks to:
- [swaggo/swag](https://github.com/swaggo/swag) for providing API documentation generation
- [google/wire](https://github.com/google/wire), [uber-go/dig](https://github.com/uber-go/dig) for supporting dependency injection patterns
- and all other excellent projects not listed individually

Finally thanks to: GitHub Copilot for information lookup, documentation organization, and coding assistance.