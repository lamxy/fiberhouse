package responsevo

import "github.com/lamxy/fiberhouse/example_application/apivo/commonvo"

type ExampleRespVo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	commonvo.Timestamps
}

type ExampleListRespVo struct {
	Items    []ExampleRespVo `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
}

// Deprecated compatibility response used by adapters pending migration.
type ExampleIdRespVo struct {
	ID string `json:"id"`
}
