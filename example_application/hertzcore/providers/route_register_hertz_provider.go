package providers

import (
	"fmt"

	"github.com/cloudwego/hertz/pkg/app/server"
	hertzSwagger "github.com/hertz-contrib/swagger"
	"github.com/lamxy/fiberhouse"
	hertzconst "github.com/lamxy/fiberhouse/example_application/hertzcore/constant"
	exampleHertzApi "github.com/lamxy/fiberhouse/example_application/module/example-hertzapi-module/api"
	swaggerFiles "github.com/swaggo/files"
)

// HertzRouteRegisterProvider 基於 hertz 的模組路由與 swagger 註冊提供者
type HertzRouteRegisterProvider struct {
	fiberhouse.IProvider
}

// NewHertzRouteRegisterProvider 建立 hertz 路由註冊提供者
func NewHertzRouteRegisterProvider() *HertzRouteRegisterProvider {
	son := &HertzRouteRegisterProvider{
		IProvider: fiberhouse.NewProvider().
			SetName("HertzRouteRegisterProvider").
			SetTarget(hertzconst.CoreTypeWithHertz).
			SetType(fiberhouse.ProviderTypeDefault().GroupRouteRegisterType),
	}
	son.MountToParent(son)
	return son
}

// Initialize 註冊 hertz 路由處理器
func (p *HertzRouteRegisterProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("provider '%s': no initFunc provided", p.Name())
	}

	instance, err := initFunc[0](p)
	if err != nil {
		return nil, err
	}

	cs, ok := instance.(fiberhouse.CoreStarter)
	if !ok {
		return nil, fmt.Errorf("provider '%s': initFunc must return fiberhouse.CoreStarter instance", p.Name())
	}

	appCtx := ctx.(fiberhouse.IApplicationContext)

	// 註冊路由
	RegisterHertzRouteHandlers(appCtx, cs)

	// 註冊 Swagger 路由
	RegisterHertzSwagger(appCtx, cs)

	return nil, nil
}

// RegisterHertzRouteHandlers 註冊各業務模組的 hertz 路由處理器
func RegisterHertzRouteHandlers(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	h, ok := cs.GetCoreApp().(*server.Hertz)
	if !ok {
		return
	}

	// 註冊 example 模組的路由處理器
	exampleHertzApi.RegisterRouteHandlers(ctx, h)

	// TODO 註冊更多業務模組路由處理器 ...
}

// RegisterHertzSwagger 註冊 Swagger UI 路由
//
// 與 Fiber/Gin 適配器一致，由 application.swagger.enable 配置開關控制。
// 注意：swaggo 註解單一來源於 Fiber handler（見 example_application/docs/README.md），
// 本適配器僅負責掛載 UI 路由，不重複標註 @Router，避免 swag init 產生重複路由衝突。
func RegisterHertzSwagger(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	h, ok := cs.GetCoreApp().(*server.Hertz)
	if !ok {
		return
	}

	if !ctx.GetConfig().Bool("application.swagger.enable") {
		return
	}

	h.GET("/swagger/*any", hertzSwagger.WrapHandler(swaggerFiles.Handler))

	// TODO 設置安全訪問配置
}
