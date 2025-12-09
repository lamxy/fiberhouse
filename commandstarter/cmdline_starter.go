// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package commandstarter

import "github.com/lamxy/fiberhouse"

// CMDLineApplication 是组合命令行应用启动器，实现 fiberhouse.FrameCmdStarter 和 fiberhouse.CoreCmdStarter 接口。
type CMDLineApplication struct {
	fiberhouse.FrameCmdStarter
	fiberhouse.CoreCmdStarter
}
