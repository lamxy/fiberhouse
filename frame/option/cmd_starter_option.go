// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package option

import (
	"fmt"
	"github.com/lamxy/fiberhouse/frame"
	"github.com/urfave/cli/v2"
)

// WithCmdRegister 返回命令行应用注册器注入选项函数
func WithCmdRegister(r frame.ApplicationCmdRegister) frame.FrameCmdStarterOption {
	return func(s frame.FrameCmdStarter) {
		if cr, ok := r.(frame.ApplicationCmdRegister); ok {
			s.RegisterApplication(cr)
		} else {
			panic(fmt.Errorf("IRegister name: %s is not an ApplicationCmdRegister", r.GetName()))
		}
	}
}

// WithCoreApp 返回核心命令行应用注入选项函数
func WithCoreApp(core *cli.App) frame.CoreCmdStarterOption {
	return func(s frame.CoreCmdStarter) {
		s.RegisterCoreApp(core)
	}
}
