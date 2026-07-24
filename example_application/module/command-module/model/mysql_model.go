// Package model 是 command-module 中基于 MySQL 的 example CLI 的驱动访问层：
// 负责持有从应用/命令上下文解析出的 *gorm.DB 句柄。它仅依赖驱动
// （component/database/dbmysql、gorm）；repository 中的调用方基于本层暴露的
// *gorm.DB 构建查询。
package model

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/database/dbmysql"
	"github.com/lamxy/fiberhouse/example_application/module/constant"
	"gorm.io/gorm"
)

// ExampleMysqlModel 持有 command-module 切片的 GORM 依赖。保持其精简使
// repository 可独立测试。
type ExampleMysqlModel struct {
	db *gorm.DB
}

// NewExampleMysqlModel 从 ctx 解析出配置的 MySQL 实例，并围绕其 GORM 客户端
// 构建一个 ExampleMysqlModel。
func NewExampleMysqlModel(ctx fiberhouse.ICommandContext) *ExampleMysqlModel {
	mysqlModel := dbmysql.NewMysqlModel(ctx, constant.MysqlInstanceKey)
	return NewExampleMysqlModelWithDB(mysqlModel.GetDB().Client)
}

// NewExampleMysqlModelWithDB 围绕一个已配置的 *gorm.DB 构建 ExampleMysqlModel，
// 允许调用方（例如测试）提供自己的连接。
func NewExampleMysqlModelWithDB(db *gorm.DB) *ExampleMysqlModel {
	return &ExampleMysqlModel{db: db}
}

// DB 返回底层的 *gorm.DB 句柄，供 repository 层使用。
func (m *ExampleMysqlModel) DB() *gorm.DB {
	return m.db
}
