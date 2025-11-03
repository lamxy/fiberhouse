// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package commandstarter

import "github.com/lamxy/fiberhouse/frame"

// CMDLineApplication 是组合命令行应用启动器，实现 frame.FrameCmdStarter 和 frame.CoreCmdStarter 接口。
type CMDLineApplication struct {
	frame.FrameCmdStarter
	frame.CoreCmdStarter
}
