// Package requestvo defines the transport-layer request DTOs for the
// example API: the shapes bound and validated from incoming HTTP
// bodies/query strings before being passed into the service layer.
package requestvo

// CreateExampleReqVo is the request body for creating an example. All
// fields are required-or-defaulted at the service layer; struct tags only
// enforce transport-level shape/length constraints.
type CreateExampleReqVo struct {
	Name        string   `json:"name" validate:"required,min=2,max=80"`
	Description string   `json:"description" validate:"max=500"`
	Status      string   `json:"status" validate:"omitempty,oneof=active archived"`
	Tags        []string `json:"tags" validate:"max=10,dive,min=1,max=30"`
}

// UpdateExampleReqVo is the request body for a partial example update.
// Pointer fields distinguish "field omitted" (nil, left unchanged) from
// "field explicitly set" (non-nil, replaces the current value) — this is
// what makes Update a patch rather than an upsert/full replace.
type UpdateExampleReqVo struct {
	Name        *string   `json:"name" validate:"omitempty,min=2,max=80"`
	Description *string   `json:"description" validate:"omitempty,max=500"`
	Status      *string   `json:"status" validate:"omitempty,oneof=active archived"`
	Tags        *[]string `json:"tags" validate:"omitempty,max=10,dive,min=1,max=30"`
}

// ListExamplesReqVo carries pagination and status-filter query parameters
// for listing examples. Call Normalize before use to apply page/page_size
// defaults.
type ListExamplesReqVo struct {
	Page     int    `query:"page" form:"page" validate:"omitempty,min=1"`
	PageSize int    `query:"page_size" form:"page_size" validate:"omitempty,min=1,max=100"`
	Status   string `query:"status" form:"status" validate:"omitempty,oneof=active archived"`
}

// Normalize returns a copy of q with Page defaulted to 1 and PageSize
// defaulted to 20 when left at their zero values.
func (q ListExamplesReqVo) Normalize() ListExamplesReqVo {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	return q
}

// ExampleReqVo is a deprecated compatibility DTO retained until the
// transport adapters using it are migrated to the canonical CRUD API above.
type ExampleReqVo struct {
	ExamName string                 `json:"exam_name"`
	Age      int                    `json:"exam_age"`
	Courses  []string               `json:"courses"`
	Profile  map[string]interface{} `json:"profile"`
}

// ObjId wraps a single id field for validating path/query identifiers
// (e.g. via ExampleHandler.validateID) independently of any specific DTO.
type ObjId struct {
	ID string `json:"id" validate:"required,alphanum,min=18,max=32"`
}

// PageReqVo is a deprecated compatibility pagination DTO retained alongside
// ExampleReqVo until legacy adapters are migrated.
type PageReqVo struct {
	Page int `json:"p" validate:"required,min=1"`
	Size int `json:"s" validate:"required,min=1,max=20"`
}
