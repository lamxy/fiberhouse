package validatecustom

import (
	"github.com/lamxy/fiberhouse/component/validate"
	tags2 "github.com/lamxy/fiberhouse/example_application/providers/validatecustom/tags"
)

// GetValidatorTagFuncs 获取注册指定或自定义tag及翻译提示
func GetValidatorTagFuncs() []validate.RegisterValidatorTagFunc {
	return []validate.RegisterValidatorTagFunc{
		tags2.StartswithRegisterTranslation,
		tags2.HascoursesRegisterValidation,
		tags2.HascoursesRegisterTranslation,
	}
}
