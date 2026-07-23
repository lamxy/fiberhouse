package model

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/database/dbmysql"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"gorm.io/gorm"
)

// ExampleMysqlModel owns the GORM dependency for the command-module slice.
// Keeping it small makes the repository independently testable.
type ExampleMysqlModel struct {
	db *gorm.DB
}

func NewExampleMysqlModel(ctx fiberhouse.ICommandContext) *ExampleMysqlModel {
	mysqlModel := dbmysql.NewMysqlModel(ctx, constant.MysqlInstanceKey)
	return NewExampleMysqlModelWithDB(mysqlModel.GetDB().Client)
}

func NewExampleMysqlModelWithDB(db *gorm.DB) *ExampleMysqlModel {
	return &ExampleMysqlModel{db: db}
}

func (m *ExampleMysqlModel) DB() *gorm.DB {
	return m.db
}
