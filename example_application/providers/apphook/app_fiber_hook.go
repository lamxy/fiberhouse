package apphook

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
)

// RegisterAppCoreHook 注册应用钩子函数
func RegisterFiberAppCoreHook(appCtx fiberhouse.IApplicationContext, cs fiberhouse.CoreStarter) {
	coreApp := cs.GetCoreApp().(*fiber.App)
	coreApp.Hooks().OnGroup(func(group fiber.Group) error {
		appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).Str("ApplicationRegister", "Application").Msg("ApplicationRegister OnGroup...")
		return nil
	})
	coreApp.Hooks().OnListen(func(listenData fiber.ListenData) error {
		appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).Str("ApplicationRegister", "Application").Msg("ApplicationRegister OnListen...")
		return nil
	})
	coreApp.Hooks().OnShutdown(func() error {
		appCtx.GetLogger().InfoWith(appCtx.GetConfig().LogOriginFrame()).Str("ApplicationRegister", "Application").Msg("ApplicationRegister OnShutdown...")
		return nil
	})
	// more hooks...
}
