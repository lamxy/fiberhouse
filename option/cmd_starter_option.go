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
//
// 注册器类型不符时 panic。需要处理该错误的调用方请改用 WithCmdRegisterWithError。
func WithCmdRegister(r fiberhouse.ApplicationCmdRegister) fiberhouse.FrameCmdStarterOption {
	opt, err := WithCmdRegisterWithError(r)
	if err != nil {
		panic(err)
	}
	return opt
}

// WithCmdRegisterWithError 与 WithCmdRegister 相同，但类型不符时返回错误而非 panic。
//
// 注：参数类型已是 ApplicationCmdRegister，故类型断言只在传入 nil 接口时才会
// 失败。保留校验并给出可分类错误，与 app_starter_option.go 的三个同构入口一致。
func WithCmdRegisterWithError(r fiberhouse.ApplicationCmdRegister) (fiberhouse.FrameCmdStarterOption, error) {
	cr, ok := interface{}(r).(fiberhouse.ApplicationCmdRegister)
	if !ok || r == nil {
		return nil, fmt.Errorf("%w: register is not an ApplicationCmdRegister",
			ErrRegisterTypeMismatch)
	}
	return func(s fiberhouse.FrameCmdStarter) { s.RegisterApplication(cr) }, nil
}

// WithCoreApp 返回核心命令行应用注入选项函数
func WithCoreApp(core *cli.App) fiberhouse.CoreCmdStarterOption {
	return func(s fiberhouse.CoreCmdStarter) {
		s.RegisterCoreApp(core)
	}
}
