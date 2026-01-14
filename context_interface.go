// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component"
	"github.com/lamxy/fiberhouse/component/validate"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/rs/zerolog"
)

// IContext 全局上下文接口
type IContext interface {
	// GetConfig 定义获取全局配置的方法
	GetConfig() appconfig.IAppConfig
	// GetLogger 定义获取全局日志器的方法
	GetLogger() bootstrap.LoggerWrapper
	// GetContainer 定义获取全局管理器的方法
	GetContainer() *globalmanager.GlobalManager
	// GetStarter 定义获取启动器实例的方法，用于获取IApplication实例方法
	GetStarter() IStarter
	// GetLoggerWithOrigin 定义获取附加来源的子日志器单例的方法（从全局管理器获取）
	GetLoggerWithOrigin(originFormCfg appconfig.LogOrigin) (*zerolog.Logger, error)
	// GetMustLoggerWithOrigin 定义获取附加来源的日志器实例的方法，若获取失败则panic（从全局管理器获取）
	GetMustLoggerWithOrigin(originFormCfg appconfig.LogOrigin) *zerolog.Logger
	// GetValidateWrap 定义获取全局验证器包装器的方法
	GetValidateWrap() validate.ValidateWrapper
}

// IApplicationContext 框架Web应用上下文接口
type IApplicationContext interface {
	IContext
	// RegisterStarterApp 挂载框架启动器app
	RegisterStarterApp(sApp ApplicationStarter)
	// GetStarterApp 获取框架应用启动器实例(如WebApplication)
	GetStarterApp() ApplicationStarter
	// RegisterAppState 注册应用启动状态
	RegisterAppState(bool)
	// GetAppState 获取应用启动状态
	GetAppState() bool
	// GetBootConfig 获取启动配置
	GetBootConfig() *BootConfig
	// RegisterBootConfig 注册启动配置
	RegisterBootConfig(bc *BootConfig)
}

// IApplicationContext 框架命令行应用上下文接口
type ICommandContext interface {
	IContext
	// GetDigContainer 获取依赖注入容器
	GetDigContainer() *component.DigContainer
	// RegisterStarterApp 挂载框架启动器app
	RegisterStarterApp(app CommandStarter)
	// GetStarterApp 获取框架启动器实例(CommandStarter)
	GetStarterApp() CommandStarter
}

// IStorage 通用键值存储接口
type IStorage interface {
	// Set 设置键值对,如果键已存在则覆盖
	Set(key string, value interface{})

	// Get 获取指定键的值,返回值和是否存在的标志
	Get(key string) (value interface{}, exists bool)

	// GetOrDefault 获取值,如果不存在则返回默认值
	GetOrDefault(key string, defaultValue interface{}) interface{}

	// Delete 删除指定键,返回是否删除成功
	Delete(key string) bool

	// Has 检查键是否存在
	Has(key string) bool

	// Clear 清空所有键值对
	Clear()

	// Keys 返回所有键的切片
	Keys() []string

	// Len 返回存储的键值对数量
	Len() int

	// Range 遍历所有键值对,如果回调函数返回false则停止遍历
	Range(f func(key string, value interface{}) bool)
}
