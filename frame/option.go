// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package frame

// ApplicationStarterOption 定义应用启动器选项函数类型
type ApplicationStarterOption func(starter ApplicationStarter)

// CommandStarterOption 定义命令启动器选项函数类型
type CommandStarterOption func(starter CommandStarter)

type FrameStarterOption func(starter FrameStarter)

type CoreStarterOption func(core CoreStarter)
