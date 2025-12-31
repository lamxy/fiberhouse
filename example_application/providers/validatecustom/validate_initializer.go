package validatecustom

import (
	"github.com/lamxy/fiberhouse/component/validate"
	validators2 "github.com/lamxy/fiberhouse/example_application/providers/validatecustom/validators"
)

// GetValidateInitializers 获取自定义的验证器初始化器列表
func GetValidateInitializers() []validate.ValidateInitializer {
	return []validate.ValidateInitializer{
		validators2.GetJaValidateInitializer(),
		validators2.GetKoValidateInitializer(),
	}
}
