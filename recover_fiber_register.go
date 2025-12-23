// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// recover 提供 Fiber 框架的全局异常恢复和错误处理中间件。
package fiberhouse

import (
	"encoding/json"
	"errors"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/jsonconvert"
	"github.com/lamxy/fiberhouse/constant"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
	"net/http"
	"runtime"
	"strings"

	"github.com/lamxy/fiberhouse/exception"

	frameUtils "github.com/lamxy/fiberhouse/utils"

	"github.com/gofiber/fiber/v2"
)

type RecoverCatch struct {
	AppCtx IApplicationContext
}

func NewRecoverCatch(ctx IApplicationContext) IRecover {
	return &RecoverCatch{
		AppCtx: ctx,
	}
}

func (r *RecoverCatch) GetContext() IApplicationContext {
	return r.AppCtx
}

// DefaultStackTraceHandler 记录请求上下文信息 + panic信息 + 堆栈信息
func (r *RecoverCatch) DefaultStackTraceHandler(ctx providerctx.ICoreContext, e interface{}) {
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

	// 原生上下文
	var (
		c  *fiber.Ctx
		ok bool
	)
	if c, ok = ctx.GetCtx().(*fiber.Ctx); !ok {
		panic("ContextProvider is not *fiber.Ctx")
	}

	// 头部debug标记
	debugFlagFromHeader := c.Get(debugFlag, "")
	// 请求requestId
	var traceId string
	if c.Locals(requestID) != nil {
		traceId = c.Locals(requestID).(string) // 请求类错误，从本地变量获取请求ID
	} else {
		traceId = "" // 非请求接口出现的错误，请求ID空值
	}
	var jsonEnCoder func(interface{}) ([]byte, error)
	// json编码器
	jsonEnc, errJec := r.GetContext().GetContainer().Get(r.GetContext().GetStarter().GetApplication().GetFastJsonCodecKey())
	if errJec != nil {
		logger.Warn(cfg.LogOriginRecover()).Str(requestID, traceId).Err(errJec).Msg("GetFastJsonCodecKey get json encoder from container failed")
		jsonEnCoder = c.App().Config().JSONEncoder
	} else {
		if jsonTmp, ok := jsonEnc.(JsonWrapper); ok {
			jsonEnCoder = jsonTmp.Marshal
		} else {
			jsonEnCoder = c.App().Config().JSONEncoder
		}
	}

	var (
		linesJson []byte
		logEvent  = logger.Error(cfg.LogOriginRecover()).Str(requestID, traceId)
	)

	switch err := e.(type) {
	case *exception.ValidateException:
		dw := jsonconvert.NewDataWrap(err.Data)

		if debugMode || enablePrintStack || (enableDebugFlag && debugFlagFromHeader == debugFlagValue) {
			// 输出堆栈信息
			msg := ErrorStack()

			// 记录reqParams、reqQueries、reqBody、reqHeaders
			var (
				reqParamsJson           = r.getParamsJson(c, logger, jsonEnCoder, traceId)
				reqQueriesJson          = r.getQueriesJson(c, logger, jsonEnCoder, traceId)
				reqBodyJson, reqBodyStr = r.getBodyJson(c)
				reqHeadersJson          = r.getHeadersJson(c, logger, jsonEnCoder, traceId)
			)

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
			if len(reqBodyJson) > 0 {
				logEvent.RawJSON("reqBody", reqBodyJson)
			} else if len(reqBodyStr) > 0 {
				logEvent.Str("reqBodyStr", reqBodyStr)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			// debug模式，增加DebugStackLines字段输出格式化的堆栈信息，方便开发环境下直接阅读
			if debugMode {
				linesJson = r.getJsonIndent(msg, logger, jsonEnCoder, traceId)
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

			// 记录reqParams、reqQueries、reqBody、reqHeaders
			var (
				reqParamsJson           = r.getParamsJson(c, logger, jsonEnCoder, traceId)
				reqQueriesJson          = r.getQueriesJson(c, logger, jsonEnCoder, traceId)
				reqBodyJson, reqBodyStr = r.getBodyJson(c)
				reqHeadersJson          = r.getHeadersJson(c, logger, jsonEnCoder, traceId)
			)

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
			if len(reqBodyJson) > 0 {
				logEvent.RawJSON("reqBody", reqBodyJson)
			} else if len(reqBodyStr) > 0 {
				logEvent.Str("reqBodyStr", reqBodyStr)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			if debugMode {
				linesJson = r.getJsonIndent(msg, logger, jsonEnCoder, traceId)
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
			var (
				reqParamsJson           = r.getParamsJson(c, logger, jsonEnCoder, traceId)
				reqQueriesJson          = r.getQueriesJson(c, logger, jsonEnCoder, traceId)
				reqBodyJson, reqBodyStr = r.getBodyJson(c)
				reqHeadersJson          = r.getHeadersJson(c, logger, jsonEnCoder, traceId)
			)
			msg := ErrorStack()

			logEvent.Int("Code", err.Code).Str("Msg", err.Error()).Str("PrintStack", "true")

			if len(reqParamsJson) > 0 {
				logEvent.RawJSON("reqParams", reqParamsJson)
			}
			if len(reqQueriesJson) > 0 {
				logEvent.RawJSON("reqQueries", reqQueriesJson)
			}
			if len(reqBodyJson) > 0 {
				logEvent.RawJSON("reqBody", reqBodyJson)
			} else if len(reqBodyStr) > 0 {
				logEvent.Str("reqBodyStr", reqBodyStr)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			if debugMode {
				linesJson = r.getJsonIndent(msg, logger, jsonEnCoder, traceId)
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
			var (
				reqParamsJson           = r.getParamsJson(c, logger, jsonEnCoder, traceId)
				reqQueriesJson          = r.getQueriesJson(c, logger, jsonEnCoder, traceId)
				reqBodyJson, reqBodyStr = r.getBodyJson(c)
				reqHeadersJson          = r.getHeadersJson(c, logger, jsonEnCoder, traceId)
			)
			msg := ErrorStack()

			logEvent.Str("Msg", err.Error()).Str("PrintStack", "true")

			if len(reqParamsJson) > 0 {
				logEvent.RawJSON("reqParams", reqParamsJson)
			}
			if len(reqQueriesJson) > 0 {
				logEvent.RawJSON("reqQueries", reqQueriesJson)
			}
			if len(reqBodyJson) > 0 {
				logEvent.RawJSON("reqBody", reqBodyJson)
			} else if len(reqBodyStr) > 0 {
				logEvent.Str("reqBodyStr", reqBodyStr)
			}
			if len(reqHeadersJson) > 0 {
				logEvent.RawJSON("reqHeaders", reqHeadersJson)
			}

			if debugMode {
				linesJson = r.getJsonIndent(msg, logger, jsonEnCoder, traceId)
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
func (r *RecoverCatch) ErrorHandler(ctx providerctx.ICoreContext, err error) error {
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
	// fiber.Error
	var (
		fe *fiber.Error
	)
	if errors.As(err, &fe) {
		if debugMode {
			return exception.New(constant.RespCoreErrorTypeCode, fe.Error(), fe.Code).JsonWithCtx(ctx, http.StatusInternalServerError) // http code 存入 data字段
		}
		return exception.New(constant.RespCoreErrorTypeCode, constant.RespCoreErrorMsg).JsonWithCtx(ctx, http.StatusInternalServerError)
	}
	// default
	if debugMode {
		return exception.GetUnknownError().RespError(err.Error()).JsonWithCtx(ctx, http.StatusInternalServerError)
	}
	return exception.GetUnknownError().JsonWithCtx(ctx, http.StatusInternalServerError)
}

// getParamsJson 获取请求参数的 JSON 编码字节切片
func (r *RecoverCatch) getParamsJson(c *fiber.Ctx, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	params := c.AllParams()
	j, err := jsonEnCoder(params)
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Str("reqParamsErr", err.Error()).Msg("getParamsJson error")
		return nil
	}
	return j
}

// getQueriesJson 获取查询参数的 JSON 编码字节切片
func (r *RecoverCatch) getQueriesJson(c *fiber.Ctx, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	queries := c.Queries()
	j, err := jsonEnCoder(queries)
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Str("reqQueriesErr", err.Error()).Msg("getQueriesJson error")
		return nil
	}
	return j
}

// getHeadersJson 获取请求头部的 JSON 编码字节切片，对敏感信息进行脱敏处理
func (r *RecoverCatch) getHeadersJson(c *fiber.Ctx, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	// 获取所有请求头
	headers := c.GetReqHeaders()

	// 对敏感头部信息进行脱敏处理
	sanitizedHeaders := make(map[string][]string, len(headers))
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		// 脱敏处理：Authorization、Cookie、Proxy-Authorization、X-Auth-Token 等认证相关头部
		if lowerKey == "authorization" ||
			lowerKey == "cookie" ||
			lowerKey == "proxy-authorization" ||
			lowerKey == "x-auth-token" ||
			lowerKey == "x-api-key" ||
			strings.Contains(lowerKey, "token") ||
			strings.Contains(lowerKey, "secret") ||
			strings.Contains(lowerKey, "password") {
			// 保留前缀，隐藏敏感部分
			maskedValues := make([]string, len(values))
			for i, v := range values {
				l := len(v)
				if l > 0 {
					if l <= 8 {
						maskedValues[i] = "***"
					} else {
						maskedValues[i] = v[:4] + "..." + "***"
					}
				} else {
					maskedValues[i] = ""
				}
			}
			sanitizedHeaders[key] = maskedValues
		} else {
			sanitizedHeaders[key] = values
		}
	}

	j, err := jsonEnCoder(sanitizedHeaders)
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Str("reqHeadersErr", err.Error()).Msg("getHeadersJson error")
		return nil
	}
	return j
}

// getJsonIndent 将堆栈字符串格式化为 JSON 字符串，保留缩进
func (r *RecoverCatch) getJsonIndent(s string, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	if len(s) == 0 {
		return nil
	}
	lines := frameUtils.DebugStackLines(s)
	if len(lines) == 0 {
		return nil
	}
	j, err := json.MarshalIndent(lines, "", "  ")
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Err(err).Msg("getJson from stack lines error")
		return nil
	}
	return j
}

// getBodyJson 获取请求体的 JSON 编码字节切片，非 JSON 格式返回空字节切片和字符串形式的请求体
func (r *RecoverCatch) getBodyJson(c *fiber.Ctx) ([]byte, string) {
	body := c.Body()
	if len(body) == 0 {
		return nil, ""
	}
	//buffer := make([]byte, len(body))
	//copy(buffer, body)
	if frameUtils.JsonValidBytes(body) {
		return body, ""
	}
	return nil, frameUtils.UnsafeString(body)
}

// New creates a new middleware Exception handler [for unexpected panic]
func NewFiberHandler(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *fiber.Ctx) (err error) { //nolint:nonamedreturns // Uses recover() to overwrite the error
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(providerctx.WithFiberContext(c)) {
			return c.Next()
		}

		// Catch panics
		defer func(c *fiber.Ctx) {
			pCtx := providerctx.WithFiberContext(c)
			if r := recover(); r != nil {
				if cfg.EnableStackTrace {
					cfg.StackTraceHandler(providerctx.WithFiberContext(c), r)
				}
				debugMode := cfg.DebugMode
				switch re := r.(type) {
				case *exception.ValidateException:
					err = re.RespError().JsonWithCtx(pCtx, fiber.StatusBadRequest) // output validation error information as is
					return
				case *exception.Exception:
					if debugMode {
						err = re.RespError().JsonWithCtx(pCtx, fiber.StatusBadRequest)
						return
					}
					err = re.RespError(nil).JsonWithCtx(pCtx, fiber.StatusBadRequest)
					return
				case fiber.Error:
					code := fiber.StatusInternalServerError
					if re.Code != 0 {
						code = re.Code
					}
					if debugMode {
						err = exception.New(constant.RespCoreErrorTypeCode, re.Error(), code).JsonWithCtx(pCtx, fiber.StatusInternalServerError) // http code save to data field
						return
					}
					err = exception.New(constant.RespCoreErrorTypeCode, constant.RespCoreErrorMsg).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
					return
				case runtime.Error:
					if debugMode {
						// panic(re)
						err = exception.New(constant.UnknownErrCode, "RuntimeError", re.Error()).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
						return
					}
					var msg string
					if strings.Contains(re.Error(), "invalid memory") || strings.Contains(re.Error(), "nil pointer") {
						msg = "NullPointerException"
					} else {
						msg = "UnknownRTException"
					}
					err = exception.New(constant.UnknownErrCode, msg).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
					return
				case error:
					if debugMode {
						err = exception.New(constant.UnknownErrCode, re.Error()).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
						return
					}
					err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
					return
				default:
					if debugMode {
						dw := jsonconvert.NewDataWrap(re)
						defer dw.Release()
						if dw.CanJSONSerializable() {
							var out interface{}
							jsonRet, _ := dw.GetJson(c.App().Config().JSONEncoder) // ignore error
							if jsonRet == nil {
								out = ""
							} else {
								out = frameUtils.UnsafeString(jsonRet)
							}
							err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, out).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
							return
						} else {
							err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, dw.GetString()).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
							return
						}
					}
					err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
					return
				}
			}
		}(c)

		// Return err if existed, else move to next handler
		return c.Next()
	}
}
