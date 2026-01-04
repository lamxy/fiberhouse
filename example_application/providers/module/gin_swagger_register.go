package module

import (
	"github.com/gin-gonic/gin"
	"github.com/lamxy/fiberhouse"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterGinSwagger 注册Swagger UI route
func RegisterGinSwagger(ctx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	app := cs.GetCoreApp().(*gin.Engine)
	registerOrNot := ctx.GetConfig().Bool("application.swagger.enable")
	if registerOrNot {
		app.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// todo 设置安全访问配置

	}
}
