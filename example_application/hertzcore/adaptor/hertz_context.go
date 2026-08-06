// Package adaptor 提供 hertz 核心引擎對框架 adaptor/context.ICoreContext 介面的實作。
//
// 框架的統一回應鏈路（fiberhouse.Response().SendWithCtx）與 panic 恢復鏈路
// （recoverPanicInternal）皆依賴 ICoreContext 抽象，實作本介面即可讓 hertz
// 復用框架既有的回應與異常處理能力，無需改動框架本身。
package adaptor

import (
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
)

// hertzContextPool HertzContext 對象池，對齊框架既有 Fiber/Gin 適配器的復用策略
var hertzContextPool = sync.Pool{
	New: func() interface{} {
		return &HertzContext{}
	},
}

// HertzContext hertz 框架的上下文適配器
type HertzContext struct {
	Ctx *app.RequestContext
}

// 編譯期確保實作了框架的核心上下文介面
var _ adaptorctx.ICoreContext = (*HertzContext)(nil)

// WithHertzContext 從對象池獲取 hertz 上下文適配器
func WithHertzContext(c *app.RequestContext) adaptorctx.ICoreContext {
	ctx := hertzContextPool.Get().(*HertzContext)
	ctx.Ctx = c
	return ctx
}

// Release 釋放 HertzContext 回對象池
//
// 框架的 releaseCoreContext 以 interface{ Release() } 鴨子型別回收上下文，
// 實作此方法即可接入框架既有的回收流程。
func (h *HertzContext) Release() {
	h.Ctx = nil
	hertzContextPool.Put(h)
}

// GetCtx 獲取原生上下文
func (h *HertzContext) GetCtx() interface{} {
	return h.Ctx
}

// JSON 以 JSON 格式回應資料
//
// hertz 的 RequestContext.JSON 無回傳值，此處統一回傳 nil 以符合介面契約。
func (h *HertzContext) JSON(statusCode int, data interface{}) error {
	defer h.Release()
	h.Ctx.JSON(statusCode, data)
	return nil
}

// Send 發送原始位元組資料
//
// 沿用呼叫方已設定的 Content-Type，未設定時交由 hertz 預設處理，
// 以保持與 Fiber/Gin 適配器一致的語義。
func (h *HertzContext) Send(statusCode int, body []byte) error {
	defer h.Release()
	h.Ctx.SetStatusCode(statusCode)
	h.Ctx.Response.SetBody(body)
	return nil
}

// GetHeader 獲取請求頭
//
// hertz 的 GetHeader 回傳 []byte，缺失時為 nil，轉換為字串後即為空字串。
func (h *HertzContext) GetHeader(key string) string {
	return string(h.Ctx.GetHeader(key))
}

// SetHeader 設置回應頭
func (h *HertzContext) SetHeader(key string, value string) {
	h.Ctx.Response.Header.Set(key, value)
}
