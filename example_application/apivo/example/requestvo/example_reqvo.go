package requestvo

type CreateExampleReqVo struct {
	Name        string   `json:"name" validate:"required,min=2,max=80"`
	Description string   `json:"description" validate:"max=500"`
	Status      string   `json:"status" validate:"omitempty,oneof=active archived"`
	Tags        []string `json:"tags" validate:"max=10,dive,min=1,max=30"`
}

type UpdateExampleReqVo struct {
	Name        *string   `json:"name" validate:"omitempty,min=2,max=80"`
	Description *string   `json:"description" validate:"omitempty,max=500"`
	Status      *string   `json:"status" validate:"omitempty,oneof=active archived"`
	Tags        *[]string `json:"tags" validate:"omitempty,max=10,dive,min=1,max=30"`
}

type ListExamplesReqVo struct {
	Page     int    `query:"page" form:"page" validate:"omitempty,min=1"`
	PageSize int    `query:"page_size" form:"page_size" validate:"omitempty,min=1,max=100"`
	Status   string `query:"status" form:"status" validate:"omitempty,oneof=active archived"`
}

func (q ListExamplesReqVo) Normalize() ListExamplesReqVo {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	return q
}

// Deprecated compatibility DTOs remain until the transport adapters are
// migrated to the canonical CRUD API.
type ExampleReqVo struct {
	ExamName string                 `json:"exam_name"`
	ExamAge  int                    `json:"exam_age"`
	Courses  []string               `json:"courses"`
	Profile  map[string]interface{} `json:"profile"`
}

type ObjId struct {
	ID string `json:"id" validate:"required,alphanum,min=18,max=32"`
}

type PageReqVo struct {
	Page int `json:"p" validate:"required,min=1"`
	Size int `json:"s" validate:"required,min=1,max=20"`
}
