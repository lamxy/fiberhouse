// Package api 提供 example 模組的 hertz 傳輸層。
//
// 與 Fiber/Gin 適配器暴露相同的 CRUD 契約，差異僅在於錯誤傳遞方式：
// hertz 沒有 gin 的 c.Error() 錯誤鏈與對應的錯誤處理中間件，
// 故此處在處理器內直接以框架統一回應格式輸出錯誤。
package api

import (
	ctxpkg "context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/lamxy/fiberhouse"
	hertzadaptor "github.com/lamxy/fiberhouse/example_application/hertzcore/adaptor"
	moduleconstant "github.com/lamxy/fiberhouse/example_application/module/constant"
)

// language 取得請求語言，缺省為 en
func language(reqCtx *app.RequestContext) string {
	if lang := string(reqCtx.GetHeader(moduleconstant.XLanguageFlag)); lang != "" {
		return lang
	}
	return "en"
}

// respondError 以框架統一的錯誤處理器輸出錯誤。
//
// hertz 沒有 gin 的 c.Error() 錯誤鏈與對應的錯誤處理中間件，
// 故此處直接委派給框架的 IErrorHandler.ErrorHandler：它已實作
// fiber.Error → ValidateException → Exception → 未知錯誤的完整分類與狀態碼映射，
// 藉此保證 hertz 與 Fiber/Gin 對外的錯誤契約完全一致。
func respondError(ctx fiberhouse.IContext, reqCtx *app.RequestContext, err error) {
	eh := fiberhouse.NewErrorHandlerOnce(ctx.(fiberhouse.IApplicationContext))
	_ = eh.ErrorHandler(hertzadaptor.WithHertzContext(reqCtx), err)
}

// respondData 以框架統一回應格式輸出成功資料
func respondData(reqCtx *app.RequestContext, status int, data interface{}) {
	_ = fiberhouse.Response().
		SuccessWithData(data).
		JsonWithCtx(hertzadaptor.WithHertzContext(reqCtx), status)
}

// requestContext 取得請求關聯的標準 context，供用例層使用
func requestContext(c ctxpkg.Context) ctxpkg.Context {
	if c == nil {
		return ctxpkg.Background()
	}
	return c
}
