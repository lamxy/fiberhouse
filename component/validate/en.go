// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package validate

import (
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslation "github.com/go-playground/validator/v10/translations/en"
)

// EnValidate 英文验证器
type EnValidate struct {
	validator  *validator.Validate
	lang       string
	translator ut.Translator
}

func NewEnValidate() *EnValidate {
	va := validator.New(validator.WithRequiredStructEnabled())
	localeEn := en.New()
	uniTrans := ut.New(localeEn, localeEn)
	trans, _ := uniTrans.GetTranslator("en")
	if err := enTranslation.RegisterDefaultTranslations(va, trans); err != nil {
		panic(err)
	}
	return &EnValidate{
		validator:  va,
		lang:       LangEn,
		translator: trans,
	}
}

// RegisterToWrap 注册英文验证器和翻译器到包装器
func (v *EnValidate) RegisterToWrap(wrap *Wrap) {
	wrap.RegisterValidator(v.lang, v.validator)
	wrap.RegisterTranslator(v.lang, v.translator)
	wrap.RegisterLangFlag(v.lang)
}

// GetEnValidateInitializer 获取英文验证器初始化器
func GetEnValidateInitializer() ValidateInitializer {
	return func() ValidateRegister {
		return NewEnValidate()
	}
}
