package tags

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/lamxy/fiberhouse/component/validate"
	"reflect"
)

// HascoursesRegisterValidation 注册新的自定义tag
func HascoursesRegisterValidation(wrap *validate.Wrap) error {
	var tagName = "hascourses"
	for l := range wrap.GetValidators() {
		if err := wrap.GetValidate(l).RegisterValidation(tagName, func(fl validator.FieldLevel) bool {
			if fl.Field().Kind() != reflect.Slice {
				return false
			} else {
				if fl.Field().Len() > 1 {
					return true
				}
			}
			return false
		}); err != nil {
			return err
		}
	}
	return nil
}

// HascoursesRegisterTranslation 注册新的自定义tag的翻译提示
func HascoursesRegisterTranslation(wrap *validate.Wrap) error {
	var (
		tagName = "hascourses"
		la      string
	)
	for i := 0; i < len(wrap.GetLangList()); i++ {
		la = wrap.GetLangList()[i]
		switch la {
		case "zh":
			if err := wrap.GetValidate(la).RegisterTranslation(tagName, wrap.GetTranslators()[la], func(ut ut.Translator) error {
				return ut.Add(tagName, "{0} 必须是数组，并且数组长度大于 {1}", true)
			}, func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tagName, fe.Field(), fe.Param())
				return t
			}); err != nil {
				return err
			}
		case "en":
			if err := wrap.GetValidate(la).RegisterTranslation(tagName, wrap.GetTranslators()[la], func(ut ut.Translator) error {
				return ut.Add(tagName, "{0} must be array and length large than 1", true)
			}, func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tagName, fe.Field())
				return t
			}); err != nil {
				return err
			}
		case "zh_tw":
			if err := wrap.GetValidate(la).RegisterTranslation(tagName, wrap.GetTranslators()[la], func(ut ut.Translator) error {
				return ut.Add(tagName, "{0} 必須是陣列，並且陣列長度大於 {1}", true)
			}, func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tagName, fe.Field(), fe.Param())
				return t
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
