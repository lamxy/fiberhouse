// Package entity 定义 command-module 中基于 MySQL 的 example CLI 面向存储的
// 领域类型：GORM 模型（ExampleRecord）及其表映射。该类型由 model 与 repository
// 共享。
package entity

import "time"

// ExampleRecord 是 example CLI 使用的 MySQL 表示。CreatedAt 在插入时设置一次并
// 在更新间保留；UpdatedAt 每次写入时刷新。status+created_at 上的复合索引支撑
// repository 的确定性 List 排序。
type ExampleRecord struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:80;not null;uniqueIndex:ux_example_records_name"`
	Description string    `json:"description" gorm:"size:500;not null;default:''"`
	Status      string    `json:"status" gorm:"size:16;not null;index:idx_example_records_status_created"`
	CreatedAt   time.Time `json:"created_at" gorm:"index:idx_example_records_status_created,sort:desc"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 覆盖 GORM 默认的复数化命名，显式将 ExampleRecord 映射到
// example_records 表。
func (ExampleRecord) TableName() string {
	return "example_records"
}
