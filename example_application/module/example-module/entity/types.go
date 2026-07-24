// Package entity 定义 example 模块面向存储的领域类型：MongoDB 文档结构
// （Example）及其状态枚举。这些类型由 model 与 repository 共享；同时携带 json
// 与 bson 两种 tag，因为同一结构体既会序列化给驱动，也会（经 service 映射到
// responsevo）间接反映在 API 响应中。
package entity

import (
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ExampleStatus 枚举 Example 的合法生命周期状态。
type ExampleStatus string

const (
	// ExampleStatusActive 表示该 example 当前正在使用。
	ExampleStatusActive ExampleStatus = "active"
	// ExampleStatusArchived 表示该 example 已归档但仍予保留。
	ExampleStatusArchived ExampleStatus = "archived"
)

// Example 是 example 集合对应的 MongoDB 文档。ID 由存储层在 Create/Insert 时
// 填充；Timestamps 内嵌 CreatedAt（仅设置一次，更新时保持不变）与 UpdatedAt
// （每次写入时刷新）。
type Example struct {
	ID                bson.ObjectID `json:"id" bson:"_id,omitempty"`
	Name              string        `json:"name" bson:"name"`
	Description       string        `json:"description" bson:"description,omitempty"`
	Status            ExampleStatus `json:"status" bson:"status"`
	Tags              []string      `json:"tags" bson:"tags,omitempty"`
	fields.Timestamps `json:"-" bson:",inline"`
}
