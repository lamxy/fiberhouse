package tags

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/lamxy/fiberhouse/component/validate"
)

// StartswithRegisterTranslation 为已存在的tag: startswith 注册指定语言的翻译提示（如该tag缺失指定语言的翻译提示，在此处自定义实现）
func StartswithRegisterTranslation(wrap *validate.Wrap) error {
	var tagName = "startswith"
	var la string
	for i := 0; i < len(wrap.GetLangList()); i++ {
		la = wrap.GetLangList()[i]
		switch la {
		case validate.LangZhCN:
			if err := wrap.GetValidate(la).RegisterTranslation(tagName, wrap.GetTranslators()[la], func(ut ut.Translator) error {
				return ut.Add(tagName, "{0} 必须以 {1} 开头", true)
			}, func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tagName, fe.Field(), fe.Param())
				return t
			}); err != nil {
				return err
			}
		case validate.LangEn:
			if err := wrap.GetValidate(la).RegisterTranslation(tagName, wrap.GetTranslators()[la], func(ut ut.Translator) error {
				return ut.Add(tagName, "{0} must start with {1}", true)
			}, func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tagName, fe.Field(), fe.Param())
				return t
			}); err != nil {
				return err
			}
		case validate.LangZhTW:
			if err := wrap.GetValidate(la).RegisterTranslation(tagName, wrap.GetTranslators()[la], func(ut ut.Translator) error {
				return ut.Add(tagName, "{0} 必須以 {1} 為開始", true)
			}, func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tagName, fe.Field(), fe.Param())
				return t
			}); err != nil {
				return err
			}
		}
		// more language...
	}
	return nil
}
