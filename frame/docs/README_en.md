# FiberHouse Framework

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![Fiber Version](https://img.shields.io/badge/fiber-v2.x-green.svg)](https://github.com/gofiber/fiber)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](../../LICENSE)

📖 English | [中文](../../README.md)

## 🏠 About FiberHouse

FiberHouse is a high-performance, composable Go web framework built on Fiber, featuring a global manager, configurator, unified logger, validation wrapper, and framework-level components including database, cache, middleware, and unified exception handling out of the box.

- Provides a powerful global management container that supports one-time registration and reuse of custom components everywhere, enabling easy replacement and feature extension.
- Defines standardized interfaces for application starters, global context, and layered architecture with built-in default implementations that support custom implementation and modular development.
- Enables building flexible and complete Go web applications by assembling FiberHouse like furnishing a "house" with "furniture" according to your needs.

### 🏆 Development Direction

Delivering a high-performance, extensible, customizable, and ready-to-use Go web framework.

## ✨ Features

- **High Performance**: Built on Fiber framework, providing blazing-fast HTTP performance with support for object pools, goroutine pools, caching, async processing, and other performance optimizations
- **Modular Design**: Clear layered architecture with defined standard interface contracts and implementations, supporting team collaboration, extension, and modular development
- **Global Manager**: Global object management container with lock-free design, immediate registration, lazy initialization, singleton characteristics, providing dependency resolution that can replace third-party dependency injection tools and unified lifecycle management
- **Global Configuration Management**: Unified configuration file loading, parsing, and management with support for multi-format configurations, environment variable overrides, adapting to different application scenarios
- **Unified Logging Management**: High-performance logging system supporting structured logging, synchronous/asynchronous writers, and various log source identification management
- **Unified Exception Handling**: Unified exception definition and handling mechanism with support for modularized error code management, integrated parameter validators, error tracing, and developer-friendly debugging experience
- **Parameter Validation**: Integrated open-source validation wrapper supporting custom language validators, tag rules, and multi-language translators
- **Database Support**: Integrated MySQL and MongoDB driver components with support for database model base classes
- **Cache Components**: Built-in high-performance combination and management of local, remote, and two-level cache components with support for cache model base classes
- **Task Queue**: Integrated Redis-based high-performance C/S architecture async task queue supporting task scheduling, delayed execution, and failure retry functionality
- **API Documentation**: Integrated swag documentation tool supporting automatic API documentation generation
- **Command Line Applications**: Complete command line application framework support following unified modular design, supporting team collaboration, feature extension, and modular development
- **Example Templates**: Provides complete web application and CMD application example template structures covering common scenarios and best practices, allowing developers to adapt them with minimal modifications
- **More**: Continuously optimizing and updating...

## 🏗️ Architecture Overview

```
frame/                              # FiberHouse Framework Core
├── Interface Definition Layer
│   ├── application_interface.go    # Application starter interface definition
│   ├── command_interface.go        # Command line application interface definition  
│   ├── context_interface.go        # Global context interface definition
│   ├── json_wraper_interface.go    # JSON wrapper interface definition
│   ├── locator_interface.go        # Service locator interface definition
│   └── model_interface.go          # Data model interface definition
├── Application Startup Layer
│   ├── applicationstarter/         # Web application starter implementation
│   │   └── frame_application.go    # Fiber-based application starter
│   ├── commandstarter/             # Command line application starter implementation
│   │   └── cmd_application.go      # Command line application starter
│   └── bootstrap/                  # Application bootstrap
│       └── bootstrap.go            # Unified bootstrap entry
├── Configuration Management Layer
│   └── appconfig/                  # Application configuration management
│       └── config.go               # Multi-format config file loading and management
├── Global Management Layer
│   ├── globalmanager/              # Global object container management
│   │   ├── interface.go            # Global manager interface
│   │   ├── manager.go              # Global manager implementation
│   │   └── types.go                # Global manager type definitions
│   └── global_utility.go           # Global utility functions
├── Data Access Layer
│   └── database/                   # Database driver support
│       ├── dbmysql/                # MySQL database component
│       │   ├── interface.go        # MySQL interface definition
│       │   ├── mysql.go            # MySQL connection implementation
│       │   └── mysql_model.go      # MySQL model base class
│       └── dbmongo/                # MongoDB database component
│           ├── interface.go        # MongoDB interface definition
│           ├── mongo.go            # MongoDB connection implementation
│           └── mongo_model.go      # MongoDB model base class
├── Cache System Layer
│   └── cache/                      # High-performance cache components
│       ├── cache_interface.go      # Cache interface definition
│       ├── cache_option.go         # Cache configuration options
│       ├── cache_utility.go        # Cache utility functions
│       ├── cache_errors.go         # Cache error definitions
│       ├── helper.go               # Cache helper functions
│       ├── cache2/                 # Two-level cache implementation
│       │   └── level2_cache.go     # Local+remote two-level cache
│       ├── cachelocal/             # Local cache implementation
│       │   ├── local_cache.go      # Memory cache implementation
│       │   └── type.go             # Local cache types
│       └── cacheremote/            # Remote cache implementation
│           ├── cache_model.go      # Remote cache model base class
│           └── redis_cache.go      # Redis cache implementation
├── Component Library Layer
│   └── component/                  # Framework core components
│       ├── dig_container.go        # dig-based dependency injection container wrapper
│       ├── jsoncodec/              # JSON encoder/decoder
│       │   └── sonicjson.go        # High-performance JSON encoder/decoder based on Sonic
│       ├── jsonconvert/            # JSON conversion tools
│       │   └── convert.go          # Conversion core implementation
│       ├── mongodecimal/           # MongoDB decimal handling
│       │   └── mongo_decimal.go    # MongoDB Decimal128 support
│       ├── validate/               # Parameter validation component
│       │   ├── type_interface.go   # Validator interface definition
│       │   ├── validate_wrapper.go # Validator wrapper implementation
│       │   ├── en.go               # English validator implementation
│       │   ├── zh_cn.go            # Simplified Chinese validator implementation
│       │   ├── zh_tw.go            # Traditional Chinese validator implementation
│       │   └── example/            # Registration examples
│       ├── tasklog/                # Task logging component
│       │   └── logger_adapter.go   # Logger adapter
│       └── writer/                 # Log writers
│           ├── async_channel_writer.go     # Async channel writer
│           ├── async_diode_writer.go       # Async diode writer
│           ├── async_diode_writer_test.go  # Async writer tests
│           └── sync_lumberjack_writer.go   # Sync rolling log writer
├── Middleware Layer
│   └── middleware/                 # HTTP middleware
│       └── recover/                # Exception recovery middleware
│           ├── config.go           # Recovery middleware configuration
│           └── recover.go          # Recovery middleware implementation
├── Response Handling Layer
│   └── response/                   # Unified response handling
│       └── response.go             # Response object pool and serialization
├── Exception Handling Layer
│   └── exception/                  # Unified exception handling
│       ├── types.go                # Exception type definitions
│       └── exception_error.go      # Exception error implementation
├── Utility Layer
│   ├── utils/                      # Common utility functions
│   │   └── common.go               # Common utility implementation
│   └── constant/                   # Framework constants
│       ├── constant.go             # Global constant definitions
│       └── exception.go            # Exception constant definitions
├── Business Layer Contracts
│   ├── api.go                      # API layer interface definition
│   ├── service.go                  # Service layer interface definition
│   ├── repository.go               # Repository layer interface definition
│   └── task.go                     # Task layer interface definition
└── Placeholder Modules
├── mq/                         # Message queue (to be implemented)
├── plugins/                    # Plugin support (to be implemented)
└── component/
├── i18n/                   # Internationalization (to be implemented)
└── rpc/                    # RPC support (to be implemented)

```

## 🚀 Quick Start

### Requirements

- Go 1.24 or higher, recommended to upgrade to 1.25+
- MySQL 5.7+ or MongoDB 4.0+
- Redis 5.0+

### Starting Database and Cache Containers with Docker for Framework Debugging

- Docker compose file: [docker-compose.yml](./docker_compose_db_redis_yaml/docker-compose.yml)
- Start command: `docker compose up -d`

```bash
cd frame/docs/docker_compose_db_redis_yaml/
docker compose up -d
```

### Installation

FiberHouse requires **Go 1.24 or higher**. If you need to install or upgrade Go, please visit the [official Go download page](https://go.dev/dl/).

To start creating a project, create a new project directory and navigate to it. Then execute the following command in the terminal to initialize your project using Go Modules:

```bash
go mod init github.com/your/repo
```

After setting up the project, you can install the FiberHouse framework using the `go get` command:

```bash
go get github.com/lamxy/fiberhouse
```

### Main File Example

Reference example: [example_main/main.go](../../example_main/main.go)

```go
package main

import (
	"github.com/lamxy/fiberhouse/example_application"
	"github.com/lamxy/fiberhouse/example_application/module"
	"github.com/lamxy/fiberhouse/frame"
	"github.com/lamxy/fiberhouse/frame/applicationstarter"
	"github.com/lamxy/fiberhouse/frame/bootstrap"
)

func main() {
	// Bootstrap initialization of startup configuration (global config, global logger), 
	// config directory defaults to `example_config/` under current working directory "."
	// You can specify absolute path or relative path based on working directory
	cfg := bootstrap.NewConfigOnce("./example_config")
	
	// Log directory defaults to `example_main/logs` under current working directory "."
	// You can specify absolute path or relative path based on working directory
	logger := bootstrap.NewLoggerOnce(cfg, "./example_main/logs")

	// Initialize global application context
	appContext := frame.NewAppContextOnce(cfg, logger)

	// Initialize application registerer, module/subsystem registerer and task registerer objects, inject to application starter
	appRegister := example_application.NewApplication(appContext)  // Need to implement application registerer interface, see frame.ApplicationRegisterer interface definition, reference example_application/application.go example implementation
	moduleRegister := module.NewModule(appContext)  // Need to implement module registerer interface, see example module module/module.go implementation
	taskRegister := module.NewTaskAsync(appContext)  // Need to implement task registerer interface, see example task module/task.go implementation

	// Instantiate framework application starter
	starterApp := applicationstarter.NewFrameApplication(appContext,
            option.WithAppRegister(appRegister),
            option.WithModuleRegister(moduleRegister),
            option.WithTaskRegister(taskRegister),
        )
  
	// Run framework application starter
	applicationstarter.RunApplicationStarter(starterApp)
}
```

### Quick Try

- Quick Web Application Experience

```bash
# Clone the framework
git clone https://github.com/lamxy/fiberhouse.git

# Enter framework directory
cd fiberhouse

# Install dependencies
go mod tidy

# Enter example_main/
cd example_main/

# View README
cat README_go_build.md

# Build application: Windows environment example, for other environments please refer to cross-compilation
# Return to application root directory (default working directory), execute the following command in working directory to build application
# Current working directory is fiberhouse/, build output to example_main/target/ directory
cd ..
# On Windows, the build output keeps the .exe suffix; on Linux, no suffix is needed.
go build "-ldflags=-X 'main.Version=v0.0.1'" -o ./example_main/target/examplewebserver.exe ./example_main/main.go

# Run application
# Return to application root directory (default working directory), execute the following command in working directory to start application
./example_main/target/examplewebserver.exe
# or Linux, MacOS
./example_main/target/examplewebserver
```

Visit the hello world endpoint: http://127.0.0.1:8080/example/hello/world

You will receive the response: {"code":0,"msg":"ok","data":"Hello World!"}

```bash
curl -sL "http://127.0.0.1:8080/example/hello/world"

# Response:
{
    "code": 0,
    "msg": "ok",
    "data": "Hello World!"
}
```

- Quick CMD Application Experience

```bash

# MySQL database preparation
mysqlsh root:root@localhost:3306 

# Create a test database
CREATE DATABASE IF NOT EXISTS test CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

# Clone the framework
git clone https://github.com/lamxy/fiberhouse.git

# Enter the framework directory
cd fiberhouse

# Install dependencies
go mod tidy

# Enter example_application/command/
cd example_application/command/

# View README
cat README_go_build.md

# Current working directory: command/
# On Windows, the build output keeps the .exe suffix; on Linux or MacOS, no suffix is needed.
go build -o ./target/cmdstarter.exe ./main.go 

# Set environment variables for the cmd application on Windows, will read the application_cmd_dev.yml configuration file
set APP_ENV_application_appType=cmd
set APP_ENV_application_env=dev

# Linux or MacOS environment
# export APP_ENV_application_appType=cmd
# export APP_ENV_application_env=dev

# Execute cmd command script, view help
./target/cmdstarter.exe -h 
# or 
./target/cmdstarter -h

# Execute subcommand, view console log output
./target/cmdstarter.exe test-orm -m ok
# or 
./target/cmdstarter test-orm -m ok

# Console output ok
# result:  ExampleMysqlService.TestOK: OK --from: ok

```

## 📖 User Guide

- Example template project structure
- Dependency injection tool description and usage
- Implementing dependency resolution without dependency injection tools through the framework's global manager
- Example CRUD API implementation
- How to add new modules and new APIs
- Task async task usage examples
- Cache component usage examples
- CMD command line application usage examples

### Example Application Template Directory Structure

- Architecture Overview and Description

```
example_application/                    # Example application root directory
├── Application Configuration Layer
│   ├── application.go                  # Application registerer implementation
│   ├── constant.go                     # Application-level constant definitions
│   └── customizer_interface.go         # Application customizer interface
├── API Interface Layer
│   └── api-vo/                         # API value object definitions
│       ├── commonvo/                   # Common VO
│       │   └── vo.go                   # Common value objects
│       └── example/                    # Example module VO
│           ├── api_interface.go        # API interface definition
│           ├── requestvo/              # Request VO
│           │   └── example_reqvo.go    # Example request objects
│           └── responsevo/             # Response VO
│               └── example_respvo.go   # Example response objects
├── Command Line Framework Application Layer
│   └── command/                        # Command line programs
│       ├── main.go                     # Command line main entry
│       ├── README_go_build.md          # Build instructions
│       ├── application/                
│       │   ├── application.go          # Command application configuration and logic
│       │   ├── constants.go            # Command constants
│       │   ├── functions.go            # Command utility functions
│       │   └── commands/               # Specific command script implementations
│       │       ├── test_orm_command.go # ORM test command
│       │       └── test_other_command.go # Other more development command scripts...
│       ├── component/                  # Command line components
│       │   ├── cron.go                 # Scheduled task component
│       │   └── readme.md               # Component documentation
│       └── target/                     # Build artifacts
│           └── cmdstarter.exe          # Command line executable
├── Exception Handling Layer
│   ├── get_exceptions.go               # Exception getter
│   └── example-module/                 # Example module exceptions, other module exceptions, each module in separate directory
│       └── exceptions.go               # Module exception aggregation
├── Middleware Layer
│   └── middleware/                     # Application-level middleware
│       └── register_app_middleware.go  # Application middleware registerer
├── Module (Subsystem) Layer
│   └── module/                         # Business modules
│       ├── module.go                   # Module registerer
│       ├── route_register.go           # Route registerer
│       ├── swagger.go                  # Swagger documentation configuration
│       ├── task.go                     # Async task registerer
│       ├── api/                        # Module-level API middleware
│       │   └── register_module_middleware.go
│       ├── command-module/             # Command line script dedicated business module
│       │   ├── entity/                 # Entity definitions
│       │   │   └── mysql_types.go      # MySQL type definitions
│       │   ├── model/                  # Data models
│       │   │   ├── mongodb_model.go    # MongoDB model
│       │   │   └── mysql_model.go      # MySQL model
│       │   └── service/                # Business services
│       │       ├── example_mysql_service.go  # MySQL service example
│       │       └── mongodb_service.go        # MongoDB service example
│       ├── common-module/              # Common module
│       │   ├── attrs/                  # Attribute definitions
│       │   │   └── attr1.go            # Attribute example
│       │   ├── command/                # Common commands
│       │   ├── fields/                 # Common fields
│       │   │   └── timestamps.go       # Timestamp fields
│       │   ├── model/                  # Common models
│       │   ├── repository/             # Common repositories
│       │   ├── service/                # Common services
│       │   └── vars/                   # Common variables
│       │       └── vars.go             # Variable definitions
│       ├── constant/                   # Constant definitions
│       │   └── constants.go            # Module constants
│       └── example-module/             # Core example module for demonstration
│           ├── api/                    # API controller layer
│           │   ├── api_provider_wire_gen.go    # Wire dependency injection generated file
│           │   ├── api_provider.go             # API provider, provides dependencies
│           │   ├── common_api.go               # Common API controller
│           │   ├── example_api.go              # Example API controller
│           │   ├── health_api.go               # Health check API controller
│           │   ├── README_wire_gen.md          # Wire generation instructions
│           │   └── register_api_router.go      # API route registration
│           ├── dto/                    # Data transfer objects
│           ├── entity/                 # Entity layer
│           │   └── types.go            # Type definitions
│           ├── model/                  # Model layer
│           │   ├── example_model.go            # Example model
│           │   ├── example_mysql_model.go      # MySQL example model
│           │   └── model_wireset.go            # Model Wire set
│           ├── repository/             # Repository layer
│           │   ├── example_repository.go       # Example repository
│           │   ├── health_repository.go        # Health check repository
│           │   └── repository_wireset.go       # Repository Wire set
│           ├── service/                # Service layer
│           │   ├── example_service.go          # Example service
│           │   ├── health_service.go           # Health check service
│           │   ├── service_wireset.go          # Service Wire set
│           │   └── test_service.go             # Test service
│           └── task/                   # Task layer
│               ├── names.go            # Task name definitions
│               ├── task.go             # Task registerer
│               └── handler/            # Task handlers
│                   ├── handle.go       # Task handling logic
│                   └── mount.go        # Task mounter
├── Utility Layer
│   └── utils/                          # Application utilities
│       └── common.go                   # Common utility functions
└── Custom Validator Layer
    └── validatecustom/                 # Custom validators
        ├── tag_register.go             # Tag registerer
        ├── validate_initializer.go     # Validator initializer
        ├── tags/                       # Custom tags
        │   ├── new_tag_hascourses.go   # Course validation tag
        │   └── tag_startswith.go       # Prefix validation tag
        └── validators/                 # Multi-language validators
            ├── ja.go                   # Japanese validator
            ├── ko.go                   # Korean validator
            └── langs_const.go          # Language constants
```

### Dependency Injection Tool Description and Usage

- Dependency injection tools and libraries
    - google wire: Dependency injection code generation tool, official site [https://github.com/google/wire](https://github.com/google/wire)
    - uber dig: Dependency injection container, recommended for use only during application startup phase, official site [https://github.com/uber-go/dig](https://github.com/uber-go/dig)
- Google wire usage instructions and examples, refer to:
    - [example_application/module/example-module/api/api_provider.go](../../example_application/module/example-module/api/api_provider.go)
    - [example_application/module/example-module/api/README_wire_gen.md](../../example_application/module/example-module/api/README_wire_gen.md)
- Uber dig usage instructions and examples, refer to:
    - [frame/component/dig_container.go](../component/dig_container.go)

### Implementing Dependency Resolution without Dependency Injection Tools through Framework's Global Manager

- See route registration example: [example_application/module/example-module/api/register_api_router.go](../../example_application/module/example-module/api/register_api_router.go)

```go
func RegisterRouteHandlers(ctx frame.ContextFramer, app fiber.Router) {
    // Get exampleApi handler
    exampleApi, _ := InjectExampleApi(ctx) // Get ExampleApi through wire compiled dependency injection function
    
    // Get CommonApi handler, directly NewCommonHandler
	
	// Direct New, no need for dependency injection (Wire injection), internal dependencies use global manager for lazy dependency component retrieval,
	// see common_api.go: api.CommonHandler
	commonApi := NewCommonHandler(ctx) 
	
    // Get and register more api handlers and corresponding routes...
    
    // Register Example module routes
    exampleGroup := app.Group("/example")
	// hello world
    exampleGroup.Get("/hello/world", exampleApi.HelloWorld).Name("ex_get_example_test")
}
```

- See CommonHandler implementing service component access without prior dependency injection through global manager: [example_application/module/example-module/api/common_api.go](../../example_application/module/example-module/api/common_api.go)

```go
// CommonHandler Example common handler, inherits from frame.ApiLocator, providing capabilities to get context, config, logger, registered instances etc.
type CommonHandler struct {
	frame.ApiLocator
	KeyTestService string // Define the global manager instance key for dependency components. Through the key, instances can be obtained via h.GetInstance(key) method, or frame.GetMustInstance[T](key) generic method,
	                      // without requiring wire or other dependency injection tools
}

// NewCommonHandler Direct New, no need for dependency injection (Wire) TestService object, internally uses global manager to get dependency components
func NewCommonHandler(ctx frame.ContextFramer) *CommonHandler {
	return &CommonHandler{
		ApiLocator:     frame.NewApi(ctx).SetName(GetKeyCommonHandler()),
		
        // Register dependent TestService instance initializer and return registered instance key, get TestService instance through h.GetInstance(key) method
		KeyTestService: service.RegisterKeyTestService(ctx), 
	}
}

// TestGetInstance Test getting registered instance, get TestService registered instance through h.GetInstance(key) method, no need for compile-time wire dependency injection
func (h *CommonHandler) TestGetInstance(c *fiber.Ctx) error {
    t := c.Query("t", "test")
    
    // Get registered instance through h.GetInstance(h.KeyTestService) method
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

### Example CRUD API Implementation

- Define entity types: See [example_application/module/example-module/entity/types.go](../../example_application/module/example-module/entity/types.go)

```go
// Example
type Example struct {
	ID                bson.ObjectID             `json:"id" bson:"_id,omitempty"`
	Name              string                    `json:"name" bson:"name"`
	Age               int                       `json:"age" bson:"age,minsize"` // minsize use int32 for storage
	Courses           []string                  `json:"courses" bson:"courses,omitempty"`
	Profile           map[string]interface{}    `json:"profile" bson:"profile,omitempty"`
	fields.Timestamps `json:"-" bson:",inline"` // inline: bson document serialization automatically promotes embedded fields i.e. automatically expand inherited common fields
}
```

- Route registration: See [example_application/module/example-module/api/register_api_router.go](../../example_application/module/example-module/api/register_api_router.go)

```go
func RegisterRouteHandlers(ctx frame.ContextFramer, app fiber.Router) {
    // Get exampleApi handler
    exampleApi, _ := InjectExampleApi(ctx) // Get through wire compiled dependency injection
	
    // Register Example module routes
    // Example route group
    exampleGroup := app.Group("/example")
	
	// hello world route
    exampleGroup.Get("/hello/world", exampleApi.HelloWorld).Name("ex_get_example_test")
	
	// CRUD routes
    exampleGroup.Get("/get/:id", exampleApi.GetExample).Name("ex_get_example")
    exampleGroup.Get("/on-async-task/get/:id", exampleApi.GetExampleWithTaskDispatcher).Name("ex_get_example_on_task")
    exampleGroup.Post("/create", exampleApi.CreateExample).Name("ex_create_example")
    exampleGroup.Get("/list", exampleApi.GetExamples).Name("ex_get_examples")
}
```

- Define example API handler: See [example_application/module/example-module/api/example_api.go](../../example_application/module/example-module/api/example_api.go)

```go
// ExampleHandler Example handler, inherits from frame.ApiLocator, providing capabilities to get context, config, logger, registered instances etc.
type ExampleHandler struct {
	frame.ApiLocator
	Service        *service.ExampleService 
	KeyTestService string                  
}

func NewExampleHandler(ctx frame.ContextFramer, es *service.ExampleService) *ExampleHandler {
	return &ExampleHandler{
		ApiLocator:     frame.NewApi(ctx).SetName(GetKeyExampleHandler()),
		Service:        es,
		KeyTestService: service.RegisterKeyTestService(ctx),
	}
}

// GetKeyExampleHandler Define and get the instance key for ExampleHandler registered to global manager
func GetKeyExampleHandler(ns ...string) string {
	return frame.RegisterKeyName("ExampleHandler", frame.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// GetExample Get example data
func (h *ExampleHandler) GetExample(c *fiber.Ctx) error {
	// Get language
	var lang = c.Get(constant.XLanguageFlag, "en")

	id := c.Params("id")

	// Construct struct to be validated
	var objId = &requestvo.ObjId{
		ID: id,
	}
	// Get validation wrapper object
	vw := h.GetContext().GetValidateWrap()

	// Get validator for specified language and validate struct
	if errVw := vw.GetValidate(lang).Struct(objId); errVw != nil {
		var errs validator.ValidationErrors
		if errors.As(errVw, &errs) {
			return vw.Errors(errs, lang, true)
		}
	}

	// Get data from service layer
	resp, err := h.Service.GetExample(id)
	if err != nil {
		return err
	}

	// Return success response
	return response.RespSuccess(resp).JsonWithCtx(c)
}
```

- Define example service: See [example_application/module/example-module/service/example_service.go](../../example_application/module/example-module/service/example_service.go)

```go
// ExampleService Example service, inherits frame.ServiceLocator service locator interface, providing capabilities to get context, config, logger, registered instances etc.
type ExampleService struct {
	frame.ServiceLocator                               // Inherit service locator interface
	Repo                 *repository.ExampleRepository // Dependent component: example repository, constructor parameter injection. Injected by wire tool
}

func NewExampleService(ctx frame.ContextFramer, repo *repository.ExampleRepository) *ExampleService {
	name := GetKeyExampleService()
	return &ExampleService{
		ServiceLocator: frame.NewService(ctx).SetName(name),
		Repo:           repo,
	}
}

// GetKeyExampleService Get ExampleService registration key name
func GetKeyExampleService(ns ...string) string {
	return frame.RegisterKeyName("ExampleService", frame.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// GetExample Get example data by ID
func (s *ExampleService) GetExample(id string) (*responsevo.ExampleRespVo, error) {
    resp := responsevo.ExampleRespVo{}
	// Call repository layer to get data
    example, err := s.Repo.GetExampleById(id)
    if err != nil {
        return nil, err
    }
	// Process data
    resp.ExamName = example.Name
    resp.ExamAge = example.Age
    resp.Courses = example.Courses
    resp.Profile = example.Profile
    resp.CreatedAt = example.CreatedAt
    resp.UpdatedAt = example.UpdatedAt
	// Return data
    return &resp, nil
}
```

- Define example repository: See [example_application/module/example-module/repository/example_repository.go](../../example_application/module/example-module/repository/example_repository.go)

```go
// ExampleRepository Example repository, responsible for Example business data persistence operations, inherits frame.RepositoryLocator repository locator interface, providing capabilities to get context, config, logger, registered instances etc.
type ExampleRepository struct {
	frame.RepositoryLocator
	Model *model.ExampleModel
}

func NewExampleRepository(ctx frame.ContextFramer, m *model.ExampleModel) *ExampleRepository {
	return &ExampleRepository{
		RepositoryLocator: frame.NewRepository(ctx).SetName(GetKeyExampleRepository()),
		Model:             m,
	}
}

// GetKeyExampleRepository Get ExampleRepository registration key name
func GetKeyExampleRepository(ns ...string) string {
	return frame.RegisterKeyName("ExampleRepository", frame.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleRepository Register ExampleRepository to container (lazy initialization) and return registration key
func RegisterKeyExampleRepository(ctx frame.ContextFramer, ns ...string) string {
	return frame.RegisterKeyInitializerFunc(GetKeyExampleRepository(ns...), func() (interface{}, error) {
		m := model.NewExampleModel(ctx)
		return NewExampleRepository(ctx, m), nil
	})
}

// GetExampleById Get Example data by ID
func (r *ExampleRepository) GetExampleById(id string) (*entity.Example, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := r.Model.GetExampleByID(ctx, id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, exception.GetNotFoundDocument() // Return error
		}
		exception.GetInternalError().RespError(err.Error()).Panic() // Direct panic
	}
	return result, nil
}
```

- Define example model: See [example_application/module/example-module/model/example_model.go](../../example_application/module/example-module/model/example_model.go)

```go
// ExampleModel Example model, inherits MongoLocator locator interface, providing capabilities to get context, config, logger, registered instances etc. as well as basic mongodb operation capabilities
type ExampleModel struct {
	dbmongo.MongoLocator
	ctx context.Context // Optional attribute
}

func NewExampleModel(ctx frame.ContextFramer) *ExampleModel {
	return &ExampleModel{
		MongoLocator: dbmongo.NewMongoModel(ctx, constant.MongoInstanceKey).SetDbName(constant.DbNameMongo).SetTable(constant.CollExample).
			SetName(GetKeyExampleModel()).(dbmongo.MongoLocator), // Set current model's config item name(mongodb) and database name(test)
		ctx: context.Background(),
	}
}

// GetKeyExampleModel Get model registration key
func GetKeyExampleModel(ns ...string) string {
	return frame.RegisterKeyName("ExampleModel", frame.GetNamespace([]string{constant.NameModuleExample}, ns...)...)
}

// RegisterKeyExampleModel Register model to container (lazy initialization) and return registration key
func RegisterKeyExampleModel(ctx frame.ContextFramer, ns ...string) string {
	return frame.RegisterKeyInitializerFunc(GetKeyExampleModel(ns...), func() (interface{}, error) {
		return NewExampleModel(ctx), nil
	})
}

// GetExampleByID Get example document by ID
func (m *ExampleModel) GetExampleByID(ctx context.Context, oid string) (*entity.Example, error) {
	_id, err := bson.ObjectIDFromHex(oid)
	if err != nil {
		exception.GetInputError().RespError(err.Error()).Panic()
	}
	filter := bson.D{{"_id", _id}}
	opts := options.FindOne().SetProjection(bson.M{
		"_id":     0,
		"profile": 0,
	})
	var example entity.Example
	err = m.GetCollection(m.GetColl()).FindOne(ctx, filter, opts).Decode(&example)
	if err != nil {
		return nil, err
	}
	return &example, nil
}
```

- Call chain summary: For example, get example data endpoint GET /example/get/:id
    - Route registration: RegisterRouteHandlers -> exampleGroup.Get("/get/:id", exampleApi.GetExample)
    - API handler: ExampleHandler.GetExample -> h.Service.GetExample
    - Service layer: ExampleService.GetExample -> s.Repo.GetExampleById
    - Repository layer: ExampleRepository.GetExampleById -> r.Model.GetExampleByID
    - Model layer: ExampleModel.GetExampleByID -> m.GetCollection(m.GetColl()).FindOne(...)
    - Entity layer: entity.Example
    - Response layer: e.g. response.RespSuccess(resp).JsonWithCtx(c) -> response.RespInfo

### How to Add New Modules and New APIs

- Reference example: [example_application/module/example-module](../../example_application/module/example-module)

- Copy example module directory: Copy from `example-module` directory as starting template for new module

```bash
cp -r example_application/module/example-module example_application/module/mymodule
```

- Modify module related files:
    - **Constant definitions**: Modify module name constants in `constant/constants.go`
    - **Entity types**: Modify entity struct definitions in `entity/types.go`
    - **Model layer**: Modify model files in `model/` directory, update model names and database table names
    - **Repository layer**: Modify repository files in `repository/` directory, update repository interfaces and implementations
    - **Service layer**: Modify service files in `service/` directory, update business logic
    - **API layer**: Modify API controller files in `api/` directory, update interface definitions

- Register new module API routes: Add new module route registration in `module/route_register.go`

```go
// Add in RegisterApiRouters function
mymodule.RegisterRouteHandlers(ctx, app)
```

- Update Wire dependency injection: Run `wire` command to regenerate dependency injection code

```bash
# Enter new module's api directory
cd example_application/module/mymodule/api

# Run wire command to generate dependency injection code, specify generated code file prefix
wire gen -output_file_prefix api_provider_
```

### Task Async Task Usage Examples

- Define unique task names: See [example_application/module/example-module/task/names.go](../../example_application/module/example-module/task/names.go)

```go
package task

// A list of task types. List of task names
const (
	// TypeExampleCreate Define task name, asynchronously create example data
	TypeExampleCreate = "ex:example:create:create-an-example"
)
```

- Create new task: See [example_application/module/example-module/task/task.go](../../example_application/module/example-module/task/task.go)

```go
/*
Task payload list Task payload list
*/

// PayloadExampleCreate Example creation payload data
type PayloadExampleCreate struct {
	frame.PayloadBase // Inherit base payload struct, automatically provides methods to get json encoder/decoder
	/**
	Payload data
	*/
	Age int8
}

// NewExampleCreateTask Generate an ExampleCreate task, get relevant parameters from caller and return task
func NewExampleCreateTask(ctx frame.IContext, age int8) (*asynq.Task, error) {
	vo := PayloadExampleCreate{
		Age: age,
	}
	// Get json encoder/decoder, encode payload data to json format byte slice
	payload, err := vo.GetMustJsonHandler(ctx).Marshal(&vo)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExampleCreate, payload, asynq.Retention(24*time.Hour), asynq.MaxRetry(3), asynq.ProcessIn(1*time.Minute)), nil
}
```

- Define task handler: See [example_application/module/example-module/task/handler/handle.go](../../example_application/module/example-module/task/handler/handle.go)

```go
// HandleExampleCreateTask Example task creation handler
func HandleExampleCreateTask(ctx context.Context, t *asynq.Task) error {
	// Get appCtx global application context from context, get components including config, logger, registered instances etc.
	appCtx, _ := ctx.Value(frame.ContextKeyAppCtx).(frame.ContextFramer)

	// Declare task payload object
	var p task.PayloadExampleCreate

	// Parse task payload
	if err := p.GetMustJsonHandler(appCtx).Unmarshal(t.Payload(), &p); err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginWeb()).Str("From", "HandleExampleCreateTask").Err(err).Msg("[Asynq]: Unmarshal error")
		return err
	}

	// Get instance for handling task, note service.TestService needs to be registered to global manager during task mounting phase
    // See task/handler/mount.go: service.RegisterKeyTestService(ctx)
	instance, err := frame.GetInstance[*service.TestService](service.GetKeyTestService())
	if err != nil {
		return err
	}

	// Pass parameters to instance's handler function
	result, err := instance.DoAgeDoubleCreateForTaskHandle(p.Age)
	if err != nil {
		return err
	}

	// Log result
	appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginTask()).Msgf("HandleExampleCreateTask executed successfully, result Age double: %d", result)
	return nil
}
```

- Task mounter: See [example_application/module/example-module/task/handler/mount.go](../../example_application/module/example-module/task/handler/mount.go)

```go
package handler

import (
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
	"github.com/lamxy/fiberhouse/frame"
)

// RegisterTaskHandlers Uniformly register task handler functions and dependent component instance initializers
func RegisterTaskHandlers(tk frame.TaskRegister) {
	// append task handler to global taskHandlerMap
	// Register task handling instance initializers through RegisterKeyXXX and get registered instance keyName

	// Uniformly register global management instance initializers, these instances can be obtained in task handler functions through tk.GetContext().GetContainer().GetXXXService() to execute specific task handling logic
	service.RegisterKeyTestService(tk.GetContext())

	// Uniformly append task handler functions to Task registerer object's task name mapping properties
	tk.AddTaskHandlerToMap(task.TypeExampleCreate, HandleExampleCreateTask)
}
```

- Push task to queue: See [example_application/module/example-module/api/example_api.go](../../example_application/module/example-module/api/example_api.go)
  Calls GetExampleWithTaskDispatcher method in [example_application/module/example-module/service/example_service.go](../../example_application/module/example-module/service/example_service.go)

```go
// GetExampleWithTaskDispatcher Example method demonstrating how to use task dispatcher for async task execution in service methods
func (s *ExampleService) GetExampleWithTaskDispatcher(id string) (*responsevo.ExampleRespVo, error) {
	resp := responsevo.ExampleRespVo{}
	example, err := s.Repo.GetExampleById(id)
	if err != nil {
		return nil, err
	}

	// Get logger with task marking, get logger with log source marking attached from global manager
	log := s.GetContext().GetMustLoggerWithOrigin(s.GetContext().GetConfig().LogOriginTask())

	// After successfully getting example data, push delayed task for async execution
	dispatcher, err := s.GetContext().(frame.ContextFramer).GetStarterApp().GetTask().GetTaskDispatcher()
	if err != nil {
		log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher GetTaskDispatcher failed")
	}
	// Create task object
	task1, err := task.NewExampleCreateTask(s.GetContext(), int8(example.Age))
	if err != nil {
		log.Warn().Err(err).Str("Category", "asynq").Msg("GetExampleWithTaskDispatcher NewExampleCountTask failed")
	}
	// Enqueue task object
	tInfo, err := dispatcher.Enqueue(task1, asynq.MaxRetry(constant.TaskMaxRetryDefault), asynq.ProcessIn(1*time.Minute)) // Enqueue task, will execute in 1 minute

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

### Cache Component Usage Examples

- See get example list endpoint: GetExamples method in [example_application/module/example-module/api/example_api.go](../../example_application/module/example-module/api/example_api.go)
  Calls GetExamplesWithCache method in example service: [example_application/module/example-module/service/example_service.go](../../example_application/module/example-module/service/example_service.go)

```go
func (s *ExampleService) GetExamples(page, size int) ([]responsevo.ExampleRespVo, error) {
	// Get cache option object from cache option pool
	co := cache.OptionPoolGet(s.GetContext())
	// Return cache option object to object pool after use
	defer cache.OptionPoolPut(co)

	// Set cache parameters: two-level cache, enable local cache, set cache key, set local cache random expiration time (10 seconds ±10%), set remote cache random expiration time (3 minutes ±1 minute), write remote cache sync strategy, set context, enable all cache protection measures
	co.Level2().EnableCache().SetCacheKey("key:example:list:page:"+strconv.Itoa(page)+":size:"+strconv.Itoa(size)).SetLocalTTLRandomPercent(10*time.Second, 0.1).
		SetRemoteTTLWithRandom(3*time.Minute, 1*time.Minute).SetSyncStrategyWriteRemoteOnly().SetContextCtx(context.TODO()).EnableProtectionAll()

	// Get cached data, call cache package's GetCached method, pass cache option object and data retrieval callback function
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

### CMD Command Line Application Usage Examples

- Command line framework application main entry: See [example_application/command/main.go](../../example_application/command/main.go)

```go
package main

import (
	"github.com/lamxy/fiberhouse/example_application/command/application"
	"github.com/lamxy/fiberhouse/frame"
	"github.com/lamxy/fiberhouse/frame/bootstrap"
	"github.com/lamxy/fiberhouse/frame/commandstarter"
)

func main() {
	// Bootstrap initialization of startup configuration (global config, global logger), config path is "./../config" under current working directory
	cfg := bootstrap.NewConfigOnce("./../../example_config")

	// Global logger, define log directory as "./logs" under current working directory
	logger := bootstrap.NewLoggerOnce(cfg, "./logs")

	// Initialize command global context
	ctx := frame.NewCmdContextOnce(cfg, logger)

	// Initialize application registerer object, inject to application starter
	appRegister := application.NewApplication(ctx) // Need to implement framework's command line application frame.ApplicationCmdRegister interface

	// Initialize command line starter object
        cmdStarter := commandstarter.NewCmdApplication(ctx, option.WithCmdRegister(appRegister))
	
	// Run command line starter
	commandstarter.RunCommandStarter(cmdStarter)
}
```

- Write a command script: See [example_application/command/application/commands/test_orm_command.go](../../example_application/command/application/commands/test_orm_command.go)

```go
// TestOrmCMD Test go-orm library CRUD operations command, needs to implement frame.CommandGetter interface, return command line command object through GetCommand method
type TestOrmCMD struct {
	Ctx frame.ContextCommander
}

func NewTestOrmCMD(ctx frame.ContextCommander) frame.CommandGetter {
	return &TestOrmCMD{
		Ctx: ctx,
	}
}

// GetCommand Get command line command object, implement GetCommand method of frame.CommandGetter interface
func (m *TestOrmCMD) GetCommand() interface{} {
	return &cli.Command{
		Name:    "test-orm",
		Aliases: []string{"orm"},
		Usage:   "Test go-orm library CRUD operations",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "method",
				Aliases:  []string{"m"},
				Usage:    "Test type(ok/orm)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "operation",
				Aliases:  []string{"o"},
				Usage:    "CRUD(c create|u update|r read|d delete)",
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

			// Use dig to inject required dependencies, inject dependency components through provide chained methods
			dc := m.Ctx.GetDigContainer().
				Provide(func() frame.ContextCommander { return m.Ctx }).
				Provide(model.NewExampleMysqlModel).
				Provide(service.NewExampleMysqlService)

			// Error handling
			if dc.GetErrorCount() > 0 {
				return fmt.Errorf("dig container init error: %v", dc.GetProvideErrs())
			}

			/*
			// Get dependency components through Invoke method, use dependency components in callback function
			err := dc.Invoke(func(ems *service.ExampleMysqlService) error {
				err := ems.AutoMigrate()
				if err != nil {
					return err
				}
				// Other operations...
				return nil
			})
			*/

			// Another way, use generic Invoke method to get dependency components, get dependency components through component.Wrap helper type
			err := component.Invoke[*service.ExampleMysqlService](wrap)
			if err != nil {
				return err
			}

			// Get dependency component
			ems = wrap.Get()

			// Auto create data table once
			err = ems.AutoMigrate()
			if err != nil {
				return err
			}

			// Get command line parameters
			method := cCtx.String("method")

			// Execute test
			if method == "ok" {
				testOk := ems.TestOk()

				fmt.Println("result: ", testOk, "--from:", method)
			} else if method == "orm" {
				// Get more command line parameters
				op := cCtx.String("operation")
				id := cCtx.Uint("id")

				// Execute test orm
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

- Command line build: See [example_application/command/README_go_build.md](../../example_application/command/README_go_build.md)

```bash
# Build
cd command/  # command ROOT Directory
go build -o ./target/cmdstarter.exe ./main.go 

# Execute command help
cd command/    ## work dir is ~/command/, configure path base on it
./target/cmdstarter.exe -h
```

- Command line application usage instructions
    - Compile command line application: `go build -o ./target/cmdstarter.exe ./main.go `
    - Run command line application to view help: `./target/cmdstarter.exe -h`
    - Run test go-orm library CRUD operations command: `./target/cmdstarter.exe test-orm --method ok` or `./target/cmdstarter.exe test-orm -m ok`
    - Run test go-orm library CRUD operations command (create data): `./target/cmdstarter.exe test-orm --method orm --operation c --id 1` or `./target/cmdstarter.exe test-orm -m orm -o c -i 1`
    - Sub-command parameter help: `./target/cmdstarter.exe test-orm -h`

## 🔧 Configuration

### Application Global Configuration
FiberHouse supports environment-based multi-configuration file management, with configuration files located in the `example_config/` directory. The global configuration object is located in the framework context object and can be accessed through the `ctx.GetConfig()` method.

- Configuration file README: See [example_config/README.md](../../example_config/README.md)

- Configuration file naming convention

```
Configuration file format: application_[application_type]_[environment].yml
Application type: web | cmd
Environment type: dev | test | prod

Example files:
- application_web_dev.yml     # Web application development environment
- application_web_test.yml    # Web application test environment  
- application_web_prod.yml    # Web application production environment
- application_cmd_test.yml    # Command line application test environment
```

- Environment variable configuration

```
# Bootstrap environment variables (APP_ENV_ prefix):
APP_ENV_application_appType=web    # Set application type: web/cmd
APP_ENV_application_env=prod       # Set runtime environment: dev/test/prod

# Configuration override environment variables (APP_CONF_ prefix):
APP_CONF_application_appName=MyApp              # Override application name
APP_CONF_application_server_port=9090           # Override server port
APP_CONF_application_appLog_level=error         # Override log level
APP_CONF_application_appLog_asyncConf_type=chan # Override async log type
```

#### Core Configuration Items

- Application basic configuration:
```yaml
application:
  appName: "FiberHouse"           # Application name
  appType: "web"                  # Application type: web/cmd
  env: "dev"                      # Runtime environment: dev/test/prod
  
  server:
    host: "127.0.0.1"              # Service host
    port: 8080                     # Service port
```

- Logging system configuration:
```yaml
application:
  appLog:
    level: "info"                # Log level: debug/info/warn/error
    enableConsole: true          # Enable console output
    consoleJSON: false           # Console JSON format
    enableFile: true             # Enable file output
    filename: "app.log"          # Log filename
    
    # Async log configuration
    asyncConf:
      enable: true              # Enable async logging
      type: "diode"             # Async type: chan/diode
      
    # Log rotation configuration  
    rotateConf:
      maxSize: 5                             # megabytes
      maxBackups: 5                          # Maximum backup files
      maxAge: 7                              # days
      compress: false                        # disabled by default
```

- Database configuration:
```yaml
# MySQL configuration
mysql:
  dsn: "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
  gorm:
    maxIdleConns: 10                       # Maximum idle connections
    maxOpenConns: 100                      # Maximum open connections
    connMaxLifetime: 3600                  # Connection max lifetime in seconds
    connMaxIdleTime: 300                   # Connection max idle time in seconds
    logger:
      level: info                        # Log level: silent、error、warn、info
      slowThreshold: 200 * time.Millisecond # Slow SQL threshold, recommended 200 * time.Millisecond, adjust according to business
      colorful: false                    # Colorful output
      enable: true                       # Enable logging
      skipDefaultFields: true            # Skip default fields
  pingTry: false
```

- Redis configuration:
```yaml
redis:
  host: "127.0.0.1"
  port: 6379
  password: ""
  database: 0
  poolSize: 100                # Connection pool size
  
  # Cluster configuration (optional)
  cluster:
    addrs: ["127.0.0.1:6379"]
    poolSize: 100
```

- Cache system configuration:
```yaml
cache:
  # Local cache
  local:                                     # Local cache configuration
    numCounters: 1000000                     # 1 million counters
    maxCost: 134217728                       # Maximum cache 128M
    bufferItems: 64                          # Buffer size per cache partition
    metrics: true                            # Enable cache metrics
    IgnoreInternalCost: false                # Ignore internal cost
      
  # Remote cache  
  redis:                                     # Remote cache configuration
    host: 127.0.0.1                          # Redis server address
    port: 6379                               # Redis server port
    password: ""                             # Redis server password
  # Async pool configuration
  asyncPool:                               # Async goroutine pool configuration for two-level cache, handling cache updates and sync strategies
    ants:                                  # ants async goroutine pool configuration
      local:
        size: 248                          # Local cache async goroutine pool size
        expiryDuration: 5                  # Idle goroutine timeout in seconds
        preAlloc: false                    # No pre-allocation
        maxBlockingTasks: 512              # Maximum blocking tasks
        nonblocking: false                 # Allow blocking
```

- Task component configuration
```yaml
  task:
    enableServer: true                       # Enable task scheduling service component support
```

- More configurations can be customized as needed

- Complete configuration examples reference:
    - Test environment configuration: [example_config/application_web_test.yml](../../example_config/application_web_test.yml)
    - Command line test environment configuration: [application_cmd_test.yml](../../example_config/application_cmd_test.yml)

## 🤝 Contribution Guidelines

### Quick Start
- Fork repository and Clone
- Create branch: git checkout -b feature/your-feature
- Develop and maintain format: go fmt ./... && golangci-lint run
- Run tests: go test ./... -race -cover
- Commit: feat(module): description
- Push and create PR

### Branch Strategy
- main: Stable release
- develop: Integration development
- feature/*: Features
- fix/*: Bug fixes
- Other categories

### PR Requirements
- Title: Consistent with commit message
- Content: Background / Solution / Impact / Tests / Related Issues
- CI must pass

### Security
Please report security vulnerabilities privately: pytho5170@hotmail.com

## 📄 License

This project is open sourced under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙋‍♂️ Support & Feedback

- If you're interested or support FiberHouse's continued development, please star on GitHub [GitHub Star](https://github.com/lamxy/fiberhouse/stargazers)
- Issue feedback: [Issues](https://github.com/lamxy/fiberhouse/issues)
- Contact email: pytho5170@hotmail.com

## 🌟 Acknowledgements

Thanks to the following open source projects:

- [gofiber/fiber](https://github.com/gofiber/fiber) - High-performance HTTP core
- [rs/zerolog](https://github.com/rs/zerolog) - High-performance structured logging
- [knadh/koanf](https://github.com/knadh/koanf) - Flexible multi-source configuration management
- [bytedance/sonic](https://github.com/bytedance/sonic) - High-performance JSON encoder/decoder
- [dgraph-io/ristretto](https://github.com/dgraph-io/ristretto) - High-performance local cache
- [hibiken/asynq](https://github.com/hibiken/asynq) - Redis-based distributed task queue
- [go.mongodb.org/mongo-driver](https://github.com/mongodb/mongo-go-driver) - Official MongoDB driver
- [gorm.io/gorm](https://gorm.io) - ORM abstraction and MySQL support
- [redis/go-redis](https://github.com/redis/go-redis) - Redis client
- [panjf2000/ants](https://github.com/panjf2000/ants) - High-performance goroutine pool

Also thanks to:
- [swaggo/swag](https://github.com/swaggo/swag) for API documentation generation
- [google/wire](https://github.com/google/wire), [uber-go/dig](https://github.com/uber-go/dig) for dependency injection pattern support
- And all other excellent projects not listed individually

Finally, thanks to GitHub Copilot for providing documentation research, organization, and coding assistance capabilities.