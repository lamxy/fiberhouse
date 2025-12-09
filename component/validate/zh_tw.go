// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package validate

import (
	"github.com/go-playground/locales/en"
	zhtw "github.com/go-playground/locales/zh_Hant_TW"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	translation "github.com/go-playground/validator/v10/translations/zh_tw"
)

// ZhTWValidate 中文(繁体)验证器
type ZhTWValidate struct {
	validator  *validator.Validate
	lang       string
	translator ut.Translator
}

func NewZhTWValidate() *ZhTWValidate {
	va := validator.New(validator.WithRequiredStructEnabled())
	localeEn := en.New()
	localeZhtw := zhtw.New()
	uniTrans := ut.New(localeEn, localeZhtw)
	trans, _ := uniTrans.GetTranslator("zh_Hant_TW")
	if err := translation.RegisterDefaultTranslations(va, trans); err != nil {
		panic(err)
	}
	return &ZhTWValidate{
		validator:  va,
		lang:       LangZhTW,
		translator: trans,
	}
}

// RegisterToWrap 注册中文(繁体)验证器和翻译器到包装器
func (v *ZhTWValidate) RegisterToWrap(wrap *Wrap) {
	wrap.RegisterValidator(v.lang, v.validator)
	wrap.RegisterTranslator(v.lang, v.translator)
	wrap.RegisterLangFlag(v.lang)
}

func GetZhTWValidateInitializer() ValidateInitializer {
	return func() ValidateRegister {
		return NewZhTWValidate()
	}
}
