package requestvo

import (
	"github.com/lamxy/fiberhouse/component/validate"
)

// ExampleReqVo 示例请求对象
type ExampleReqVo struct {
	ExamName string                 `json:"exam_name" validate:"required,min=5,max=20"`  // Required field, min 5 char long max 20
	ExamAge  int                    `json:"exam_age" validate:"omitempty,min=16,max=60"` // 可选字段，存在则验证后续tag
	Courses  []string               `json:"courses" validate:"required"`                 // 自定义tag标签
	Profile  map[string]interface{} `json:"profile" validate:"required"`
}

// ObjId 统一ID对象
type ObjId struct {
	ID string `json:"id" validate:"required,alphanum,min=18,max=32"`
}

// PageReqVo 分页请求
type PageReqVo struct {
	Page int `json:"p" validate:"required,min=1"`
	Size int `json:"s" validate:"required,min=1,max=20"`
}

/*
RegisterValidationWrap 接口请求到后端控制器时，注册指定的tag和翻译后进行验证，建议应用启动阶段验证器全局统一注册替代
*/
func (req *ExampleReqVo) RegisterValidationWrap(wrap *validate.Wrap, lang ...string) error {
	return nil
}

/*
RegisterTranslationWrap 接口请求到后端控制器时，注册指定的tag和翻译后进行验证，建议应用启动阶段验证器全局统一注册替代
*/
func (req *ExampleReqVo) RegisterTranslationWrap(wrap *validate.Wrap, lang ...string) error {
	return nil
}
