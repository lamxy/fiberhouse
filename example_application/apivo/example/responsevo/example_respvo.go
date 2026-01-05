package responsevo

import (
	"github.com/lamxy/fiberhouse/example_application/apivo/commonvo"
)

// ExampleRespVo 示例响应对象
type ExampleRespVo struct {
	ID       string                 `json:"id"`
	ExamName string                 `json:"exam_name"`
	ExamAge  int                    `json:"exam_age"`
	Courses  []string               `json:"courses,omitempty"`
	Profile  map[string]interface{} `json:"profile,omitempty"`
	commonvo.Timestamps
}

// ExampleIdRespVo 示例ID相应对象
type ExampleIdRespVo struct {
	ID string
}

// ExampleListRespVo 示例列表响应对象
type ExampleListRespVo struct {
	List []ExampleRespVo `json:"list"`
}
