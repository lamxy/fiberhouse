package entity

import (
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Example
type Example struct {
	ID                bson.ObjectID             `json:"id" bson:"_id,omitempty"`
	Name              string                    `json:"name" bson:"name"`
	Age               int                       `json:"age" bson:"age,minsize"` // minsize 取int32存储数据
	Courses           []string                  `json:"courses" bson:"courses,omitempty"`
	Profile           map[string]interface{}    `json:"profile" bson:"profile,omitempty"`
	fields.Timestamps `json:"-" bson:",inline"` // inline: bson文档序列化自动提升嵌入字段即自动展开继承的公共字段
}
