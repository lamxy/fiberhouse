package fiberhouse

import (
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
