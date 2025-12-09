package handler

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
)

// RegisterTaskHandlers 统一注册任务处理函数和依赖的组件实例初始化器
func RegisterTaskHandlers(tk fiberhouse.TaskRegister) {
	// append task handler to global taskHandlerMap
	// 通过RegisterKeyXXX注册任务处理的实例初始化器，并获取注册实例的keyName

	// 统一注册全局管理实例初始化器，该实例可在任务处理函数中通过tk.GetContext().GetContainer().GetXXXService()获取，用来执行具体的任务处理逻辑
	service.RegisterKeyTestService(tk.GetContext())

	// 统一追加任务处理函数到Task对象任务名称映射中
	tk.AddTaskHandlerToMap(task.TypeExampleCreate, HandleExampleCreateTask)

}
