// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse/component/validate"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
)

// IStarter 启动器接口，定义了获取应用实例的方法
//
// # IApplication 定义了一些通用方法，如获取预定义的全局对象实例Key方法
//
// 全局对象实例Key，如数据库、缓存、任务调度器等，通过实例Key在全局管理器中获取对应的实例
type IStarter interface {
	GetApplication() IApplication
}

// FrameStarter 应用框架启动器接口
type FrameStarter interface {
	IStarter
	// GetContext 获取应用上下文
	// 返回全局应用上下文，提供配置、日志器、全局容器等基础设施访问
	GetContext() ContextFramer

	// RegisterApplication 注册应用注册器
	// 将应用注册器实例注入到启动器中，用于后续的全局对象初始化和配置
	RegisterApplication(application ApplicationRegister)

	// RegisterModule 注册模块注册器
	// 将模块注册器实例注入到启动器中，用于模块级中间件、路由和Swagger的注册
	RegisterModule(module ModuleRegister)

	// GetModule 获取模块注册器
	// 返回已注册的模块注册器实例
	GetModule() ModuleRegister

	// RegisterTask 注册任务注册器
	// 将任务注册器实例注入到启动器中，用于异步任务服务器的初始化和启动
	RegisterTask(task TaskRegister)

	// GetTask 获取任务注册器
	// 返回已注册的任务注册器实例
	GetTask() TaskRegister

	// RegisterToCtx 注册启动器到上下文
	// 将启动器实例注册到应用上下文中，便于其他组件访问
	RegisterToCtx(starter ApplicationStarter)

	// RegisterApplicationGlobals 注册应用全局对象和初始化
	// 注册全局对象初始化器、初始化必要的全局实例、配置验证器等
	// 包括数据库、缓存、Redis、验证器、自定义标签等的初始化
	RegisterApplicationGlobals()

	// RegisterLoggerWithOriginToContainer 注册带来源标识的日志器
	// 将配置文件中预定义的不同来源的子日志器初始化器注册到容器中
	// 便于获取已附加来源标记的专用日志器实例
	RegisterLoggerWithOriginToContainer()

	// RegisterGlobalsKeepalive 注册全局对象保活机制
	// 启动后台健康检测服务，定期检查全局对象状态并自动重建不健康的实例
	RegisterGlobalsKeepalive()

	// RegisterTaskServer 注册异步任务服务器
	// 根据配置启动异步任务服务器，注册任务处理器，运行后台任务worker服务并开始监听任务队列
	RegisterTaskServer()

	// GetFrameApp 获取框架启动器实例
	GetFrameApp() FrameStarter
}

// CoreStarter 应用核心启动器接口
type CoreStarter interface {
	// GetAppContext 获取应用上下文
	// 返回全局应用上下文，提供配置、日志器、全局容器等基础设施访问
	GetAppContext() ContextFramer

	// InitCoreApp 初始化核心应用
	// 创建并配置底层HTTP服务实例（如Fiber应用）
	InitCoreApp(fs FrameStarter)

	// RegisterAppMiddleware 注册应用级中间件
	// 注册应用级别的中间件，如错误恢复、请求日志、CORS等全局中间件
	RegisterAppMiddleware(fs FrameStarter)

	// RegisterModuleSwagger 注册模块Swagger文档
	// 根据配置决定是否注册Swagger API文档路由
	RegisterModuleSwagger(fs FrameStarter)

	// RegisterAppHooks 注册应用钩子函数
	// 注册应用生命周期钩子函数，如启动、关闭时的回调处理
	RegisterAppHooks(fs FrameStarter)

	// RegisterModuleInitialize 注册模块初始化
	// 执行模块级别的初始化，包括模块中间件和路由处理器的注册
	RegisterModuleInitialize(fs FrameStarter)

	// AppCoreRun 启动应用核心运行
	// 启动HTTP服务监听，处理优雅关闭信号
	AppCoreRun()

	// GetCoreApp 获取核心实例
	GetCoreApp() interface{}
}

// ApplicationStarter 定义框架应用启动器的接口(定义启动方法和启动流程)
type ApplicationStarter interface {
	FrameStarter
	CoreStarter
}

// IRegister 注册器接口
//
// 用来将用户自定义的应用、模块/子系统及任务的注册器实例，注册进应用启动器
type IRegister interface {
	// GetName 获取注册器名称
	GetName() string
	// SetName 设置注册器名称
	SetName(name string)
}

// InstanceKeyFlag 预定义配置的全局对象实例key的标识，映射具体的InstanceKey，
// 通过该标识自定义需要从全局管理器获取特定实例
type InstanceKeyFlag string

// String 获取实例Key标识字符串
func (p *InstanceKeyFlag) String() string {
	return string(*p)
}

// InstanceKey 定义全局对象实例的Key类型
type InstanceKey string

// String 获取实例Key字符串
func (ik *InstanceKey) String() globalmanager.KeyName {
	return string(*ik)
}

// PrefixString 带前缀的实例Key字符串，用于容器注册
func (ik *InstanceKey) PrefixString() globalmanager.KeyName {
	return constant.RegisterKeyPrefix + string(*ik)
}

// StringWithPrefix 带自定义前缀的实例Key字符串, 用于容器注册
func (ik *InstanceKey) StringWithPrefix(pfx string) globalmanager.KeyName {
	return pfx + string(*ik)
}

// IApplication 应用接口，定义了一些通用方法，如获取预定义的全局对象实例Key方法，
//
// 框架启动器通过这些预定义方法，获取必要的全局对象实例key，
// 以便在应用启动器启动阶段，注册和初始化这些全局对象实例，完成框架的启动流程
type IApplication interface {
	GetDBKey() globalmanager.KeyName               // 定义数据库实例key
	GetDBMongoKey() globalmanager.KeyName          // 定义mongodb实例的key，应用启动器用到
	GetDBMysqlKey() globalmanager.KeyName          // 定义mysql实例的key，应用启动器用到
	GetRedisKey() globalmanager.KeyName            // 定义redis实例key，应用启动器异步任务注册用到
	GetFastJsonCodecKey() globalmanager.KeyName    // 定义快速的json编解码器，应用启动器用到
	GetDefaultJsonCodecKey() globalmanager.KeyName // 定义默认的json编解码器，应用启动器用到
	GetTaskDispatcherKey() globalmanager.KeyName   // 定义异步任务客户端实例的key
	GetCacheKey() globalmanager.KeyName            // 定义缓存实例的key，
	GetTaskServerKey() globalmanager.KeyName       // 定义异步任务服务端实例的key
	GetLocalCacheKey() globalmanager.KeyName       // 定义本地缓存实例的key
	GetRemoteCacheKey() globalmanager.KeyName      // 定义远程缓存实例的key
	GetLevel2CacheKey() globalmanager.KeyName      // 定义二级缓存实例的key

	// more keys...

	// GetInstanceKey 自定义全局对象实例key的通用获取方法，获取框架上述预定义外的实例key
	GetInstanceKey(flag InstanceKeyFlag) InstanceKey // 按预定义flag获取全局对象实例key
}

// ApplicationRegister 应用注册器
//
// 在应用启动阶段由启动器调用，用于：
// 1. 注册应用的自定义配置、依赖与初始化逻辑；
// 2. 将注册器实例绑定到 ApplicationStarter 的 application 字段，供启动流程使用。
type ApplicationRegister interface {
	IRegister
	IApplication
	// GetContext 返回全局上下文
	GetContext() ContextFramer

	// ConfigGlobalInitializers 配置并返回全局对象初始化器的列表映射
	ConfigGlobalInitializers() globalmanager.InitializerMap
	// ConfigRequiredGlobalKeys 配置并返回需要初始化的全局对象keyName的切片
	ConfigRequiredGlobalKeys() []globalmanager.KeyName
	// ConfigCustomValidateInitializers 配置自定义语言验证器初始化器的切片
	//见框架组件: validate.Wrap
	ConfigCustomValidateInitializers() []validate.ValidateInitializer
	// ConfigValidatorCustomTags 配置并返回需要注册的验证器自定义tag及翻译的切片(当验证tag缺乏所需语言的翻译时，可以自定义tag翻译)
	//见框架组件: validate.RegisterValidatorTagFunc
	ConfigValidatorCustomTags() []validate.RegisterValidatorTagFunc

	// RegisterAppMiddleware 注册应用级别中间件
	RegisterAppMiddleware(cs CoreStarter)

	// RegisterCoreHook 注册核心应用(coreApp)的生命周期钩子
	RegisterCoreHook(cs CoreStarter)
}

// ModuleRegister 模块注册器
//
// 用于注册应用的模块/子系统，包括中间件、路由、swagger等
// 启动器会调用模块注册器完成模块初始化
type ModuleRegister interface {
	IRegister
	// GetContext 返回全局上下文
	GetContext() ContextFramer

	// RegisterModuleMiddleware 注册模块级别/子系统中间件
	RegisterModuleMiddleware(cs CoreStarter)
	// RegisterModuleRouteHandlers 注册模块级别/子系统路由处理器
	RegisterModuleRouteHandlers(cs CoreStarter)
	// RegisterSwagger 注册swagger
	RegisterSwagger(cs CoreStarter)
}

// TaskRegister 任务注册器（基于 asynq）
//
// 用户需实现此接口并在应用启动阶段注册到 ApplicationStarter
// 注册后的任务注册器实例会绑定到 ApplicationStarter 的 task 属性，由启动器调用其方法完成任务组件的初始化
//
// 当全局配置开启异步任务组件时，任务注册器负责：
// 1. 集中声明并注册任务类型（asynq 任务名）与其处理函数到映射容器。
// 2. 将任务调度器（Dispatcher）与任务工作器（Worker）的初始化器注册到全局容器。
// 3. 提供获取任务调度器与工作器实例的访问方法。
type TaskRegister interface {
	IRegister
	// GetContext 返回全局上下文
	GetContext() ContextFramer

	// GetTaskHandlerMap 返回任务处理器配置map
	//
	// 示例:
	// func myTaskHandler(ctx context.Context, t *asynq.Task) error {
	//     // 处理任务逻辑
	//     return nil // 或返回错误
	// }
	//
	// taskHandlerMap := map[string]func(context.Context, *asynq.Task) error{
	//     "task_type_1": myTaskHandler,
	//     // 更多任务类型和对应的处理器函数
	// }
	GetTaskHandlerMap() map[string]func(context.Context, *asynq.Task) error

	// AddTaskHandlerToMap 向任务处理器映射中添加一个新的任务处理器
	//
	// 示例:
	// func myTaskHandler2(ctx context.Context, t *asynq.Task) error {
	//     // 处理任务逻辑
	//     return nil // 或返回错误
	// }
	//
	// taskRegister.AddTaskHandlerToMap("task_type_2", myTaskHandler2)
	AddTaskHandlerToMap(pattern string, handler func(context.Context, *asynq.Task) error)

	// RegisterTaskServerToContainer 注册异步任务服务器初始化器到容器
	RegisterTaskServerToContainer()

	// RegisterTaskDispatcherToContainer 注册异步任务客户端初始化器到容器
	RegisterTaskDispatcherToContainer()

	// GetTaskDispatcher 获取任务客户端/调度器实例
	GetTaskDispatcher() (*TaskDispatcher, error)

	// GetTaskWorker 获取任务服务器/工作器实例
	GetTaskWorker(key string) (*TaskWorker, error)
}
