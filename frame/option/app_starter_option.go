package option

import (
	"fmt"
	"github.com/lamxy/fiberhouse/frame"
)

// WithAppRegister 返回应用注册器注入选项函数
func WithAppRegister(r frame.IRegister) frame.ApplicationStarterOption {
	return func(s frame.ApplicationStarter) {
		if ar, ok := r.(frame.ApplicationRegister); ok {
			s.RegisterApplication(ar)
		} else {
			panic(fmt.Errorf("IRegister name: '%s' is not an ApplicationRegister", r.GetName()))
		}
	}
}

// WithModuleRegister 返回模块/子系统注册器注入选项函数
func WithModuleRegister(r frame.IRegister) frame.ApplicationStarterOption {
	return func(s frame.ApplicationStarter) {
		if mr, ok := r.(frame.ModuleRegister); ok {
			s.RegisterModule(mr)
		} else {
			panic(fmt.Errorf("IRegister name: '%s' is not a ModuleRegister", r.GetName()))
		}
	}
}

// WithTaskRegister 返回任务注册器注入选项函数
func WithTaskRegister(r frame.IRegister) frame.ApplicationStarterOption {
	return func(s frame.ApplicationStarter) {
		if tr, ok := r.(frame.TaskRegister); ok {
			s.RegisterTask(tr)
		} else {
			panic(fmt.Errorf("IRegister name: '%s' is not a TaskRegister", r.GetName()))
		}
	}
}
