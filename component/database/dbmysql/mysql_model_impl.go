// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package dbmysql

import (
	"fmt"

	"github.com/lamxy/fiberhouse"
)

// MysqlModel 定义 MysqlModel 结构体
// 该结构体实现了MysqlLocator接口，用于被具体的业务模型继承
// 包含应用上下文、数据库实例、数据库配置名称、表名和模型名称等字段
type MysqlModel struct {
	Ctx        fiberhouse.IContext
	Db         *MysqlDb
	dbConfName string
	Table      string
	name       string
}

// NewMysqlModel 创建 MysqlModel 实例。
//
// 若未找到对应 MysqlDb 实例，或该 key 下注册的不是 *MysqlDb，将 panic。
// 本变体用于装配期的链式调用；需要处理错误请使用 NewMysqlModelWithError。
func NewMysqlModel(ctx fiberhouse.IContext, instanceKey ...fiberhouse.InstanceKey) *MysqlModel {
	model, err := NewMysqlModelWithError(ctx, instanceKey...)
	if err != nil {
		panic(err)
	}
	return model
}

// NewMysqlModelWithError 创建 MysqlModel 实例，解析失败时返回错误而非 panic。
//
// 两种失败都是**可恢复**的装配错误，调用方应当能够处理而不是被 panic 中断：
// key 未注册（应用没有注册该 MySQL 实例），或 key 下注册的对象不是 *MysqlDb
// （key 冲突或注册了错误的类型）。后者尤其需要显式检查——容器的 Get 会成功返回，
// 未经检查的类型断言会在这里 panic，且 panic 信息不包含 key 名，难以定位。
//
// 返回的错误始终包含 key 名，并对 Get 失败保留原始错误链。
func NewMysqlModelWithError(ctx fiberhouse.IContext, instanceKey ...fiberhouse.InstanceKey) (*MysqlModel, error) {
	var key string
	if len(instanceKey) > 0 {
		key = instanceKey[0].String()
	} else {
		key = ctx.GetStarter().GetApplication().GetDBMysqlKey()
	}
	instance, err := ctx.GetContainer().Get(key)
	if err != nil {
		return nil, fmt.Errorf("resolve MySQL instance '%s' for model: %w", key, err)
	}
	db, ok := instance.(*MysqlDb)
	if !ok {
		return nil, fmt.Errorf("global object '%s' is %T, want *dbmysql.MysqlDb", key, instance)
	}
	return &MysqlModel{
		Ctx: ctx,
		Db:  db,
	}, nil
}

// GetContext 获取应用上下文
func (mo *MysqlModel) GetContext() fiberhouse.IContext {
	return mo.Ctx
}

// GetDB 获取MysqlDb实例
func (mo *MysqlModel) GetDB() *MysqlDb {
	return mo.Db
}

// GetDbName 获取当前使用的数据库配置名称
func (mo *MysqlModel) GetDbName() string {
	return mo.dbConfName
}

// SetDbName 设置当前使用的数据库配置名称
func (mo *MysqlModel) SetDbName(name string) fiberhouse.Modeler {
	mo.dbConfName = name
	return mo
}

// GetTable 返回当前模型使用的表名
func (mo *MysqlModel) GetTable() string {
	return mo.Table
}

// SetTable 设置当前模型使用的表名
func (mo *MysqlModel) SetTable(name string, prefix ...string) fiberhouse.Modeler {
	le := len(prefix)
	if le > 0 {
		if le == 1 {
			mo.Table = prefix[0] + "_" + name
			return mo
		}
		if le == 2 {
			mo.Table = prefix[0] + "_" + prefix[1] + "_" + name
			return mo
		}
	}
	mo.Table = name
	return mo
}

// GetTableName 返回自定义指定单个或多个前缀的表名
func (mo *MysqlModel) GetTableName(name string, prefix ...string) string {
	le := len(prefix)
	if le > 0 {
		if le == 1 {
			return prefix[0] + "_" + name
		}
		if le == 2 {
			return prefix[0] + "_" + prefix[1] + "_" + name
		}
	}
	return name
}

// GetName 返回当前模型的名称
func (mo *MysqlModel) GetName() string {
	return mo.name
}

// SetName 设置当前模型的名称
func (mo *MysqlModel) SetName(name string) fiberhouse.Locator {
	mo.name = name
	return mo
}

// GetInstance 获取实例（从全局管理器获取具体的单例）
func (mo *MysqlModel) GetInstance(namespaceKey string) (interface{}, error) {
	gm := mo.GetContext().GetContainer()
	return gm.Get(namespaceKey)
}
