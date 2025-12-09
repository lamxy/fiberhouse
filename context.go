// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"fmt"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component"
	"github.com/lamxy/fiberhouse/component/validate"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/rs/zerolog"
	"sync"
)

var (
	// applicationContext Web应用上下文单例
	applicationContext ContextFramer
	once               sync.Once

	// commandContext 命令行应用上下文单例
	commandContext ContextCommander
	onceCmd        sync.Once
)

// AppContext Web应用上下文实现
type AppContext struct {
	Cfg          appconfig.IAppConfig
	logger       bootstrap.LoggerWrapper
	container    *globalmanager.GlobalManager
	starterApp   ApplicationStarter
	appState     bool
	appStateOnce sync.Once
	vw           *validate.Wrap
	storage      map[string]interface{}
	lock         sync.RWMutex
}

// NewAppContext 获取新的全局上下文对象
func NewAppContext(cfg *appconfig.AppConfig, logger bootstrap.LoggerWrapper) ContextFramer {
	return &AppContext{
		Cfg:          cfg,
		logger:       logger,
		container:    globalmanager.NewGlobalManagerOnce(),
		appState:     false,
		appStateOnce: sync.Once{},
		vw:           validate.NewWrap(cfg),
		storage:      make(map[string]interface{}),
	}
}

// NewAppContextOnce 获取全局应用上下文单例
func NewAppContextOnce(cfg appconfig.IAppConfig, logger bootstrap.LoggerWrapper) ContextFramer {
	once.Do(func() {
		applicationContext = &AppContext{
			Cfg:       cfg,
			logger:    logger,
			container: globalmanager.NewGlobalManagerOnce(),
			vw:        validate.NewWrap(cfg),
			storage:   make(map[string]interface{}),
		}
	})
	return applicationContext
}

// setAppState 设置应用启动状态
func (c *AppContext) SetAppState(state bool) {
	c.appStateOnce.Do(func() {
		c.appState = state
	})
}

// GetAppState 获取应用状态
func (c *AppContext) GetAppState() bool {
	return c.appState
}

// GetConfig 获取全局配置
func (c *AppContext) GetConfig() appconfig.IAppConfig {
	return c.Cfg
}

// GetLogger 获取全局日志器
func (c *AppContext) GetLogger() bootstrap.LoggerWrapper {
	return c.logger
}

// GetLoggerWithOrigin 依据配置文件预定义LogOrigin来源，从全局管理器获取指定来源的子日志器单例
func (c *AppContext) GetLoggerWithOrigin(originFromCfg appconfig.LogOrigin) (*zerolog.Logger, error) {
	origin := originFromCfg.String()
	if origin == "" {
		return c.GetLogger().GetZeroLogger(), nil
	}
	key := constant.LogOriginKeyPrefix + origin
	instance, err := c.GetContainer().Get(key)
	if err != nil {
		return nil, err
	}
	return instance.(*zerolog.Logger), nil
}

// GetMustLoggerWithOrigin 依据配置文件预定义LogOrigin来源，从全管理器获取指定来源的子日志器单例
func (c *AppContext) GetMustLoggerWithOrigin(originFromCfg appconfig.LogOrigin) *zerolog.Logger {
	origin := originFromCfg.String()
	if origin == "" {
		return c.GetLogger().GetZeroLogger()
	}
	key := constant.LogOriginKeyPrefix + origin
	instance, err := c.GetContainer().Get(key)
	if err != nil {
		panic(err)
	}
	return instance.(*zerolog.Logger)
}

// GetContainer 获取全局管理容器实例
func (c *AppContext) GetContainer() *globalmanager.GlobalManager {
	return c.container
}

// GetValidateWrap 获取全局验证包装器
func (c *AppContext) GetValidateWrap() validate.ValidateWrapper {
	return c.vw
}

// RegisterStarterApp 挂载框架启动器app
func (c *AppContext) RegisterStarterApp(sApp ApplicationStarter) {
	c.starterApp = sApp
}

// GetStarterApp 获取框架启动器实例(FrameApplication)
func (c *AppContext) GetStarterApp() ApplicationStarter {
	return c.starterApp
}

// GetStarter 获取IStarter启动器实例(框架Web应用启动器实例FrameApplication)
//
// 注意：IStarter接口是为了兼容AppContext（web应用上下文）和CmdContext（命令行应用上下文）两种上下文抽象出公共的方法的实现
//
//	但实际上在Web应用上下文中，IStarter接口的实现是 ApplicationStarter Web应用启动器,
//	在命令行上下文中，IStarter接口的实现是 CommandStarter 命令行启动器
//
//	这两者在实际使用中是不同的，AppContext作用的是FrameApplication（web框架应用启动器），而CmdContext作用的是CommandApplication（CMD命令行应用启动器）,
//	但为了保持接口一致性，这里仍然使用IStarter接口,
//	在实际应用类别中，开发者需要根据上下文类型来判断具体的实现，并断言成具体的实现，以获取除公共方法外的具体方法的调用
func (c *AppContext) GetStarter() IStarter {
	return c.starterApp
}

// GetValue 从AppContext的存储中获取指定key的值
func (c *AppContext) GetValue(key string) (interface{}, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.storage[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("no key named '%s' in the context map", key)
}

// SetValue 向AppContext的存储中设置指定key的值，若key已存在则返回错误
func (c *AppContext) SetValue(key string, val interface{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, ok := c.storage[key]; ok {
		return fmt.Errorf("the key named '%s' already exists in the AppContext storage. Duplicate settings are not allowed. You can reset it after deleting it", key)
	}
	c.storage[key] = val
	return nil
}

// DeleteValue 从AppContext的存储中删除指定key的值，若key不存在也返回true
func (c *AppContext) DeleteValue(key string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.storage, key)
	return true
}

// CmdContext 命令行应用上下文实现
type CmdContext struct {
	Cfg          appconfig.IAppConfig
	logger       bootstrap.LoggerWrapper
	container    *globalmanager.GlobalManager // 全局管理器
	starterApp   CommandStarter
	digContainer *component.DigContainer // uber dig 依赖注入器
}

// NewCmdContextOnce 获取命令行应用上下文对象单例
func NewCmdContextOnce(cfg appconfig.IAppConfig, logger bootstrap.LoggerWrapper) ContextCommander {
	onceCmd.Do(func() {
		commandContext = &CmdContext{
			Cfg:          cfg,
			logger:       logger,
			container:    globalmanager.NewGlobalManagerOnce(),
			digContainer: component.NewDigContainerOnce(),
		}
	})
	return commandContext
}

// GetLoggerWithOrigin 依据配置文件预定义LogOrigin来源，从全管理器获取指定来源的子日志器单例
func (c *CmdContext) GetLoggerWithOrigin(originFromCfg appconfig.LogOrigin) (*zerolog.Logger, error) {
	origin := originFromCfg.String()
	if origin == "" {
		return c.logger.GetZeroLogger(), nil
	}
	key := constant.LogOriginKeyPrefix + origin
	instance, err := c.GetContainer().Get(key)
	if err != nil {
		return nil, err
	}
	return instance.(*zerolog.Logger), nil
}

// GetMustLoggerWithOrigin 依据配置文件预定义LogOrigin来源，从全管理器获取指定来源的子日志器单例
func (c *CmdContext) GetMustLoggerWithOrigin(originFromCfg appconfig.LogOrigin) *zerolog.Logger {
	origin := originFromCfg.String()
	if origin == "" {
		return c.logger.GetZeroLogger()
	}
	key := constant.LogOriginKeyPrefix + origin
	instance, err := c.GetContainer().Get(key)
	if err != nil {
		panic(err)
	}
	return instance.(*zerolog.Logger)
}

// GetConfig 获取全局配置
func (c *CmdContext) GetConfig() appconfig.IAppConfig {
	return c.Cfg
}

// GetLogger 获取全局日志器
func (c *CmdContext) GetLogger() bootstrap.LoggerWrapper {
	return c.logger
}

// GetContainer 获取全局管理容器实例
func (c *CmdContext) GetContainer() *globalmanager.GlobalManager {
	return c.container
}

// GetDigContainer 获取依赖注入容器
func (c *CmdContext) GetDigContainer() *component.DigContainer {
	return c.digContainer
}

// RegisterStarterApp 挂载框架启动器app
func (c *CmdContext) RegisterStarterApp(app CommandStarter) {
	c.starterApp = app
}

// GetStarterApp 获取框架启动器实例
func (c *CmdContext) GetStarterApp() CommandStarter {
	return c.starterApp
}

// GetStarter 获取IStarter启动器实例(框架命令行启动器实例CommandApplication)
//
// 注意：IStarter接口是为了兼容CmdContext（命令行应用上下文）和AppContext（web应用上下文）两种上下文抽象出公共的方法的实现
//
//	但实际上在命令行上下文中，IStarter接口的实现是 CommandStarter 命令行启动器,
//	在Web应用上下文中，IStarter接口的实现是 ApplicationStarter Web应用启动器
//
//	这两者在实际使用中是不同的，CmdContext作用的是CommandApplication（CMD命令行应用启动器），而AppContext作用的是FrameApplication（web框架应用启动器）,
//	但为了保持接口一致性，这里仍然使用IStarter接口,
//	在实际应用类别中，开发者需要根据上下文类型来判断具体的实现，并断言成具体的实现，以获取除公共方法外的具体方法的调用
func (c *CmdContext) GetStarter() IStarter {
	return c.starterApp
}

// GetValidateWrap 获取验证器包装器
func (c *CmdContext) GetValidateWrap() validate.ValidateWrapper {
	return nil
}
