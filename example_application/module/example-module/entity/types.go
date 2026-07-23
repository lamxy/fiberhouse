package entity

import (
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type ExampleStatus string

const (
	ExampleStatusActive   ExampleStatus = "active"
	ExampleStatusArchived ExampleStatus = "archived"
)

type Example struct {
	ID                bson.ObjectID `json:"id" bson:"_id,omitempty"`
	Name              string        `json:"name" bson:"name"`
	Description       string        `json:"description" bson:"description,omitempty"`
	Status            ExampleStatus `json:"status" bson:"status"`
	Tags              []string      `json:"tags" bson:"tags,omitempty"`
	fields.Timestamps `json:"-" bson:",inline"`
}
