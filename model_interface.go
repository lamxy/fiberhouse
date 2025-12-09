// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

// Modeler 模型接口，定义了数据库模型的基本操作方法
// MongoLocator 定义了MongoDB的定位器接口，见database/dbmongo/interface.go
// MysqlLocator 定义了MySQL的定位器接口，见database/dbmysql/interface.go
type Modeler interface {
	Locator
	// 获取数据库名称
	GetDbName() string
	// 设置数据库名称，返回当前对象以支持链式调用
	SetDbName(string) Modeler // replace interface{}
	// 获取数据表名称
	GetTable() string
	// 设置数据表名称，返回当前对象以支持链式调用
	SetTable(string, ...string) Modeler // replace interface{}
}
