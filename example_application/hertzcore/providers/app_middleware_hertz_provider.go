package providers

import (
	ctxpkg "context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/google/uuid"
	"github.com/lamxy/fiberhouse"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
)

// traceIDKey 追蹤 ID 在 hertz 上下文與回應頭中的鍵名，
// 與 hertzcore/recovery 的取值鍵保持一致
const (
	traceIDKey    = "traceId"
	traceIDHeader = "X-Request-Id"
)

// HertzAppMiddlewareProvider 基於 hertz 的應用級中間件提供者
type HertzAppMiddlewareProvider struct {
	fiberhouse.IProvider
}

// NewHertzAppMiddlewareProvider 建立 hertz 應用中間件提供者
func NewHertzAppMiddlewareProvider() *HertzAppMiddlewareProvider {
	son := &HertzAppMiddlewareProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("HertzAppMiddlewareProvider").
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType),
	}
	son.MountToParent(son)
	return son
}

// Initialize 註冊應用級中間件到 hertz 引擎
func (p *HertzAppMiddlewareProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': initFunc must not be empty", p.Name())
	}

	instance, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}

	cs, ok := instance.(fiberhouse.CoreStarter)
	if !ok {
		return nil, fmt.Errorf("provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name())
	}

	h, ok := cs.GetCoreApp().(*server.Hertz)
	if !ok {
		return nil, fmt.Errorf("provider '%s': core app must be *server.Hertz, got %T", p.Name(), cs.GetCoreApp())
	}

	// 註冊 TraceId 中間件：hertz 無內建 requestid，此處以 uuid 產生並回寫回應頭
	h.Use(requestIDMiddleware())

	// 其他中間件...

	p.SetStatus(fiberhouse.StateLoaded)
	return h, nil
}

// requestIDMiddleware 為每個請求產生追蹤 ID，供恢復中間件與日誌鏈路取用。
//
// 已帶入 X-Request-Id 請求頭時沿用該值，以支援跨服務的鏈路串接。
func requestIDMiddleware() app.HandlerFunc {
	return func(c ctxpkg.Context, reqCtx *app.RequestContext) {
		traceID := string(reqCtx.GetHeader(traceIDHeader))
		if traceID == "" {
			traceID = uuid.NewString()
		}
		reqCtx.Set(traceIDKey, traceID)
		reqCtx.Response.Header.Set(traceIDHeader, traceID)
		reqCtx.Next(c)
	}
}
