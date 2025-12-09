// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmongo

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/exception"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoModel MongoDB模型基类
// 该结构体实现了MongoLocator接口，用于被具体的业务模型继承
// 继承该结构体的模型可直接使用其方法操作数据库和集合
type MongoModel struct {
	Ctx    fiberhouse.IContext
	Db     *MongoDb
	dbName string
	Coll   string
	name   string
}

// NewMongoModel 创建并返回一个MongoModel实例
// 未注册或名称错误会panic
func NewMongoModel(ctx fiberhouse.IContext, instanceKey ...fiberhouse.InstanceKey) *MongoModel {
	var key string
	if len(instanceKey) > 0 {
		key = instanceKey[0].String()
	} else {
		key = ctx.GetStarter().GetApplication().GetDBMongoKey()
	}
	db, err := ctx.GetContainer().Get(key)
	if err != nil {
		panic(err.Error())
	}
	return &MongoModel{
		Ctx: ctx,
		Db:  db.(*MongoDb),
	}
}

// GetContext 获取应用上下文
func (mo *MongoModel) GetContext() fiberhouse.IContext {
	return mo.Ctx
}

// GetDB 获取MongoDb实例
func (mo *MongoModel) GetDB() *MongoDb {
	return mo.Db
}

// GetDbName 获取默认的库名
func (mo *MongoModel) GetDbName() string {
	return mo.dbName
}

// SetDbName 设置默认的库名
func (mo *MongoModel) SetDbName(name string) fiberhouse.Modeler {
	mo.dbName = name
	return mo
}

// GetTable 获取默认的集合名
func (mo *MongoModel) GetTable() string {
	return mo.Coll
}

// SetTable 设置默认的集合名
func (mo *MongoModel) SetTable(name string, prefix ...string) fiberhouse.Modeler {
	le := len(prefix)
	if le > 0 {
		if le == 1 {
			mo.Coll = prefix[0] + "_" + name
			return mo
		}
		if le == 2 {
			mo.Coll = prefix[0] + "_" + prefix[1] + "_" + name
			return mo
		}
	}
	mo.Coll = name
	return mo
}

// GetColl 获取默认的集合名
func (mo *MongoModel) GetColl() string {
	return mo.GetTable()
}

// SetColl 设置默认的集合名
func (mo *MongoModel) SetColl(name string, prefix ...string) fiberhouse.Modeler {
	return mo.SetTable(name, prefix...)
}

// GetName 获取模型名称
func (mo *MongoModel) GetName() string {
	return mo.name
}

// SetName 设置模型名称
func (mo *MongoModel) SetName(name string) fiberhouse.Locator {
	mo.name = name
	return mo
}

// GetDatabase 获取默认的库
func (mo *MongoModel) GetDatabase(opts ...options.Lister[options.DatabaseOptions]) *mongo.Database {
	if mo.dbName == "" {
		exception.GetInternalError().RespError("Unknown database name").Panic()
	}
	return mo.Db.Client.Database(mo.dbName, opts...)
}

// GetClientDatabase 获非默认库
func (mo *MongoModel) GetClientDatabase(dbName string, opts ...options.Lister[options.DatabaseOptions]) *mongo.Database {
	return mo.Db.Client.Database(dbName, opts...)
}

// GetCollection 获取默认库下的指定集合
// options.Collection().SetReadPreference(readpref.Primary()) 默认读从，
// 可临时修改读主，一般用于重要业务时，实时读取最新数据
func (mo *MongoModel) GetCollection(coll string, opts ...options.Lister[options.CollectionOptions]) *mongo.Collection {
	if mo.dbName == "" {
		exception.GetInternalError().RespError("Unknown database name").Panic()
	}
	if coll == "" {
		exception.GetInternalError().RespError("Unknown database table name").Panic()
	}
	return mo.Db.Client.Database(mo.dbName).Collection(coll, opts...)
}

// GetInstance 获取实例（从全局管理器获取具体的单例）
func (mo *MongoModel) GetInstance(namespaceKey string) (interface{}, error) {
	gm := mo.GetContext().GetContainer()
	return gm.Get(namespaceKey)
}
