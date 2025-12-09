package example

import "github.com/lamxy/fiberhouse/component/validate"

// GetValidatorTagFuncs 获取用于注册自定义的校验标签的函数列表，该函数可以接受*Wrap验证包装器对象并将自定义标签和翻译注册到验证器中
func GetValidatorTagFuncs() []validate.RegisterValidatorTagFunc {
	return []validate.RegisterValidatorTagFunc{
		HascoursesRegisterValidation,  // 验证标签 hascourses 的注册函数
		HascoursesRegisterTranslation, // 验证标签 hascourses 的翻译注册函数
	}
}
