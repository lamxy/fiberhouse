package module

import (
	"context"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/cache"
	"github.com/lamxy/fiberhouse/component/tasklog"
	exampleTaskHandler "github.com/lamxy/fiberhouse/example_application/module/example-module/task/handler"
)

// TaskAsync 任务注册器
type TaskAsync struct {
	name           string // 用于标记注册器名称或用于容器的keyName
	Ctx            fiberhouse.IApplicationContext
	taskHandlerMap map[string]func(context.Context, *asynq.Task) error
}

func NewTaskAsync(ctx fiberhouse.IApplicationContext) fiberhouse.TaskRegister {
	return &TaskAsync{
		name:           "task",
		Ctx:            ctx,
		taskHandlerMap: make(map[string]func(context.Context, *asynq.Task) error, 64), // 初始化任务处理器map 预设容量50
	}
}

// GetName 获取注册器名称
func (ta *TaskAsync) GetName() string {
	return ta.name
}

// SetName 设置注册器名称
func (ta *TaskAsync) SetName(name string) {
	ta.name = name
}

// GetContext 获取应用上下文
func (ta *TaskAsync) GetContext() fiberhouse.IApplicationContext {
	return ta.Ctx
}

// GetTaskHandlerMap 获取任务处理器map
func (ta *TaskAsync) GetTaskHandlerMap() map[string]func(context.Context, *asynq.Task) error {
	// 注册任务处理器到map
	// 通过调用各模块下的任务处理器注册函数，统一注册任务处理器到map
	// 例如：exampleTaskHandler.RegisterTaskHandlers(ta)
	// 注意：每个模块下的任务处理器注册函数内，需要调用AddTaskHandlerToMap方法将任务名和处理器添加到map中
	// 以便在启动任务工作器时，能够正确注册所有任务处理器

	// 注册 example-module 模块下的任务处理器
	exampleTaskHandler.RegisterTaskHandlers(ta)

	// 注册更多的模块下的任务处理器...

	return ta.taskHandlerMap
}

// AddTaskHandlerToMap 添加新的任务名和任务处理器到map
func (ta *TaskAsync) AddTaskHandlerToMap(pattern string, handler func(context.Context, *asynq.Task) error) {
	if _, exists := ta.taskHandlerMap[pattern]; exists {
		ta.Ctx.GetLogger().Warn().Msgf("failed to add TaskHandler to Map: task handler for pattern '%s'", pattern)
		return
	}
	ta.taskHandlerMap[pattern] = handler
}

// RegisterKeyTaskServer 注册异步任务服务器/工作器初始化器到全局容器
func (ta *TaskAsync) RegisterTaskServerToContainer() {
	if !ta.Ctx.GetConfig().Bool("application.task.enableServer") {
		return
	}
	key := ta.Ctx.GetStarterApp().GetApplication().GetRedisKey()

	cacheIns, err := ta.Ctx.GetContainer().Get(key)
	if err != nil {
		panic(err.Error())
	}
	rdb := cacheIns.(cache.IRedisClient)
	ta.Ctx.GetContainer().Register(ta.Ctx.GetStarterApp().GetApplication().GetTaskServerKey(), func() (interface{}, error) {
		// 注入应用上下文
		worker := fiberhouse.NewTaskWorker(ta.GetContext(), rdb.GetRedisClient(), asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			Logger:   tasklog.NewTaskLoggerAdapter(ta.Ctx), // 任务日志适配器，统一接入框架系统日志器
			LogLevel: asynq.WarnLevel,                      // 指定日志级别
		})
		return worker, nil
	})
}

// RegisterKeyTaskDispatcher 注册异步任务分发器初始化器到全局容器
func (ta *TaskAsync) RegisterTaskDispatcherToContainer() {
	if !ta.Ctx.GetConfig().Bool("application.task.enableServer") {
		return
	}
	cacheIns, err := ta.Ctx.GetContainer().Get(ta.Ctx.GetStarterApp().GetApplication().GetRedisKey())
	if err != nil {
		panic(err.Error())
	}
	rdb := cacheIns.(cache.IRedisClient)
	ta.Ctx.GetContainer().Register(ta.Ctx.GetStarterApp().GetApplication().GetTaskDispatcherKey(), func() (interface{}, error) {
		dispatcher := fiberhouse.NewTaskDispatcher(rdb.GetRedisClient())
		return dispatcher, nil
	})
}

// GetTaskDispatcher 从容器获取任务分发器实例
func (ta *TaskAsync) GetTaskDispatcher() (*fiberhouse.TaskDispatcher, error) {
	key := ta.Ctx.GetStarterApp().GetApplication().GetTaskDispatcherKey()
	instance, err := ta.Ctx.GetContainer().Get(key)
	if err != nil {
		return nil, err
	}
	if result, ok := instance.(*fiberhouse.TaskDispatcher); ok {
		return result, nil
	}
	return nil, fmt.Errorf("assertion failure for type of '%s' instance", key)
}

// GetTaskWorker 从容器获取任务工作服务器实例
func (ta *TaskAsync) GetTaskWorker(key string) (*fiberhouse.TaskWorker, error) {
	// 启动任务服务
	instance, err := ta.Ctx.GetContainer().Get(key)
	if err != nil {
		return nil, err
	}
	if worker, ok := instance.(*fiberhouse.TaskWorker); ok {
		return worker, nil
	}
	return nil, fmt.Errorf("assertion failure for type of '%s' instance", key)
}
