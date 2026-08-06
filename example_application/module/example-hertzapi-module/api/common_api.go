package api

import (
	ctxpkg "context"
	"fmt"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/lamxy/fiberhouse"
	moduleconstant "github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
)

// CommonHandler 示例公共處理器，繼承自 fiberhouse.ApiLocator，
// 具備取得上下文、配置、日誌、註冊實例等能力
type CommonHandler struct {
	fiberhouse.ApiLocator
	KeyTestService string
}

// NewCommonHandler 建立公共處理器，內部依賴走全域管理器延遲取得
func NewCommonHandler(ctx fiberhouse.IApplicationContext) *CommonHandler {
	return &CommonHandler{
		ApiLocator:     fiberhouse.NewApi(ctx).SetName(GetKeyCommonHandler()),
		KeyTestService: service.RegisterKeyTestService(ctx),
	}
}

// GetKeyCommonHandler 取得 CommonHandler 註冊到全域管理器的實例 key
func GetKeyCommonHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("CommonHandler",
		fiberhouse.GetNamespace([]string{moduleconstant.NameModuleExample}, ns...)...)
}

// TestGetInstance 測試以 h.GetInstance(key) 取得註冊實例
func (h *CommonHandler) TestGetInstance(c ctxpkg.Context, reqCtx *app.RequestContext) {
	t := reqCtx.DefaultQuery("t", "test")

	testService, err := h.GetInstance(h.KeyTestService)
	if err != nil {
		respondError(h.GetContext(), reqCtx,err)
		return
	}

	if ts, ok := testService.(*service.TestService); ok {
		respondData(reqCtx, http.StatusOK, t+":"+ts.HelloWorld())
		return
	}
	respondData(reqCtx, http.StatusOK, t)
}

// TestGetMustInstance 測試以 fiberhouse.GetMustInstance[T](key) 泛型方法取得註冊實例
func (h *CommonHandler) TestGetMustInstance(c ctxpkg.Context, reqCtx *app.RequestContext) {
	t := reqCtx.DefaultQuery("t", "test")
	testService := fiberhouse.GetMustInstance[*service.TestService](h.KeyTestService)
	respondData(reqCtx, http.StatusOK, t+testService.HelloWorld())
}

// TestGetMustInstanceFailed 測試取得註冊實例失敗的情形，
// 以錯誤型別取得實例會觸發 panic，此處捕獲後轉為統一錯誤回應
func (h *CommonHandler) TestGetMustInstanceFailed(c ctxpkg.Context, reqCtx *app.RequestContext) {
	t := reqCtx.DefaultQuery("t", "test")

	defer func() {
		if r := recover(); r != nil {
			respondError(h.GetContext(), reqCtx,fmt.Errorf("get instance failed: %v", r))
		}
	}()

	testService := fiberhouse.GetMustInstance[service.TestService](h.KeyTestService)
	respondData(reqCtx, http.StatusOK, t+testService.HelloWorld())
}
