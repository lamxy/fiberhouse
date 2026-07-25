// Package responsevo 定义 example API 的传输层响应 DTO：即返回给客户端的 JSON
// 结构，由 service 通过 toResponse 从 entity.Example 值构建。
package responsevo

import "github.com/lamxy/fiberhouse/example_application/apivo/commonvo"

// ExampleRespVo 是 Create、Get、Update 返回的单个 example 的 JSON 表示。
//
// swagger:model ExampleRespVo
type ExampleRespVo struct {
	// example 的唯一标识。
	// example: 01f8h6z8k9q2p3r4s5t6u7v8w9
	ID string `json:"id"`
	// example 的展示名称。
	// example: My Example
	Name string `json:"name"`
	// 自由文本描述。
	// example: A short description of the example.
	Description string `json:"description,omitempty"`
	// 生命周期状态；取 active/archived 之一。
	// example: active
	Status string `json:"status"`
	// 与该 example 关联的自由标签。
	// example: ["demo","swagger"]
	Tags []string `json:"tags"`
	commonvo.Timestamps
}

// ExampleListRespVo 是一页 example 的 JSON 表示，
// 在返回匹配总数 Total 的同时回显实际生效的 Page/PageSize。
//
// swagger:model ExampleListRespVo
type ExampleListRespVo struct {
	// 本页的 example 列表。
	Items []ExampleRespVo `json:"items"`
	// 实际返回的页码。
	// example: 1
	Page int `json:"page"`
	// 实际返回的每页数量。
	// example: 20
	PageSize int `json:"page_size"`
	// 满足过滤条件的 example 总数（跨所有页）。
	// example: 42
	Total int64 `json:"total"`
}

// ExampleIdRespVo 是已废弃的兼容性响应，供尚未迁移到 ExampleRespVo 的适配器使用。
type ExampleIdRespVo struct {
	ID string `json:"id"`
}
