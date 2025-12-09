// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package option

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
	"github.com/urfave/cli/v2"
)

// WithCmdRegister 返回命令行应用注册器注入选项函数
func WithCmdRegister(r fiberhouse.ApplicationCmdRegister) fiberhouse.FrameCmdStarterOption {
	return func(s fiberhouse.FrameCmdStarter) {
		if cr, ok := r.(fiberhouse.ApplicationCmdRegister); ok {
			s.RegisterApplication(cr)
		} else {
			panic(fmt.Errorf("IRegister name: %s is not an ApplicationCmdRegister", r.GetName()))
		}
	}
}

// WithCoreApp 返回核心命令行应用注入选项函数
func WithCoreApp(core *cli.App) fiberhouse.CoreCmdStarterOption {
	return func(s fiberhouse.CoreCmdStarter) {
		s.RegisterCoreApp(core)
	}
}
