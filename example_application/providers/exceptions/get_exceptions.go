package exceptions

import (
	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/example_application/providers/exceptions/example-module"
	"github.com/lamxy/fiberhouse/exception"
)

var (
	exceptions = exception.ExceptionMap{
		"InputParamError": {
			Code: 400001,
			Msg:  "Invalid request parameters",
			Data: nil,
		},
		"InternalError": {
			Code: 500001,
			Msg:  constant.UnknownErrMsg,
			Data: "Unknown Internal error",
		},
		"UnknownError": {
			Code: constant.UnknownErrCode,
			Msg:  constant.UnknownErrMsg,
			Data: exception.ErrorData{"msg": "Unknown request error"},
		},
		"NotFoundDocument": {
			Code: 400002,
			Msg:  "No matching records found",
			Data: nil,
		},
		"IllegalRequest": {
			Code: 400003,
			Msg:  "Illegal request",
			Data: nil,
		},
		"NotNeedToUpdate": {
			Code: 200001,
			Msg:  "No records to update",
			Data: nil,
		},
		"NotNeedToDelete": {
			Code: 200002,
			Msg:  "No records to delete",
			Data: nil,
		},
		"SqlProxyExecError": {
			Code: 200003,
			Msg:  "Sql proxy execute error",
			Data: nil,
		},
	}
)

// GetGlobalExceptions 获取所有系统模块的异常map
func GetGlobalExceptions() exception.ExceptionMap {
	AllExceptions := []exception.ExceptionMap{
		example_module.GetExampleExceptions(), // 获取example业务模块异常map
		// 更多系统模块的异常map ...
	}
	return MergeExceptions(AllExceptions...) // 获取各系统模块的异常map
}

// MergeExceptions 合并多个异常map
func MergeExceptions(exceptionMaps ...exception.ExceptionMap) exception.ExceptionMap {
	for _, m := range exceptionMaps {
		for k := range m {
			exceptions[k] = m[k]
		}
	}
	return exceptions
}
