package fiberhouse

import (
	"errors"
	"net/http"
	"runtime"
	"strings"

	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/component/jsonconvert"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	frameUtils "github.com/lamxy/fiberhouse/utils"
)

// recoverPanicInternal 全局恢复panic函数，用于defer fn()
func recoverPanicInternal(pCtx adaptorctx.ICoreContext, cfg RecoverConfig) {
	if r := recover(); r != nil {
		if cfg.EnableStackTrace {
			cfg.StackTraceHandler(pCtx, r)
		}
		// 与 ErrorHandler 一致：*fiber.Error（含包装）按自身状态码响应
		if err, ok := r.(error); ok {
			if code, message, ok := fiberHTTPError(err); ok {
				_ = Response().Reset(code, message, nil).SendWithCtx(pCtx, code)
				return
			}
		}
		// 与 ErrorHandler 一致：从包装链中取出业务错误，交给下面原有的分支处理
		r = unwrapBusinessError(r)
		debugMode := cfg.DebugMode
		switch re := r.(type) {
		case *exception.ValidateException:
			_ = Response().From(re.RespData(), true).SendWithCtx(pCtx, http.StatusBadRequest)
			return
		case *exception.Exception:
			if debugMode {
				//_ = re.RespData().JsonWithCtx(pCtx, http.StatusBadRequest)
				_ = Response().From(re.RespData(), true).SendWithCtx(pCtx, http.StatusBadRequest)
				return
			}
			//_ = re.RespData(nil).JsonWithCtx(pCtx, http.StatusBadRequest)
			_ = Response().From(re.RespData(nil), true).SendWithCtx(pCtx, http.StatusBadRequest)
			return
		case runtime.Error:
			if debugMode {
				// panic(re)
				_ = Response().From(exception.New(constant.UnknownErrCode, "RuntimeError", re.Error()), true).SendWithCtx(pCtx, http.StatusInternalServerError)
				return
			}
			var msg string
			if strings.Contains(re.Error(), "invalid memory") || strings.Contains(re.Error(), "nil pointer") {
				msg = "NullPointerException"
			} else {
				msg = "UnknownRTException"
			}
			_ = Response().From(exception.New(constant.UnknownErrCode, msg), true).SendWithCtx(pCtx, http.StatusInternalServerError)
			return
		case error:
			if debugMode {
				_ = Response().From(exception.New(constant.UnknownErrCode, re.Error()), true).SendWithCtx(pCtx, http.StatusInternalServerError)
				return
			}
			_ = Response().From(exception.New(constant.UnknownErrCode, constant.UnknownErrMsg), true).SendWithCtx(pCtx, http.StatusInternalServerError)
			return
		default:
			if debugMode {
				dw := jsonconvert.NewDataWrap(re)
				defer dw.Release()
				if dw.CanJSONSerializable() {
					var out interface{}
					jsonRet, _ := dw.GetJson(cfg.JsonCodec) // ignore error
					if jsonRet == nil {
						out = ""
					} else {
						out = frameUtils.UnsafeString(jsonRet)
					}
					_ = Response().From(exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, out), true).SendWithCtx(pCtx, http.StatusInternalServerError)
					return
				} else {
					_ = Response().From(exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, dw.GetString()), true).SendWithCtx(pCtx, http.StatusInternalServerError)
					return
				}
			}
			_ = Response().From(exception.New(constant.UnknownErrCode, constant.UnknownErrMsg), true).SendWithCtx(pCtx, http.StatusInternalServerError)
			return
		}
	}
}

// unwrapBusinessError 从包装链中取出业务错误，使 panic 路径与 ErrorHandler 的 errors.As 判定一致
func unwrapBusinessError(r interface{}) interface{} {
	err, ok := r.(error)
	if !ok {
		return r
	}
	var ve *exception.ValidateException
	if errors.As(err, &ve) {
		return ve
	}
	var ee *exception.Exception
	if errors.As(err, &ee) {
		return ee
	}
	return r
}
