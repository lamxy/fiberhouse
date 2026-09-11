// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package option

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
)

// ErrRegisterTypeMismatch 标记注册器未实现所需接口。
//
// 装配期的程序设计错误：传入的 IRegister 没有实现目标接口。选项函数在启动链
// 深处执行，panic 信息不含调用现场，因此可分类的哨兵在这里尤其有用。
var ErrRegisterTypeMismatch = errors.New("fiberhouse/option: register does not implement the required interface")

// WithAppRegister 返回应用注册器注入选项函数
//
// 注册器类型不符时 panic。需要处理该错误的调用方请改用 WithAppRegisterWithError。
func WithAppRegister(r fiberhouse.IRegister) fiberhouse.FrameStarterOption {
	opt, err := WithAppRegisterWithError(r)
	if err != nil {
		panic(err)
	}
	return opt
}

// WithAppRegisterWithError 与 WithAppRegister 相同，但类型不符时返回错误而非 panic。
//
// 校验提前到选项构造期而非选项执行期：调用方因此能在装配开始前就拿到错误，
// 而不是等到启动链深处才被 panic 打断。
func WithAppRegisterWithError(r fiberhouse.IRegister) (fiberhouse.FrameStarterOption, error) {
	ar, ok := r.(fiberhouse.ApplicationRegister)
	if !ok {
		return nil, fmt.Errorf("%w: IRegister name '%s' is not an ApplicationRegister",
			ErrRegisterTypeMismatch, r.GetName())
	}
	return func(s fiberhouse.FrameStarter) { s.RegisterApplication(ar) }, nil
}

// WithModuleRegister 返回模块/子系统注册器注入选项函数
//
// 注册器类型不符时 panic。需要处理该错误的调用方请改用 WithModuleRegisterWithError。
func WithModuleRegister(r fiberhouse.IRegister) fiberhouse.FrameStarterOption {
	opt, err := WithModuleRegisterWithError(r)
	if err != nil {
		panic(err)
	}
	return opt
}

// WithModuleRegisterWithError 与 WithModuleRegister 相同，但类型不符时返回错误。
func WithModuleRegisterWithError(r fiberhouse.IRegister) (fiberhouse.FrameStarterOption, error) {
	mr, ok := r.(fiberhouse.ModuleRegister)
	if !ok {
		return nil, fmt.Errorf("%w: IRegister name '%s' is not a ModuleRegister",
			ErrRegisterTypeMismatch, r.GetName())
	}
	return func(s fiberhouse.FrameStarter) { s.RegisterModule(mr) }, nil
}

// WithTaskRegister 返回任务注册器注入选项函数
//
// 注册器类型不符时 panic。需要处理该错误的调用方请改用 WithTaskRegisterWithError。
func WithTaskRegister(r fiberhouse.IRegister) fiberhouse.FrameStarterOption {
	opt, err := WithTaskRegisterWithError(r)
	if err != nil {
		panic(err)
	}
	return opt
}

// WithTaskRegisterWithError 与 WithTaskRegister 相同，但类型不符时返回错误。
func WithTaskRegisterWithError(r fiberhouse.IRegister) (fiberhouse.FrameStarterOption, error) {
	tr, ok := r.(fiberhouse.TaskRegister)
	if !ok {
		return nil, fmt.Errorf("%w: IRegister name '%s' is not a TaskRegister",
			ErrRegisterTypeMismatch, r.GetName())
	}
	return func(s fiberhouse.FrameStarter) { s.RegisterTask(tr) }, nil
}

// WithCoreCfg 返回核心应用配置注入选项函数
func WithCoreCfg(cfg *fiber.Config) fiberhouse.CoreStarterOption {
	return func(s fiberhouse.CoreStarter) {
		if core, ok := s.(*fiberhouse.CoreWithFiber); ok {
			core.CoreCfg = cfg
		}
	}
}
