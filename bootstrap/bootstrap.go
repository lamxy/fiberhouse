// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package bootstrap 提供应用程序启动时的核心初始化功能，包括应用配置管理和日志系统的初始化。
package bootstrap

import (
	"fmt"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/component/writer"
	"github.com/lamxy/fiberhouse/constant"
	frameUtils "github.com/lamxy/fiberhouse/utils"
	"github.com/rs/zerolog"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	envAppTypeWeb        = "web"                                       // web应用
	envAppTypeCmd        = "cmd"                                       // 命令行应用
	defaultAppConfigFile = "application_" + envAppTypeWeb + "_dev.yml" // test、dev、prod
	defaultEnv           = "dev"
	envPrefix            = "APP_ENV_"
	envConfigPrefix      = "APP_CONF_"
)

var (
	// AppConfigured 全局应用配置
	AppConfigured appconfig.IAppConfig
	cfgOnce       sync.Once
	// Logger 全局日志器
	Logger  LoggerWrapper
	logOnce sync.Once
)

// LoggerWrapper 定义日志记录器接口，提供了多种日志记录方法
type LoggerWrapper interface {
	GetLevel() zerolog.Level
	GetZeroLogger() *zerolog.Logger
	Debug(origin ...appconfig.LogOrigin) *zerolog.Event
	DebugWith(origin appconfig.LogOrigin) *zerolog.Event
	Info(origin ...appconfig.LogOrigin) *zerolog.Event
	InfoWith(origin appconfig.LogOrigin) *zerolog.Event
	Warn(origin ...appconfig.LogOrigin) *zerolog.Event
	WarnWith(origin appconfig.LogOrigin) *zerolog.Event
	Error(origin ...appconfig.LogOrigin) *zerolog.Event
	ErrorWith(origin appconfig.LogOrigin) *zerolog.Event
	Fatal(origin ...appconfig.LogOrigin) *zerolog.Event
	FatalWith(origin appconfig.LogOrigin) *zerolog.Event
	Panic(origin ...appconfig.LogOrigin) *zerolog.Event
	PanicWith(origin appconfig.LogOrigin) *zerolog.Event
	Err(err error) *zerolog.Event
	With() zerolog.Context
	Close() error
}

// LoggerWrap 包装zerolog.Logger，提供日志记录方法
type LoggerWrap struct {
	logger  *zerolog.Logger
	wCloser io.WriteCloser // 用于回收底层writer
}

// NewLoggerWrap 创建一个新的LoggerWrap实例
func NewLoggerWrap(logger *zerolog.Logger, logCloser ...io.WriteCloser) LoggerWrapper {
	if len(logCloser) > 0 {
		return &LoggerWrap{
			logger:  logger,
			wCloser: logCloser[0],
		}
	}
	return &LoggerWrap{
		logger: logger,
	}
}

// GetLevel 返回当前日志级别
func (lw *LoggerWrap) GetLevel() zerolog.Level {
	return lw.logger.GetLevel()
}

// GetZeroLogger 返回底层的zerolog.Logger实例
func (lw *LoggerWrap) GetZeroLogger() *zerolog.Logger {
	return lw.logger
}

// Close 关闭日志写入器
func (lw *LoggerWrap) Close() error {
	if lw.wCloser != nil {
		return lw.wCloser.Close()
	}
	return nil
}

// Debug 和 DebugWith 方法用于记录调试级别的日志
func (lw *LoggerWrap) Debug(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Debug().Str("Origin", origin[0].String())
	}
	return lw.logger.Debug()
}

// DebugWith 方法用于记录调试级别的日志，并指定日志来源
func (lw *LoggerWrap) DebugWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Debug().Str("Origin", origin.String())
}

// Info 和 InfoWith 方法用于记录信息级别的日志
func (lw *LoggerWrap) Info(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Info().Str("Origin", origin[0].String())
	}
	return lw.logger.Info()
}

// InfoWith 方法用于记录信息级别的日志，并指定日志来源
func (lw *LoggerWrap) InfoWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Info().Str("Origin", origin.String())
}

// Warn 和 WarnWith 方法用于记录警告级别的日志
func (lw *LoggerWrap) Warn(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Warn().Str("Origin", origin[0].String())
	}
	return lw.logger.Warn()
}

// WarnWith 方法用于记录警告级别的日志，并指定日志来源
func (lw *LoggerWrap) WarnWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Warn().Str("Origin", origin.String())
}

// Error 和 ErrorWith 方法用于记录错误级别的日志
func (lw *LoggerWrap) Error(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Error().Str("Origin", origin[0].String())
	}
	return lw.logger.Error()
}

// ErrorWith 方法用于记录错误级别的日志，并指定日志来源
func (lw *LoggerWrap) ErrorWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Error().Str("Origin", origin.String())
}

// Err 方法用于记录错误事件，通常用于捕获异常或错误信息
func (lw *LoggerWrap) Err(err error) *zerolog.Event {
	return lw.logger.Err(err)
}

// Fatal 和 FatalWith 方法用于记录致命级别的日志
func (lw *LoggerWrap) Fatal(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Fatal().Str("Origin", origin[0].String())
	}
	return lw.logger.Fatal()
}

// FatalWith 方法用来记录致命级别的日志，并指定日志来源
func (lw *LoggerWrap) FatalWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Fatal().Str("Origin", origin.String())
}

// Panic 和 PanicWith 方法用于记录恐慌级别的日志
func (lw *LoggerWrap) Panic(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Panic().Str("Origin", origin[0].String())
	}
	return lw.logger.Panic()
}

// PanicWith 方法用于记录恐慌级别的日志，并指定日志来源
func (lw *LoggerWrap) PanicWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Panic().Str("Origin", origin.String())
}

// With 方法用于创建一个新的日志上下文，可以在此上下文中添加更多字段
func (lw *LoggerWrap) With() zerolog.Context {
	return lw.logger.With()
}

// NewConfigOnce 初始化并注册全局应用配置
// path 可选参数，指定配置文件的路径目录，默认为当前工作目录下的./config目录
func NewConfigOnce(path ...string) appconfig.IAppConfig {
	cfgOnce.Do(func() {
		// 创建应用配置实例
		aCfg := appconfig.NewAppConfig()
		if len(path) > 0 {
			aCfg.SetConfPath(path[0])
		}
		// 加载基础环境变量： 前缀 APP_ENV_ ； e.g. APP_ENV_application_env=prod ==> application.env = prod、 APP_ENV_application_appType=web ==> application.appType=web
		// 注意: APP_ENV_为大写，application_env跟yml文件配置的application.env对应，且必须大小写保持一致
		// 加载环境类型和应用类别：用于支持不同环境和应用类别的配置文件选择
		aCfg.LoadFunc(func(aConf appconfig.IAppConfig) appconfig.IAppConfig {
			err := aConf.GetCore().(*koanf.Koanf).Load(env.ProviderWithValue(envPrefix, ".", func(s string, v string) (string, interface{}) {
				// Strip out the MYVAR_ prefix and lowercase and get the key while also replacing
				// the _ character with . in the key (koanf delimeter).
				key := strings.Replace(strings.TrimPrefix(s, envPrefix), "_", ".", -1)
				// If there is a space in the value, split the value into a slice by the space.
				if strings.Contains(v, " ") {
					return key, strings.Split(v, " ")
				}
				// Otherwise, return the plain string.
				return key, v
			}), nil)
			if err != nil {
				panic("LoadEnv: " + err.Error())
			}
			return aConf
		})
		// 选择环境类型 prod dev test
		envSelect := aCfg.String("application.env", defaultEnv) // 默认 dev
		confFilename := "application_" + envSelect + ".yml"     // 比如： env=test ==> 配置文件名为 application_test.yml

		// 加载文件配置
		aCfg.LoadYaml(confFilename)

		// 回写选择的环境类型和应用类别到配置中
		aCfg.LoadDefault(map[string]interface{}{
			"application.env": envSelect,
		})

		// 加载配置参数环境变量： 前缀 APP_CONF_ ； e.g. APP_CONF_application_appName=xxx ==> application.appName = xxx
		// 注意: APP_CONF_ 为大写，application_appName跟yml文件配置的application.appName对应，且必须大小写保持一致
		// 加载环境配置：用于支持环境变量配置覆盖配置文件配置
		aCfg.LoadFunc(func(aConf appconfig.IAppConfig) appconfig.IAppConfig {
			err := aConf.GetCore().(*koanf.Koanf).Load(env.ProviderWithValue(envConfigPrefix, ".", func(s string, v string) (string, interface{}) {
				// Strip out the MYVAR_ prefix and lowercase and get the key while also replacing
				// the _ character with . in the key (koanf delimeter).
				key := strings.Replace(strings.TrimPrefix(s, envConfigPrefix), "_", ".", -1)
				// If there is a space in the value, split the value into a slice by the space.
				if strings.Contains(v, " ") {
					return key, strings.Split(v, " ")
				}
				// Otherwise, return the plain string.
				return key, v
			}), nil)
			if err != nil {
				panic("LoadEnvConf: " + err.Error())
			}
			return aConf
		})

		// 执行必要的初始化并返回最终的应用配置实例
		AppConfigured = aCfg.Initialize()
	})

	return AppConfigured
}

// NewLoggerOnce 初始化并注册全局日志对象
// path 可选参数，指定日志目录，默认为当前工作目录下的./logs目录
func NewLoggerOnce(cfg appconfig.IAppConfig, path ...string) LoggerWrapper {
	logOnce.Do(func() {
		var (
			writers      []io.Writer
			loggerWriter io.WriteCloser
		)
		if cfg.Bool("application.appLog.enableConsole") {
			consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
			if cfg.Bool("application.appLog.consoleJSON") {
				writers = append(writers, os.Stdout)
			} else {
				writers = append(writers, consoleWriter)
			}
		}
		if cfg.Bool("application.appLog.enableFile") {
			LogFilename := cfg.String("application.appLog.filename", "app.log")
			var logPath string
			if len(path) > 0 {
				logPath = strings.TrimRight(path[0], "/") + "/"
			} else {
				logPath = strings.TrimRight(frameUtils.GetWD(), "/") + "/logs/"
			}
			filename := filepath.ToSlash(logPath + LogFilename)

			if cfg.Bool("application.appLog.asyncConf.enable") {
				// 异步日志写入
				loggerWriter = NewWriterAsync(cfg, filename)
			} else {
				// 同步日志写入
				loggerWriter = NewWriterSync(cfg, filename)
			}
			writers = append(writers, loggerWriter)
		}
		// 设置全局日志级别
		level, err := zerolog.ParseLevel(strings.ToLower(cfg.String("application.appLog.level")))
		if err != nil {
			level = zerolog.TraceLevel
		}
		zerolog.SetGlobalLevel(level)
		// 设置全局日志时间格式
		zerolog.TimeFieldFormat = time.RFC3339Nano

		if len(writers) == 0 {
			// 默认输出到标准输出
			writers = append(writers, os.Stdout)
		}

		//zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

		// 创建日志记录器，使用多写入器支持同时输出到多个目标
		log := zerolog.New(io.MultiWriter(writers...)).With().Timestamp().Logger().Level(level)
		Logger = NewLoggerWrap(&log, loggerWriter)
	})
	return Logger
}

// NewWriterSync 同步写日志
func NewWriterSync(cfg appconfig.IAppConfig, filename string) io.WriteCloser {
	return writer.NewSyncLumberjackWriter(cfg, filename)
}

// NewWriterAsync 异步写日志
func NewWriterAsync(cfg appconfig.IAppConfig, filename string) io.WriteCloser {
	// 注册默认异步Writer写入器
	cfg.GetContainer().Register(constant.LogWriterKeyPrefix+"chan", func() (interface{}, error) {
		return writer.NewAsyncChannelWriter(cfg, filename), nil
	})
	cfg.GetContainer().Register(constant.LogWriterKeyPrefix+"diode", func() (interface{}, error) {
		return writer.NewAsyncDiodeWriter(cfg, filename), nil
	})

	writerType := cfg.String("application.appLog.asyncConf.type", "diode") // 默认为 diode

	// 获取配置的异步writer写入器实例
	internalWriter, err := cfg.GetContainer().Get(constant.LogWriterKeyPrefix + writerType)
	if err != nil {
		panic(fmt.Sprintf("Failed to get async logger: '%s' from container: %v", writerType, err))
	}
	logWriter, ok := internalWriter.(io.WriteCloser)
	if !ok {
		panic(fmt.Sprintf("Async logger writer type '%s' is not io.WriteCloser", writerType))
	}
	return logWriter
}
