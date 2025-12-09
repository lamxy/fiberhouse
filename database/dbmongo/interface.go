// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmongo

import (
	"github.com/lamxy/fiberhouse"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoLocator 接口定义了在 frame 中进行 MongoDB 操作的方法。
type MongoLocator interface {
	fiberhouse.Modeler
	// 获取MongoDb实例
	GetDB() *MongoDb
	// 获取集合名
	GetColl() string
	// 设置集合名，返回模型器接口，以便链式调用
	SetColl(string, ...string) fiberhouse.Modeler // replace interface{}
	// 获取数据库实例，默认使用配置的默认数据库
	GetDatabase(...options.Lister[options.DatabaseOptions]) *mongo.Database
	// 获取指定名称的数据库实例
	GetClientDatabase(string, ...options.Lister[options.DatabaseOptions]) *mongo.Database
	// 获取指定名称的集合实例，默认使用配置的默认数据库
	GetCollection(string, ...options.Lister[options.CollectionOptions]) *mongo.Collection
}
