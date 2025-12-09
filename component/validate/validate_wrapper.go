// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package validate 提供基于 go-playground/validator 的多语言验证器包装器，支持自定义验证规则和错误消息翻译。
package validate

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/samber/lo"
	"strings"
)

// 验证包装器接口，非并发读写安全
type ValidateWrapper interface {
	GetValidate(lang ...LangFlag) *validator.Validate
	GetTranslator(lang ...LangFlag) ut.Translator
	GetValidators() map[string]*validator.Validate
	GetTranslators() map[string]ut.Translator
	RegisterCustomTags(tagRegisters []RegisterValidatorTagFunc) []error
	GetLangList() []LangFlag
	Errors(errs validator.ValidationErrors, lang LangFlag, snakeCase ...bool) *exception.ValidateException
	ErrorsVar(errs validator.ValidationErrors, varName string, lang LangFlag, snakeCase ...bool) *exception.ValidateException
	ErrorsMap(errsMap map[string]interface{}, lang LangFlag, snakeCase ...bool) *exception.ValidateException
	RegisterValidator(langFlag LangFlag, validator *validator.Validate)
	RegisterTranslator(langFlag LangFlag, translator ut.Translator)
	RegisterLangFlag(langFlag LangFlag)
}

// 语言标志类型
type LangFlag = string

// 全局validate在并发中不同语言翻译是否被重用
const (
	DefaultLang = "en"
	LangZhCN    = "zh-cn"
	LangZhTW    = "zh-tw"
	LangEn      = "en"
	// more lang setup
)

// Wrap 验证器包装器，包含多语言的验证器和翻译器，非并发读写安全
// 推荐应用启动阶段初始化和注册好所需语言验证器，运行时阶段只读
type Wrap struct {
	validators map[LangFlag]*validator.Validate `desc:"map for lang to validator"`
	langList   []LangFlag                       `desc:"language list"`
	transMap   map[LangFlag]ut.Translator       `desc:"language sign for translator"`
}

// NewWrap 创建并初始化验证器包装器，根据配置文件语言标志设置对应的验证器和翻译器，未设置则使用默认英文。
func NewWrap(cfg appconfig.IAppConfig) *Wrap {
	vw := &Wrap{
		validators: make(map[string]*validator.Validate),
		langList:   []LangFlag{},
		transMap:   make(map[string]ut.Translator),
	}

	// 获取配置文件中设置的语言标志列表
	langFlags := cfg.Strings("application.validate.langFlags")

	// 如果配置文件中没有设置语言标志，则使用默认的语言标志
	var defaultValidateList []ValidateInitializer

	if len(langFlags) == 0 {
		defaultValidateList = append(defaultValidateList, GetEnValidateInitializer())
	} else {
		for _, langFlag := range langFlags {
			switch strings.ToLower(langFlag) {
			case LangZhCN:
				defaultValidateList = append(defaultValidateList, GetZhCNValidateInitializer())
			case LangZhTW:
				defaultValidateList = append(defaultValidateList, GetZhTWValidateInitializer())
			case LangEn:
				defaultValidateList = append(defaultValidateList, GetEnValidateInitializer())
			}
		}
	}

	for i := range defaultValidateList {
		validateInstance := defaultValidateList[i]()
		validateInstance.RegisterToWrap(vw)
	}

	return vw
}

// RegisterValidator 注册指定语言的验证器
func (vw *Wrap) RegisterValidator(langFlag LangFlag, validator *validator.Validate) {
	vw.validators[langFlag] = validator
}

// RegisterTranslator 注册指定语言的翻译器
func (vw *Wrap) RegisterTranslator(langFlag LangFlag, translator ut.Translator) {
	vw.transMap[langFlag] = translator
}

// RegisterLangFlag 注册语言标志到语言列表
func (vw *Wrap) RegisterLangFlag(langFlag LangFlag) {
	vw.langList = append(vw.langList, langFlag)
}

// GetTranslator 获取翻译器，用于错误处理时返回包含翻译消息的异常
func (vw *Wrap) GetTranslator(lang ...LangFlag) ut.Translator {
	if len(lang) > 0 {
		la := strings.ToLower(lang[0])
		if lo.Contains(vw.langList, la) {
			return vw.transMap[la]
		}
	}
	return vw.transMap[DefaultLang]
}

// GetValidate 获取指定语言的验证器
func (vw *Wrap) GetValidate(lang ...LangFlag) *validator.Validate {
	if len(lang) > 0 {
		la := strings.ToLower(lang[0])
		if lo.Contains(vw.langList, la) {
			return vw.validators[la]
		}
	}
	return vw.validators[DefaultLang]
}

// GetValidators 获取全部语言的验证器
func (vw *Wrap) GetValidators() map[LangFlag]*validator.Validate {
	return vw.validators
}

// GetTranslators 获取全部语言的翻译器
func (vw *Wrap) GetTranslators() map[LangFlag]ut.Translator {
	return vw.transMap
}

// RegisterCustomTags validate.getRegisterFuncList 获取注册函数列表参数
func (vw *Wrap) RegisterCustomTags(tagRegisters []RegisterValidatorTagFunc) []error {
	errs := make([]error, 0, len(tagRegisters))
	for i := range tagRegisters {
		if err := tagRegisters[i](vw); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// GetDefaultLang 获取默认语言
func GetDefaultLang() LangFlag {
	return DefaultLang
}

// GetLangList 获取全部语言列表
func (vw *Wrap) GetLangList() []LangFlag {
	return vw.langList
}

// Errors 用于针对验证器验证[结构体]输入参数的错误返回对应语言的错误信息的处理，返回框架统一的错误响应结构
//
//	lang := "zh-tw"
//	vw := NewWrap(config)
//	if errVW := vw.GetValidate(lang).Struct(&req); errVW != nil {
//		var errs validator.ValidationErrors
//		if errors.As(errVW, &errs) {
//			return vw.Errors(errs, lang, true)
//		}
//	}
func (vw *Wrap) Errors(errs validator.ValidationErrors, lang LangFlag, snakeCase ...bool) *exception.ValidateException {
	var errMap = make(map[string]string, len(errs))
	if len(snakeCase) > 0 && snakeCase[0] {
		for i := range errs {
			// 蛇形风格命名
			key, val := errs[i].StructField(), errs[i].Translate(vw.GetTranslator(lang))
			errMap[lo.SnakeCase(key)] = strings.Replace(val, key, lo.CamelCase(key), 1)
		}
	} else {
		for i := range errs {
			// 驼峰风格命名
			key, val := errs[i].StructField(), errs[i].Translate(vw.GetTranslator(lang))
			errMap[lo.CamelCase(key)] = strings.Replace(val, key, lo.CamelCase(key), 1)
		}
	}
	//return exception.Get("InputParamError").RespError(errs.Translate(vw.TransMap[vw.lang])) // fe.ns namespace.xxx
	return exception.VeGet("InputParamError").RespError(errMap)
}

// ErrorsVar 用于动态验证指定的变量，手动指定变量名称参数，作为验证字段名称输出错误信息的处理，返回框架统一的错误响应结构
//
//	 Usage:
//			lang := "zh-tw"
//		 	validateRule := "required,min=20,max=500"
//			vw := NewWrap(config)
//			if errsVar := vw.GetValidate(lang).Var(req.SomeAttrName, validateRule); errsVar != nil {
//				var errs validator.ValidationErrors
//				if errors.As(errsVar, &errs) {
//					return vw.ErrorsVar(errs, "SomeAttrName", lang, true)
//				}
//			}
func (vw *Wrap) ErrorsVar(errs validator.ValidationErrors, varName string, lang LangFlag, snakeCase ...bool) *exception.ValidateException {
	var errMap = make(map[string]string, len(errs))
	if len(snakeCase) > 0 && snakeCase[0] {
		for i := range errs {
			// map key 转成蛇形命名
			key := errs[i].Field()
			if key == "" {
				key = varName
			}
			val := errs[i].Translate(vw.GetTranslator(lang))
			errMap[lo.SnakeCase(key)] = lo.CamelCase(key) + " " + val
		}
	} else {
		for i := range errs {
			// map key 保持属性的驼峰命名
			key := errs[i].Field()
			if key == "" {
				key = varName
			}
			val := errs[i].Translate(vw.GetTranslator(lang))
			errMap[lo.CamelCase(key)] = lo.CamelCase(key) + " " + val
		}
	}
	//return exception.Get("InputParamError").RespError(errs.Translate(vw.TransMap[vw.lang])) // fe.ns带命名空间
	return exception.VeGet("InputParamError").RespError(errMap)
}

// ErrorsMap 用于依据动态验证map规则验证组合的map字段的错误处理，返回框架统一的错误响应结构
/** Usage:
vw := NewWrap(config)
vMap := fiber.Map{
	"PriceRate": req.PriceRate,
	"Currency": req.Currency,
}
vRule := fiber.Map{
	"PriceRate": "required,numeric",
	"Currency": "required,len=3",
}
if errsMap := vw.GetValidate(lang).ValidateMap(vMap, vRule); len(errsMap) > 0 {
	return vw.ErrorsMap(errsMap, lang, true)
}
*/
func (vw *Wrap) ErrorsMap(errsMap map[string]interface{}, lang LangFlag, snakeCase ...bool) *exception.ValidateException {
	outMap := make(map[string]interface{}, len(errsMap))

	for field := range errsMap {
		if vErrs, ok := errsMap[field].(validator.ValidationErrors); ok {
			if len(snakeCase) > 0 && snakeCase[0] {
				for i := range vErrs {
					// map key 转成蛇形命名
					key := vErrs[i].Field()
					if key == "" {
						key = field
					}
					val := vErrs[i].Translate(vw.GetTranslator(lang))
					outMap[lo.SnakeCase(key)] = lo.CamelCase(key) + " " + val
				}
			} else {
				for i := range vErrs {
					// map key 保持属性的驼峰命名
					key := vErrs[i].Field()
					if key == "" {
						key = field
					}
					val := vErrs[i].Translate(vw.GetTranslator(lang))
					outMap[lo.CamelCase(key)] = lo.CamelCase(key) + " " + val
				}
			}
		}
	}

	return exception.VeGetInputError().RespError(outMap)
}
