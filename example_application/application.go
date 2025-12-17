package example_application

import (
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/cache"
	"github.com/lamxy/fiberhouse/cache/cache2"
	"github.com/lamxy/fiberhouse/cache/cachelocal"
	"github.com/lamxy/fiberhouse/cache/cacheremote"
	"github.com/lamxy/fiberhouse/component/jsoncodec"
	"github.com/lamxy/fiberhouse/component/validate"
	"github.com/lamxy/fiberhouse/database/dbmongo"
	"github.com/lamxy/fiberhouse/database/dbmysql"
	"github.com/lamxy/fiberhouse/example_application/exceptions"
	"github.com/lamxy/fiberhouse/example_application/middleware"
	"github.com/lamxy/fiberhouse/example_application/providers"
	"github.com/lamxy/fiberhouse/example_application/validatecustom"
	"github.com/lamxy/fiberhouse/globalmanager"
)

// Application 实现Global全局接口
type Application struct {
	name            string // for marking & container key
	Ctx             fiberhouse.IApplicationContext
	instanceFlagMap map[fiberhouse.InstanceKeyFlag]fiberhouse.InstanceKey // 预定义实例KeyName的keyFlag映射
	KeyMongoLog     string
	KeyRedisTest    string
}

// NewApplication new项目应用
func NewApplication(ctx fiberhouse.IApplicationContext) fiberhouse.ApplicationRegister {
	app := &Application{
		name:            "application",
		Ctx:             ctx,
		instanceFlagMap: make(map[fiberhouse.InstanceKeyFlag]fiberhouse.InstanceKey), // 初始化时,预定义好Flag跟实例key的映射
	}
	app.initFlagMap()
	return app
}

// initFlagMap 初始化自定义的实例key映射
func (app *Application) initFlagMap() {
	cfmProvider := providers.NewCustomFlagMapProvider()
	initFunc := func(iProvider fiberhouse.IProvider) (any, error) {
		app.instanceFlagMap["__custom_flag_1"] = "__custom_instance_key_1"
		app.instanceFlagMap["__custom_flag2"] = "__custom_instance_key_2"
		return nil, nil
	}
	_, _ = cfmProvider.Initialize(app.Ctx, initFunc)
}

// GetName 获取应用名称
func (app *Application) GetName() string {
	return app.name
}

// SetName 设置应用名称
func (app *Application) SetName(name string) {
	app.name = name
}

// GetContext 获取应用上下文
func (app *Application) GetContext() fiberhouse.IApplicationContext {
	return app.Ctx
}

// ConfigGlobalInitializers 配置全局对象初始化器
func (app *Application) ConfigGlobalInitializers() globalmanager.InitializerMap {
	return globalmanager.InitializerMap{
		KEY_MONGODB: func() (interface{}, error) {
			confPath := "database.mongodb"
			return dbmongo.NewMongoDb(app.Ctx, confPath)
		},
		KEY_MYSQL: func() (interface{}, error) {
			confPath := "database.mysql"
			return dbmysql.NewMysqlDb(app.Ctx, confPath)
		},
		KEY_REDIS: func() (interface{}, error) {
			confPath := "cache.redis"
			return cacheremote.NewRedisDb(app.Ctx, confPath)
		},
		KEY_EXCEPTIONS: func() (interface{}, error) {
			return exceptions.GetGlobalExceptions(), nil
		},
		KEY_JSON_SONIC_ESCAPE: func() (interface{}, error) {
			return jsoncodec.SonicJsonEscape(), nil
		},
		KEY_JSON_SONIC_FAST: func() (interface{}, error) {
			return jsoncodec.SonicJsonFastest(), nil
		},
		KEY_LOCAL_CACHE: func() (interface{}, error) {
			return cachelocal.NewLocalCache(app.Ctx)
		},
		KEY_REMOTE_CACHE: func() (interface{}, error) {
			return app.GetContext().GetContainer().Get(KEY_REDIS)
		},
		KEY_LEVEL2_CACHE: func() (interface{}, error) {
			localCache, err := app.GetContext().GetContainer().Get(KEY_LOCAL_CACHE)
			if err != nil {
				return nil, err
			}
			remoteCache, err := app.GetContext().GetContainer().Get(KEY_REMOTE_CACHE)
			if err != nil {
				return nil, err
			}
			return cache2.NewLevel2Cache(app.Ctx, localCache.(cache.Cache), remoteCache.(cache.Cache)), nil
		},
	}
}

// ConfigRequiredGlobalKeys 配置并返回全局管理容器中在启动时必须初始化的key
func (app *Application) ConfigRequiredGlobalKeys() []globalmanager.KeyName {
	return []string{KEY_MONGODB, KEY_REDIS, KEY_JSON_SONIC_ESCAPE, KEY_JSON_SONIC_FAST, KEY_MYSQL}
}

// ConfigCustomValidateInitializers 配置并返回自定义更多的语言验证器初始化器
func (app *Application) ConfigCustomValidateInitializers() []validate.ValidateInitializer {
	// 返回自定义语言的验证器初始化器
	return validatecustom.GetValidateInitializers()
}

// ConfigValidatorCustomTags 配置并返回验证器自定义tag函数
func (app *Application) ConfigValidatorCustomTags() []validate.RegisterValidatorTagFunc {
	return validatecustom.GetValidatorTagFuncs()
}

// RegisterAppMiddleware 注册应用中间件
func (app *Application) RegisterAppMiddleware(cs fiberhouse.CoreStarter) {
	middleware.RegisterMiddleware(app.Ctx, cs.GetCoreApp().(*fiber.App))
}

// 统一定义"获取部分必要对象在全局管理容器中的实例Key"   // TODO 独立出来由实例KEY提供者

func (app *Application) GetDBMongoKey() string {
	return KEY_MONGODB
}
func (app *Application) GetDBMysqlKey() string {
	return KEY_MYSQL
}
func (app *Application) GetRedisKey() string {
	return KEY_REDIS
}
func (app *Application) GetFastJsonCodecKey() string {
	return KEY_JSON_SONIC_FAST
}
func (app *Application) GetDefaultJsonCodecKey() string {
	return KEY_JSON_SONIC_ESCAPE
}
func (app *Application) GetTaskDispatcherKey() string {
	return KEY_TASK_CLIENT
}
func (app *Application) GetTaskServerKey() string {
	return KEY_TASK_SERVER
}
func (app *Application) GetDBKey() string {
	return KEY_MONGODB
}
func (app *Application) GetCacheKey() string {
	return KEY_REDIS
}
func (app *Application) GetLocalCacheKey() string {
	return KEY_LOCAL_CACHE
}
func (app *Application) GetRemoteCacheKey() string {
	return KEY_REMOTE_CACHE
}
func (app *Application) GetLevel2CacheKey() string {
	return KEY_LEVEL2_CACHE
}

// GetInstanceKey 获取除框架预定义实例key外的由用户自定义标识映射的实例key
func (app *Application) GetInstanceKey(flag fiberhouse.InstanceKeyFlag) fiberhouse.InstanceKey {
	if ik, ok := app.instanceFlagMap[flag]; ok {
		return ik
	}
	return ""
}

// GetCustomKey 获取自定义实例key，实现了IApplicationCustomizer接口
func (app *Application) GetCustomKey() globalmanager.KeyName {
	// 示例：自定义xxx全局对象key的获取方法
	// 如业务层需要使用时，将application转成IApplicationCustomizer接口，即可调用框架预定义实例key外的更多自定义的实例key
	return "__key_custom_example" // 注意：这里是示例key
}

// RegisterCoreHook 注册核心应用的生命周期钩子函数
func (app *Application) RegisterCoreHook(cs fiberhouse.CoreStarter) {
	coreApp := cs.GetCoreApp().(*fiber.App)
	coreApp.Hooks().OnGroup(func(group fiber.Group) error {
		app.GetContext().GetLogger().InfoWith(app.GetContext().GetConfig().LogOriginFrame()).Str("ApplicationRegister", "Application").Msg("ApplicationRegister OnGroup...")
		return nil
	})
	coreApp.Hooks().OnListen(func(listenData fiber.ListenData) error {
		app.GetContext().GetLogger().InfoWith(app.GetContext().GetConfig().LogOriginFrame()).Str("ApplicationRegister", "Application").Msg("ApplicationRegister OnListen...")
		return nil
	})
	coreApp.Hooks().OnShutdown(func() error {
		app.GetContext().GetLogger().InfoWith(app.GetContext().GetConfig().LogOriginFrame()).Str("ApplicationRegister", "Application").Msg("ApplicationRegister OnShutdown...")
		return nil
	})
	// more hooks...
}
