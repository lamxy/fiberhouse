package handler

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/task"
)

// RegisterTaskHandlers registers the example module's stable task contracts.
func RegisterTaskHandlers(tk fiberhouse.TaskRegister) {
	tk.AddTaskHandlerToMap(task.TypeExampleChanged, HandleExampleChangedTask)
}
