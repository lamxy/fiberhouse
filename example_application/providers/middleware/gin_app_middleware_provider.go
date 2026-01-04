package middleware

import (
	"fmt"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
)

// GinAppMiddlewareProvider 基于Gin的应用级中间件提供者
type GinAppMiddlewareProvider struct {
	*fiberhouse.Provider
}

// NewGinAppMiddlewareProvider 创建Gin应用中间件提供者实例
func NewGinAppMiddlewareProvider() *GinAppMiddlewareProvider {
	son := &GinAppMiddlewareProvider{
		Provider: fiberhouse.NewProvider(),
	}
	son.SetName("GinAppMiddlewareProvider").
		SetTarget("gin").
		SetType(fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType)
	son.MountToParent(son)
	return son
}

// Initialize 初始化并注册中间件到Gin引擎
func (g *GinAppMiddlewareProvider) Initialize(ctx fiberhouse.IContext, initFunc ...fiberhouse.ProviderInitFunc) (any, error) {
	if len(initFunc) == 0 {
		return nil, fmt.Errorf("Provider '%s': initFunc must not be empty", g.Name())
	}

	instance, err := initFunc[0](g)
	if err != nil {
		return nil, err
	}

	cs, ok := instance.(fiberhouse.CoreStarter)
	if !ok {
		return nil, fmt.Errorf("Provider '%s': initFunc must return fiberhouse.CoreStarter instance", g.Name())
	}

	app, ok := cs.GetCoreApp().(*gin.Engine)
	if !ok {
		return nil, fmt.Errorf("Provider '%s': core app must be *gin.Engine", g.Name())
	}

	// 注册 TraceId 中间件
	app.Use(requestid.New())

	// 注册基本认证中间件
	authorized := app.Group("/uuid")
	authorized.Use(gin.BasicAuth(gin.Accounts{
		"admin": "admin123",
	}))

	// 注册 CSRF 中间件
	//app.Use(csrf.Middleware(csrf.Options{
	//	Secret: "csrf-secret-key-32bytes-long!!!",
	//	ErrorFunc: func(c *gin.Context) {
	//		_ = c.Error(errors.New( "CSRF token mismatch"))
	//		c.Abort()
	//	},
	//}))

	g.SetStatus(fiberhouse.StateLoaded)
	return app, nil
}
