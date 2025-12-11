// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

// ApplicationStarterOption 定义应用启动器选项函数类型
type ApplicationStarterOption func(starter ApplicationStarter)

// CommandStarterOption 定义命令启动器选项函数类型
type CommandStarterOption func(starter CommandStarter)

// FrameCmdStarterOption 框架命令行启动器选项函数类型
type FrameCmdStarterOption func(starter FrameCmdStarter)

// CoreCmdStarterOption 核心命令行启动器选项函数类型
type CoreCmdStarterOption func(core CoreCmdStarter)

// FrameStarterOption 定义框架启动器选项函数类型
type FrameStarterOption func(starter FrameStarter)

// CoreStarterOption 定义核心启动器选项函数类型
type CoreStarterOption func(core CoreStarter)
