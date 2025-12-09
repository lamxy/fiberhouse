package entity

import (
	"database/sql"
	"gorm.io/gorm"
)

/**
Mysql数据库ORM 表模型结构
*/

type User struct {
	Name      string
	Age       uint8
	Desc      sql.NullString
	UpdatedTs int64 `gorm:"autoUpdateTime:milli"`
	gorm.Model
}

type Class struct {
	Name   string
	Isbn   string
	UserId uint64
	gorm.Model
}
