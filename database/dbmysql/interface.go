// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmysql

import "github.com/lamxy/fiberhouse"

// MysqlLocator 接口定义了在 frame 中进行 Mysql 操作的方法
type MysqlLocator interface {
	fiberhouse.Modeler
	// GetDB 获取 MysqlDb 对象以进行数据库操作
	GetDB() *MysqlDb
}
