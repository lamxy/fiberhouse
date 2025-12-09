package apivo

import "github.com/lamxy/fiberhouse/component/validate"

// ValidatorApiRegister 用于支持 reqVo 对象实现对指定接口请求时调用注册验证器tag和多语言翻译接口方法完成注册后进行验证
// 除此之外，也可以在应用启动器启动阶段集中配置并由启动器统一注册自定义验证器的tag和翻译，见ApplicationRegister接口的ConfigValidatorCustomTags()配置方法
type ValidatorApiRegister interface {
	RegisterValidationWrap(wrap *validate.Wrap, lang ...string) error
	RegisterTranslationWrap(wrap *validate.Wrap, lang ...string) error
}
