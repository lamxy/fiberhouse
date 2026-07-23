package entity

import "time"

// ExampleRecord is the MySQL representation used by the example CLI.
type ExampleRecord struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:80;not null;uniqueIndex:ux_example_records_name"`
	Description string    `json:"description" gorm:"size:500;not null;default:''"`
	Status      string    `json:"status" gorm:"size:16;not null;index:idx_example_records_status_created"`
	CreatedAt   time.Time `json:"created_at" gorm:"index:idx_example_records_status_created,sort:desc"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ExampleRecord) TableName() string {
	return "example_records"
}
