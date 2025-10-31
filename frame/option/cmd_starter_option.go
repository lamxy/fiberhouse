package option

import (
	"fmt"
	"github.com/lamxy/fiberhouse/frame"
)

// WithCmdRegister 返回命令行应用注册器注入选项函数
func WithCmdRegister(r frame.ApplicationCmdRegister) frame.CommandStarterOption {
	return func(s frame.CommandStarter) {
		if cr, ok := r.(frame.ApplicationCmdRegister); ok {
			s.RegisterApplication(cr)
		} else {
			panic(fmt.Errorf("IRegister name: %s is not an ApplicationCmdRegister", r.GetName()))
		}
	}
}
