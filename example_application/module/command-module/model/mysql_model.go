// Package model is the driver-access layer for the command-module's
// MySQL-backed example CLI: it owns the *gorm.DB handle resolved from the
// application/command context. It depends only on the driver
// (component/database/dbmysql, gorm); callers in repository build queries
// against the *gorm.DB this layer exposes.
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

// NewExampleMysqlModel resolves the configured MySQL instance from ctx and
// builds an ExampleMysqlModel around its GORM client.
func NewExampleMysqlModel(ctx fiberhouse.ICommandContext) *ExampleMysqlModel {
	mysqlModel := dbmysql.NewMysqlModel(ctx, constant.MysqlInstanceKey)
	return NewExampleMysqlModelWithDB(mysqlModel.GetDB().Client)
}

// NewExampleMysqlModelWithDB builds an ExampleMysqlModel around an
// already-configured *gorm.DB, allowing callers (e.g. tests) to supply their
// own connection.
func NewExampleMysqlModelWithDB(db *gorm.DB) *ExampleMysqlModel {
	return &ExampleMysqlModel{db: db}
}

// DB returns the underlying *gorm.DB handle for use by the repository layer.
func (m *ExampleMysqlModel) DB() *gorm.DB {
	return m.db
}
