// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package appconfig 提供应用配置管理功能，支持多格式配置文件加载、线程安全的配置读写操作和日志源管理。
package appconfig

import (
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	defaultDelimiter string = "."
	defaultBaseDir   string = "."
	defaultDir       string = "/config"
	defaultFile      string = "application.yml"
)

// LogOrigin 日志来源分类标识
type LogOrigin string

// String LogOrigin的字符串
func (lo LogOrigin) String() string {
	return string(lo)
}

// InstanceKey 获取预定义的日志源分类的子日志器在全局管理容器的key
func (lo LogOrigin) InstanceKey() string {
	return constant.LogOriginKeyPrefix + lo.String()
}

// IAppConfig 定义应用配置的接口，规范了应用配置的获取、设置、加载及相关管理方法。
// 该接口适用于应用启动阶段的配置初始化和运行时的只读访问，建议通过安全读写方法进行并发访问。
type IAppConfig interface {
	/**
	应用ID、名称和版本基础配置的Getter和Setter
	*/

	GetAppId() string          // 获取应用唯一ID
	SetAppId(id string)        // 设置应用唯一ID
	GetAppName() string        // 获取应用名称
	SetAppName(name string)    // 设置应用名称
	GetVersion() string        // 获取应用版本
	SetVersion(version string) // 设置应用版本

	/**
	获取应用基本配置项副本
	*/

	GetApplication() ConfApplicationBase // 获取应用基础配置副本
	GetRecover() ConfRecoverBase         // 获取异常恢复配置副本
	GetAppLog() ConfAppLogBase           // 获取日志配置副本
	GetTrace() ConfTraceBase             // 获取链路追踪配置副本

	// Initialize 初始化AppConfig显示配置属性的方法
	Initialize() IAppConfig // 初始化配置属性

	// GetContainer 获取全局管理器
	GetContainer() *globalmanager.GlobalManager // 获取全局管理容器

	/**
	日志源管理，常见日志源分类
	*/

	LogOriginCoreHttp() LogOrigin // HTTP请求相关日志源
	LogOriginFrame() LogOrigin    // 框架自身日志源
	LogOriginRecover() LogOrigin  // 全局错误处理器日志源
	LogOriginWeb() LogOrigin      // Web业务日志源
	LogOriginCMD() LogOrigin      // 命令脚本日志源
	LogOriginTask() LogOrigin     // 异步任务日志源
	LogOriginCache() LogOrigin    // 缓存相关日志源
	LogOriginDatabase() LogOrigin // 数据库相关日志源
	LogOriginMq() LogOrigin       // MQ中间件日志源
	LogOriginMongodb() LogOrigin  // Mongodb日志源
	LogOriginMysql() LogOrigin    // Mysql日志源
	LogOriginTest() LogOrigin     // 测试相关日志源

	/**
	日志源管理
	*/

	RegisterLogOrigin(key string, customLogOrigin LogOrigin) error // 注册自定义日志源
	GetLogOrigin(key string) LogOrigin                             // 获取指定key的日志源
	LogOriginCustom(key string) LogOrigin                          // 获取自定义日志源
	GetLogOriginMap() map[string]LogOrigin                         // 获取日志源map

	// GetMiddlewareSwitch 获取中间件开关
	GetMiddlewareSwitch(key string) bool // 获取指定中间件开关状态

	/**
	安全读写方法
	*/

	SafeGet(key string, getter func(key string, byConf IAppConfig) (interface{}, error)) (interface{}, error)     // 并发安全获取配置值
	SafeSet(key string, val interface{}, setter func(key string, val interface{}, byConf IAppConfig) error) error // 并发安全设置配置值

	/**
	配置加载方法
	*/

	LoadDefault(m map[string]interface{}) IAppConfig   // 加载默认Map配置，后加载覆盖先加载的配置；可用于测试时快速设定指定的配置项
	LoadYaml(filename ...string) IAppConfig            // 加载yaml配置文件，后加载覆盖先加载的配置
	LoadFunc(f func(IAppConfig) IAppConfig) IAppConfig // 通过回调自定义加载配置，后加载覆盖先加载的配置

	/**
	配置路径管理
	*/

	SetConfPath(path string) IAppConfig // 设置配置文件路径
	GetConfPath() string                // 获取配置文件路径

	// GetCore 获取底层核心配置对象，访问更多的配置信息
	GetCore() interface{}

	/**
	配置值获取，封装了底层核心配置对象的常用方法
	*/

	String(keyPath string, defVal ...string) string                   // 获取字符串配置
	Strings(keyPath string, defVal ...[]string) []string              // 获取字符串切片配置
	Int64(keyPath string, defVal ...int64) int64                      // 获取int64配置
	Int(keyPath string, defVal ...int) int                            // 获取int配置
	Float64(keyPath string, defVal ...float64) float64                // 获取float64配置
	Bool(keyPath string) bool                                         // 获取bool配置
	Duration(key string, defaultValue ...time.Duration) time.Duration // 获取时间间隔配置
	GetBytes(key string, defaultValue ...[]byte) []byte               // 获取字节切片配置
}

type ConfApplicationBase struct {
	// 应用唯一ID
	AppID string
	// 应用名称
	AppName string
	// 应用版本
	Version string
	// 配置路径
	ConfigPath string
}

type ConfAppLogBase struct {
	// 日志文件名
	Filename string
	// 日志级别
	Level string
	// 是否开启指标监控
	EnableMetrics bool
	// 是否开启告警hook
	EnableAlertHook bool
}

type ConfRecoverBase struct {
	// true，表示更详细的接口异常响应信息、且不受限制地打印日志堆栈
	DebugMode bool
	// true，默认启动打印堆栈，对于当前环境为非调式模式的生产环境时，false可关闭生产环境的日志堆栈打印，节省服务器资源
	EnablePrintStack bool

	// true，表示请求头部字段debugFlag生效，当等于debugFlagValue时，不论环境及enablePrintStack影响，都打印日志堆栈
	EnableDebugFlag bool
	// 调试标识key，http头部字段
	DebugFlag string
	// 调试标识value，与调试标识头部字段携带的值进行比较，相同将启动日志器打印详细堆栈
	DebugFlagValue string
}

type ConfTraceBase struct {
	// 请求唯一ID
	RequestID string
}

// GetCoreWithConfig 获取配置核心泛型对象
func GetCoreWithConfig[T any](config IAppConfig) (T, error) {
	var (
		zero T
		ok   bool
	)
	if zero, ok = config.GetCore().(T); ok {
		return zero, nil
	}
	return zero, errors.New("type assertion failure for core config")
}

// AppConfig 应用配置对象
// 注意：应用配置非并发安全，建议只读，仅在应用启动阶段按需可写，运行时阶段禁止任何直接写，以免引起并发数据竞争问题。有限使用安全读写方法。
type AppConfig struct {
	ko        *koanf.Koanf // 私有属性
	confPath  string
	container *globalmanager.GlobalManager
	lock      sync.RWMutex // 安全读写锁

	// 基本配置项
	applicationBase ConfApplicationBase
	appLogBase      ConfAppLogBase
	recoverBase     ConfRecoverBase
	traceBase       ConfTraceBase

	// 日志源分类map: 日志源名称 -> 日志源标识
	logOriginEnum map[string]LogOrigin
	// 中间件开关map
	middleware map[string]bool

	// 其他配置
	// ...
}

// NewAppConfig new app config
func NewAppConfig(delim ...string) *AppConfig {
	var (
		d string
		p = defaultBaseDir + defaultDir
	)
	if len(delim) > 0 {
		d = delim[0]
	} else {
		d = defaultDelimiter
	}
	ko := koanf.New(d)

	return &AppConfig{
		ko:        ko,
		confPath:  p,
		container: globalmanager.NewGlobalManagerOnce(),
		lock:      sync.RWMutex{},
		logOriginEnum: map[string]LogOrigin{
			"coreHttp": "FiberHttpLog",
			"frame":    "Frame",
			"recover":  "Recover",
			"web":      "Web",
			"cmd":      "CMD",
			"task":     "Task",
			"cache":    "Cache",
			"database": "Database",
			"mq":       "MqMiddleware",
			"mongodb":  "Mongodb",
			"mysql":    "Mysql",
			"test":     "Test",
		},
		middleware: map[string]bool{
			"coreHttp": false,
		},
	}
}

// GetApplication 获取应用基础配置副本
func (ac *AppConfig) GetApplication() ConfApplicationBase {
	return ac.applicationBase // 副本
}

// GetRecover 获取应用异常恢复配置副本
func (ac *AppConfig) GetRecover() ConfRecoverBase {
	return ac.recoverBase
}

// GetAppLog 获取应用日志器配置副本
func (ac *AppConfig) GetAppLog() ConfAppLogBase {
	return ac.appLogBase
}

// GetTrace 获取应用链路配置副本
func (ac *AppConfig) GetTrace() ConfTraceBase {
	return ac.traceBase
}

// GetAppId 应用ID
func (ac *AppConfig) GetAppId() string {
	return ac.applicationBase.AppID
}

// SetAppId 设置应用ID
func (ac *AppConfig) SetAppId(id string) {
	_ = ac.SafeSet("", id, func(confPath string, value interface{}, c IAppConfig) error {
		if v, ok := value.(string); ok {
			ac.applicationBase.AppID = v
			return nil
		}
		return fmt.Errorf("SetAppId: type assertion failure for string")
	})
}

// GetAppName 应用名称
func (ac *AppConfig) GetAppName() string {
	return ac.applicationBase.AppName
}

// SetAppName 设置应用名称
func (ac *AppConfig) SetAppName(name string) {
	_ = ac.SafeSet("", name, func(confPath string, value interface{}, c IAppConfig) error {
		if v, ok := value.(string); ok {
			ac.applicationBase.AppName = v
			return nil
		}
		return fmt.Errorf("SetAppName: type assertion failure for string")
	})
}

// GetVersion 应用版本
func (ac *AppConfig) GetVersion() string {
	return ac.applicationBase.Version
}

// SetVersion 设置应用版本
func (ac *AppConfig) SetVersion(version string) {
	_ = ac.SafeSet("", version, func(confPath string, value interface{}, c IAppConfig) error {
		if v, ok := value.(string); ok {
			ac.applicationBase.Version = v
			return nil
		}
		return fmt.Errorf("SetVersion: type assertion failure for string")
	})
}

// Initialize 初始化配置属性参数
func (ac *AppConfig) Initialize() IAppConfig {
	// 初始化基础配置结构体
	ac.applicationBase = ConfApplicationBase{
		AppID:      ac.String("application.appId", ""),
		AppName:    ac.String("application.appName", "XXX APP"),
		Version:    ac.String("application.version", "v0.0.1"),
		ConfigPath: ac.confPath,
	}
	ac.appLogBase = ConfAppLogBase{
		Filename:        ac.String("application.appLog.filename", ""),
		Level:           ac.String("application.appLog.level", ""),
		EnableMetrics:   ac.Bool("application.appLog.enableMetrics"),
		EnableAlertHook: ac.Bool("application.appLog.enableAlertHook"),
	}
	ac.recoverBase = ConfRecoverBase{
		DebugMode:        ac.Bool("application.recover.debugMode"),
		EnablePrintStack: ac.Bool("application.recover.enablePrintStack"),
		EnableDebugFlag:  ac.Bool("application.recover.enableDebugFlag"),
		DebugFlag:        ac.String("application.recover.debugFlag", ""),
		DebugFlagValue:   ac.String("application.recover.debugFlagValue", ""),
	}
	ac.traceBase = ConfTraceBase{
		RequestID: ac.String("application.trace.requestID", "requestId"),
	}

	// 初始化日志来源分类配置
	logOriginMap := ac.GetKOANF().StringMap("application.appLog.logOriginEnum")
	for k, v := range logOriginMap {
		if v != "" {
			ac.logOriginEnum[k] = LogOrigin(v)
		}
	}

	// 初始化核心应用(*fiber.App)中间件开关
	middlewareMap := ac.GetKOANF().BoolMap("application.middleware")
	for k, v := range middlewareMap {
		ac.middleware[k] = v
	}

	// 初始化其他配置...

	return ac
}

// GetContainer 获取全局管理容器
func (ac *AppConfig) GetContainer() *globalmanager.GlobalManager {
	return ac.container
}

// LogOriginCoreHttp 返回底层 server 的 HTTP 请求相关日志源标识
func (ac *AppConfig) LogOriginCoreHttp() LogOrigin {
	return ac.GetLogOrigin("coreHttp")
}

// LogOriginFrame 返回框架自身相关日志源标识
func (ac *AppConfig) LogOriginFrame() LogOrigin {
	return ac.GetLogOrigin("frame")
}

// LogOriginRecover 返回应用全局错误处理器相关日志源标识
func (ac *AppConfig) LogOriginRecover() LogOrigin {
	return ac.GetLogOrigin("recover")
}

// LogOriginWeb 返回 Web 业务请求相关日志源标识
func (ac *AppConfig) LogOriginWeb() LogOrigin {
	return ac.GetLogOrigin("web")
}

// LogOriginCMD 返回 CMD 命令脚本相关日志源标识
func (ac *AppConfig) LogOriginCMD() LogOrigin {
	return ac.GetLogOrigin("cmd")
}

// LogOriginTask 返回异步任务相关日志源标识
func (ac *AppConfig) LogOriginTask() LogOrigin {
	return ac.GetLogOrigin("task")
}

// LogOriginCache 返回 Redis 相关日志源标识
func (ac *AppConfig) LogOriginCache() LogOrigin {
	return ac.GetLogOrigin("cache")
}

// LogOriginDatabase 返回 Database 相关日志源标识
func (ac *AppConfig) LogOriginDatabase() LogOrigin {
	return ac.GetLogOrigin("database")
}

// LogOriginMq 返回 MQ 中间件相关日志源标识
func (ac *AppConfig) LogOriginMq() LogOrigin {
	return ac.GetLogOrigin("mq")
}

// LogOriginMongodb 返回 Mongodb 相关日志源标识
func (ac *AppConfig) LogOriginMongodb() LogOrigin {
	return ac.GetLogOrigin("mongodb")
}

// LogOriginMysql 返回 Mysql 相关日志源标识
func (ac *AppConfig) LogOriginMysql() LogOrigin {
	return ac.GetLogOrigin("mysql")
}

// LogOriginTest 返回 Test 相关日志源标识
func (ac *AppConfig) LogOriginTest() LogOrigin {
	return ac.GetLogOrigin("test")
}

// RegisterLogOrigin 应用启动阶段，注册自定义日志源标识，非线程安全，且不会覆盖已有key。运行阶段需要设置，使用安全读写方法。
func (ac *AppConfig) RegisterLogOrigin(key string, customLogOrigin LogOrigin) error {
	if _, ok := ac.logOriginEnum[key]; !ok {
		ac.logOriginEnum[key] = customLogOrigin
	} else {
		return fmt.Errorf("LogOriginKey '%s' exists, duplicate settings are not allowed", key)
	}
	return nil
}

// GetLogOrigin 按key获取日志源标识
func (ac *AppConfig) GetLogOrigin(key string) LogOrigin {
	if v, ok := ac.logOriginEnum[key]; ok {
		return v
	}
	return ""
}

// LogOriginCustom 按日志源map的key获取日志源标识符
func (ac *AppConfig) LogOriginCustom(key string) LogOrigin {
	return ac.GetLogOrigin(key)
}

// GetLogOriginMap 获取日志器来源map
func (ac *AppConfig) GetLogOriginMap() map[string]LogOrigin {
	return ac.logOriginEnum
}

func (ac *AppConfig) GetMiddlewareSwitch(key string) bool {
	if v, ok := ac.middleware[key]; ok {
		return v
	}
	return false
}

// SafeGet 安全的获取配置值
func (ac *AppConfig) SafeGet(pathname string, getter func(pathname string, c IAppConfig) (interface{}, error)) (interface{}, error) {
	ac.lock.RLock()
	defer ac.lock.RUnlock()
	valIFace, err := getter(pathname, ac)
	return valIFace, err
}

// SafeSet 安全的设置配置值
func (ac *AppConfig) SafeSet(pathname string, value interface{}, setter func(pathname string, value interface{}, c IAppConfig) error) error {
	ac.lock.Lock()
	defer ac.lock.Unlock()
	err := setter(pathname, value, ac)
	return err
}

// LoadDefault 从map加载默认配置
func (ac *AppConfig) LoadDefault(m map[string]interface{}) IAppConfig {
	if err := ac.GetKOANF().Load(confmap.Provider(m, "."), nil); err != nil {
		panic("LoadMap: " + err.Error())
	}
	return ac
}

// LoadYaml 装载yaml文件配置
func (ac *AppConfig) LoadYaml(filename ...string) IAppConfig {
	var (
		baseDir = strings.TrimRight(ac.confPath, "/") + "/"
		fName   string
	)
	if len(filename) > 0 {
		fName = filename[0]
	} else {
		fName = defaultFile
	}
	fName = filepath.ToSlash(baseDir + strings.TrimLeft(fName, "/"))
	if err := ac.GetKOANF().Load(file.Provider(fName), yaml.Parser()); err != nil {
		panic(fmt.Sprintf("LoadYaml: %s, filename: %s", err.Error(), fName))
	}
	return ac
}

// LoadFunc 自定义回调装载配置
func (ac *AppConfig) LoadFunc(f func(config IAppConfig) IAppConfig) IAppConfig {
	return f(ac)
}

// SetConfPath 自定义配置文件目录
func (ac *AppConfig) SetConfPath(path string) IAppConfig {
	ac.confPath = strings.TrimRight(filepath.ToSlash(path), "/")
	return ac
}

// GetConfPath 获取当前配置文件目录
func (ac *AppConfig) GetConfPath() string {
	ac.confPath = filepath.ToSlash(ac.confPath)
	return strings.TrimRight(ac.confPath, "/")
}

// GetCore 获取底层配置对象
func (ac *AppConfig) GetCore() interface{} {
	return ac.GetKOANF()
}

// GetKOANF 获取底层配置对象
func (ac *AppConfig) GetKOANF() *koanf.Koanf {
	return ac.ko
}

// String 获取带默认值的字符串配置
func (ac *AppConfig) String(keyPath string, defVal ...string) string {
	v := ac.ko.String(keyPath)
	if v == "" && len(defVal) > 0 {
		return defVal[0]
	}
	return v
}

// Strings 获取带默认值的字符串切片配置
func (ac *AppConfig) Strings(keyPath string, defVal ...[]string) []string {
	ret := ac.GetKOANF().Strings(keyPath)
	if len(ret) == 0 && len(defVal) > 0 {
		return defVal[0]
	}
	return ret
}

// Int64 获取带默认值的int64配置
func (ac *AppConfig) Int64(keyPath string, defVal ...int64) int64 {
	v := ac.ko.Int64(keyPath)
	if v == 0 && len(defVal) > 0 {
		return defVal[0]
	}
	return v
}

// Int 获取带默认值的int配置
func (ac *AppConfig) Int(keyPath string, defVal ...int) int {
	v := ac.ko.Int(keyPath)
	if v == 0 && len(defVal) > 0 {
		return defVal[0]
	}
	return v
}

// Float64 获取带默认值的float64配置
func (ac *AppConfig) Float64(keyPath string, defVal ...float64) float64 {
	v := ac.ko.Float64(keyPath)
	if v == 0 && len(defVal) > 0 {
		return defVal[0]
	}
	return v
}

// Bool 获取bool配置
func (ac *AppConfig) Bool(keyPath string) bool {
	return ac.ko.Bool(keyPath)
}

// Duration 获取带默认值的时间间隔配置
func (ac *AppConfig) Duration(key string, defaultValue ...time.Duration) time.Duration {
	v := ac.ko.Duration(key)
	if v == 0 && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}

// GetBytes 获取带默认值的字节切片配置
func (ac *AppConfig) GetBytes(key string, defaultValue ...[]byte) []byte {
	v := ac.ko.Bytes(key)
	if len(v) == 0 && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}
