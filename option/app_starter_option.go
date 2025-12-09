// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package option

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse"
)

// WithAppRegister 返回应用注册器注入选项函数
func WithAppRegister(r fiberhouse.IRegister) fiberhouse.FrameStarterOption {
	return func(s fiberhouse.FrameStarter) {
		if ar, ok := r.(fiberhouse.ApplicationRegister); ok {
			s.RegisterApplication(ar)
		} else {
			panic(fmt.Errorf("IRegister name: '%s' is not an ApplicationRegister", r.GetName()))
		}
	}
}

// WithModuleRegister 返回模块/子系统注册器注入选项函数
func WithModuleRegister(r fiberhouse.IRegister) fiberhouse.FrameStarterOption {
	return func(s fiberhouse.FrameStarter) {
		if mr, ok := r.(fiberhouse.ModuleRegister); ok {
			s.RegisterModule(mr)
		} else {
			panic(fmt.Errorf("IRegister name: '%s' is not a ModuleRegister", r.GetName()))
		}
	}
}

// WithTaskRegister 返回任务注册器注入选项函数
func WithTaskRegister(r fiberhouse.IRegister) fiberhouse.FrameStarterOption {
	return func(s fiberhouse.FrameStarter) {
		if tr, ok := r.(fiberhouse.TaskRegister); ok {
			s.RegisterTask(tr)
		} else {
			panic(fmt.Errorf("IRegister name: '%s' is not a TaskRegister", r.GetName()))
		}
	}
}

// WithCoreCfg 返回核心应用配置注入选项函数
func WithCoreCfg(cfg *fiber.Config) fiberhouse.CoreStarterOption {
	return func(s fiberhouse.CoreStarter) {
		s.RegisterCoreCfg(cfg)
	}
}
