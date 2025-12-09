package validatecustom

import (
	"github.com/lamxy/fiberhouse/component/validate"
	"github.com/lamxy/fiberhouse/example_application/validatecustom/validators"
)

// GetValidateInitializers 获取自定义的验证器初始化器列表
func GetValidateInitializers() []validate.ValidateInitializer {
	return []validate.ValidateInitializer{
		validators.GetJaValidateInitializer(),
		validators.GetKoValidateInitializer(),
	}
}
