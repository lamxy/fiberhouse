package api

import (
	ctxpkg "context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/lamxy/fiberhouse"
	moduleconstant "github.com/lamxy/fiberhouse/example_application/module/constant"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
)

// HealthHandler 应用健康状态的 hertz 传输层，
// 与 Fiber 适配器在同一路径（GET /health/livez）上暴露一致的契约
type HealthHandler struct {
	fiberhouse.ApiLocator
	Service *service.HealthService
}

// NewHealthHandler 创建 hertz 的健康检查处理器
func NewHealthHandler(ctx fiberhouse.IApplicationContext, serv *service.HealthService) *HealthHandler {
	return &HealthHandler{
		ApiLocator: fiberhouse.NewApi(ctx).SetName(GetKeyHealthHandler()),
		Service:    serv,
	}
}

// GetKeyHealthHandler 获取 HealthHandler 注册到全局管理器的实例 key
func GetKeyHealthHandler(ns ...string) string {
	return fiberhouse.RegisterKeyName("HealthHandler",
		fiberhouse.GetNamespace([]string{moduleconstant.NameModuleExample}, ns...)...)
}

// Liveness 处理 GET /health/livez，用于存活探针
func (ha *HealthHandler) Liveness(c ctxpkg.Context, reqCtx *app.RequestContext) {
	respondData(reqCtx, http.StatusOK, ha.Service.GetHealth())
}
