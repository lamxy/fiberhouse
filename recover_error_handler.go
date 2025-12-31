// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"github.com/lamxy/fiberhouse/component/jsonconvert"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
	"net/http"
	"sync"

	"github.com/lamxy/fiberhouse/exception"

	"github.com/gofiber/fiber/v2"
)

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

// RecoverMiddleware 返回恢复中间件函数，根据核心类型返回对应的中间件
// 通过恢复中间件管理器依据启动配置选择相应的提供者自动返回对应的恢复中间件
func (r *ErrorHandler) RecoverMiddleware(config ...Config) any {
	// 如果管理器未设置，尝试从位置点获取
	if r.recoverManager == nil {
		msg := "Recovery: recover manager is not set"
		r.GetContext().GetLogger().ErrorWith(r.GetContext().GetConfig().LogOriginRecover()).Msg(msg)
		panic(msg)
	}

	// 通过管理器加载对应核心类型的恢复中间件提供者
	recovery, err := r.recoverManager.LoadProvider()

	if err != nil {
		msg := "Recovery: failed to load recover provider"
		r.AppCtx.GetLogger().Error(r.AppCtx.GetConfig().LogOriginRecover()).Err(err).Msg(msg)
		panic(err.Error())
	}
	if recoverMiddleware, ok := recovery.(IRecover); ok {
		return recoverMiddleware.RecoverPanic(config...)
	}
	msg := "Recovery: loaded recover provider does not implement IRecover"
	r.AppCtx.GetLogger().Error(r.AppCtx.GetConfig().LogOriginRecover()).Msg(msg)
	panic(msg)
}

// DefaultStackTraceHandler 记录请求上下文信息 + panic信息 + 堆栈信息
func (r *ErrorHandler) DefaultStackTraceHandler(ctx providerctx.ICoreContext, e interface{}) {
	// 从配置文件获取调试相关参数和请求ID参数的配置值
	cfg := r.GetContext().GetConfig()
	recoverConfig := cfg.GetRecover()
	traceConfig := cfg.GetTrace()

	debugFlag = recoverConfig.DebugFlag
	debugFlagValue = recoverConfig.DebugFlagValue
	requestID = traceConfig.RequestID
	enablePrintStack := recoverConfig.EnablePrintStack
	enableDebugFlag := recoverConfig.EnableDebugFlag
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
	if lp != nil {
		if recovery, ok = lp.(IRecover); !ok {
			logger.Error(cfg.LogOriginRecover()).Msg("DefaultStackTraceHandler logger provider from recover manager type assert to IRecover failed")
			return
		}
	}

	// 头部debug标记
	debugFlagFromHeader := recovery.GetHeader(ctx, debugFlag)
	// 请求requestId
	traceId := recovery.GetHeader(ctx, requestID)

	var jsonEnCoder func(interface{}) ([]byte, error)
	// json编码器
	jsonEnc, errJec := r.GetContext().GetContainer().Get(r.GetContext().GetStarter().GetApplication().GetFastJsonCodecKey())
	if errJec != nil {
		logger.Error(cfg.LogOriginRecover()).Str(requestID, traceId).Err(errJec).Msg("GetFastJsonCodecKey get json encoder from container failed")
		return
	} else {
		if jsonTmp, ok := jsonEnc.(JsonWrapper); ok {
			jsonEnCoder = jsonTmp.Marshal
		} else {
			logger.Error(cfg.LogOriginRecover()).Str(requestID, traceId).Msg("GetFastJsonCodecKey json encoder from container type assert to JsonWrapper failed")
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
	case fiber.Error:
		code := fiber.StatusInternalServerError
		if err.Code == 0 {
			err.Code = code
		}
		if debugMode || enablePrintStack || (enableDebugFlag && debugFlagFromHeader == debugFlagValue) { // 输出堆栈信息
			msg := ErrorStack()

			logEvent.Int("Code", err.Code).Str("Msg", err.Error()).Str("PrintStack", "true")

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
			logEvent.Int("Code", err.Code).Msg(err.Error())
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

// ErrorHandler 用于fiber.New配置全局错误处理器，处理业务级错误
func (r *ErrorHandler) ErrorHandler(ctx providerctx.ICoreContext, err error) error {
	// 记录日志 & 堆栈
	r.DefaultStackTraceHandler(ctx, err)

	// ValidateException
	var (
		debugMode = r.GetContext().GetConfig().GetRecover().DebugMode
		eve       *exception.ValidateException
	)
	okVe := errors.As(err, &eve)
	if okVe {
		// 验证器错误，响应完整错误信息到客户端
		return eve.RespError().JsonWithCtx(ctx, http.StatusBadRequest)
	}
	// Exception
	var ee *exception.Exception
	okEe := errors.As(err, &ee)
	if okEe {
		if debugMode {
			return ee.RespError().JsonWithCtx(ctx, http.StatusBadRequest)
		}
		return ee.RespError(nil).JsonWithCtx(ctx, http.StatusBadRequest)
	}
	// default
	if debugMode {
		return exception.GetUnknownError().RespError(err.Error()).JsonWithCtx(ctx, http.StatusInternalServerError)
	}
	return exception.GetUnknownError().JsonWithCtx(ctx, http.StatusInternalServerError)
}
