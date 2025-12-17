package fiberhouse

import (
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
	frameUtils "github.com/lamxy/fiberhouse/utils"
)

type IRecover interface {
	DefaultStackTraceHandler(providerCtx.ICoreContext, interface{})
	ErrorHandler(providerCtx.ICoreContext, error) error
	GetContext() IApplicationContext
}

var (
	debugFlag      = "X-your-custom-debug-flag"             // 自定义debug标记key，由后端recover配置定义覆盖
	debugFlagValue = "f0dc4970-ed31-4598-acd8-b5c5fd66c12e" // 自定义debug标记值，由后端recover配置定义覆盖
	requestID      = "traceId"                              // 请求ID字段名称，由后端trace配置定义覆盖
)

// ErrorStack 获取当前的堆栈信息字符串
func ErrorStack(debugStack ...bool) string {
	//if len(debugStack) > 0 && debugStack[0] {
	//	return frameUtils.StackMsg()
	//}
	return frameUtils.CaptureStack()
}
