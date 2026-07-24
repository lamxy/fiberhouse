// Package entity defines the storage-facing domain types for the example
// module: the MongoDB document shape (Example) and its status enum. These
// types are shared by model and repository; they carry both json and bson
// tags because the same struct is serialized to the driver and (via
// service's mapping to responsevo) indirectly reflected in API responses.
package entity

import (
	"github.com/lamxy/fiberhouse/example_application/module/common-module/fields"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ExampleStatus enumerates the valid lifecycle states of an Example.
type ExampleStatus string

const (
	// ExampleStatusActive marks an example as currently in use.
	ExampleStatusActive ExampleStatus = "active"
	// ExampleStatusArchived marks an example as retired but retained.
	ExampleStatusArchived ExampleStatus = "archived"
)

// Example is the MongoDB document for the example collection. ID is
// populated by the store on Create/Insert; Timestamps embeds CreatedAt (set
// once, preserved across updates) and UpdatedAt (refreshed on every write).
type Example struct {
	ID                bson.ObjectID `json:"id" bson:"_id,omitempty"`
	Name              string        `json:"name" bson:"name"`
	Description       string        `json:"description" bson:"description,omitempty"`
	Status            ExampleStatus `json:"status" bson:"status"`
	Tags              []string      `json:"tags" bson:"tags,omitempty"`
	fields.Timestamps `json:"-" bson:",inline"`
}
