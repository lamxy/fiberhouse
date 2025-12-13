// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package ginrecover 提供 Gin 框架的全局异常恢复和错误处理中间件。
package ginrecover

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/jsonconvert"
	"github.com/lamxy/fiberhouse/constant"
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/lamxy/fiberhouse/exception"

	frameUtils "github.com/lamxy/fiberhouse/utils"

	"github.com/gin-contrib/requestid"
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/middleware"
)

var (
	debugFlag      = "X-your-custom-debug-flag"             // 自定义debug标记key，由后端recover配置定义覆盖
	debugFlagValue = "f0dc4970-ed31-4598-acd8-b5c5fd66c12e" // 自定义debug标记值，由后端recover配置定义覆盖
	requestID      = "traceId"                              // 请求ID字段名称，由后端trace配置定义覆盖
)

type RecoverCatch struct {
	AppCtx fiberhouse.IApplicationContext
}

func NewRecoverCatch(ctx fiberhouse.IApplicationContext) middleware.IRecover {
	return &RecoverCatch{
		AppCtx: ctx,
	}
}

func (r *RecoverCatch) GetContext() fiberhouse.IApplicationContext {
	return r.AppCtx
}

// DefaultStackTraceHandler 记录请求上下文信息 + panic信息 + 堆栈信息
func (r *RecoverCatch) DefaultStackTraceHandler(ctx providerCtx.ContextProvider, e interface{}) {
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
		c  *gin.Context
		ok bool
	)
	if c, ok = ctx.GetCtx().(*gin.Context); !ok {
		panic("ContextProvider is not *gin.Context")
	}

	// 头部debug标记
	debugFlagFromHeader := c.GetHeader(debugFlag)
	// 请求requestId
	var traceId = requestid.Get(c)

	var jsonEnCoder func(interface{}) ([]byte, error)
	jsonEnCoder = ginJson.API.Marshal

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

// ErrorHandler 用于gin全局错误处理器中间件，处理业务级错误
func (r *RecoverCatch) ErrorHandler(c providerCtx.ContextProvider, err error) error {
	// 记录日志 & 堆栈
	r.DefaultStackTraceHandler(c, err)

	// ValidateException
	var (
		debugMode = r.GetContext().GetConfig().GetRecover().DebugMode
		eve       *exception.ValidateException
	)
	okVe := errors.As(err, &eve)
	if okVe {
		// 验证器错误，响应完整错误信息到客户端
		return eve.RespError().JsonWithCtx(c, http.StatusBadRequest)
	}
	// Exception
	var ee *exception.Exception
	okEe := errors.As(err, &ee)
	if okEe {
		if debugMode {
			return ee.RespError().JsonWithCtx(c, http.StatusBadRequest)
		}
		return ee.RespError(nil).JsonWithCtx(c, http.StatusBadRequest)
	}
	// default
	if debugMode {
		return exception.GetUnknownError().RespError(err.Error()).JsonWithCtx(c, http.StatusInternalServerError)
	}
	return exception.GetUnknownError().JsonWithCtx(c, http.StatusInternalServerError)
}

// getParamsJson 获取请求参数的 JSON 编码字节切片
func (r *RecoverCatch) getParamsJson(c *gin.Context, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	// 获取所有路径参数
	params := make(map[string]string)

	for i := range c.Params {
		params[c.Params[i].Key] = c.Params[i].Value
	}
	j, err := jsonEnCoder(params)
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Str("reqParamsErr", err.Error()).Msg("getParamsJson error")
		return nil
	}
	return j
}

// getQueriesJson 获取查询参数的 JSON 编码字节切片
func (r *RecoverCatch) getQueriesJson(c *gin.Context, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	// 获取所有查询参数
	queries := c.Request.URL.Query()
	j, err := jsonEnCoder(queries)
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Str("reqQueriesErr", err.Error()).Msg("getQueriesJson error")
		return nil
	}
	return j
}

// getHeadersJson 获取请求头部的 JSON 编码字节切片，对敏感信息进行脱敏处理
func (r *RecoverCatch) getHeadersJson(c *gin.Context, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	// 获取所有请求头
	headers := c.Request.Header

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
func (r *RecoverCatch) getBodyJson(c *gin.Context) ([]byte, string) {
	// 优先从上下文缓存中获取
	if cachedBody, exists := c.Get("__request_body_cache__"); exists {
		if bodyBytes, ok := cachedBody.([]byte); ok && len(bodyBytes) > 0 {
			// 判断是否为有效 JSON
			if frameUtils.JsonValidBytes(bodyBytes) {
				return bodyBytes, ""
			}
			return nil, frameUtils.UnsafeString(bodyBytes)
		}
	}

	// 如果缓存中没有，尝试直接读取（可能已被消费）
	if c.Request.Body == nil {
		return nil, ""
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, ""
	}

	// 恢复 Body 供后续使用
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if len(bodyBytes) == 0 {
		return nil, ""
	}

	// 判断是否为有效 JSON
	if frameUtils.JsonValidBytes(bodyBytes) {
		return bodyBytes, ""
	}
	return nil, frameUtils.UnsafeString(bodyBytes)
}

// RequestBodyCacheMiddleware 中间件：提前读取并缓存请求体
func RequestBodyCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}

		// 读取请求体
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}

		// 恢复 Body 供后续使用
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// 将读取的内容存储到上下文中
		c.Set("__request_body_cache__", bodyBytes)

		c.Next()
	}
}

// ErrorStack 获取当前的堆栈信息字符串
func ErrorStack(debugStack ...bool) string {
	//if len(debugStack) > 0 && debugStack[0] {
	//	return frameUtils.StackMsg()
	//}
	return frameUtils.CaptureStack()
}

// New creates a recover middleware error handler for Gin framework.
func New(config ...Config) gin.HandlerFunc {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *gin.Context) {
		// Don't execute middleware if Cfg Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			c.Next()
		}

		// Catch panics
		defer func(c *gin.Context) {
			if r := recover(); r != nil {
				pCtx := providerCtx.WithGinContext(c)
				if cfg.EnableStackTrace {
					cfg.StackTraceHandler(pCtx, r)
				}
				debugMode := cfg.DebugMode
				switch re := r.(type) {
				case *exception.ValidateException:
					_ = re.RespError().JsonWithCtx(pCtx, http.StatusBadRequest)
					return
				case *exception.Exception:
					if debugMode {
						_ = re.RespError().JsonWithCtx(pCtx, http.StatusBadRequest)
						return
					}
					_ = re.RespError(nil).JsonWithCtx(pCtx, http.StatusBadRequest)
					return
				case runtime.Error:
					if debugMode {
						// panic(re)
						_ = exception.New(constant.UnknownErrCode, "RuntimeError", re.Error()).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
						return
					}
					var msg string
					if strings.Contains(re.Error(), "invalid memory") || strings.Contains(re.Error(), "nil pointer") {
						msg = "NullPointerException"
					} else {
						msg = "UnknownRTException"
					}
					_ = exception.New(constant.UnknownErrCode, msg).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
					return
				case error:
					if debugMode {
						_ = exception.New(constant.UnknownErrCode, re.Error()).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
						return
					}
					_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
					return
				default:
					if debugMode {
						dw := jsonconvert.NewDataWrap(re)
						defer dw.Release()
						if dw.CanJSONSerializable() {
							var out interface{}
							jsonRet, _ := dw.GetJson(ginJson.API.Marshal) // ignore error
							if jsonRet == nil {
								out = ""
							} else {
								out = frameUtils.UnsafeString(jsonRet)
							}
							_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, out).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
							return
						} else {
							_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, dw.GetString()).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
							return
						}
					}
					_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(pCtx, fiber.StatusInternalServerError)
					return
				}
			}
		}(c)

		c.Next()
	}
}
