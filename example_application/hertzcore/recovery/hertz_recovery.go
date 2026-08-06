// Package recovery 提供 hertz 核心引擎對框架 fiberhouse.IRecover 介面的實作。
//
// 框架的 RecoveryPManager 依 BootConfig.CoreType 選取對應 Target 的恢復提供者，
// 本套件讓 CoreType="hertz" 時能復用框架統一的 panic 恢復與錯誤回應鏈路。
package recovery

import (
	ctxpkg "context"
	"net/http"
	"runtime"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/lamxy/fiberhouse"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	hertzadaptor "github.com/lamxy/fiberhouse/example_application/hertzcore/adaptor"
)

// traceIDKey 請求追蹤 ID 在 hertz 上下文中的鍵名，與框架 recover_error_handler_impl.go 的預設值一致
const traceIDKey = "traceId"

// HertzRecovery hertz 框架的請求資料與恢復中間件實作
type HertzRecovery struct {
	AppCtx fiberhouse.IApplicationContext
}

// 編譯期確保實作了框架的恢復介面
var _ fiberhouse.IRecover = (*HertzRecovery)(nil)

// New 建立 hertz 恢復實例
func New(ctx fiberhouse.IApplicationContext) *HertzRecovery {
	return &HertzRecovery{AppCtx: ctx}
}

// nativeCtx 從框架核心上下文取出 hertz 原生請求上下文。
//
// 傳入非 hertz 上下文時回傳 nil，呼叫方據此安全降級，
// 避免恢復流程本身再次 panic（對齊框架 Fiber/Gin 實作的容錯策略）。
func nativeCtx(ctx adaptorctx.ICoreContext) *app.RequestContext {
	c, ok := ctx.GetCtx().(*app.RequestContext)
	if !ok {
		return nil
	}
	return c
}

// GetParamsJson 獲取路由參數的 JSON 編碼位元組切片
func (h *HertzRecovery) GetParamsJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c := nativeCtx(ctx)
	if c == nil {
		return nil
	}
	params := make(map[string]string, len(c.Params))
	for _, p := range c.Params {
		params[p.Key] = p.Value
	}
	j, err := jsonEncoder(params)
	if err != nil {
		h.warn(log, traceId, "reqParamsErr", err, "getParamsJson error")
		return nil
	}
	return j
}

// GetQueriesJson 獲取查詢參數的 JSON 編碼位元組切片
func (h *HertzRecovery) GetQueriesJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c := nativeCtx(ctx)
	if c == nil {
		return nil
	}
	queries := make(map[string][]string)
	c.QueryArgs().VisitAll(func(key, value []byte) {
		k := string(key)
		queries[k] = append(queries[k], string(value))
	})
	j, err := jsonEncoder(queries)
	if err != nil {
		h.warn(log, traceId, "reqQueriesErr", err, "getQueriesJson error")
		return nil
	}
	return j
}

// GetHeadersJson 獲取請求頭的 JSON 編碼位元組切片（敏感資訊脫敏）
func (h *HertzRecovery) GetHeadersJson(ctx adaptorctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c := nativeCtx(ctx)
	if c == nil {
		return nil
	}
	headers := make(map[string][]string)
	c.Request.Header.VisitAll(func(key, value []byte) {
		k := string(key)
		headers[k] = append(headers[k], string(value))
	})
	j, err := jsonEncoder(sanitizeHeaders(headers))
	if err != nil {
		h.warn(log, traceId, "reqHeadersErr", err, "getHeadersJson error")
		return nil
	}
	return j
}

// GetHeader 獲取指定請求頭
func (h *HertzRecovery) GetHeader(ctx adaptorctx.ICoreContext, key string) string {
	c := nativeCtx(ctx)
	if c == nil {
		return ""
	}
	return string(c.GetHeader(key))
}

// TraceID 獲取請求追蹤 ID
//
// 與 Fiber/Gin 實作不同，此處對非 hertz 上下文或缺失鍵回傳空字串而不 panic，
// 因恢復流程本身不應成為新的故障點。
func (h *HertzRecovery) TraceID(ctx adaptorctx.ICoreContext, flag ...string) string {
	c := nativeCtx(ctx)
	if c == nil {
		return ""
	}
	key := traceIDKey
	if len(flag) > 0 && flag[0] != "" {
		key = flag[0]
	}
	if v, ok := c.Get(key); ok {
		if s, isStr := v.(string); isStr {
			return s
		}
	}
	return ""
}

// RecoverPanic 回傳 hertz 的 panic 恢復中間件
//
// 框架以 MustRecoverMiddleware[app.HandlerFunc] 取出中間件，故回傳 app.HandlerFunc。
// 框架內部的 recoverPanicInternal 未匯出，此處以匯出 API（fiberhouse.Response、
// exception）復刻等價的 panic 分類與回應行為，保持跨核心一致的錯誤契約。
func (h *HertzRecovery) RecoverPanic(config ...fiberhouse.RecoverConfig) any {
	var cfg fiberhouse.RecoverConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return app.HandlerFunc(func(c ctxpkg.Context, reqCtx *app.RequestContext) {
		pCtx := hertzadaptor.WithHertzContext(reqCtx)
		completed := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					if cfg.EnableStackTrace && cfg.StackTraceHandler != nil {
						cfg.StackTraceHandler(pCtx, r)
					}
					respondPanic(pCtx, r, cfg.DebugMode)
				}
			}()
			if cfg.Next != nil && cfg.Next(pCtx) {
				reqCtx.Next(c)
				completed = true
				return
			}
			reqCtx.Next(c)
			completed = true
		}()
		if completed {
			if releasable, ok := pCtx.(interface{ Release() }); ok {
				releasable.Release()
			}
		}
	})
}

// respondPanic 將 panic 值分類並以框架統一回應格式輸出。
//
// 分類規則對齊框架 recover_internal.go 的 recoverPanicInternal，
// 確保 hertz 與 fiber/gin 對外的錯誤契約一致。
func respondPanic(pCtx adaptorctx.ICoreContext, r interface{}, debugMode bool) {
	switch re := r.(type) {
	case *exception.ValidateException:
		_ = fiberhouse.Response().From(re.RespData(), true).SendWithCtx(pCtx, http.StatusBadRequest)
	case *exception.Exception:
		if debugMode {
			_ = fiberhouse.Response().From(re.RespData(), true).SendWithCtx(pCtx, http.StatusBadRequest)
			return
		}
		_ = fiberhouse.Response().From(re.RespData(nil), true).SendWithCtx(pCtx, http.StatusBadRequest)
	case runtime.Error:
		if debugMode {
			_ = fiberhouse.Response().From(exception.New(constant.UnknownErrCode, "RuntimeError", re.Error()), true).
				SendWithCtx(pCtx, http.StatusInternalServerError)
			return
		}
		msg := "UnknownRTException"
		if strings.Contains(re.Error(), "invalid memory") || strings.Contains(re.Error(), "nil pointer") {
			msg = "NullPointerException"
		}
		_ = fiberhouse.Response().From(exception.New(constant.UnknownErrCode, msg), true).
			SendWithCtx(pCtx, http.StatusInternalServerError)
	case error:
		if debugMode {
			_ = fiberhouse.Response().From(exception.New(constant.UnknownErrCode, re.Error()), true).
				SendWithCtx(pCtx, http.StatusInternalServerError)
			return
		}
		_ = fiberhouse.Response().From(exception.New(constant.UnknownErrCode, constant.UnknownErrMsg), true).
			SendWithCtx(pCtx, http.StatusInternalServerError)
	default:
		_ = fiberhouse.Response().From(exception.New(constant.UnknownErrCode, constant.UnknownErrMsg), true).
			SendWithCtx(pCtx, http.StatusInternalServerError)
	}
}

// warn 記錄編碼失敗的警告日誌，log 為 nil 時（單元測試場景）靜默跳過
func (h *HertzRecovery) warn(log bootstrap.LoggerWrapper, traceId, field string, err error, msg string) {
	if log == nil || h.AppCtx == nil {
		return
	}
	log.Warn(h.AppCtx.GetConfig().LogOriginRecover()).
		Str("traceId", traceId).
		Str(field, err.Error()).
		Msg(msg)
}

// sanitizeHeaders 對敏感頭部資訊脫敏
//
// 框架的同名函式未匯出，此處複刻其規則以保持跨核心的一致行為。
func sanitizeHeaders(headers map[string][]string) map[string][]string {
	sanitized := make(map[string][]string, len(headers))
	for key, values := range headers {
		if isSensitiveHeader(strings.ToLower(key)) {
			masked := make([]string, len(values))
			for i, v := range values {
				masked[i] = maskValue(v)
			}
			sanitized[key] = masked
			continue
		}
		sanitized[key] = values
	}
	return sanitized
}

func isSensitiveHeader(key string) bool {
	return key == "authorization" ||
		key == "cookie" ||
		key == "proxy-authorization" ||
		key == "x-auth-token" ||
		key == "x-api-key" ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password")
}

func maskValue(v string) string {
	l := len(v)
	if l == 0 {
		return ""
	}
	if l <= 8 {
		return "***"
	}
	return v[:4] + "...***"
}
