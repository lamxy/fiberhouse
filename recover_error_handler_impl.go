// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/jsonconvert"
	frameUtils "github.com/lamxy/fiberhouse/utils"

	"github.com/lamxy/fiberhouse/exception"

	"github.com/gofiber/fiber/v2"
)

var (
	debugFlag      = "X-your-custom-debug-flag"             // 自定义debug标记key，由后端recover配置定义覆盖
	debugFlagValue = "f0dc4970-ed31-4598-acd8-b5c5fd66c12e" // 自定义debug标记值，由后端recover配置定义覆盖
	requestID      = "traceId"                              // 请求ID字段名称，由后端trace配置定义覆盖
)

// ErrorStack 获取当前的堆栈信息字符串
func ErrorStack(debugStack ...bool) string {
	//if len(debugStack) > 0 && debugStack[0] {
	//	return frameUtils.StackMsg()
	//}
	return frameUtils.CaptureStack()
}

// GetJsonIndent 从堆栈字符串获取堆栈行并转换为JSON缩进格式字节切片
func GetJsonIndent(appCtx IApplicationContext, s string, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	if len(s) == 0 {
		return nil
	}
	lines := frameUtils.DebugStackLines(s)
	if len(lines) == 0 {
		return nil
	}
	j, err := json.MarshalIndent(lines, "", "  ")
	if err != nil {
		log.Warn(appCtx.GetConfig().LogOriginRecover()).Str(requestID, traceId).Err(err).Msg("getJson from stack lines error")
		return nil
	}
	return j
}

// ErrorHandler 错误处理器，提供错误处理和恢复中间件功能
type ErrorHandler struct {
	AppCtx         IApplicationContext
	recoverManager IProviderManager // 恢复中间件管理器
}

var (
	errorHandlerInstance *ErrorHandler
	errorHandlerOnce     sync.Once
)

// NewErrorHandler 创建错误处理器
func NewErrorHandler(ctx IApplicationContext) *ErrorHandler {
	eh := &ErrorHandler{
		AppCtx: ctx,
	}
	managers := ProviderLocationDefault().LocationAppMiddlewareInit.GetManagers()
	for _, m := range managers {
		if m.Type().GetTypeID() == ProviderTypeDefault().GroupRecoverMiddlewareChoose.GetTypeID() {
			eh.recoverManager = m
			break
		}
	}
	return eh
}

// NewErrorHandlerOnce 单例模式创建错误处理器
func NewErrorHandlerOnce(ctx IApplicationContext) *ErrorHandler {
	errorHandlerOnce.Do(func() {
		errorHandlerInstance = NewErrorHandler(ctx)
	})
	return errorHandlerInstance
}

// GetContext 获取应用框架上下文
func (r *ErrorHandler) GetContext() IApplicationContext {
	return r.AppCtx
}

// SetRecoverManager 设置恢复中间件管理器
func (r *ErrorHandler) SetRecoverManager(manager IProviderManager) {
	r.recoverManager = manager
}

// ErrRecoverMiddlewareUnavailable 标记恢复中间件装配失败。
//
// 三种失败：恢复中间件管理器未设置、提供者加载失败、加载结果未实现 IRecover。
// 三者都发生在中间件注册期，调用方有条件改用其他 recovery 实现或降级。
var ErrRecoverMiddlewareUnavailable = errors.New("fiberhouse: recover middleware is unavailable")

// 两处失败的 panic 文本。抽成常量是为了让 RecoverMiddleware 委派回 panic 时
// 能还原出与旧实现逐字一致的值——既有调用方（含测试）依赖具体的 panic 值。
const (
	msgRecoverManagerNotSet = "Recovery: recover manager is not set"
	msgRecoverNotIRecover   = "Recovery: loaded recover provider does not implement IRecover"
)

// RecoverMiddleware 返回恢复中间件函数，根据核心类型返回对应的中间件
// 通过恢复中间件管理器依据启动配置选择相应的提供者自动返回对应的恢复中间件
// 装配失败时 panic。需要处理该错误的调用方请改用 RecoverMiddlewareWithError。
//
// panic 值与文本保持不变：调用方（含既有测试）可能依赖具体的 panic 值，
// 因此这里不统一包装成 error 再 panic，而是原样抛出各自的值。
func (r *ErrorHandler) RecoverMiddleware(config ...RecoverConfig) any {
	middleware, err := r.RecoverMiddlewareWithError(config...)
	if err != nil {
		panic(recoverPanicValue(err))
	}
	return middleware
}

// RecoverMiddlewareWithError 与 RecoverMiddleware 相同，但装配失败时返回错误
// 而非 panic。
//
// 三种失败都是装配期的错误：恢复中间件管理器未设置（应用没有装配 recovery
// provider 集合）、提供者加载失败、加载结果未实现 IRecover。三者都发生在
// 中间件注册期，调用方有条件改用其他 recovery 实现或降级。
func (r *ErrorHandler) RecoverMiddlewareWithError(config ...RecoverConfig) (any, error) {
	// 如果管理器未设置，尝试从位置点获取
	if r.recoverManager == nil {
		r.GetContext().GetLogger().ErrorWith(r.GetContext().GetConfig().LogOriginRecover()).
			Msg(msgRecoverManagerNotSet)
		return nil, fmt.Errorf("%w: %s", ErrRecoverMiddlewareUnavailable, msgRecoverManagerNotSet)
	}

	// 通过管理器加载对应核心类型的恢复中间件提供者
	recovery, err := r.recoverManager.LoadProvider()

	if err != nil {
		r.AppCtx.GetLogger().Error(r.AppCtx.GetConfig().LogOriginRecover()).Err(err).
			Msg("Recovery: failed to load recover provider")
		return nil, fmt.Errorf("%w: load recover provider: %w", ErrRecoverMiddlewareUnavailable, err)
	}
	if recoverMiddleware, ok := recovery.(IRecover); ok {
		return recoverMiddleware.RecoverPanic(config...), nil
	}
	r.AppCtx.GetLogger().Error(r.AppCtx.GetConfig().LogOriginRecover()).Msg(msgRecoverNotIRecover)
	return nil, fmt.Errorf("%w: %s", ErrRecoverMiddlewareUnavailable, msgRecoverNotIRecover)
}

// recoverPanicValue 还原各失败分支原本的 panic 值。
//
// 旧实现三处 panic 的值各不相同：两处是字符串常量，一处是被包装错误的
// Error() 文本。调用方可能依赖这些具体值（既有测试正是如此），因此委派回
// panic 时必须逐一还原，而不是统一抛出包装后的 error。
func recoverPanicValue(err error) any {
	switch {
	case strings.Contains(err.Error(), msgRecoverManagerNotSet):
		return msgRecoverManagerNotSet
	case strings.Contains(err.Error(), msgRecoverNotIRecover):
		return msgRecoverNotIRecover
	default:
		// 提供者加载失败：旧实现 panic 的是底层错误的文本。
		return errors.Unwrap(err).Error()
	}
}

// DefaultStackTraceHandler 记录请求上下文信息 + panic信息 + 堆栈信息
func (r *ErrorHandler) DefaultStackTraceHandler(ctx adaptorctx.ICoreContext, e interface{}) {
	// 从配置文件获取调试相关参数和请求ID参数的配置值
	cfg := r.GetContext().GetConfig()
	// 配置中的 recover 恢复 error 错误的配置段
	recoverConfig := cfg.GetRecover()
	// 配置中 trace 请求ID 配置段
	traceConfig := cfg.GetTrace()

	// 调试标记和调试标记值
	debugFlag = recoverConfig.DebugFlag
	debugFlagValue = recoverConfig.DebugFlagValue
	// request id
	requestID = traceConfig.RequestID
	// 是否打印堆栈信息
	enablePrintStack := recoverConfig.EnablePrintStack
	// 是否启动 debug 调试标记
	enableDebugFlag := recoverConfig.EnableDebugFlag
	// 是否开启了调试模式
	debugMode := recoverConfig.DebugMode

	// 日志器
	logger := r.GetContext().GetLogger()

	var (
		recovery IRecover
		ok       bool
	)
	lp, err := r.recoverManager.LoadProvider()
	if err != nil {
		logger.Error(cfg.LogOriginRecover()).Err(err).Msg("DefaultStackTraceHandler load logger provider from recover manager failed")
		return
	}
	if lp == nil {
		logger.Error(cfg.LogOriginRecover()).Msg("DefaultStackTraceHandler: recover provider is nil")
		return
	}
	if recovery, ok = lp.(IRecover); !ok {
		logger.Error(cfg.LogOriginRecover()).Msg("DefaultStackTraceHandler logger provider from recover manager type assert to IRecover failed")
		return
	}

	// http头部debug标记
	debugFlagFromHeader := recovery.GetHeader(ctx, debugFlag)
	// 请求requestId
	traceId := recovery.GetHeader(ctx, requestID)

	var jsonEnCoder func(interface{}) ([]byte, error)
	// json编码器
	jsonEnc, errJec := r.GetContext().GetContainer().Get(r.GetContext().GetStarter().GetApplication().GetFastTrafficCodecKey())
	if errJec != nil {
		logger.Error(cfg.LogOriginRecover()).Str(requestID, traceId).Err(errJec).Msg("GetFastTrafficCodecKey get json encoder from container failed")
		return
	} else {
		if jsonTmp, ok := jsonEnc.(JsonWrapper); ok {
			jsonEnCoder = jsonTmp.Marshal
		} else {
			logger.Error(cfg.LogOriginRecover()).Str(requestID, traceId).Msg("GetFastTrafficCodecKey json encoder from container type assert to JsonWrapper failed")
			return
		}
	}

	var (
		linesJson []byte
		logEvent  = logger.Error(cfg.LogOriginRecover()).Str(requestID, traceId)
	)

	// 记录reqParams、reqQueries、reqBody、reqHeaders
	var (
		reqParamsJson  = recovery.GetParamsJson(ctx, logger, jsonEnCoder, traceId)
		reqQueriesJson = recovery.GetQueriesJson(ctx, logger, jsonEnCoder, traceId)
		reqHeadersJson = recovery.GetHeadersJson(ctx, logger, jsonEnCoder, traceId)
		//reqBodyJson, reqBodyStr = recovery.getBodyJson(c)
	)

	// 非 error 的 panic 值（如 panic("...")）没有对应分支，此前会被静默丢弃、不留日志；
	// 统一转成 error 走 error 分支记录。
	switch e.(type) {
	case *exception.ValidateException, *exception.Exception, fiber.Error, *fiber.Error, error:
	default:
		e = fmt.Errorf("panic: %v", e)
	}

	switch err := e.(type) {
	case *exception.ValidateException:
		dw := jsonconvert.NewDataWrap(err.Data)

		if debugMode || enablePrintStack || (enableDebugFlag && debugFlagFromHeader == debugFlagValue) {
			// 输出堆栈信息
			msg := ErrorStack()

			if dw.CanJSONSerializable() {
				data, errJson := dw.GetJson(jsonEnCoder)
				if errJson != nil {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", "").Str("DataWrap-GetJson-error", errJson.Error()).Str("PrintStack", "true")
				} else {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).RawJSON("Data", data).Str("PrintStack", "true")
				}
			} else {
				logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", dw.GetString()).Str("PrintStack", "true")
			}

			if len(reqParamsJson) > 0 {
				logEvent.RawJSON("reqParams", reqParamsJson)
			}
			if len(reqQueriesJson) > 0 {
				logEvent.RawJSON("reqQueries", reqQueriesJson)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			// debug模式，增加DebugStackLines字段输出格式化的堆栈信息，方便开发环境下直接阅读
			if debugMode {
				linesJson = GetJsonIndent(r.GetContext(), msg, logger, jsonEnCoder, traceId)
				if linesJson != nil {
					logEvent.RawJSON("DebugStackLines", linesJson)
				}
			}
			logEvent.Msg(msg)
		} else {
			if dw.CanJSONSerializable() {
				data, errJson := dw.GetJson(jsonEnCoder)
				if errJson != nil {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", "").Str("DataWrap-GetJson-error", errJson.Error()).Msg(err.Error())
				} else {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).RawJSON("Data", data).Msg(err.Error())
				}
			} else {
				logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", dw.GetString()).Msg(err.Error())
			}
		}
		dw.Release()
	case *exception.Exception:
		dw := jsonconvert.NewDataWrap(err.Data)

		if debugMode || enablePrintStack || (enableDebugFlag && debugFlagFromHeader == debugFlagValue) {
			// 输出堆栈信息
			msg := ErrorStack()

			if dw.CanJSONSerializable() {
				data, errJson := dw.GetJson(jsonEnCoder)
				if errJson != nil {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", "").Str("DataWrap-GetJson-error", errJson.Error()).Str("PrintStack", "true")
				} else {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).RawJSON("Data", data).Str("PrintStack", "true")
				}
			} else {
				logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", dw.GetString()).Str("PrintStack", "true")
			}

			if len(reqParamsJson) > 0 {
				logEvent.RawJSON("reqParams", reqParamsJson)
			}
			if len(reqQueriesJson) > 0 {
				logEvent.RawJSON("reqQueries", reqQueriesJson)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			if debugMode {
				linesJson = GetJsonIndent(r.GetContext(), msg, logger, jsonEnCoder, traceId)
				if linesJson != nil {
					logEvent.RawJSON("DebugStackLines", linesJson)
				}
			}
			logEvent.Msg(msg)
		} else {
			if dw.CanJSONSerializable() {
				data, errJson := dw.GetJson(jsonEnCoder)
				if errJson != nil {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", "").Str("DataWrap-GetJson-error", errJson.Error()).Msg(err.Error())
				} else {
					logEvent.Int("Code", err.Code).Str("Msg", err.Msg).RawJSON("Data", data).Msg(err.Error())
				}
			} else {
				logEvent.Int("Code", err.Code).Str("Msg", err.Msg).Str("Data", dw.GetString()).Msg(err.Error())
			}
		}
		dw.Release()
	case fiber.Error, *fiber.Error:
		var fiberErr *fiber.Error
		switch typed := err.(type) {
		case fiber.Error:
			fiberErr = &typed
		case *fiber.Error:
			fiberErr = typed
		}
		if fiberErr == nil {
			logEvent.Str("Msg", "typed-nil *fiber.Error").Msg("typed-nil *fiber.Error")
			break
		}
		code := fiber.StatusInternalServerError
		if fiberErr.Code == 0 {
			fiberErr.Code = code
		}
		if debugMode || enablePrintStack || (enableDebugFlag && debugFlagFromHeader == debugFlagValue) { // 输出堆栈信息
			msg := ErrorStack()

			logEvent.Int("Code", fiberErr.Code).Str("Msg", fiberErr.Error()).Str("PrintStack", "true")

			if len(reqParamsJson) > 0 {
				logEvent.RawJSON("reqParams", reqParamsJson)
			}
			if len(reqQueriesJson) > 0 {
				logEvent.RawJSON("reqQueries", reqQueriesJson)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			if debugMode {
				linesJson = GetJsonIndent(r.GetContext(), msg, logger, jsonEnCoder, traceId)
				if linesJson != nil {
					logEvent.RawJSON("DebugStackLines", linesJson)
				}
			}
			logEvent.Msg(msg)
		} else {
			logEvent.Int("Code", fiberErr.Code).Msg(fiberErr.Error())
		}
	case error:
		if debugMode || enablePrintStack || (enableDebugFlag && debugFlagFromHeader == debugFlagValue) { // 输出堆栈信息
			msg := ErrorStack()

			logEvent.Str("Msg", err.Error()).Str("PrintStack", "true")

			if len(reqParamsJson) > 0 {
				logEvent.RawJSON("reqParams", reqParamsJson)
			}
			if len(reqQueriesJson) > 0 {
				logEvent.RawJSON("reqQueries", reqQueriesJson)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			if debugMode {
				linesJson = GetJsonIndent(r.GetContext(), msg, logger, jsonEnCoder, traceId)
				if linesJson != nil {
					logEvent.RawJSON("DebugStackLines", linesJson)
				}
			}
			logEvent.Msg(msg)
		} else {
			logEvent.Msg(err.Error())
		}
	}
}

// ErrorHandler 处理错误并返回对应的HTTP响应
func (r *ErrorHandler) ErrorHandler(ctx adaptorctx.ICoreContext, err error) error {
	// 记录日志 & 堆栈
	r.DefaultStackTraceHandler(ctx, err)
	if code, message, ok := fiberHTTPError(err); ok {
		return Response().Reset(code, message, nil).SendWithCtx(ctx, code)
	}

	// ValidateException
	var (
		debugMode = r.GetContext().GetConfig().GetRecover().DebugMode
		eve       = ValidateException()
	)
	okVe := errors.As(err, &eve)
	if okVe {
		// 验证器错误，响应完整错误信息到客户端
		//return eve.RespData().JsonWithCtx(ctx, http.StatusBadRequest)
		return Response().From(eve, true).SendWithCtx(ctx, http.StatusBadRequest)
	}
	// Exception
	var ee = Exception()
	okEe := errors.As(err, &ee)
	if okEe {
		if debugMode {
			return Response().From(ee, true).SendWithCtx(ctx, http.StatusBadRequest)
		}
		return Response().From(ee.RespData(nil), true).SendWithCtx(ctx, http.StatusBadRequest)
	}
	// default
	if debugMode {
		return Response().From(exception.GetUnknownError().RespData(err.Error()), true).SendWithCtx(ctx, http.StatusInternalServerError)
	}
	return Response().From(exception.GetUnknownError(), true).SendWithCtx(ctx, http.StatusInternalServerError)
}

func fiberHTTPError(err error) (int, string, bool) {
	var code int
	var message string
	switch typed := err.(type) {
	case *fiber.Error:
		if typed == nil {
			return 0, "", false
		}
		code, message = typed.Code, typed.Message
	default:
		var fiberErr *fiber.Error
		if !errors.As(err, &fiberErr) || fiberErr == nil {
			return 0, "", false
		}
		code, message = fiberErr.Code, fiberErr.Message
	}
	if code == 0 {
		code = http.StatusInternalServerError
	}
	if message == "" {
		message = http.StatusText(code)
	}
	return code, message, true
}
