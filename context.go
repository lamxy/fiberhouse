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
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/rs/zerolog"
	"sync"
)

var (
	// applicationContext Web应用上下文单例
	applicationContext IApplicationContext
	once               sync.Once

	// commandContext 命令行应用上下文单例
	commandContext ICommandContext
	onceCmd        sync.Once
)

// AppContext Web应用上下文实现
type AppContext struct {
	IStorage     // 内存键值存储接口实现
	cfg          appconfig.IAppConfig
	logger       bootstrap.LoggerWrapper
	container    *globalmanager.GlobalManager
	starterApp   ApplicationStarter
	appState     bool
	appStateOnce sync.Once
	vw           *validate.Wrap
	storage      map[string]interface{}
	lock         sync.RWMutex
	bootCfg      *BootConfig
	bootCfgOnce  sync.Once
}

// NewAppContext 获取新的全局上下文对象
func NewAppContext(cfg *appconfig.AppConfig, logger bootstrap.LoggerWrapper) IApplicationContext {
	return &AppContext{
		IStorage:     NewDefaultStorage(),
		cfg:          cfg,
		logger:       logger,
		container:    globalmanager.NewGlobalManagerOnce(),
		appState:     false,
		appStateOnce: sync.Once{},
		vw:           validate.NewWrap(cfg),
		storage:      make(map[string]interface{}),
	}
}

// NewAppContextOnce 获取全局应用上下文单例
func NewAppContextOnce(cfg appconfig.IAppConfig, logger bootstrap.LoggerWrapper) IApplicationContext {
	once.Do(func() {
		applicationContext = &AppContext{
			IStorage:  NewDefaultStorage(),
			cfg:       cfg,
			logger:    logger,
			container: globalmanager.NewGlobalManagerOnce(),
			vw:        validate.NewWrap(cfg),
			storage:   make(map[string]interface{}),
		}
	})
	return applicationContext
}

// GetBootConfig 获取应用启动配置
func (c *AppContext) GetBootConfig() *BootConfig {
	return c.bootCfg
}

// RegisterBootConfig 注册应用启动配置
func (c *AppContext) RegisterBootConfig(bc *BootConfig) {
	// 仅启动时注册一次
	c.bootCfgOnce.Do(func() {
		c.bootCfg = bc
	})
}

// RegisterAppState 注册应用启动状态
func (c *AppContext) RegisterAppState(state bool) {
	// 仅启动完成时注册一次
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
	return c.cfg
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

// CmdContext 命令行应用上下文实现
type CmdContext struct {
	IStorage     // 内存键值存储接口实现
	Cfg          appconfig.IAppConfig
	logger       bootstrap.LoggerWrapper
	container    *globalmanager.GlobalManager // 全局管理器
	starterApp   CommandStarter
	digContainer *component.DigContainer // uber dig 依赖注入器
}

// NewCmdContextOnce 获取命令行应用上下文对象单例
func NewCmdContextOnce(cfg appconfig.IAppConfig, logger bootstrap.LoggerWrapper) ICommandContext {
	onceCmd.Do(func() {
		commandContext = &CmdContext{
			IStorage:     NewDefaultStorage(),
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

// GetValidateWrap 获取全局验证包装器
func (c *CmdContext) GetValidateWrap() validate.ValidateWrapper {
	return nil
}

// DefaultStorage 默认的内存键值存储实现
type DefaultStorage struct {
	data map[string]interface{}
	lock sync.RWMutex
}

// NewDefaultStorage 创建默认存储实例
func NewDefaultStorage() IStorage {
	return &DefaultStorage{
		data: make(map[string]interface{}),
	}
}

// Set 设置键值对
func (s *DefaultStorage) Set(key string, value interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data[key] = value
}

// Get 获取指定键的值
func (s *DefaultStorage) Get(key string) (interface{}, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// GetOrDefault 获取值,不存在则返回默认值
func (s *DefaultStorage) GetOrDefault(key string, defaultValue interface{}) interface{} {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if v, ok := s.data[key]; ok {
		return v
	}
	return defaultValue
}

// Delete 删除指定键
func (s *DefaultStorage) Delete(key string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		return true
	}
	return false
}

// Has 检查键是否存在
func (s *DefaultStorage) Has(key string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	_, ok := s.data[key]
	return ok
}

// Clear 清空所有键值对
func (s *DefaultStorage) Clear() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data = make(map[string]interface{})
}

// Keys 返回所有键
func (s *DefaultStorage) Keys() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Len 返回键值对数量
func (s *DefaultStorage) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.data)
}

// Range 遍历所有键值对
func (s *DefaultStorage) Range(f func(key string, value interface{}) bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for k, v := range s.data {
		if !f(k, v) {
			break
		}
	}
}
