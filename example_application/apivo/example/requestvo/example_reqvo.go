// Package requestvo 定义 example API 的传输层请求 DTO：即在传入 service 层之前，
// 从 HTTP 请求体/查询字符串绑定并校验得到的数据结构。
package requestvo

// CreateExampleReqVo 是创建 example 的请求体。所有字段的必填或默认值处理都在
// service 层完成；结构体 tag 仅约束传输层的形态/长度。
//
// swagger:model CreateExampleReqVo
type CreateExampleReqVo struct {
	// example 的展示名称。必填，2-80 个字符。
	// example: My Example
	Name string `json:"name" validate:"required,min=2,max=80"`
	// 自由文本描述，最多 500 个字符。
	// example: A short description of the example.
	Description string `json:"description" validate:"max=500"`
	// 生命周期状态；取 active/archived 之一。默认值在 service 层应用。
	// example: active
	Status string `json:"status" validate:"omitempty,oneof=active archived"`
	// 最多 10 个自由标签，每个 1-30 个字符。
	// example: ["demo","swagger"]
	Tags []string `json:"tags" validate:"max=10,dive,min=1,max=30"`
}

// UpdateExampleReqVo 是部分更新 example 的请求体。
// 指针字段用于区分「字段被省略」（nil，保持不变）与
// 「字段被显式设置」（非 nil，替换当前值）——正是这一点使 Update 成为
// patch 局部更新，而非 upsert/整体替换。
//
// swagger:model UpdateExampleReqVo
type UpdateExampleReqVo struct {
	// 新的展示名称，2-80 个字符。省略则保持不变。
	// example: My Renamed Example
	Name *string `json:"name" validate:"omitempty,min=2,max=80"`
	// 新的描述，最多 500 个字符。省略则保持不变。
	// example: Updated description.
	Description *string `json:"description" validate:"omitempty,max=500"`
	// 新的状态；取 active/archived 之一。省略则保持不变。
	// example: archived
	Status *string `json:"status" validate:"omitempty,oneof=active archived"`
	// 新的标签集合，最多 10 个、每个 1-30 个字符。省略则保持不变。
	// example: ["demo"]
	Tags *[]string `json:"tags" validate:"omitempty,max=10,dive,min=1,max=30"`
}

// ListExamplesReqVo 承载列出 example 时的分页与状态过滤查询参数。
// 使用前请先调用 Normalize 以应用 page/page_size 的默认值。
//
// swagger:model ListExamplesReqVo
type ListExamplesReqVo struct {
	// 页码，从 1 开始。经 Normalize 默认为 1。
	// example: 1
	Page int `query:"page" form:"page" validate:"omitempty,min=1"`
	// 每页数量，1-100。经 Normalize 默认为 20。
	// example: 20
	PageSize int `query:"page_size" form:"page_size" validate:"omitempty,min=1,max=100"`
	// 可选的状态过滤；取 active/archived 之一。
	// example: active
	Status string `query:"status" form:"status" validate:"omitempty,oneof=active archived"`
}

// Normalize 返回 q 的副本，当 Page、PageSize 为零值时分别默认为 1 和 20。
func (q ListExamplesReqVo) Normalize() ListExamplesReqVo {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	return q
}

// ExampleReqVo 是已废弃的兼容性 DTO，在使用它的传输适配器迁移到上文规范的
// CRUD API 之前暂予保留。
type ExampleReqVo struct {
	ExamName string                 `json:"exam_name"`
	Age      int                    `json:"exam_age"`
	Courses  []string               `json:"courses"`
	Profile  map[string]interface{} `json:"profile"`
}

// ObjId 包装单个 id 字段，用于独立于任何具体 DTO 校验路径/查询中的标识符
// （例如通过 ExampleHandler.validateID）。
type ObjId struct {
	ID string `json:"id" validate:"required,alphanum,min=18,max=32"`
}

// PageReqVo 是已废弃的兼容性分页 DTO，与 ExampleReqVo 一同保留，
// 直至旧适配器完成迁移。
type PageReqVo struct {
	Page int `json:"p" validate:"required,min=1"`
	Size int `json:"s" validate:"required,min=1,max=20"`
}
