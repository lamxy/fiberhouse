// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package recover 提供 Fiber 框架的全局异常恢复和错误处理中间件。
package recover

import (
	"encoding/json"
	"errors"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/jsonconvert"
	"github.com/lamxy/fiberhouse/constant"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/lamxy/fiberhouse/exception"

	frameUtils "github.com/lamxy/fiberhouse/utils"

	"github.com/gofiber/fiber/v2"
)

var (
	debugFlag      = "X-your-custom-debug-flag"             // 自定义debug标记key，由后端recover配置定义覆盖
	debugFlagValue = "f0dc4970-ed31-4598-acd8-b5c5fd66c12e" // 自定义debug标记值，由后端recover配置定义覆盖
	requestID      = "traceId"                              // 请求ID字段名称，由后端trace配置定义覆盖
)

type IRecover interface {
	DefaultStackTraceHandler(c *fiber.Ctx, e interface{})
	ErrorHandler(c *fiber.Ctx, err error) error
	GetContext() fiberhouse.ContextFramer
}

type RecoverCatch struct {
	AppCtx fiberhouse.ContextFramer
}

func NewRecoverCatch(ctx fiberhouse.ContextFramer) IRecover {
	return &RecoverCatch{
		AppCtx: ctx,
	}
}

func (r *RecoverCatch) GetContext() fiberhouse.ContextFramer {
	return r.AppCtx
}

// DefaultStackTraceHandler 记录请求上下文信息 + panic信息 + 堆栈信息
func (r *RecoverCatch) DefaultStackTraceHandler(c *fiber.Ctx, e interface{}) {
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
		if jsonTmp, ok := jsonEnc.(fiberhouse.JsonWrapper); ok {
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

			// 记录reqParams、reqQueries、reqBody
			var (
				reqParamsJson           = r.getParamsJson(c, logger, jsonEnCoder, traceId)
				reqQueriesJson          = r.getQueriesJson(c, logger, jsonEnCoder, traceId)
				reqBodyJson, reqBodyStr = r.getBodyJson(c)
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

			// 记录reqParams、reqQueries、reqBody
			var (
				reqParamsJson           = r.getParamsJson(c, logger, jsonEnCoder, traceId)
				reqQueriesJson          = r.getQueriesJson(c, logger, jsonEnCoder, traceId)
				reqBodyJson, reqBodyStr = r.getBodyJson(c)
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
func (r *RecoverCatch) ErrorHandler(c *fiber.Ctx, err error) error {
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
		return eve.RespError().JsonWithCtx(c, fiber.StatusBadRequest)
	}
	// Exception
	var ee *exception.Exception
	okEe := errors.As(err, &ee)
	if okEe {
		if debugMode {
			return ee.RespError().JsonWithCtx(c, fiber.StatusBadRequest)
		}
		return ee.RespError(nil).JsonWithCtx(c, fiber.StatusBadRequest)
	}
	// fiber.Error
	var (
		fe *fiber.Error
	)
	if errors.As(err, &fe) {
		if debugMode {
			return exception.New(constant.RespCoreErrorTypeCode, fe.Error(), fe.Code).JsonWithCtx(c, fiber.StatusInternalServerError) // http code 存入 data字段
		}
		return exception.New(constant.RespCoreErrorTypeCode, constant.RespCoreErrorMsg).JsonWithCtx(c, fiber.StatusInternalServerError)
	}
	// default
	if debugMode {
		return exception.GetUnknownError().RespError(err.Error()).JsonWithCtx(c, fiber.StatusInternalServerError)
	}
	return exception.GetUnknownError().JsonWithCtx(c, fiber.StatusInternalServerError)
}

func (r *RecoverCatch) getParamsJson(c *fiber.Ctx, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	params := c.AllParams()
	j, err := jsonEnCoder(params)
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Str("reqParamsErr", err.Error()).Msg("getParamsJson error")
		return nil
	}
	return j
}

func (r *RecoverCatch) getQueriesJson(c *fiber.Ctx, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	queries := c.Queries()
	j, err := jsonEnCoder(queries)
	if err != nil {
		log.Warn(r.GetContext().GetConfig().LogOriginRecover()).Str(requestID, traceId).Str("reqQueriesErr", err.Error()).Msg("getQueriesJson error")
		return nil
	}
	return j
}

func (r *RecoverCatch) getJsonIndent(s string, log bootstrap.LoggerWrapper, jsonEnCoder func(interface{}) ([]byte, error), traceId string) []byte {
	if len(s) == 0 {
		return nil
	}
	lines := debugStackLines(s)
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

// StackMsg 获取当前 goroutine 的完整调用栈信息，需将字节切片转为字符串
func StackMsg() string {
	return frameUtils.UnsafeString(debug.Stack())
}

func ErrorStack(debugStack ...bool) string {
	//if len(debugStack) > 0 && debugStack[0] {
	//	return StackMsg()
	//}
	return CaptureStack()
}

// CaptureStack 捕获当前 goroutine 的调用栈信息，跳过前3层调用栈
// 固定 64 个 uintptr 数组，栈上分配
func CaptureStack() string {
	const size = 64
	var pcs [size]uintptr
	n := runtime.Callers(3, pcs[:]) // skip跳过前3层
	frames := runtime.CallersFrames(pcs[:n])

	var strBuilder strings.Builder
	strBuilder.WriteString("stack trace:\n")

	for {
		frm, more := frames.Next()
		strBuilder.WriteString(frm.Function)
		strBuilder.WriteString("\n\t")
		strBuilder.WriteString(frm.File)
		strBuilder.WriteByte(':')
		strBuilder.WriteString(strconv.Itoa(frm.Line))
		strBuilder.WriteByte('\n')

		if !more {
			break
		}
	}
	return strBuilder.String()
}

// debugStackLines 获取当前 goroutine 的调用栈信息行切片
func debugStackLines(stacks string) []string {
	if len(stacks) == 0 || !strings.Contains(stacks, "\n") {
		return nil
	}
	return strings.Split(strings.ReplaceAll(stacks, "\t", "    "), "\n")
}

// New creates a new middleware Exception handler [for unexpected panic]
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *fiber.Ctx) (err error) { //nolint:nonamedreturns // Uses recover() to overwrite the error
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Catch panics
		defer func(c *fiber.Ctx) {
			if r := recover(); r != nil {
				if cfg.EnableStackTrace {
					cfg.StackTraceHandler(c, r)
				}
				debugMode := cfg.DebugMode
				switch re := r.(type) {
				case *exception.ValidateException:
					err = re.RespError().JsonWithCtx(c, fiber.StatusBadRequest) // output validation error information as is
					return
				case *exception.Exception:
					if debugMode {
						err = re.RespError().JsonWithCtx(c, fiber.StatusBadRequest)
						return
					}
					err = re.RespError(nil).JsonWithCtx(c, fiber.StatusBadRequest)
					return
				case fiber.Error:
					code := fiber.StatusInternalServerError
					if re.Code != 0 {
						code = re.Code
					}
					if debugMode {
						err = exception.New(constant.RespCoreErrorTypeCode, re.Error(), code).JsonWithCtx(c, fiber.StatusInternalServerError) // http code save to data field
						return
					}
					err = exception.New(constant.RespCoreErrorTypeCode, constant.RespCoreErrorMsg).JsonWithCtx(c, fiber.StatusInternalServerError)
					return
				case runtime.Error:
					if debugMode {
						// panic(re)
						err = exception.New(constant.UnknownErrCode, "RuntimeError", re.Error()).JsonWithCtx(c, fiber.StatusInternalServerError)
						return
					}
					var msg string
					if strings.Contains(re.Error(), "invalid memory") || strings.Contains(re.Error(), "nil pointer") {
						msg = "NullPointerException"
					} else {
						msg = "UnknownRTException"
					}
					err = exception.New(constant.UnknownErrCode, msg).JsonWithCtx(c, fiber.StatusInternalServerError)
					return
				case error:
					if debugMode {
						err = exception.New(constant.UnknownErrCode, re.Error()).JsonWithCtx(c, fiber.StatusInternalServerError)
						return
					}
					err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(c, fiber.StatusInternalServerError)
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
							err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, out).JsonWithCtx(c, fiber.StatusInternalServerError)
							return
						} else {
							err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, dw.GetString()).JsonWithCtx(c, fiber.StatusInternalServerError)
							return
						}
					}
					err = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(c, fiber.StatusInternalServerError)
					return
				}
			}
		}(c)

		// Return err if existed, else move to next handler
		return c.Next()
	}
}
