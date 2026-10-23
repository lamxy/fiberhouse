// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package bootstrap 提供应用程序启动时的核心初始化功能，包括应用配置管理和日志系统的初始化。
package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/component/logging/writer"
	"github.com/lamxy/fiberhouse/constant"
	frameUtils "github.com/lamxy/fiberhouse/utils"
	"github.com/rs/zerolog"
)

const (
	defaultEnv      = "dev" // test、dev、prod
	envPrefix       = "APP_ENV_"
	envConfigPrefix = "APP_CONF_"
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
	// closeOnce / closeErr 保证 wCloser 只被关闭一次，并让后续调用复用首次结果。
	//
	// 关闭入口有多个（Fiber 的 clearApplicationGlobalsOnce、Gin 的 Shutdown
	// 尾部、CLI 的 releaseCommandGlobals、应用自行调用），守卫必须在这一层，
	// 契约才对任意注入的 io.WriteCloser 成立。
	closeOnce sync.Once
	closeErr  error
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
//
// 幂等：底层 wCloser 只被关闭一次，后续调用返回首次的结果。
//
// 这不是锦上添花。日志器的关闭有多个入口——Fiber 的 clearApplicationGlobalsOnce、
// Gin 的 Shutdown 尾部、CLI 的 releaseCommandGlobals、以及应用自行调用——此前
// 本方法直接转发给 wCloser，重复关闭是否安全完全取决于注入的 writer。框架自带的
// 两个异步 writer 恰好以 CAS + closeDone 实现了幂等（async_channel_writer.go:146、
// async_diode_writer.go:134），但那是它们的实现细节，不是 LoggerWrapper 的契约；
// 使用方注入的任意 io.WriteCloser 没有义务容忍重复 Close。
//
// 守卫放在这一层，契约才对所有注入的 writer 成立。
func (lw *LoggerWrap) Close() error {
	lw.closeOnce.Do(func() {
		if lw.wCloser != nil {
			lw.closeErr = lw.wCloser.Close()
		}
	})
	return lw.closeErr
}

// Debug 和 DebugWith 方法用于记录调试级别的日志
func (lw *LoggerWrap) Debug(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Debug().Str("Origin", origin[0].String()).Caller()
	}
	return lw.logger.Debug().Caller()
}

// DebugWith 方法用于记录调试级别的日志，并指定日志来源
func (lw *LoggerWrap) DebugWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Debug().Str("Origin", origin.String()).Caller()
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
		return lw.logger.Error().Str("Origin", origin[0].String()).Caller()
	}
	return lw.logger.Error().Caller()
}

// ErrorWith 方法用于记录错误级别的日志，并指定日志来源
func (lw *LoggerWrap) ErrorWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Error().Str("Origin", origin.String()).Caller()
}

// Err 方法用于记录错误事件，通常用于捕获异常或错误信息
func (lw *LoggerWrap) Err(err error) *zerolog.Event {
	return lw.logger.Err(err).Caller()
}

// Fatal 和 FatalWith 方法用于记录致命级别的日志
func (lw *LoggerWrap) Fatal(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Fatal().Str("Origin", origin[0].String()).Caller()
	}
	return lw.logger.Fatal().Caller()
}

// FatalWith 方法用来记录致命级别的日志，并指定日志来源
func (lw *LoggerWrap) FatalWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Fatal().Str("Origin", origin.String()).Caller()
}

// Panic 和 PanicWith 方法用于记录恐慌级别的日志
func (lw *LoggerWrap) Panic(origin ...appconfig.LogOrigin) *zerolog.Event {
	if len(origin) > 0 {
		return lw.logger.Panic().Str("Origin", origin[0].String()).Caller()
	}
	return lw.logger.Panic().Caller()
}

// PanicWith 方法用于记录恐慌级别的日志，并指定日志来源
func (lw *LoggerWrap) PanicWith(origin appconfig.LogOrigin) *zerolog.Event {
	return lw.logger.Panic().Str("Origin", origin.String()).Caller()
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
				recordConfigLoadFailure(fmt.Errorf("LoadEnv: %w", err))
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
				recordConfigLoadFailure(fmt.Errorf("LoadEnvConf: %w", err))
			}
			return aConf
		})

		// 执行必要地初始化并返回最终的应用配置实例
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

// ErrConfigBootstrapFailed 标记引导期配置加载失败。
//
// 环境变量加载失败发生在 cfgOnce 的初始化闭包内，且外层 LoadFunc 回调只返回
// IAppConfig、没有 error 通道，因此失败只能先记录、再由 NewConfigOnceWithError
// 取回。
var ErrConfigBootstrapFailed = errors.New("fiberhouse/bootstrap: config bootstrap failed")

// configBootstrapErr 保存引导期配置加载失败的原因。
//
// 与 AppConfigured 一同被 cfgOnce 守卫：闭包只执行一次，失败之后的调用不会
// 重跑，若不记录错误，后续调用便无从得知配置是带伤加载的。
var configBootstrapErr error

// recordConfigLoadFailure 记录引导期配置加载失败。
//
// 只记录首个失败：后续失败往往是首个失败的连锁反应，覆盖它反而丢掉根因。
func recordConfigLoadFailure(err error) {
	if err == nil || configBootstrapErr != nil {
		return
	}
	configBootstrapErr = fmt.Errorf("%w: %w", ErrConfigBootstrapFailed, err)
}

// NewConfigOnceWithError 与 NewConfigOnce 相同，但引导期配置加载失败时一并返回
// 错误。
//
// 环境变量加载失败此前直接 panic，调用方无从选择。现在改为记录并在此返回：
// 环境变量解析失败通常意味着某个 APP_ENV_ / APP_CONF_ 变量格式有误，应用往往
// 仍能以文件配置继续启动，因此这类失败应当可被处理而不是终止进程。
//
// 返回的配置实例始终可用：失败只影响那一层环境变量覆盖，其余配置照常加载。
func NewConfigOnceWithError(path ...string) (appconfig.IAppConfig, error) {
	cfg := NewConfigOnce(path...)
	return cfg, configBootstrapErr
}

// ErrAsyncWriterUnavailable 标记异步日志 writer 不可用。
//
// 两种装配失败：配置指定的 writer 类型未注册（配置键写错），或注册的实例不是
// io.WriteCloser（容器 key 冲突）。
var ErrAsyncWriterUnavailable = errors.New("fiberhouse/bootstrap: async log writer is unavailable")

// NewWriterAsync 异步写日志
//
// 装配失败时 panic。需要处理该错误的调用方请改用 NewWriterAsyncWithError。
func NewWriterAsync(cfg appconfig.IAppConfig, filename string) io.WriteCloser {
	logWriter, err := NewWriterAsyncWithError(cfg, filename)
	if err != nil {
		panic(err)
	}
	return logWriter
}

// NewWriterAsyncWithError 与 NewWriterAsync 相同，但装配失败时返回错误而非 panic。
//
// 日志 writer 装配失败是可恢复的：调用方可以退回同步 writer 或 console，
// 让应用带着降级的日志继续启动，而不是在启动期直接终止进程。
func NewWriterAsyncWithError(cfg appconfig.IAppConfig, filename string) (io.WriteCloser, error) {
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
		return nil, fmt.Errorf("%w: get async log writer '%s' from container: %w",
			ErrAsyncWriterUnavailable, writerType, err)
	}
	logWriter, ok := internalWriter.(io.WriteCloser)
	if !ok {
		return nil, fmt.Errorf("%w: async log writer '%s' is %T, not an io.WriteCloser",
			ErrAsyncWriterUnavailable, writerType, internalWriter)
	}
	return logWriter, nil
}
