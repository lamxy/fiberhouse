// Package responsevo defines the transport-layer response DTOs for the
// example API: the JSON shapes returned to clients, built by service from
// entity.Example values via toResponse.
package responsevo

import "github.com/lamxy/fiberhouse/example_application/apivo/commonvo"

// ExampleRespVo is the JSON representation of a single example returned by
// Create, Get, and Update.
//
// swagger:model ExampleRespVo
type ExampleRespVo struct {
	// Unique identifier of the example.
	// example: 01f8h6z8k9q2p3r4s5t6u7v8w9
	ID string `json:"id"`
	// Display name of the example.
	// example: My Example
	Name string `json:"name"`
	// Free-text description.
	// example: A short description of the example.
	Description string `json:"description,omitempty"`
	// Lifecycle status; one of active/archived.
	// example: active
	Status string `json:"status"`
	// Free-form tags associated with the example.
	// example: ["demo","swagger"]
	Tags []string `json:"tags"`
	commonvo.Timestamps
}

// ExampleListRespVo is the JSON representation of a page of examples,
// echoing back the effective Page/PageSize alongside the Total matching
// count.
//
// swagger:model ExampleListRespVo
type ExampleListRespVo struct {
	// The examples on this page.
	Items []ExampleRespVo `json:"items"`
	// Effective page number returned.
	// example: 1
	Page int `json:"page"`
	// Effective page size returned.
	// example: 20
	PageSize int `json:"page_size"`
	// Total number of examples matching the filter, across all pages.
	// example: 42
	Total int64 `json:"total"`
}

// ExampleIdRespVo is a deprecated compatibility response used by adapters
// pending migration to ExampleRespVo.
type ExampleIdRespVo struct {
	ID string `json:"id"`
}
