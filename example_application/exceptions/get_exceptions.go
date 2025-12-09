package exceptions

import (
	"github.com/lamxy/fiberhouse/constant"
	example_module "github.com/lamxy/fiberhouse/example_application/exceptions/example-module"
	"github.com/lamxy/fiberhouse/exception"
)

var (
	exceptions = exception.ExceptionMap{
		"InputParamError":   {400001, "Invalid request parameters", nil},
		"InternalError":     {500001, constant.UnknownErrMsg, "Unknown Internal error"},
		"UnknownError":      {constant.UnknownErrCode, constant.UnknownErrMsg, exception.ErrorData{"msg": "Unknown request error"}},
		"NotFoundDocument":  {400002, "No matching records found", nil},
		"IllegalRequest":    {400003, "Illegal request", nil},
		"NotNeedToUpdate":   {200001, "No records to update", nil},
		"NotNeedToDelete":   {200002, "No records to delete", nil},
		"SqlProxyExecError": {200003, "Sql proxy execute error", nil},
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
