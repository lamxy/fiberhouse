package applicationstarter

import "github.com/lamxy/fiberhouse/frame"

// WebApplication Web应用启动器，框架和核心启动器组合体，实现了 frame.FrameStarter 和 frame.CoreStarter 接口
type WebApplication struct {
	frame.FrameStarter
	frame.CoreStarter
}
