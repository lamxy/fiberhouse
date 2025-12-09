// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// Package dbmongo 提供基于 MongoDB 的数据库连接和模型操作功能。
package dbmongo

import (
	"context"
	"github.com/govalues/decimal"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/mongodecimal"
	"github.com/lamxy/fiberhouse/constant"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
	"reflect"
	"sync"
	"time"
)

type MongoDb struct {
	Client       *mongo.Client
	Ctx          fiberhouse.IContext
	lock         *sync.RWMutex
	confPathname string
}

// NewMongoDb 创建 MongoDb 实例
// dbConfName 可选，指定配置路径名称，默认 constant.DefaultMongoDBConfName
func NewMongoDb(appCtx fiberhouse.IContext, confPath ...string) (*MongoDb, error) {
	client, err := NewClient(appCtx, confPath...)
	if err != nil {
		appCtx.GetLogger().Error(appCtx.GetConfig().LogOriginMongodb()).Err(err).Msg("initial mongo client failed.")
		return nil, err
	}
	db := &MongoDb{
		Client:       client,
		Ctx:          appCtx,
		lock:         &sync.RWMutex{},
		confPathname: constant.DefaultMongoDBConfName,
	}
	if len(confPath) > 0 {
		db.confPathname = confPath[0]
	}
	return db, nil
}

// NewClient 依据配置实例化不同的db连接实例
func NewClient(appCtx fiberhouse.IContext, confPath ...string) (*mongo.Client, error) {
	var basePath string
	if len(confPath) > 0 && confPath[0] != "" {
		basePath = confPath[0]
	} else {
		basePath = constant.DefaultMongoDBConfName
	}

	// 读取配置
	aConf := appCtx.GetConfig()
	var (
		applyUri          = aConf.String(basePath + ".applyURI")
		maxPoolSize       = uint64(aConf.Int64(basePath + ".maxPoolSize"))
		minPoolSize       = uint64(aConf.Int64(basePath + ".minPoolSize"))
		maxConnIdleTime   = aConf.Duration(basePath+".maxConnIdleTime") * time.Second
		connectTimeout    = aConf.Duration(basePath+".connectTimeout") * time.Second
		clientTimeout     = aConf.Duration(basePath+".socketTimeout") * time.Second
		heartbeatInterval = aConf.Duration(basePath+".heartbeatInterval") * time.Second
		//readPreference    = aConf.String(basePath + ".ReadPreference")
	)

	// 注册decimal.Decimal编组解组注册表
	registry := bson.NewRegistry()
	registry.RegisterTypeEncoder(reflect.TypeOf(decimal.Decimal{}), new(mongodecimal.MongoDecimal))
	registry.RegisterTypeDecoder(reflect.TypeOf(decimal.Decimal{}), new(mongodecimal.MongoDecimal))

	// SetBSONOptions(&options.BSONOptions{}) 设置bson序列化反序列化的规则，如nil值字段是否编组为空的切片或map
	clientOptions := options.Client().
		SetRegistry(registry).
		ApplyURI(applyUri).
		SetBSONOptions(&options.BSONOptions{UseJSONStructTags: true, ErrorOnInlineDuplicates: true, IntMinSize: true}). // 驱动程序在未指定"bson"结构标记的情况下使用"json"结构标记
		SetWriteConcern(writeconcern.Majority()).                                                                       // PSA副本集默认写级别为1
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetMaxConnIdleTime(maxConnIdleTime).
		SetConnectTimeout(connectTimeout).
		SetHeartbeatInterval(heartbeatInterval).
		SetReadPreference(readpref.SecondaryPreferred()) // readPreference

	//uri := "mongodb://admin:xxx@wslserver:27018,wslserver:27017,wslserver:27019/?replicaSet=rs0&authSource=admin&w=majority&maxPoolSize=100&minPoolSize=20&connectTimeoutMS=2000&socketTimeoutMS=10000&readPreference=secondaryPreferred"
	//client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	clientOptions.Timeout = &clientTimeout
	return mongo.Connect(clientOptions)
}

// GetConfPath 获取当前实例使用的配置路径
func (md *MongoDb) GetConfPath() string {
	return md.confPathname
}

// Close 关闭 MongoDB 客户端连接
// 谨慎使用Close关闭链接
func (md *MongoDb) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return md.Client.Disconnect(ctx)
}

// IsHealthy 检查MongoDB连接是否健康
func (md *MongoDb) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return md.PingTry(ctx)
}

// Rebuild 重建MongoDB客户端连接
func (md *MongoDb) Rebuild(confPath ...interface{}) (interface{}, error) {
	if len(confPath) > 0 {
		return md.ReNewClient(confPath[0].(string))
	}
	return md.ReNewClient()
}

// ReNewClient 重建MongoDB客户端连接
func (md *MongoDb) ReNewClient(confPath ...string) (*MongoDb, error) {
	md.lock.Lock()
	defer md.lock.Unlock()

	client, errNc := NewClient(md.Ctx, confPath...)
	if errNc != nil {
		md.Ctx.GetLogger().Error(md.Ctx.GetConfig().LogOriginMongodb()).Err(errNc).Stack().Msg("Mongo ReNewClient error")
		return md, errNc
	}
	md.Client = client
	return md, nil
}

// PingTry 仅仅用于监控检查
func (md *MongoDb) PingTry(ctx context.Context) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if sr := md.Client.Database("test").RunCommand(ctx, bson.D{{"ping", 1}}); sr.Err() != nil {
		md.Ctx.GetLogger().Error(md.Ctx.GetConfig().LogOriginMongodb()).Err(sr.Err()).Stack().Msg("Mongo PingTry error")
		return false
	}
	return true
}
