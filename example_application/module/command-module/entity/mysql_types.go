// Package entity defines the storage-facing domain types for the
// command-module's MySQL-backed example CLI: the GORM model
// (ExampleRecord) and its table mapping. This type is shared by model and
// repository.
package entity

import "time"

// ExampleRecord is the MySQL representation used by the example CLI.
// CreatedAt is set once at insert and preserved across updates; UpdatedAt is
// refreshed on every write. The composite index on status+created_at
// backs the repository's deterministic List ordering.
type ExampleRecord struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:80;not null;uniqueIndex:ux_example_records_name"`
	Description string    `json:"description" gorm:"size:500;not null;default:''"`
	Status      string    `json:"status" gorm:"size:16;not null;index:idx_example_records_status_created"`
	CreatedAt   time.Time `json:"created_at" gorm:"index:idx_example_records_status_created,sort:desc"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName overrides GORM's default pluralization, mapping ExampleRecord to
// the example_records table explicitly.
func (ExampleRecord) TableName() string {
	return "example_records"
}
