package validators

import (
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/ko"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	translation "github.com/go-playground/validator/v10/translations/ko"
	"github.com/lamxy/fiberhouse/component/validate"
)

// KoValidate 韩语验证器
type KoValidate struct {
	validator  *validator.Validate
	lang       string
	translator ut.Translator
}

func NewKoValidate() *KoValidate {
	va := validator.New(validator.WithRequiredStructEnabled())
	localeEn := en.New()
	localeKo := ko.New()
	uniTrans := ut.New(localeEn, localeKo)
	trans, _ := uniTrans.GetTranslator("ko")
	if err := translation.RegisterDefaultTranslations(va, trans); err != nil {
		panic(err)
	}
	return &KoValidate{
		validator:  va,
		lang:       LangKo,
		translator: trans,
	}
}

// RegisterToWrap 注册到验证包装器
func (v *KoValidate) RegisterToWrap(wrap *validate.Wrap) {
	wrap.RegisterValidator(v.lang, v.validator)
	wrap.RegisterTranslator(v.lang, v.translator)
	wrap.RegisterLangFlag(v.lang)
}

// GetKoValidateInitializer 获取韩语验证器初始化器
func GetKoValidateInitializer() validate.ValidateInitializer {
	return func() validate.ValidateRegister {
		return NewKoValidate()
	}
}
