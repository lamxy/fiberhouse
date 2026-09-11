package api

import (
	ctxpkg "context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/lamxy/fiberhouse"
	hertzadaptor "github.com/lamxy/fiberhouse/example_application/hertzcore/adaptor"
	moduleconstant "github.com/lamxy/fiberhouse/example_application/module/constant"
)

// BinaryHandler 二進制回應協商的示範處理器。
//
// 模組其餘 handler 經 respondData 走 JsonWithCtx，永遠是 JSON；
// 只有 ResponseWrap.SendWithCtx 會依請求 MIME 協商 MsgPack / Protobuf。
// 本 handler 是倉庫內唯一實際執行該協商路徑的示例，
// 對應 feature-status.md 對 MsgPack / Protobuf 能力的晉級條件 5。
type BinaryHandler struct {
	fiberhouse.ApiLocator
}

// NewBinaryHandler 建立二進制示範處理器，無依賴注入需求
func NewBinaryHandler(ctx fiberhouse.IApplicationContext) *BinaryHandler {
	return &BinaryHandler{
		ApiLocator: fiberhouse.NewApi(ctx).SetName(GetKeyBinaryHandler()),
	}
}

// GetKeyBinaryHandler 取得 BinaryHandler 註冊到全域管理器的實例 key
func GetKeyBinaryHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("BinaryHandler",
		fiberhouse.GetNamespace([]string{moduleconstant.NameModuleExample}, ns...)...)
}

// newBinaryEchoData 建立二進制示範回應的 data 負載。
//
// 刻意回傳 map[string]interface{} 而非自訂 struct：Protobuf 分支的
// RespInfoPB.Reset 以 structpb.NewValue 轉換 data，而該函式只接受
// nil、bool、數值、string、[]byte、map[string]interface{} 與 []interface{}，
// 傳入自訂 struct 會失敗。Reset 又把轉換錯誤吞掉（response_proto_impl.go:109-117
// 的兩處 if err == nil 都沒有 else），失敗時 data 靜默留為 nil——
// 實測自訂 struct 的 Protobuf body 只剩 4 bytes，code 與 data 全失。
// 用 map 可讓同一份 payload 在 JSON、MsgPack、Protobuf 三條路徑都完整往返。
func newBinaryEchoData(format, echo string) map[string]interface{} {
	return map[string]interface{}{
		"format": format,
		"echo":   echo,
	}
}

// Negotiate 處理 GET /examples/binary，示範二進制回應協商。
//
// 請求端以 Content-Type 或 Accept 指定 application/msgpack 或
// application/x-protobuf 即可取得對應二進制 body；未指定、指定 JSON、
// 通配符或未知 MIME 一律回退 JSON，且不會回 406。
//
// 協商由 ResponseWrap.SendWithCtx 完成，前提是 BootConfig
// EnableBinaryProtocolSupport 為 true（example_main 已啟用）。
//
// 注意：本模組不標註 swaggo @Router，註解單一來源於 Fiber handler，
// 見 route_register_hertz_provider.go 的 RegisterHertzSwagger 說明。
func (h *BinaryHandler) Negotiate(c ctxpkg.Context, reqCtx *app.RequestContext) {
	echo := reqCtx.DefaultQuery("echo", "hello")

	// 回應的實際格式由請求 MIME 決定，此處不預先判斷；
	// 回填 format 只是讓 JSON 回退時，呼叫方仍能看出走的是哪條路徑。
	format := string(reqCtx.GetHeader("Content-Type"))
	if format == "" {
		format = string(reqCtx.GetHeader("Accept"))
	}
	if format == "" {
		format = "application/json"
	}

	_ = fiberhouse.Response().
		SuccessWithData(newBinaryEchoData(format, echo)).
		SendWithCtx(hertzadaptor.WithHertzContext(reqCtx), http.StatusOK)
}
