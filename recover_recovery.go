package fiberhouse

import (
	"encoding/json"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse/component/jsonconvert"
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/exception"
	frameUtils "github.com/lamxy/fiberhouse/utils"
	"net/http"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/bootstrap"
	providerctx "github.com/lamxy/fiberhouse/provider/context"
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

// FiberRecovery Fiber 框架的请求数据实现
type FiberRecovery struct {
	AppCtx IApplicationContext
}

// NewFiberRecovery 创建 Fiber 恢复实例
func NewFiberRecovery(ctx IApplicationContext) *FiberRecovery {
	return &FiberRecovery{
		AppCtx: ctx,
	}
}

func (f *FiberRecovery) GetParamsJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return nil
	}
	params := c.AllParams()
	j, err := jsonEncoder(params)
	if err != nil {
		log.Warn(f.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqParamsErr", err.Error()).Msg("getParamsJson error")
		return nil
	}
	return j
}

func (f *FiberRecovery) GetQueriesJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return nil
	}
	queries := c.Queries()
	j, err := jsonEncoder(queries)
	if err != nil {
		log.Warn(f.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqQueriesErr", err.Error()).Msg("getQueriesJson error")
		return nil
	}
	return j
}

func (f *FiberRecovery) GetHeadersJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return nil
	}
	headers := c.GetReqHeaders()
	sanitizedHeaders := sanitizeHeaders(headers)
	j, err := jsonEncoder(sanitizedHeaders)
	if err != nil {
		log.Warn(f.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqHeadersErr", err.Error()).Msg("getHeadersJson error")
		return nil
	}
	return j
}

func (f *FiberRecovery) GetHeader(ctx providerctx.ICoreContext, key string) string {
	c, ok := ctx.GetCtx().(*fiber.Ctx)
	if !ok {
		return ""
	}
	return c.Get(key)
}

func (f *FiberRecovery) RecoverPanic(config ...RecoverConfig) any {
	// 使用恢复中间件提供者来返回相关核心引擎的恢复中间件函数，
	//通过恢复中间件管理器依据核心类型配置选择相应的提供者自动返回对应的恢复中间件，返回一个any
	//同时结合全局的泛型方法获取指定核心的恢复函数

	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *fiber.Ctx) error {
		pCtx := providerctx.WithFiberContext(c)
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(pCtx) {
			return c.Next()
		}

		// Catch panics
		defer RecoverPanicInternal(pCtx, cfg)

		// Return err if existed, else move to next handler
		return c.Next()
	}
}

func (f *FiberRecovery) TraceID(ctx providerctx.ICoreContext, flag ...string) string {
	// 原生上下文
	var (
		c  *fiber.Ctx
		ok bool
	)
	if c, ok = ctx.GetCtx().(*fiber.Ctx); !ok {
		panic("ContextProvider is not *fiber.Ctx")
	}
	// 请求requestId
	var (
		traceId string
		rID     = requestID
	)
	if len(flag) > 0 {
		rID = flag[0]
	}
	if c.Locals(rID) != nil {
		traceId = c.Locals(rID).(string) // 请求类错误，从本地变量获取请求ID
	} else {
		traceId = "" // 非请求接口出现的错误，请求ID空值
	}
	return traceId
}

// sanitizeHeaders 对敏感头部信息进行脱敏处理
func sanitizeHeaders(headers map[string][]string) map[string][]string {
	sanitized := make(map[string][]string, len(headers))
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if isSensitiveHeader(lowerKey) {
			maskedValues := make([]string, len(values))
			for i, v := range values {
				maskedValues[i] = maskValue(v)
			}
			sanitized[key] = maskedValues
		} else {
			sanitized[key] = values
		}
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

// GinRecovery Gin 框架的请求数据实现
type GinRecovery struct {
	AppCtx IApplicationContext
}

// NewGinRecovery 创建 Gin 请求数据实例
func NewGinRecovery(ctx IApplicationContext) *GinRecovery {
	return &GinRecovery{
		AppCtx: ctx,
	}
}

func (g *GinRecovery) GetParamsJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return nil
	}
	params := make(map[string]string)
	for _, p := range c.Params {
		params[p.Key] = p.Value
	}
	j, err := jsonEncoder(params)
	if err != nil {
		log.Warn(g.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqParamsErr", err.Error()).Msg("getParamsJson error")
		return nil
	}
	return j
}

func (g *GinRecovery) GetQueriesJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return nil
	}
	queries := make(map[string][]string)
	for key, values := range c.Request.URL.Query() {
		queries[key] = values
	}
	j, err := jsonEncoder(queries)
	if err != nil {
		log.Warn(g.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqQueriesErr", err.Error()).Msg("getQueriesJson error")
		return nil
	}
	return j
}

func (g *GinRecovery) GetHeadersJson(ctx providerctx.ICoreContext, log bootstrap.LoggerWrapper, jsonEncoder func(interface{}) ([]byte, error), traceId string) []byte {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return nil
	}
	headers := make(map[string][]string)
	for key, values := range c.Request.Header {
		headers[key] = values
	}
	sanitizedHeaders := sanitizeHeaders(headers)
	j, err := jsonEncoder(sanitizedHeaders)
	if err != nil {
		log.Warn(g.AppCtx.GetConfig().LogOriginRecover()).Str("traceId", traceId).Str("reqHeadersErr", err.Error()).Msg("getHeadersJson error")
		return nil
	}
	return j
}

func (g *GinRecovery) GetHeader(ctx providerctx.ICoreContext, key string) string {
	c, ok := ctx.GetCtx().(*gin.Context)
	if !ok {
		return ""
	}
	return c.GetHeader(key)
}

func (g *GinRecovery) RecoverPanic(config ...RecoverConfig) any {
	// 使用恢复中间件提供者来创建和返回相应核心的恢复中间件函数，
	//通过恢复中间件管理器内部依据核心类型自动返回相应的恢复中间件，返回一个any，同时结合泛型方法
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *gin.Context) {
		pCtx := providerctx.WithGinContext(c)
		// Don't execute middleware if Cfg Next returns true
		if cfg.Next != nil && cfg.Next(pCtx) {
			c.Next()
		}

		// Catch panics
		defer RecoverPanicInternal(pCtx, cfg)

		c.Next()
	}
}

func (g *GinRecovery) TraceID(ctx providerctx.ICoreContext, flag ...string) string {
	// 原生上下文
	var (
		c  *gin.Context
		ok bool
	)
	if c, ok = ctx.GetCtx().(*gin.Context); !ok {
		panic("ContextProvider is not *gin.Context")
	}
	// 请求requestId
	return requestid.Get(c)
}

// RecoverPanicInternal 全局恢复panic函数，用于defer fn()
func RecoverPanicInternal(pCtx providerctx.ICoreContext, cfg RecoverConfig) {
	if r := recover(); r != nil {
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
				_ = exception.New(constant.UnknownErrCode, "RuntimeError", re.Error()).JsonWithCtx(pCtx, http.StatusInternalServerError)
				return
			}
			var msg string
			if strings.Contains(re.Error(), "invalid memory") || strings.Contains(re.Error(), "nil pointer") {
				msg = "NullPointerException"
			} else {
				msg = "UnknownRTException"
			}
			_ = exception.New(constant.UnknownErrCode, msg).JsonWithCtx(pCtx, http.StatusInternalServerError)
			return
		case error:
			if debugMode {
				_ = exception.New(constant.UnknownErrCode, re.Error()).JsonWithCtx(pCtx, http.StatusInternalServerError)
				return
			}
			_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(pCtx, http.StatusInternalServerError)
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
					_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, out).JsonWithCtx(pCtx, http.StatusInternalServerError)
					return
				} else {
					_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg, dw.GetString()).JsonWithCtx(pCtx, http.StatusInternalServerError)
					return
				}
			}
			_ = exception.New(constant.UnknownErrCode, constant.UnknownErrMsg).JsonWithCtx(pCtx, http.StatusInternalServerError)
			return
		}
	}
}
