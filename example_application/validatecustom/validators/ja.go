package validators

import (
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/ja"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	translation "github.com/go-playground/validator/v10/translations/ja"
	"github.com/lamxy/fiberhouse/component/validate"
)

// 日语验证器
type JaValidate struct {
	validator  *validator.Validate
	lang       string
	translator ut.Translator
}

func NewJaValidate() *JaValidate {
	va := validator.New(validator.WithRequiredStructEnabled())
	localeEn := en.New()
	localeJa := ja.New()
	uniTrans := ut.New(localeEn, localeJa)
	trans, _ := uniTrans.GetTranslator("de")
	if err := translation.RegisterDefaultTranslations(va, trans); err != nil {
		panic(err)
	}
	return &JaValidate{
		validator:  va,
		lang:       LangJa,
		translator: trans,
	}
}

// RegisterToWrap 注册到验证器包装器
func (v *JaValidate) RegisterToWrap(wrap *validate.Wrap) {
	wrap.RegisterValidator(v.lang, v.validator)
	wrap.RegisterTranslator(v.lang, v.translator)
	wrap.RegisterLangFlag(v.lang)
}

// GetJaValidateInitializer 获取日语验证器初始化器
func GetJaValidateInitializer() validate.ValidateInitializer {
	return func() validate.ValidateRegister {
		return NewJaValidate()
	}
}
