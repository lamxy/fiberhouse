// Package responsevo defines the transport-layer response DTOs for the
// example API: the JSON shapes returned to clients, built by service from
// entity.Example values via toResponse.
package responsevo

import "github.com/lamxy/fiberhouse/example_application/apivo/commonvo"

// ExampleRespVo is the JSON representation of a single example returned by
// Create, Get, and Update.
type ExampleRespVo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	commonvo.Timestamps
}

// ExampleListRespVo is the JSON representation of a page of examples,
// echoing back the effective Page/PageSize alongside the Total matching
// count.
type ExampleListRespVo struct {
	Items    []ExampleRespVo `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
}

// ExampleIdRespVo is a deprecated compatibility response used by adapters
// pending migration to ExampleRespVo.
type ExampleIdRespVo struct {
	ID string `json:"id"`
}
