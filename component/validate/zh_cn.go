// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package validate

import (
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	translation "github.com/go-playground/validator/v10/translations/zh"
)

// ZhCNValidate 中文验证器
type ZhCNValidate struct {
	validator  *validator.Validate
	lang       string
	translator ut.Translator
}

func NewZhCNValidate() *ZhCNValidate {
	va := validator.New(validator.WithRequiredStructEnabled())
	localeEn := en.New()
	localeZh := zh.New()
	uniTrans := ut.New(localeEn, localeZh)
	trans, _ := uniTrans.GetTranslator("zh")
	if err := translation.RegisterDefaultTranslations(va, trans); err != nil {
		panic(err)
	}
	return &ZhCNValidate{
		validator:  va,
		lang:       LangZhCN,
		translator: trans,
	}
}

// RegisterToWrap 注册中文(简体)验证器和翻译器到包装器
func (v *ZhCNValidate) RegisterToWrap(wrap *Wrap) {
	wrap.RegisterValidator(v.lang, v.validator)
	wrap.RegisterTranslator(v.lang, v.translator)
	wrap.RegisterLangFlag(v.lang)
}

// GetZhCNValidateInitializer 获取中文验证器初始化器
func GetZhCNValidateInitializer() ValidateInitializer {
	return func() ValidateRegister {
		return NewZhCNValidate()
	}
}
